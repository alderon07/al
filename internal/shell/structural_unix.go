//go:build !windows

package shell

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	neutralcatalog "github.com/alderon07/al/internal/catalog"
	sharedentry "github.com/alderon07/al/internal/entry"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

type EntryMetadata = sharedentry.EntryMetadata

var shadowCommandName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)
var shadowFunctionName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var shadowMetadataToken = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

const shadowSourceLimit = 8 << 20
const shadowLineLimit = 1 << 20
const shadowMaxResults = 20000
const shadowMaxEntries = 10000
const shadowMaxLines = 100000

type Diagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type Result struct {
	Unit            int                   `json:"unit"`
	Name            string                `json:"name,omitempty"`
	Kind            string                `json:"kind,omitempty"`
	Status          string                `json:"status"`
	StartByte       int                   `json:"start_byte"`
	EndByte         int                   `json:"end_byte"`
	StartLine       int                   `json:"start_line"`
	EndLine         int                   `json:"end_line"`
	DifferentFields []string              `json:"different_fields,omitempty"`
	Diagnostics     []Diagnostic          `json:"diagnostics"`
	Origin          string                `json:"-"`
	Entry           *neutralcatalog.Entry `json:"-"`
	Rendered        []byte                `json:"-"`
}
type SourceLine struct {
	Bytes              []byte
	Start, End, Number int
}

func LineTooLong(source []byte) bool {
	start := 0
	for start < len(source) {
		offset := bytes.IndexByte(source[start:], '\n')
		if offset < 0 {
			return len(source)-start > shadowLineLimit
		}
		if offset > shadowLineLimit {
			return true
		}
		start += offset + 1
	}
	return false
}
func SourceLineCount(source []byte) int {
	if len(source) == 0 {
		return 0
	}
	count := bytes.Count(source, []byte{'\n'})
	if source[len(source)-1] != '\n' {
		count++
	}
	return count
}
func ImportShadowSource(shell string, source []byte) []Result {
	if shell != "bash" && shell != "zsh" || len(source) > shadowSourceLimit || LineTooLong(source) || SourceLineCount(source) > shadowMaxLines || bytes.IndexByte(source, 0) >= 0 || !utf8.Valid(source) {
		return []Result{{Status: "unsupported", EndByte: len(source), Diagnostics: []Diagnostic{{Code: "unsupported_source", Message: "unsupported shell or source bounds"}}}}
	}

	digest := sha256.Sum256(source)
	lines := SourceLines(source)
	results := []Result{}
	pendingComments := []SourceLine{}
	for index := 0; index < len(lines); {
		line := lines[index]
		trimmed := strings.TrimSpace(string(line.Bytes))
		if trimmed == "" {
			pendingComments = nil
			index++
			continue
		}
		if strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "# al-shadow-origin:") {
			pendingComments = append(pendingComments, line)
			index++
			continue
		}
		start := line.Start
		startLine := index + 1
		description := ""
		metadata := EntryMetadata{}
		commentsValid := true
		ownedComments := ownedShadowComments(pendingComments)
		if len(ownedComments) > 0 && ownedComments[len(ownedComments)-1].End == line.Start {
			start = ownedComments[0].Start
			startLine = ownedComments[0].Number
			description, metadata, commentsValid = parseShadowComments(ownedComments)
		}
		pendingComments = nil
		if name, value, ok := parseShadowAlias(trimmed); ok {
			if !commentsValid {
				results = append(results, Result{Unit: len(results), Status: "unsupported", StartByte: start, EndByte: line.End, StartLine: startLine, EndLine: index + 1, Diagnostics: []Diagnostic{{Code: "unsupported_metadata", Message: "metadata is outside the safe shadow grammar"}}})
				index++
				continue
			}
			entry := newShadowEntry(shell, name, "command", value, description, metadata)
			results = append(results, newShadowResult(digest[:], shell, start, line.End, startLine, index+1, &entry))
			index++
			continue
		}
		if name, body, endIndex, ok := parseShadowFunction(source, lines, index); ok {
			if !commentsValid {
				results = append(results, Result{Unit: len(results), Status: "unsupported", StartByte: start, EndByte: lines[endIndex].End, StartLine: startLine, EndLine: endIndex + 1, Diagnostics: []Diagnostic{{Code: "unsupported_metadata", Message: "metadata is outside the safe shadow grammar"}}})
				index = endIndex + 1
				continue
			}
			entry := newShadowEntry(shell, name, "function", body, description, metadata)
			results = append(results, newShadowResult(digest[:], shell, start, lines[endIndex].End, startLine, endIndex+1, &entry))
			index = endIndex + 1
			continue
		}
		if shadowAmbiguousDefinitionStart(trimmed) {
			end := line.End
			endLine := index + 1
			if len(lines) > 0 {
				end = lines[len(lines)-1].End
				endLine = lines[len(lines)-1].Number
			}
			results = append(results, Result{Unit: len(results), Status: "unsupported", StartByte: start, EndByte: end, StartLine: startLine, EndLine: endLine, Diagnostics: []Diagnostic{{Code: "ambiguous_definition", Message: "an incomplete definition consumes the rest of the file"}}})
			break
		}
		endIndex, ambiguous := shadowUnsupportedRange(lines, index)
		code := "unsupported_syntax"
		message := "definition is outside the safe shadow grammar"
		if ambiguous {
			code = "ambiguous_syntax"
			message = "an incomplete shell construct consumes the rest of the file"
		}
		results = append(results, Result{Unit: len(results), Status: "unsupported", StartByte: start, EndByte: lines[endIndex].End, StartLine: startLine, EndLine: endIndex + 1, Diagnostics: []Diagnostic{{Code: code, Message: message}}})
		index = endIndex + 1
		if ambiguous {
			break
		}
	}
	return results
}
func shadowUnsupportedRange(lines []SourceLine, start int) (int, bool) {
	line := lines[start].Bytes
	operator := findShadowHeredocOperator(line)
	if operator >= 0 && (operator+2 >= len(line) || line[operator+2] != '<') {
		spec, _, ok := parseShadowHeredoc(line, operator+2)
		if !ok {
			return len(lines) - 1, true
		}
		for index := start + 1; index < len(lines); index++ {
			candidate := bytes.TrimSuffix(lines[index].Bytes, []byte{'\n'})
			candidate = bytes.TrimSuffix(candidate, []byte{'\r'})
			if spec.stripTabs {
				candidate = bytes.TrimLeft(candidate, "\t")
			}
			if bytes.Equal(candidate, spec.delimiter) {
				return index, false
			}
		}
		return len(lines) - 1, true
	}
	if shadowLineIsAmbiguous(string(line)) {
		return len(lines) - 1, true
	}
	return start, false
}
func findShadowHeredocOperator(line []byte) int {
	quote := byte(0)
	escaped := false
	arithmeticDepth := 0
	for index := 0; index < len(line); index++ {
		character := line[index]
		if escaped {
			escaped = false
			continue
		}
		if quote != 0 {
			if character == '\\' && quote != '\'' {
				escaped = true
			} else if character == quote {
				quote = 0
			}
			continue
		}
		if arithmeticDepth > 0 {
			if character == '(' {
				arithmeticDepth++
			} else if character == ')' {
				arithmeticDepth--
			}
			continue
		}
		if character == '\'' || character == '"' || character == '`' {
			quote = character
			continue
		}
		if character == '$' && index+2 < len(line) && line[index+1] == '(' && line[index+2] == '(' {
			arithmeticDepth = 2
			index += 2
			continue
		}
		if character == '<' && index+1 < len(line) && line[index+1] == '<' {
			end := index + 2
			for end < len(line) && line[end] == '<' {
				end++
			}
			if end == index+2 {
				return index
			}
			index = end - 1
		}
	}
	return -1
}
func shadowLineIsAmbiguous(line string) bool {
	quote := rune(0)
	escaped := false
	commandDepth := 0
	for index, character := range line {
		if escaped {
			escaped = false
			continue
		}
		if character == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if character == quote {
				quote = 0
			}
			continue
		}
		if character == '\'' || character == '"' || character == '`' {
			quote = character
			continue
		}
		if character == '(' && index > 0 && line[index-1] == '$' {
			commandDepth++
		} else if character == ')' && commandDepth > 0 {
			commandDepth--
		}
	}
	return quote != 0 || commandDepth != 0 || strings.HasSuffix(strings.TrimRight(line, "\r\n"), `\`)
}
func shadowFunctionStart(line string) bool {
	open := strings.IndexByte(line, '{')
	if open < 0 {
		prefix := strings.TrimSpace(line)
		if strings.HasSuffix(prefix, "()") {
			return shadowFunctionName.MatchString(strings.TrimSpace(strings.TrimSuffix(prefix, "()")))
		}
		if strings.HasPrefix(prefix, "function ") {
			return shadowFunctionName.MatchString(strings.TrimSpace(strings.TrimPrefix(prefix, "function ")))
		}
		return false
	}
	prefix := strings.TrimSpace(line[:open])
	if strings.HasSuffix(prefix, "()") {
		return shadowFunctionName.MatchString(strings.TrimSpace(strings.TrimSuffix(prefix, "()")))
	}
	if strings.HasPrefix(prefix, "function ") {
		return shadowFunctionName.MatchString(strings.TrimSpace(strings.TrimPrefix(prefix, "function ")))
	}
	return false
}
func shadowAmbiguousDefinitionStart(line string) bool {
	if shadowFunctionStart(line) {
		return true
	}
	if !strings.HasPrefix(line, "alias") || len(line) == len("alias") || (line[len("alias")] != ' ' && line[len("alias")] != '\t') || !strings.ContainsRune(line, '=') {
		return false
	}
	return shadowLineIsAmbiguous(line)
}
func SourceLines(source []byte) []SourceLine {
	lines := []SourceLine{}
	start := 0
	number := 1
	for start < len(source) {
		offset := bytes.IndexByte(source[start:], '\n')
		end := len(source)
		if offset >= 0 {
			end = start + offset + 1
		}
		lines = append(lines, SourceLine{Bytes: source[start:end], Start: start, End: end, Number: number})
		start = end
		number++
	}
	return lines
}
func ownedShadowComments(lines []SourceLine) []SourceLine {
	start := len(lines)
	for start > 0 {
		trimmed := strings.TrimSpace(string(lines[start-1].Bytes))
		if strings.HasPrefix(trimmed, "# al:") || (strings.HasPrefix(trimmed, "# ") && len(strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))) > 0) {
			start--
			continue
		}
		break
	}
	return lines[start:]
}
func parseShadowComments(lines []SourceLine) (string, EntryMetadata, bool) {
	description := ""
	metadata := EntryMetadata{}
	metadataSeen := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(string(line.Bytes))
		if strings.HasPrefix(trimmed, "# al:") {
			if metadataSeen || !strings.HasPrefix(trimmed, "# al: ") {
				return "", EntryMetadata{}, false
			}
			parsed, ok := parseShadowMetadata(strings.TrimPrefix(trimmed, "# al: "))
			if !ok {
				return "", EntryMetadata{}, false
			}
			metadata = parsed
			metadataSeen = true
			continue
		}
		if metadataSeen {
			return "", EntryMetadata{}, false
		}
		text := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
		if text != "" {
			description = text
		}
	}
	if len(description) > 1024 || !utf8.ValidString(description) || strings.ContainsRune(description, 0) {
		return "", EntryMetadata{}, false
	}
	return description, metadata, true
}
func parseShadowMetadata(value string) (EntryMetadata, bool) {
	fields := strings.Fields(value)
	if len(fields) == 0 || strings.Join(fields, " ") != value {
		return EntryMetadata{}, false
	}
	metadata := EntryMetadata{}
	lastOrder := -1
	seenKeys := map[string]bool{}
	order := map[string]int{"tags": 0, "platforms": 1, "favorite": 2, "category": 3}
	for _, field := range fields {
		key, item, ok := strings.Cut(field, "=")
		position, known := order[key]
		if !ok || !known || seenKeys[key] || position < lastOrder || item == "" {
			return EntryMetadata{}, false
		}
		seenKeys[key] = true
		lastOrder = position
		switch key {
		case "tags":
			values, ok := parseShadowList(item, nil)
			if !ok {
				return EntryMetadata{}, false
			}
			metadata.Tags = values
		case "platforms":
			values, ok := parseShadowList(item, map[string]bool{"linux": true, "macos": true, "wsl": true, "windows": true})
			if !ok {
				return EntryMetadata{}, false
			}
			canonical := []string{"linux", "macos", "wsl", "windows"}
			filtered := []string{}
			for _, candidate := range canonical {
				for _, actual := range values {
					if candidate == actual {
						filtered = append(filtered, actual)
					}
				}
			}
			if strings.Join(values, ",") != strings.Join(filtered, ",") {
				return EntryMetadata{}, false
			}
			metadata.Platforms = values
		case "favorite":
			if item != "true" {
				return EntryMetadata{}, false
			}
			metadata.Favorite = true
		case "category":
			if !shadowMetadataToken.MatchString(item) {
				return EntryMetadata{}, false
			}
			metadata.Category = item
		}
	}
	return metadata, true
}
func parseShadowList(value string, allowed map[string]bool) ([]string, bool) {
	parts := strings.Split(value, ",")
	seen := map[string]bool{}
	for _, part := range parts {
		if !shadowMetadataToken.MatchString(part) || seen[part] || (allowed != nil && !allowed[part]) {
			return nil, false
		}
		seen[part] = true
	}
	return parts, true
}
func parseShadowAlias(line string) (string, string, bool) {
	name, value, reason := parseShadowAliasReason(line)
	return name, value, reason == ""
}
func parseShadowAliasReason(line string) (string, string, string) {
	if !strings.HasPrefix(line, "alias") || len(line) == len("alias") || (line[len("alias")] != ' ' && line[len("alias")] != '\t') {
		return "", "", "alias_definition"
	}
	rest := strings.TrimLeft(line[len("alias"):], " \t")
	equal := strings.IndexByte(rest, '=')
	if equal <= 0 {
		return "", "", "alias_assignment"
	}
	name := rest[:equal]
	if !shadowCommandName.MatchString(name) {
		return "", "", "alias_name"
	}
	quoted := rest[equal+1:]
	value, tail, ok := decodeShadowSingleQuote(quoted)
	if !ok {
		return "", "", "alias_quoting"
	}
	if tail != "" {
		if tail[0] != ' ' && tail[0] != '\t' {
			return "", "", "alias_trailing"
		}
		tail = strings.TrimLeft(tail, " \t")
	}
	if tail != "" && !strings.HasPrefix(tail, "#") {
		return "", "", "alias_trailing"
	}
	return name, value, ""
}
func decodeShadowSingleQuote(input string) (string, string, bool) {
	if !strings.HasPrefix(input, "'") {
		return "", "", false
	}
	var output strings.Builder
	position := 1
	for position < len(input) {
		next := strings.IndexByte(input[position:], '\'')
		if next < 0 {
			return "", "", false
		}
		next += position
		output.WriteString(input[position:next])
		position = next + 1
		if strings.HasPrefix(input[position:], `\''`) {
			output.WriteByte('\'')
			position += 3
			continue
		}
		return output.String(), input[position:], true
	}
	return "", "", false
}
func parseShadowFunction(source []byte, lines []SourceLine, start int) (string, string, int, bool) {
	text := string(lines[start].Bytes)
	open := strings.IndexByte(text, '{')
	if open < 0 {
		return "", "", start, false
	}
	prefix := strings.TrimSpace(text[:open])
	name := ""
	if strings.HasSuffix(prefix, "()") {
		name = strings.TrimSpace(strings.TrimSuffix(prefix, "()"))
	} else if strings.HasPrefix(prefix, "function ") {
		name = strings.TrimSpace(strings.TrimPrefix(prefix, "function "))
	}
	if !shadowFunctionName.MatchString(name) {
		return "", "", start, false
	}
	startByte := lines[start].Start
	contents := source[startByte:]
	open = bytes.IndexByte(contents, '{')
	close, ok := matchingOuterBrace(contents, open)
	if !ok {
		return "", "", start, false
	}
	absoluteClose := startByte + close
	lineOffset := sort.Search(len(lines)-start, func(offset int) bool {
		return lines[start+offset].End > absoluteClose
	})
	if lineOffset >= len(lines)-start {
		return "", "", start, false
	}
	endIndex := start + lineOffset
	lineEnd := lines[endIndex].End - startByte
	rawTail := string(contents[close+1 : lineEnd])
	tail := strings.TrimSpace(rawTail)
	if tail != "" && ((rawTail[0] != ' ' && rawTail[0] != '\t') || !strings.HasPrefix(tail, "#")) {
		return "", "", start, false
	}
	return name, string(contents[open+1 : close]), endIndex, true
}
func matchingOuterBrace(contents []byte, open int) (int, bool) {
	close, reason := matchingOuterBraceReason(contents, open)
	return close, reason == ""
}
func matchingOuterBraceReason(contents []byte, open int) (int, string) {
	type heredocSpec struct {
		delimiter []byte
		stripTabs bool
		quoted    bool
	}
	heredocs := []heredocSpec{}
	heredocActive := false
	quote := byte(0)
	wordStart := true
	commandStart := true
	for index := open + 1; index < len(contents); index++ {
		if heredocActive {
			lineEnd := bytes.IndexByte(contents[index:], '\n')
			if lineEnd < 0 {
				return index, "function_heredoc_boundary"
			}
			lineEnd += index
			line := contents[index:lineEnd]
			if !heredocs[0].quoted && bytes.HasSuffix(line, []byte{'\\'}) {
				return index, "function_heredoc_continuation"
			}
			if heredocs[0].stripTabs {
				line = bytes.TrimLeft(line, "\t")
			}
			if bytes.Equal(line, heredocs[0].delimiter) {
				heredocs = heredocs[1:]
				heredocActive = len(heredocs) > 0
			}
			index = lineEnd
			wordStart, commandStart = true, true
			continue
		}
		character := contents[index]
		if character == '\n' && quote != 0 && len(heredocs) > 0 {
			return index, "function_heredoc_quote"
		}
		if quote == '\'' {
			if character == quote {
				quote = 0
			}
			continue
		}
		if character == '\\' {
			if index+1 >= len(contents) {
				return index, "function_escape_boundary"
			}
			next := contents[index+1]
			if next == '\n' {
				return index, "function_continuation"
			}
			if quote == '"' && !bytes.ContainsRune([]byte("$`\"\\\n"), rune(next)) {
				continue
			}
			index++
			wordStart, commandStart = false, false
			continue
		}
		if character == '`' {
			return index, "function_backticks"
		}
		if character == '$' && index+1 < len(contents) {
			next := contents[index+1]
			if next == '{' || next == '[' || next == '\'' || next == '"' {
				reason := map[byte]string{'{': "function_parameter_expansion", '[': "function_legacy_arithmetic", '\'': "function_dollar_single_quote", '"': "function_dollar_double_quote"}[next]
				return index, reason
			}
			if next == '(' {
				end, ok := shadowNumericArithmetic(contents, index)
				if !ok {
					if index+2 < len(contents) && contents[index+2] == '(' {
						return index, "function_arithmetic_expression"
					}
					return index, "function_command_substitution"
				}
				index = end
				wordStart, commandStart = false, false
				continue
			}
		}
		if quote == '"' {
			if character == quote {
				quote = 0
			}
			continue
		}
		if character == '\'' || character == '"' {
			quote = character
			wordStart, commandStart = false, false
			continue
		}
		if character == '#' && wordStart {
			for index < len(contents) && contents[index] != '\n' {
				index++
			}
			if len(heredocs) > 0 {
				index--
			} else {
				wordStart, commandStart = true, true
			}
			continue
		}
		if character == '<' && index+1 < len(contents) && contents[index+1] == '<' {
			if index+2 < len(contents) && contents[index+2] == '<' {
				return index, "function_here_string"
			}
			spec, next, ok := parseShadowHeredoc(contents, index+2)
			if !ok {
				return index, "function_heredoc_delimiter"
			}
			heredocs = append(heredocs, spec)
			index = next - 1
			wordStart = true
			continue
		}
		if character == '{' || character == '(' || character == ')' {
			return index, "function_grouping"
		}
		if character == '}' {
			if !commandStart || !wordStart || len(heredocs) > 0 || (index+1 < len(contents) && !bytes.ContainsRune([]byte(" \t\n;"), rune(contents[index+1]))) {
				return index, "function_closing_brace"
			}
			return index, ""
		}
		switch character {
		case ' ', '\t':
			wordStart = true
		case '\n':
			heredocActive = len(heredocs) > 0
			wordStart, commandStart = true, true
		case ';', '|', '&':
			wordStart, commandStart = true, true
		case '<', '>':
			wordStart = true
		case '\r':
			return index, "function_carriage_return"
		default:
			wordStart, commandStart = false, false
		}
	}
	return len(contents), "function_boundary"
}

func shadowNumericArithmetic(contents []byte, start int) (int, bool) {
	if start+2 >= len(contents) || contents[start+2] != '(' {
		return 0, false
	}
	depth := 0
	for index := start + 3; index < len(contents); index++ {
		character := contents[index]
		if character == '(' {
			depth++
			continue
		}
		if character == ')' {
			if depth > 0 {
				depth--
				continue
			}
			return index + 1, index+1 < len(contents) && contents[index+1] == ')'
		}
		if character < '0' || character > '9' {
			if !bytes.ContainsRune([]byte(" \t\n+-*/%<>&|^~!?:="), rune(character)) {
				return 0, false
			}
		}
	}
	return 0, false
}

func parseShadowHeredoc(contents []byte, position int) (struct {
	delimiter []byte
	stripTabs bool
	quoted    bool
}, int, bool) {
	result := struct {
		delimiter []byte
		stripTabs bool
		quoted    bool
	}{}
	if position < len(contents) && contents[position] == '-' {
		result.stripTabs = true
		position++
	}
	for position < len(contents) && (contents[position] == ' ' || contents[position] == '\t') {
		position++
	}
	quote := byte(0)
	for position < len(contents) {
		character := contents[position]
		if character == '\n' && quote != 0 {
			return result, position, false
		}
		if quote == '\'' {
			if character == quote {
				quote = 0
			} else {
				result.delimiter = append(result.delimiter, character)
			}
			position++
			continue
		}
		if character == '\\' {
			result.quoted = true
			if position+1 >= len(contents) || contents[position+1] == '\n' {
				return result, position, false
			}
			next := contents[position+1]
			if quote == '"' && !bytes.ContainsRune([]byte("$`\"\\"), rune(next)) {
				result.delimiter = append(result.delimiter, character)
				position++
			} else {
				result.delimiter = append(result.delimiter, next)
				position += 2
			}
			continue
		}
		if character == '$' || character == '`' || character == '\r' {
			return result, position, false
		}
		if quote == '"' {
			if character == quote {
				quote = 0
			} else {
				result.delimiter = append(result.delimiter, character)
			}
			position++
			continue
		}
		if character == '\'' || character == '"' {
			result.quoted = true
			quote = character
			position++
			continue
		}
		if bytes.ContainsRune([]byte(" \t\n;|&()<>"), rune(character)) {
			break
		}
		result.delimiter = append(result.delimiter, character)
		position++
	}
	return result, position, quote == 0 && len(result.delimiter) > 0
}

func newShadowEntry(shell, name, kind, value, description string, metadata EntryMetadata) neutralcatalog.Entry {
	entry := neutralcatalog.Entry{Name: name, Kind: kind, Description: description, Category: metadata.Category, Tags: metadata.Tags, Platforms: metadata.Platforms, Favorite: metadata.Favorite, Native: map[string]neutralcatalog.NativeImplementation{}}
	implementation := neutralcatalog.NativeImplementation{}
	if kind == "command" {
		implementation.AliasValue = &value
	} else {
		implementation.FunctionBody = &value
	}
	entry.Native[shell] = implementation
	return entry
}
func newShadowResult(digest []byte, shell string, start, end, startLine, endLine int, entry *neutralcatalog.Entry) Result {
	origin := shadowOriginFromDigest(shell, digest, start, end)
	entry.ID = origin[:32]
	return Result{Unit: 0, Name: entry.Name, Kind: entry.Kind, Status: "equivalent", StartByte: start, EndByte: end, StartLine: startLine, EndLine: endLine, Diagnostics: []Diagnostic{}, Origin: origin, Entry: entry}
}
func shadowOrigin(shell string, source []byte, start, end int) string {
	digest := sha256.Sum256(source)
	return shadowOriginFromDigest(shell, digest[:], start, end)
}
func shadowOriginFromDigest(shell string, digest []byte, start, end int) string {
	buffer := bytes.Buffer{}
	frame := func(value []byte) {
		_ = binary.Write(&buffer, binary.BigEndian, uint64(len(value)))
		buffer.Write(value)
	}
	frame([]byte(shell))
	frame(digest)
	offset := make([]byte, 8)
	binary.BigEndian.PutUint64(offset, uint64(start))
	frame(offset)
	binary.BigEndian.PutUint64(offset, uint64(end))
	frame(offset)
	sum := sha256.Sum256(buffer.Bytes())
	return hex.EncodeToString(sum[:])
}
func shadowGenerationHash(renderer, platform string, approvalHash, body []byte) string {
	buffer := bytes.Buffer{}
	for _, value := range [][]byte{[]byte(renderer), []byte(platform), approvalHash, body} {
		_ = binary.Write(&buffer, binary.BigEndian, uint64(len(value)))
		buffer.Write(value)
	}
	sum := sha256.Sum256(buffer.Bytes())
	return hex.EncodeToString(sum[:])
}
func MarkShadowDuplicates(results []Result) {
	groups := map[string][]int{}
	for index, result := range results {
		if result.Entry != nil {
			groups[result.Name] = append(groups[result.Name], index)
		}
	}
	for _, indices := range groups {
		if len(indices) < 2 {
			continue
		}
		for _, index := range indices {
			results[index].Status = "duplicate"
			results[index].Entry = nil
			results[index].Diagnostics = append(results[index].Diagnostics, Diagnostic{Code: "duplicate_name", Message: "every definition with this name is excluded"})
		}
	}
}
func ValidateShadowCandidates(results []Result) {
	for index := range results {
		if results[index].Entry == nil {
			continue
		}
		candidate := neutralcatalog.Catalog{SchemaVersion: neutralcatalog.SchemaVersion, Entries: []neutralcatalog.Entry{*results[index].Entry}}
		if diagnostics := neutralcatalog.Validate(candidate); len(diagnostics) > 0 {
			results[index].Status = "invalid"
			results[index].Entry = nil
			results[index].Diagnostics = append(results[index].Diagnostics, Diagnostic{Code: "invalid_candidate", Message: "entry does not satisfy the catalog schema"})
			continue
		}
		results[index].Rendered = RenderShadowEntry(*results[index].Entry, results[index].Origin)
	}
}
func RenderShadowEntry(entry neutralcatalog.Entry, origin string) []byte {
	var output strings.Builder
	output.WriteString("# al-shadow-origin: ")
	output.WriteString(origin)
	output.WriteByte('\n')
	if entry.Description != "" {
		output.WriteString("# ")
		output.WriteString(entry.Description)
		output.WriteByte('\n')
	}
	metadata := EntryMetadata{Tags: entry.Tags, Platforms: entry.Platforms, Favorite: entry.Favorite, Category: entry.Category}
	if line := sharedentry.MetadataLine(metadata); line != "" {
		output.WriteString(line)
		output.WriteByte('\n')
	}
	implementation := entry.Native
	shells := make([]string, 0, len(implementation))
	for shell := range implementation {
		shells = append(shells, shell)
	}
	sort.Strings(shells)
	for _, shell := range shells {
		native := implementation[shell]
		if entry.Kind == "command" {
			output.WriteString("alias ")
			output.WriteString(entry.Name)
			output.WriteByte('=')
			output.WriteString(QuoteShadow(*native.AliasValue))
			output.WriteByte('\n')
		} else {
			output.WriteString(entry.Name)
			output.WriteString("() {")
			output.WriteString(*native.FunctionBody)
			output.WriteString("}\n")
		}
		break
	}
	return []byte(output.String())
}
func QuoteShadow(value string) string { return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'" }
func CompareShadowRoundTrip(shell string, result *Result) {
	newline := bytes.IndexByte(result.Rendered, '\n')
	if newline < 0 || string(result.Rendered[:newline]) != "# al-shadow-origin: "+result.Origin {
		result.Status = "invalid"
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: "invalid_origin_marker", Message: "rendered definition lost its internal identity marker"})
		return
	}
	reparsed := ImportShadowSource(shell, result.Rendered[newline+1:])
	if len(reparsed) != 1 || reparsed[0].Entry == nil {
		result.Status = "invalid"
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: "reparse_failed", Message: "rendered definition could not be parsed again"})
		return
	}
	reparsed[0].Entry.ID = result.Entry.ID
	changes, diagnostics := neutralcatalog.Compare(
		neutralcatalog.Catalog{SchemaVersion: neutralcatalog.SchemaVersion, Entries: []neutralcatalog.Entry{*result.Entry}},
		neutralcatalog.Catalog{SchemaVersion: neutralcatalog.SchemaVersion, Entries: []neutralcatalog.Entry{*reparsed[0].Entry}},
	)
	if len(diagnostics) > 0 {
		result.Status = "invalid"
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: "comparison_failed", Message: "round-trip comparison could not complete"})
		return
	}
	if len(changes) > 0 {
		result.Status = "different"
		result.DifferentFields = append([]string(nil), changes[0].Fields...)
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: "round_trip_changed", Message: "supported fields changed during the round trip"})
	}
}
func ValidateSyntax(overall context.Context, shell string, contents []byte, resolver func(string) (string, error)) error {
	if shell != "bash" && shell != "zsh" {
		return fmt.Errorf("unsupported shell %q (use bash or zsh)", shell)
	}
	if resolver == nil {
		resolver = TrustedShadowShell
	}
	if err := validateSource(contents); err != nil {
		return err
	}
	path, err := resolver(shell)
	if err != nil {
		return err
	}
	home, err := os.MkdirTemp("", "alias-lens-shadow-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(home)
	ctx, cancel := context.WithTimeout(overall, 5*time.Second)
	defer cancel()
	arguments := []string{"--noprofile", "--norc", "-n"}
	if shell == "zsh" {
		arguments = []string{"-f", "-n"}
	}
	command := exec.Command(path, arguments...)
	command.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + home, "ZDOTDIR=" + home, "BASH_ENV=/dev/null", "ENV=/dev/null", "LC_ALL=C"}
	command.Stdin = bytes.NewReader(contents)
	var stdout, stderr limitedBuffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		return err
	}
	waited := make(chan error, 1)
	go func() { waited <- command.Wait() }()
	select {
	case err = <-waited:
	case <-ctx.Done():
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		<-waited
		return ctx.Err()
	}
	if stdout.total > 0 {
		return fmt.Errorf("validator produced standard output")
	}
	if stdout.total > 65536 || stderr.total > 65536 {
		return fmt.Errorf("validator output exceeded 64 KiB")
	}
	return err
}
func (value *limitedBuffer) Write(contents []byte) (int, error) {
	original := len(contents)
	value.total += original
	remaining := 65537 - value.buffer.Len()
	if remaining > 0 {
		if len(contents) > remaining {
			contents = contents[:remaining]
		}
		_, _ = value.buffer.Write(contents)
	}
	return original, nil
}
func TrustedShadowShell(shell string) (string, error) {
	if shell != "bash" && shell != "zsh" {
		return "", fmt.Errorf("unsupported shell %q (use bash or zsh)", shell)
	}
	names := []string{"/bin/" + shell, "/usr/bin/" + shell}
	for _, name := range names {
		resolved, err := filepath.EvalSymlinks(name)
		if err != nil {
			continue
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 || !shadowRootOwned(info) {
			continue
		}
		trusted := true
		for directory := filepath.Dir(resolved); ; directory = filepath.Dir(directory) {
			info, err = os.Stat(directory)
			if err != nil || !info.IsDir() || !shadowRootOwned(info) {
				trusted = false
				break
			}
			if directory == string(filepath.Separator) {
				break
			}
		}
		if trusted {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("trusted %s validator is not installed", shell)
}
func shadowRootOwned(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == 0 && info.Mode().Perm()&0o022 == 0
}

type limitedBuffer struct {
	buffer bytes.Buffer
	total  int
}

func validateSource(source []byte) error {
	if len(source) > shadowSourceLimit || LineTooLong(source) || SourceLineCount(source) > shadowMaxLines || bytes.IndexByte(source, 0) >= 0 || !utf8.Valid(source) {
		return fmt.Errorf("shell source has unsupported encoding or bounds")
	}
	return nil
}
