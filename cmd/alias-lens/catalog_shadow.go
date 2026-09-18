//go:build !windows

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	neutralcatalog "alias-lens/internal/catalog"
)

var shadowCommandName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)
var shadowFunctionName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var shadowMetadataToken = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
var shadowValidatorPath = trustedShadowShell
var shadowSyntaxValidator = validateShadowSyntax
var shadowStdout io.Writer = os.Stdout

var errShadowDuplicateConfigField = errors.New("duplicate configuration field")

const (
	shadowConfigLimit = 1 << 20
	shadowSourceLimit = 8 << 20
	shadowLineLimit   = 1 << 20
	shadowReportLimit = 16 << 20
	shadowMaxResults  = 20000
	shadowMaxEntries  = 10000
	shadowMaxLaunches = 256
)

type shadowDiagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type shadowResult struct {
	Unit            int                `json:"unit"`
	Name            string             `json:"name,omitempty"`
	Kind            string             `json:"kind,omitempty"`
	Status          string             `json:"status"`
	StartByte       int                `json:"start_byte"`
	EndByte         int                `json:"end_byte"`
	StartLine       int                `json:"start_line"`
	EndLine         int                `json:"end_line"`
	DifferentFields []string           `json:"different_fields,omitempty"`
	Diagnostics     []shadowDiagnostic `json:"diagnostics"`
	origin          string
	entry           *neutralcatalog.Entry
	rendered        []byte
}
type shadowSummary struct {
	Equivalent  int `json:"equivalent"`
	Unsupported int `json:"unsupported"`
	Different   int `json:"different"`
	Duplicate   int `json:"duplicate"`
	Invalid     int `json:"invalid"`
	Blocked     int `json:"blocked"`
}
type shadowReport struct {
	SchemaVersion int                `json:"schema_version"`
	Shell         string             `json:"shell"`
	Summary       shadowSummary      `json:"summary"`
	Diagnostics   []shadowDiagnostic `json:"diagnostics"`
	Results       []shadowResult     `json:"results"`
}

func runCatalogCommand(arguments []string) (exitCode int, commandErr error) {
	defer func() {
		if recover() != nil {
			exitCode = 2
			commandErr = fmt.Errorf("shadow inspection stopped unexpectedly")
		}
	}()
	if len(arguments) == 0 || arguments[0] != "shadow" {
		return 2, fmt.Errorf("usage: al catalog shadow [--shell bash|zsh] [--json]")
	}
	jsonOutput := false
	shellName := ""
	for index := 1; index < len(arguments); index++ {
		switch arguments[index] {
		case "--json":
			jsonOutput = true
		case "--shell":
			if index+1 >= len(arguments) {
				return 2, fmt.Errorf("--shell requires bash or zsh")
			}
			index++
			shellName = arguments[index]
		default:
			return 2, fmt.Errorf("unknown catalog shadow option %q", arguments[index])
		}
	}
	report, err := inspectCatalogShadow(shellName)
	if err != nil {
		return 2, err
	}
	output, err := renderShadowReport(report, jsonOutput)
	if err != nil {
		return 2, err
	}
	if len(output) > shadowReportLimit {
		return 2, fmt.Errorf("shadow report exceeds 16 MiB")
	}
	written, err := shadowStdout.Write(output)
	if err != nil {
		return 2, fmt.Errorf("write shadow report: %w", err)
	}
	if written != len(output) {
		return 2, fmt.Errorf("write shadow report: %w", io.ErrShortWrite)
	}
	for _, result := range report.Results {
		if result.Status != "equivalent" {
			return 1, nil
		}
	}
	return 0, nil
}

func inspectCatalogShadow(explicitShell string) (shadowReport, error) {
	adapter, err := shadowShellAdapter(explicitShell)
	if err != nil {
		return shadowReport{}, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return shadowReport{}, fmt.Errorf("find home directory: %w", err)
	}
	path := filepath.Join(home, adapter.AliasFilename())
	source, err := readShadowRegular(path, shadowSourceLimit)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return shadowReport{}, fmt.Errorf("selected alias file is missing; run al setup %s", adapter.Name())
		}
		return shadowReport{}, fmt.Errorf("selected alias file is unreadable; check its type, size, and permissions")
	}
	if bytes.IndexByte(source, 0) >= 0 || !utf8.Valid(source) {
		return shadowReport{}, fmt.Errorf("alias file is not valid UTF-8 text")
	}
	if lineTooLong(source) {
		return shadowReport{}, fmt.Errorf("alias file contains a line over 1 MiB")
	}
	findings := findSecretFindings(source)
	results := importShadowSource(adapter.Name(), source)
	definitions := 0
	for _, result := range results {
		if result.entry != nil {
			definitions++
		}
	}
	if definitions > shadowMaxEntries {
		return shadowReport{}, fmt.Errorf("alias file contains more than 10000 proven definitions")
	}
	if len(results) > shadowMaxResults {
		return shadowReport{}, fmt.Errorf("alias file produces more than 20000 inspection ranges")
	}
	results = blockShadowSecrets(results, findings, source)
	if len(results) > shadowMaxResults {
		return shadowReport{}, fmt.Errorf("alias file produces more than 20000 inspection ranges")
	}
	assignShadowUnits(results)
	markShadowDuplicates(results)
	validateShadowCandidates(results)
	validateShadowRendered(adapter.Name(), results)
	report := shadowReport{SchemaVersion: 1, Shell: adapter.Name(), Diagnostics: []shadowDiagnostic{}, Results: results}
	finalizeShadowReport(&report)
	return report, nil
}

func assignShadowUnits(results []shadowResult) {
	indices := make([]int, len(results))
	for index := range results {
		indices[index] = index
	}
	sort.SliceStable(indices, func(i, j int) bool {
		left, right := results[indices[i]], results[indices[j]]
		if left.StartByte != right.StartByte {
			return left.StartByte < right.StartByte
		}
		return left.EndByte < right.EndByte
	})
	for unit, index := range indices {
		results[index].Unit = unit
	}
}

func shadowShellAdapter(explicit string) (ShellAdapter, error) {
	if explicit != "" {
		return shellAdapter(explicit)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".config", "alias-lens", "config.json")
	contents, err := readShadowRegular(path, shadowConfigLimit)
	if errors.Is(err, os.ErrNotExist) {
		return bashShellAdapter{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("configuration unreadable; check config.json type, size, and permissions")
	}
	var raw map[string]json.RawMessage
	if err := decodeUniqueJSON(contents, &raw); err != nil {
		if errors.Is(err, errShadowDuplicateConfigField) {
			return nil, errShadowDuplicateConfigField
		}
		return nil, fmt.Errorf("invalid configuration")
	}
	version := 0
	if value, ok := raw["version"]; ok {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, fmt.Errorf("invalid configuration")
		}
		if err := json.Unmarshal(value, &version); err != nil {
			return nil, fmt.Errorf("invalid configuration")
		}
		if version < 0 {
			return nil, fmt.Errorf("invalid configuration version")
		}
	}
	if version > currentConfigVersion {
		return nil, fmt.Errorf("configuration version unsupported")
	}
	shellName := "bash"
	if value, ok := raw["shell"]; ok {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, fmt.Errorf("invalid configuration")
		}
		if err := json.Unmarshal(value, &shellName); err != nil {
			return nil, fmt.Errorf("invalid configuration")
		}
		if strings.TrimSpace(shellName) == "" {
			shellName = "bash"
		}
	}
	adapter, err := shellAdapter(shellName)
	if err != nil {
		return nil, fmt.Errorf("unsupported configured shell %q", shellName)
	}
	return adapter, nil
}

func decodeUniqueJSON(contents []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 32 {
			return fmt.Errorf("JSON is too deeply nested")
		}
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delimiter {
		case '{':
			seen := map[string]bool{}
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok {
					return fmt.Errorf("invalid object field")
				}
				if seen[key] {
					return errShadowDuplicateConfigField
				}
				seen[key] = true
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		case '[':
			for decoder.More() {
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		}
		return nil
	}
	if err := walk(0); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return fmt.Errorf("trailing JSON data")
	}
	strict := json.NewDecoder(bytes.NewReader(contents))
	return strict.Decode(target)
}

func readShadowRegular(path string, limit int64) ([]byte, error) {
	descriptor, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(descriptor), path)
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file")
	}
	reader := io.LimitReader(file, limit+1)
	contents, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if int64(len(contents)) > limit {
		return nil, fmt.Errorf("file exceeds %d bytes", limit)
	}
	return contents, nil
}
func lineTooLong(source []byte) bool {
	for _, line := range bytes.Split(source, []byte{'\n'}) {
		if len(line) > shadowLineLimit {
			return true
		}
	}
	return false
}

func importShadowSource(shell string, source []byte) []shadowResult {
	lines := sourceLines(source)
	results := []shadowResult{}
	pendingComments := []sourceLine{}
	for index := 0; index < len(lines); {
		line := lines[index]
		trimmed := strings.TrimSpace(string(line.bytes))
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
		start := line.start
		startLine := index + 1
		description := ""
		metadata := EntryMetadata{}
		commentsValid := true
		ownedComments := ownedShadowComments(pendingComments)
		if len(ownedComments) > 0 && ownedComments[len(ownedComments)-1].end == line.start {
			start = ownedComments[0].start
			startLine = ownedComments[0].number
			description, metadata, commentsValid = parseShadowComments(ownedComments)
		}
		pendingComments = nil
		if name, value, ok := parseShadowAlias(trimmed); ok {
			if !commentsValid {
				results = append(results, shadowResult{Unit: len(results), Status: "unsupported", StartByte: start, EndByte: line.end, StartLine: startLine, EndLine: index + 1, Diagnostics: []shadowDiagnostic{{Code: "unsupported_metadata", Message: "metadata is outside the safe shadow grammar"}}})
				index++
				continue
			}
			entry := newShadowEntry(shell, name, "command", value, description, metadata)
			results = append(results, newShadowResult(source, shell, start, line.end, startLine, index+1, &entry))
			index++
			continue
		}
		if name, body, endIndex, ok := parseShadowFunction(lines, index); ok {
			if !commentsValid {
				results = append(results, shadowResult{Unit: len(results), Status: "unsupported", StartByte: start, EndByte: lines[endIndex].end, StartLine: startLine, EndLine: endIndex + 1, Diagnostics: []shadowDiagnostic{{Code: "unsupported_metadata", Message: "metadata is outside the safe shadow grammar"}}})
				index = endIndex + 1
				continue
			}
			entry := newShadowEntry(shell, name, "function", body, description, metadata)
			results = append(results, newShadowResult(source, shell, start, lines[endIndex].end, startLine, endIndex+1, &entry))
			index = endIndex + 1
			continue
		}
		if shadowAmbiguousDefinitionStart(trimmed) {
			end := line.end
			endLine := index + 1
			if len(lines) > 0 {
				end = lines[len(lines)-1].end
				endLine = lines[len(lines)-1].number
			}
			results = append(results, shadowResult{Unit: len(results), Status: "unsupported", StartByte: start, EndByte: end, StartLine: startLine, EndLine: endLine, Diagnostics: []shadowDiagnostic{{Code: "ambiguous_definition", Message: "an incomplete definition consumes the rest of the file"}}})
			break
		}
		endIndex, ambiguous := shadowUnsupportedRange(lines, index)
		code := "unsupported_syntax"
		message := "definition is outside the safe shadow grammar"
		if ambiguous {
			code = "ambiguous_syntax"
			message = "an incomplete shell construct consumes the rest of the file"
		}
		results = append(results, shadowResult{Unit: len(results), Status: "unsupported", StartByte: start, EndByte: lines[endIndex].end, StartLine: startLine, EndLine: endIndex + 1, Diagnostics: []shadowDiagnostic{{Code: code, Message: message}}})
		index = endIndex + 1
		if ambiguous {
			break
		}
	}
	return results
}

func shadowUnsupportedRange(lines []sourceLine, start int) (int, bool) {
	line := lines[start].bytes
	operator := findShadowHeredocOperator(line)
	if operator >= 0 && (operator+2 >= len(line) || line[operator+2] != '<') {
		spec, _, ok := parseShadowHeredoc(line, operator+2)
		if !ok {
			return len(lines) - 1, true
		}
		for index := start + 1; index < len(lines); index++ {
			candidate := bytes.TrimSuffix(lines[index].bytes, []byte{'\n'})
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
		if character == '<' && index+1 < len(line) && line[index+1] == '<' && (index+2 >= len(line) || line[index+2] != '<') {
			return index
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

type sourceLine struct {
	bytes              []byte
	start, end, number int
}

func sourceLines(source []byte) []sourceLine {
	lines := []sourceLine{}
	start := 0
	number := 1
	for start < len(source) {
		offset := bytes.IndexByte(source[start:], '\n')
		end := len(source)
		if offset >= 0 {
			end = start + offset + 1
		}
		lines = append(lines, sourceLine{bytes: source[start:end], start: start, end: end, number: number})
		start = end
		number++
	}
	return lines
}

func ownedShadowComments(lines []sourceLine) []sourceLine {
	start := len(lines)
	for start > 0 {
		trimmed := strings.TrimSpace(string(lines[start-1].bytes))
		if strings.HasPrefix(trimmed, "# al:") || (strings.HasPrefix(trimmed, "# ") && len(strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))) > 0) {
			start--
			continue
		}
		break
	}
	return lines[start:]
}

func parseShadowComments(lines []sourceLine) (string, EntryMetadata, bool) {
	description := ""
	metadata := EntryMetadata{}
	metadataSeen := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(string(line.bytes))
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
	if !strings.HasPrefix(line, "alias") || len(line) == len("alias") || (line[len("alias")] != ' ' && line[len("alias")] != '\t') {
		return "", "", false
	}
	rest := strings.TrimLeft(line[len("alias"):], " \t")
	equal := strings.IndexByte(rest, '=')
	if equal <= 0 {
		return "", "", false
	}
	name := rest[:equal]
	if !shadowCommandName.MatchString(name) {
		return "", "", false
	}
	quoted := rest[equal+1:]
	value, tail, ok := decodeShadowSingleQuote(quoted)
	if !ok {
		return "", "", false
	}
	if tail != "" {
		if tail[0] != ' ' && tail[0] != '\t' {
			return "", "", false
		}
		tail = strings.TrimLeft(tail, " \t")
	}
	if tail != "" && !strings.HasPrefix(tail, "#") {
		return "", "", false
	}
	return name, value, true
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

func parseShadowFunction(lines []sourceLine, start int) (string, string, int, bool) {
	text := string(lines[start].bytes)
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
	combined := []byte{}
	for index := start; index < len(lines); index++ {
		combined = append(combined, lines[index].bytes...)
		if close, ok := matchingOuterBrace(combined, bytes.IndexByte(combined, '{')); ok {
			rawTail := string(combined[close+1:])
			tail := strings.TrimSpace(rawTail)
			if tail != "" && ((rawTail[0] != ' ' && rawTail[0] != '\t') || !strings.HasPrefix(tail, "#")) {
				return "", "", start, false
			}
			return name, string(combined[bytes.IndexByte(combined, '{')+1 : close]), index, true
		}
	}
	return "", "", start, false
}
func matchingOuterBrace(contents []byte, open int) (int, bool) {
	type heredocSpec struct {
		delimiter []byte
		stripTabs bool
	}
	heredocs := []heredocSpec{}
	depth := 0
	arithmeticDepth := 0
	quote := byte(0)
	escaped := false
	for index := open; index < len(contents); index++ {
		if len(heredocs) > 0 && (index == 0 || contents[index-1] == '\n') {
			lineEnd := bytes.IndexByte(contents[index:], '\n')
			if lineEnd < 0 {
				lineEnd = len(contents)
			} else {
				lineEnd += index
			}
			line := bytes.TrimSuffix(contents[index:lineEnd], []byte{'\r'})
			if heredocs[0].stripTabs {
				line = bytes.TrimLeft(line, "\t")
			}
			if bytes.Equal(line, heredocs[0].delimiter) {
				heredocs = heredocs[1:]
			}
			index = lineEnd
			continue
		}
		character := contents[index]
		if escaped {
			escaped = false
			continue
		}
		if quote != 0 {
			if character == '\\' && quote != '\'' {
				escaped = true
				continue
			}
			if character == quote {
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
		if character == '$' && index+2 < len(contents) && contents[index+1] == '(' && contents[index+2] == '(' {
			arithmeticDepth = 2
			index += 2
			continue
		}
		if character == '#' {
			if index == 0 || contents[index-1] == '\n' || contents[index-1] == ' ' || contents[index-1] == '\t' {
				for index < len(contents) && contents[index] != '\n' {
					index++
				}
				continue
			}
		}
		if character == '<' && index+1 < len(contents) && contents[index+1] == '<' && (index+2 >= len(contents) || contents[index+2] != '<') {
			spec, next, ok := parseShadowHeredoc(contents, index+2)
			if !ok {
				return 0, false
			}
			heredocs = append(heredocs, spec)
			index = next - 1
			continue
		}
		if character == '{' {
			depth++
		}
		if character == '}' {
			depth--
			if depth == 0 {
				return index, true
			}
		}
	}
	return 0, false
}

func parseShadowHeredoc(contents []byte, position int) (struct {
	delimiter []byte
	stripTabs bool
}, int, bool) {
	result := struct {
		delimiter []byte
		stripTabs bool
	}{}
	if position < len(contents) && contents[position] == '-' {
		result.stripTabs = true
		position++
	}
	for position < len(contents) && (contents[position] == ' ' || contents[position] == '\t') {
		position++
	}
	if position >= len(contents) {
		return result, position, false
	}
	quote := byte(0)
	if contents[position] == '\'' || contents[position] == '"' {
		quote = contents[position]
		position++
	}
	start := position
	for position < len(contents) {
		character := contents[position]
		if quote != 0 {
			if character == quote {
				result.delimiter = append([]byte(nil), contents[start:position]...)
				position++
				break
			}
			position++
			continue
		}
		if bytes.ContainsRune([]byte(" \t\r\n;|&()<>"), rune(character)) {
			result.delimiter = append([]byte(nil), contents[start:position]...)
			break
		}
		position++
	}
	if quote != 0 && (position == 0 || contents[position-1] != quote) {
		return result, position, false
	}
	if quote == 0 && result.delimiter == nil {
		result.delimiter = append([]byte(nil), contents[start:position]...)
	}
	return result, position, len(result.delimiter) > 0
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
func newShadowResult(source []byte, shell string, start, end, startLine, endLine int, entry *neutralcatalog.Entry) shadowResult {
	origin := shadowOrigin(shell, source, start, end)
	entry.ID = origin[:32]
	return shadowResult{Unit: 0, Name: entry.Name, Kind: entry.Kind, Status: "equivalent", StartByte: start, EndByte: end, StartLine: startLine, EndLine: endLine, Diagnostics: []shadowDiagnostic{}, origin: origin, entry: entry}
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

func blockShadowSecrets(results []shadowResult, findings []SecretFinding, source []byte) []shadowResult {
	lines := sourceLines(source)
	for _, finding := range findings {
		mapped := false
		for index := range results {
			if finding.Line >= results[index].StartLine && finding.Line <= results[index].EndLine {
				mapped = true
				results[index].Status = "blocked"
				results[index].entry = nil
				results[index].Diagnostics = append(results[index].Diagnostics, shadowDiagnostic{Code: "secret_detected", Message: fmt.Sprintf("possible %s on line %d", finding.Kind, finding.Line)})
			}
		}
		if !mapped && finding.Line > 0 && finding.Line <= len(lines) {
			line := lines[finding.Line-1]
			results = append(results, shadowResult{Unit: len(results), Status: "blocked", StartByte: line.start, EndByte: line.end, StartLine: finding.Line, EndLine: finding.Line, Diagnostics: []shadowDiagnostic{{Code: "secret_detected", Message: fmt.Sprintf("possible %s on line %d", finding.Kind, finding.Line)}}})
		}
	}
	return results
}
func markShadowDuplicates(results []shadowResult) {
	groups := map[string][]int{}
	for index, result := range results {
		if result.entry != nil {
			groups[result.Name] = append(groups[result.Name], index)
		}
	}
	for _, indices := range groups {
		if len(indices) < 2 {
			continue
		}
		for _, index := range indices {
			results[index].Status = "duplicate"
			results[index].entry = nil
			results[index].Diagnostics = append(results[index].Diagnostics, shadowDiagnostic{Code: "duplicate_name", Message: "every definition with this name is excluded"})
		}
	}
}
func validateShadowCandidates(results []shadowResult) {
	for index := range results {
		if results[index].entry == nil {
			continue
		}
		candidate := neutralcatalog.Catalog{SchemaVersion: 1, Entries: []neutralcatalog.Entry{*results[index].entry}}
		if diagnostics := neutralcatalog.Validate(candidate); len(diagnostics) > 0 {
			results[index].Status = "invalid"
			results[index].entry = nil
			results[index].Diagnostics = append(results[index].Diagnostics, shadowDiagnostic{Code: "invalid_candidate", Message: "entry does not satisfy the catalog schema"})
			continue
		}
		results[index].rendered = renderShadowEntry(*results[index].entry, results[index].origin)
	}
}
func renderShadowEntry(entry neutralcatalog.Entry, origin string) []byte {
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
	if line := metadataLine(metadata); line != "" {
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
			output.WriteString(quoteShadow(*native.AliasValue))
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
func quoteShadow(value string) string { return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'" }

func validateShadowRendered(shell string, results []shadowResult) {
	indices := []int{}
	for index := range results {
		if results[index].entry != nil {
			indices = append(indices, index)
		}
	}
	if len(indices) == 0 {
		return
	}
	overall, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	launches := 0
	contentsFor := func(batch []int) []byte {
		var body bytes.Buffer
		for _, index := range batch {
			body.Write(results[index].rendered)
		}
		return body.Bytes()
	}
	blockBatch := func(batch []int) {
		for _, index := range batch {
			blockShadowValidationResult(&results[index])
		}
	}
	var validateBatch func([]int)
	validateBatch = func(batch []int) {
		if len(batch) == 0 {
			return
		}
		if overall.Err() != nil || launches >= shadowMaxLaunches {
			blockBatch(batch)
			return
		}
		launches++
		err := shadowSyntaxValidator(overall, shell, contentsFor(batch))
		if err == nil {
			for _, index := range batch {
				compareShadowRoundTrip(shell, &results[index])
			}
			return
		}
		if shadowValidationBlocked(err) {
			blockBatch(batch)
			return
		}
		if len(batch) == 1 {
			index := batch[0]
			results[index].Status = "invalid"
			results[index].Diagnostics = append(results[index].Diagnostics, shadowDiagnostic{Code: "native_syntax", Message: "rendered definition failed native syntax validation"})
			return
		}
		middle := len(batch) / 2
		validateBatch(batch[:middle])
		validateBatch(batch[middle:])
	}
	validateBatch(indices)
}

func compareShadowRoundTrip(shell string, result *shadowResult) {
	newline := bytes.IndexByte(result.rendered, '\n')
	if newline < 0 || string(result.rendered[:newline]) != "# al-shadow-origin: "+result.origin {
		result.Status = "invalid"
		result.Diagnostics = append(result.Diagnostics, shadowDiagnostic{Code: "invalid_origin_marker", Message: "rendered definition lost its internal identity marker"})
		return
	}
	reparsed := importShadowSource(shell, result.rendered[newline+1:])
	if len(reparsed) != 1 || reparsed[0].entry == nil {
		result.Status = "invalid"
		result.Diagnostics = append(result.Diagnostics, shadowDiagnostic{Code: "reparse_failed", Message: "rendered definition could not be parsed again"})
		return
	}
	reparsed[0].entry.ID = result.entry.ID
	changes, diagnostics := neutralcatalog.Compare(
		neutralcatalog.Catalog{SchemaVersion: 1, Entries: []neutralcatalog.Entry{*result.entry}},
		neutralcatalog.Catalog{SchemaVersion: 1, Entries: []neutralcatalog.Entry{*reparsed[0].entry}},
	)
	if len(diagnostics) > 0 {
		result.Status = "invalid"
		result.Diagnostics = append(result.Diagnostics, shadowDiagnostic{Code: "comparison_failed", Message: "round-trip comparison could not complete"})
		return
	}
	if len(changes) > 0 {
		result.Status = "different"
		result.DifferentFields = append([]string(nil), changes[0].Fields...)
		result.Diagnostics = append(result.Diagnostics, shadowDiagnostic{Code: "round_trip_changed", Message: "supported fields changed during the round trip"})
	}
}
func shadowValidationBlocked(err error) bool {
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) {
		return true
	}
	if exitError.ProcessState == nil {
		return false
	}
	status, ok := exitError.Sys().(syscall.WaitStatus)
	return !ok || status.Signaled()
}
func blockShadowValidationResult(result *shadowResult) {
	result.Status = "blocked"
	result.Diagnostics = append(result.Diagnostics, shadowDiagnostic{Code: "validator_blocked", Message: "native syntax validation could not complete"})
}
func validateShadowSyntax(overall context.Context, shell string, contents []byte) error {
	path, err := shadowValidatorPath(shell)
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

type limitedBuffer struct {
	buffer bytes.Buffer
	total  int
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
func trustedShadowShell(shell string) (string, error) {
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

func finalizeShadowReport(report *shadowReport) {
	report.Summary = shadowSummary{}
	sort.SliceStable(report.Results, func(i, j int) bool {
		rank := map[string]int{"invalid": 0, "blocked": 1, "duplicate": 2, "different": 3, "unsupported": 4, "equivalent": 5}
		if rank[report.Results[i].Status] != rank[report.Results[j].Status] {
			return rank[report.Results[i].Status] < rank[report.Results[j].Status]
		}
		if report.Results[i].StartByte != report.Results[j].StartByte {
			return report.Results[i].StartByte < report.Results[j].StartByte
		}
		return report.Results[i].Unit < report.Results[j].Unit
	})
	for index := range report.Results {
		sort.SliceStable(report.Results[index].Diagnostics, func(i, j int) bool {
			return report.Results[index].Diagnostics[i].Code < report.Results[index].Diagnostics[j].Code
		})
		result := report.Results[index]
		switch result.Status {
		case "equivalent":
			report.Summary.Equivalent++
		case "unsupported":
			report.Summary.Unsupported++
		case "different":
			report.Summary.Different++
		case "duplicate":
			report.Summary.Duplicate++
		case "invalid":
			report.Summary.Invalid++
		case "blocked":
			report.Summary.Blocked++
		}
	}
	limitShadowDiagnostics(report)
}

func limitShadowDiagnostics(report *shadowReport) {
	sort.SliceStable(report.Diagnostics, func(i, j int) bool { return report.Diagnostics[i].Code < report.Diagnostics[j].Code })
	remaining := 100
	omitted := 0
	if len(report.Diagnostics) > remaining {
		omitted += len(report.Diagnostics) - remaining
		report.Diagnostics = append([]shadowDiagnostic(nil), report.Diagnostics[:remaining]...)
		remaining = 0
	} else {
		remaining -= len(report.Diagnostics)
	}
	for index := range report.Results {
		diagnostics := report.Results[index].Diagnostics
		if len(diagnostics) > remaining {
			omitted += len(diagnostics) - remaining
			report.Results[index].Diagnostics = append([]shadowDiagnostic(nil), diagnostics[:remaining]...)
			remaining = 0
		} else {
			remaining -= len(diagnostics)
		}
	}
	if omitted > 0 {
		report.Diagnostics = append(report.Diagnostics, shadowDiagnostic{Code: "diagnostics_truncated", Message: fmt.Sprintf("%d additional diagnostics omitted", omitted)})
	}
}
func renderShadowReport(report shadowReport, jsonOutput bool) ([]byte, error) {
	if jsonOutput {
		contents, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return nil, err
		}
		return append(contents, '\n'), nil
	}
	var output strings.Builder
	fmt.Fprintf(&output, "Alias Lens shadow inspection · %s\n\n", report.Shell)
	for _, result := range report.Results {
		label := result.Name
		if label == "" {
			label = fmt.Sprintf("lines %d-%d", result.StartLine, result.EndLine)
		}
		fmt.Fprintf(&output, "%-11s %s\n", result.Status, label)
	}
	fmt.Fprintf(&output, "\n%d equivalent · %d unsupported · %d different · %d duplicate · %d invalid · %d blocked\n", report.Summary.Equivalent, report.Summary.Unsupported, report.Summary.Different, report.Summary.Duplicate, report.Summary.Invalid, report.Summary.Blocked)
	return []byte(output.String()), nil
}
