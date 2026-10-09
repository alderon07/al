//go:build !windows

package shell

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
)

type startupDeclaration struct {
	name, kind string
	line, end  int
}

func startupPlacementError(line int, reason string) error {
	return fmt.Errorf("startup line %d: %s; review al catalog enable --startup-path PATH or al setup --repair", line, reason)
}

func startupQuotedEnd(source []byte, start, depth int) (int, bool) {
	if depth > 64 {
		return 0, false
	}
	quote := source[start]
	for position := start + 1; position < len(source); position++ {
		if source[position] == quote {
			return position, true
		}
		if quote == '\'' {
			continue
		}
		if source[position] == '\\' {
			position++
			continue
		}
		if source[position] == '$' && position+1 < len(source) && (source[position+1] == '(' || source[position+1] == '{') {
			end, ok := startupExpansionEnd(source, position+1, depth+1)
			if !ok {
				return 0, false
			}
			position = end
		}
	}
	return 0, false
}

func startupExpansionEnd(source []byte, start, depth int) (int, bool) {
	if depth > 64 {
		return 0, false
	}
	open := source[start]
	close := byte(')')
	if open == '{' {
		close = '}'
	}
	nesting := 1
	for position := start + 1; position < len(source); position++ {
		character := source[position]
		if character == '\\' {
			position++
			continue
		}
		if character == '\'' || character == '"' || character == '`' {
			end, ok := startupQuotedEnd(source, position, depth+1)
			if !ok {
				return 0, false
			}
			position = end
			continue
		}
		if character == open {
			nesting++
		}
		if character == close {
			nesting--
			if nesting == 0 {
				return position, true
			}
		}
	}
	return 0, false
}

func startupAnalysisMask(source []byte) ([]byte, error) { return startupMask(source, true) }

func startupMask(source []byte, comments bool) ([]byte, error) {
	masked := append([]byte{}, source...)
	wordStart := true
	for position := 0; position < len(source); position++ {
		character := source[position]
		end := position
		if character == '#' && wordStart {
			for end+1 < len(source) && source[end+1] != '\n' {
				end++
			}
			if !comments {
				position = end
				wordStart = false
				continue
			}
		} else if character == '\'' || character == '"' || character == '`' {
			var ok bool
			end, ok = startupQuotedEnd(source, position, 0)
			if !ok {
				return nil, startupPlacementError(1+bytes.Count(source[:position], []byte("\n")), "quoted startup boundary is unproven")
			}
		} else if character == '$' && position+1 < len(source) && (source[position+1] == '{' || source[position+1] == '(') {
			var ok bool
			end, ok = startupExpansionEnd(source, position+1, 0)
			if !ok {
				return nil, startupPlacementError(1, "startup expansion boundary is unproven")
			}
		} else if character == '\\' {
			if position+1 >= len(source) {
				return nil, startupPlacementError(1+bytes.Count(source[:position], []byte("\n")), "startup escape is incomplete")
			}
			end++
		} else {
			wordStart = bytes.ContainsRune([]byte(" \t\n;|&(){}"), rune(character))
			continue
		}
		for index := position; index <= end; index++ {
			if masked[index] != '\n' {
				masked[index] = ' '
			}
		}
		position = end
		wordStart = false
	}
	return masked, nil
}

func startupLiteralAlias(line string) (string, bool) {
	if name, _, ok := parseShadowAlias(line); ok {
		return name, true
	}
	if !strings.HasPrefix(line, "alias ") && !strings.HasPrefix(line, "alias\t") {
		return "", false
	}
	rest := strings.TrimLeft(line[5:], " \t")
	name, value, ok := strings.Cut(rest, "=")
	if !ok || !shadowCommandName.MatchString(name) || len(value) == 0 || value[0] != '"' {
		return "", false
	}
	end, ok := startupQuotedEnd([]byte(value), 0, 0)
	if !ok {
		return "", false
	}
	for index := 1; index < end; index++ {
		if value[index] == '\\' {
			index++
			continue
		}
		if value[index] == '$' || value[index] == '`' || value[index] == '\n' {
			return "", false
		}
	}
	tail := value[end+1:]
	if tail != "" && tail[0] != ' ' && tail[0] != '\t' {
		return "", false
	}
	tail = strings.TrimLeft(tail, " \t")
	return name, tail == "" || strings.HasPrefix(tail, "#")
}

func startupControlWord(line string) string {
	word := strings.Fields(line)
	if len(word) == 0 {
		return ""
	}
	first := strings.TrimRight(word[0], ";")
	if first == "return" || first == "exit" || first == "exec" || first == "source" || first == "." {
		return first
	}
	return ""
}

func startupStaticOtherSource(line, shell, home, nativePath string) bool {
	fields := strings.Fields(line)
	if len(fields) < 2 || fields[0] != "source" && fields[0] != "." {
		return false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
	value, tail := "", ""
	if strings.HasPrefix(rest, "'") {
		var ok bool
		value, tail, ok = decodeShadowSingleQuote(rest)
		if !ok {
			return false
		}
	} else if strings.HasPrefix(rest, "\"") {
		end, ok := startupQuotedEnd([]byte(rest), 0, 0)
		if !ok {
			return false
		}
		value = rest[1:end]
		tail = rest[end+1:]
		value = strings.ReplaceAll(strings.ReplaceAll(value, "${HOME}/", home+"/"), "$HOME/", home+"/")
		if strings.ContainsAny(value, "$`\\\n") {
			return false
		}
	} else {
		end := strings.IndexAny(rest, " \t")
		if end < 0 {
			value = rest
		} else {
			value, tail = rest[:end], rest[end:]
		}
		if strings.ContainsAny(value, "$`\\;|&()<>{}\n") {
			return false
		}
	}
	if tail != "" && tail[0] != ' ' && tail[0] != '\t' {
		return false
	}
	tail = strings.TrimSpace(tail)
	if tail != "" && !strings.HasPrefix(tail, "#") {
		return false
	}
	if value == "" || strings.Contains(value, ".bash_aliases") || strings.Contains(value, ".zsh_aliases") || strings.Contains(value, nativePath) {
		return false
	}
	if strings.HasPrefix(value, "~/") {
		value = filepath.Join(home, value[2:])
	}
	if !filepath.IsAbs(value) {
		return false
	}
	if referent, err := filepath.EvalSymlinks(value); err == nil && referent == nativePath {
		return false
	}
	return true
}

func startupCommandStarts(line string) []int {
	starts := []int{}
	command := true
	for position := 0; position < len(line); {
		character := line[position]
		if character == ' ' || character == '\t' {
			position++
			continue
		}
		if character == ';' || character == '|' || character == '&' {
			command = true
			position++
			continue
		}
		start := position
		for position < len(line) && !bytes.ContainsRune([]byte(" \t;|&"), rune(line[position])) {
			position++
		}
		word := line[start:position]
		if command {
			if catalogAssignmentWord(word) {
				continue
			}
			starts = append(starts, start)
			if word == "then" || word == "do" || word == "else" {
				continue
			}
			command = false
		}
	}
	return starts
}

const catalogExplicitNativeStart = "# >>> Alias Lens explicitly enrolled native source >>>\n"
const catalogExplicitNativeEnd = "# <<< Alias Lens explicitly enrolled native source <<<\n"

func startupVisibleMarkerOffsets(source []byte, marker string) ([]int, error) {
	mask, err := startupMask(source, false)
	if err != nil {
		return nil, err
	}
	analysis, err := startupAnalysisMask(source)
	if err != nil {
		return nil, err
	}
	positions := []int{}
	braces := 0
	for _, line := range SourceLines(mask) {
		if braces == 0 && strings.TrimSpace(string(line.Bytes)) == strings.TrimSpace(marker) {
			positions = append(positions, line.Start)
		}
		for _, character := range analysis[line.Start:line.End] {
			if character == '{' {
				braces++
			}
			if character == '}' {
				braces--
			}
		}
	}
	return positions, nil
}

func CatalogStartupBlockSpan(source []byte) (int, int, error) {
	starts, err := startupVisibleMarkerOffsets(source, catalogLoaderStart)
	if err != nil {
		return 0, 0, err
	}
	ends, err := startupVisibleMarkerOffsets(source, catalogLoaderEnd)
	if err != nil {
		return 0, 0, err
	}
	if len(starts) != 1 || len(ends) != 1 || ends[0] < starts[0] {
		return 0, 0, fmt.Errorf("catalog startup block is incomplete")
	}
	prefix := source[:starts[0]]
	mask, err := startupMask(prefix, false)
	if err != nil {
		return 0, 0, err
	}
	mask, err = startupDeferredMask(prefix, mask)
	if err != nil {
		return 0, 0, err
	}
	if !startupOwnedVisible(prefix, mask, len(prefix), "") {
		return 0, 0, fmt.Errorf("catalog startup block is nested")
	}
	start, end := starts[0], ends[0]+len(catalogLoaderEnd)
	if start > 0 && source[start-1] == '\n' {
		start--
	}
	if end < len(source) && source[end] == '\n' {
		end++
	}
	return start, end, nil
}

func CatalogExplicitNativeRoute(block []byte) ([]byte, error) {
	starts, err := startupVisibleMarkerOffsets(block, catalogExplicitNativeStart)
	if err != nil {
		return nil, err
	}
	ends, err := startupVisibleMarkerOffsets(block, catalogExplicitNativeEnd)
	if err != nil {
		return nil, err
	}
	if len(starts) == 0 && len(ends) == 0 {
		return nil, nil
	}
	if len(starts) != 1 || len(ends) != 1 || ends[0] < starts[0] {
		return nil, fmt.Errorf("explicit native startup segment is incomplete; run al setup --repair")
	}
	return append([]byte{}, block[starts[0]+len(catalogExplicitNativeStart):ends[0]]...), nil
}

func CatalogExplicitNativeSourceTail(contents []byte, shell, home, nativePath string) ([]byte, []byte, error) {
	lines := SourceLines(contents)
	if len(lines) == 0 {
		return nil, nil, startupPlacementError(1, "explicit native source tail is missing")
	}
	last := len(lines) - 1
	start := last
	if strings.TrimSpace(string(lines[last].Bytes)) == "fi" && last >= 2 {
		header := strings.TrimSpace(string(lines[last-2].Bytes))
		prefix, suffix := "if [ -f ", " ]; then"
		if shell == "zsh" && strings.HasPrefix(header, "if [[ -f ") {
			prefix, suffix = "if [[ -f ", " ]]; then"
		}
		if !strings.HasPrefix(header, prefix) || !strings.HasSuffix(header, suffix) || !CatalogStaticSource("source "+strings.TrimSuffix(strings.TrimPrefix(header, prefix), suffix), home, nativePath) || !CatalogStaticSource(strings.TrimSpace(string(lines[last-1].Bytes)), home, nativePath) {
			return nil, nil, startupPlacementError(last+1, "explicit native source tail is unproven")
		}
		start = last - 2
	} else if !CatalogStaticSource(strings.TrimSpace(string(lines[last].Bytes)), home, nativePath) {
		return nil, nil, startupPlacementError(last+1, "explicit native source tail is unproven")
	}
	return append([]byte{}, contents[:lines[start].Start]...), append([]byte{}, contents[lines[start].Start:]...), nil
}

func CatalogGuardExplicitBlock(shell string, block string, route []byte) (string, error) {
	disable, restore, state := "", "", ""
	switch shell {
	case "bash":
		state = "_alias_lens_startup_alias_state=$(\\builtin shopt -p expand_aliases || \\builtin true)\n"
		disable = "\\builtin shopt -u expand_aliases\n"
		restore = "case $_alias_lens_startup_alias_state in\n  'shopt -s expand_aliases') \\builtin shopt -s expand_aliases ;;\nesac\n\\builtin unset _alias_lens_startup_alias_state\n"
	case "zsh":
		state = "_alias_lens_startup_alias_state=\"$options[aliases]\"\n"
		disable = "\\builtin unsetopt aliases\n"
		restore = "case $_alias_lens_startup_alias_state in\n  on) \\builtin setopt aliases ;;\nesac\n\\builtin unset _alias_lens_startup_alias_state\n"
	default:
		return "", fmt.Errorf("unsupported explicit startup shell")
	}
	header := catalogLoaderStart + "\n"
	if strings.Count(block, header) != 1 || strings.Count(block, catalogLoaderEnd) != 1 {
		return "", fmt.Errorf("explicit catalog startup block is unproven")
	}
	block = strings.Replace(block, header, header+state+disable+catalogExplicitNativeStart+string(route)+catalogExplicitNativeEnd, 1)
	return strings.Replace(block, catalogLoaderEnd, restore+catalogLoaderEnd, 1), nil
}

func startupQuotedControl(line, analysis string) bool {
	starts := []int{0}
	for index, character := range analysis {
		if character == ';' || character == '|' || character == '&' {
			starts = append(starts, index+1)
		}
	}
	for _, start := range starts {
		rest := strings.TrimSpace(line[start:])
		fields := strings.Fields(rest)
		for len(fields) > 0 && catalogAssignmentWord(fields[0]) {
			rest = strings.TrimSpace(strings.TrimPrefix(rest, fields[0]))
			fields = strings.Fields(rest)
		}
		word := ""
		if strings.HasPrefix(rest, "'") {
			value, _, ok := decodeShadowSingleQuote(rest)
			if ok {
				word = value
			}
		} else if strings.HasPrefix(rest, "\"") {
			end, ok := startupQuotedEnd([]byte(rest), 0, 0)
			if ok {
				word = rest[1:end]
			}
		} else if strings.HasPrefix(rest, "\\") {
			word = strings.TrimPrefix(strings.Fields(rest)[0], "\\")
		}
		switch word {
		case "alias", "builtin", "command", "source", ".", "eval", "return", "exit", "exec", "function":
			return true
		}
	}
	return false
}

func startupMasksControl(name, kind string) bool {
	if kind == "function" {
		return name == "builtin" || name == "command"
	}
	for _, control := range strings.Fields(". : [ [[ alias autoload bind bindkey builtin case command declare do done else esac eval exec export false fc fi for function history if in local print printf return select set setopt shift source then true typeset unalias unset until while zle") {
		if name == control {
			return true
		}
	}
	return false
}

func startupDeferredMask(source, masked []byte) ([]byte, error) {
	lines := SourceLines(source)
	for index := 0; index < len(lines); index++ {
		if strings.TrimSpace(string(masked[lines[index].Start:lines[index].End])) == "" {
			continue
		}
		_, open, ok := nativeFunctionHeader(source, lines[index].Start)
		if !ok {
			continue
		}
		close, reason := (nativeBoundaryScanner{source: source}).region(open+1, '}', 0)
		if reason != "" {
			return nil, startupPlacementError(index+1, "deferred function boundary is unsupported")
		}
		for position := open + 1; position <= close; position++ {
			if masked[position] != '\n' {
				masked[position] = ' '
			}
		}
		for index+1 < len(lines) && lines[index+1].Start <= close {
			index++
		}
	}
	return masked, nil
}

func startupOwnedVisible(source, masked []byte, start int, phrase string) bool {
	if !bytes.Contains(masked[start:], []byte(phrase)) {
		return false
	}
	prefix := append([]byte{}, masked[:start]...)
	for _, guard := range []string{"case $- in\n    *i*) ;;\n      *) return;;\nesac", "case $- in\n    *i*) ;;\n      *) return ;;\nesac"} {
		if position := bytes.Index(source[:start], []byte(guard)); position >= 0 && strings.HasPrefix(strings.TrimSpace(string(prefix[position:position+len(guard)])), "case $- in") {
			for index := position; index < position+len(guard); index++ {
				if prefix[index] != '\n' {
					prefix[index] = ' '
				}
			}
		}
	}
	depth := 0
	for _, raw := range strings.Split(string(prefix), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "if ") || strings.HasPrefix(line, "for ") || strings.HasPrefix(line, "while ") || strings.HasPrefix(line, "case ") {
			depth++
		}
		if line == "fi" || line == "esac" || line == "done" {
			depth--
		}
	}
	return depth == 0
}

func startupHintValue(input, home string, hints map[string]string) (string, string, bool) {
	value, tail := input, ""
	quoted := false
	if strings.HasPrefix(input, "'") {
		var ok bool
		value, tail, ok = decodeShadowSingleQuote(input)
		if !ok {
			return "", "", false
		}
	} else if strings.HasPrefix(input, "\"") {
		end, ok := startupQuotedEnd([]byte(input), 0, 0)
		if !ok {
			return "", "", false
		}
		value, tail = input[1:end], input[end+1:]
		quoted = true
	} else {
		if end := strings.IndexAny(input, " \t"); end >= 0 {
			value, tail = input[:end], input[end:]
		}
	}
	if tail != "" && tail[0] != ' ' && tail[0] != '\t' {
		return "", "", false
	}
	tail = strings.TrimSpace(tail)
	if tail != "" && !strings.HasPrefix(tail, "#") {
		return "", "", false
	}
	if strings.ContainsAny(value, "`\\\n*?[];|&()<>") {
		return "", "", false
	}
	var expanded strings.Builder
	for position := 0; position < len(value); position++ {
		if value[position] != '$' {
			expanded.WriteByte(value[position])
			continue
		}
		if !quoted {
			return "", "", false
		}
		start := position + 1
		end := start
		braced := start < len(value) && value[start] == '{'
		if braced {
			start++
			end = start
		}
		for end < len(value) && (value[end] >= 'a' && value[end] <= 'z' || value[end] >= 'A' && value[end] <= 'Z' || value[end] == '_' || end > start && value[end] >= '0' && value[end] <= '9') {
			end++
		}
		if end == start || braced && (end >= len(value) || value[end] != '}') {
			return "", "", false
		}
		name := value[start:end]
		replacement, ok := hints[name]
		if name == "HOME" {
			replacement, ok = home, true
		}
		if !ok {
			return "", "", false
		}
		expanded.WriteString(replacement)
		position = end - 1
		if braced {
			position = end
		}
	}
	result := expanded.String()
	if strings.HasPrefix(result, "~/") {
		result = filepath.Join(home, result[2:])
	}
	if !filepath.IsAbs(result) {
		return "", "", false
	}
	return result, tail, true
}

func startupHintCommandSegments(line string) [][]string {
	source := []byte(line)
	segments := [][]string{}
	words := []string{}
	for position := 0; position < len(source); {
		if source[position] == ' ' || source[position] == '\t' {
			position++
			continue
		}
		if source[position] == '#' {
			break
		}
		if strings.ContainsRune(";|&", rune(source[position])) {
			if len(words) > 0 {
				segments = append(segments, words)
				words = nil
			}
			position++
			continue
		}
		start := position
		for position < len(source) && !strings.ContainsRune(" \t;|&", rune(source[position])) {
			if source[position] == '\'' || source[position] == '"' {
				end, ok := startupQuotedEnd(source, position, 0)
				if !ok {
					position = len(source)
					break
				}
				position = end + 1
			} else if source[position] == '\\' && position+1 < len(source) {
				position += 2
			} else {
				position++
			}
		}
		words = append(words, string(source[start:position]))
	}
	if len(words) > 0 {
		segments = append(segments, words)
	}
	return segments
}

func startupHintTarget(word string) (string, bool) {
	target, _, _ := strings.Cut(word, "=")
	target = strings.TrimSuffix(target, "+")
	if len(target) >= 2 && (target[0] == '\'' && target[len(target)-1] == '\'' || target[0] == '"' && target[len(target)-1] == '"') {
		target = target[1 : len(target)-1]
	}
	if index := strings.IndexByte(target, '['); index >= 0 {
		target = target[:index]
	}
	return target, shadowFunctionName.MatchString(target)
}

func startupUpdateHints(line string, depth int, home string, hints map[string]string, seen map[string]bool) bool {
	original := strings.TrimSpace(line)
	assignment := strings.TrimSpace(strings.TrimPrefix(original, "export "))
	name, value, ok := strings.Cut(assignment, "=")
	resolved, _, valid := startupHintValue(value, home, nil)
	establish := ok && shadowFunctionName.MatchString(name) && !seen[name] && depth == 0 && valid && name != "HOME"
	invalidate := func(word string) {
		target, literal := startupHintTarget(word)
		if !literal {
			for target := range hints {
				delete(hints, target)
				seen[target] = true
			}
			seen["HOME"] = true
			return
		}
		delete(hints, target)
		seen[target] = true
	}
	for _, words := range startupHintCommandSegments(line) {
		index := 0
		for index < len(words) {
			if _, _, isAssignment := strings.Cut(words[index], "="); !isAssignment {
				break
			}
			target, literal := startupHintTarget(words[index])
			if literal && establish && target == name && strings.HasPrefix(original, name+"=") {
				hints[name] = resolved
				seen[name] = true
				establish = false
			} else {
				invalidate(words[index])
			}
			index++
		}
		if index >= len(words) {
			continue
		}
		for index < len(words) {
			prefix, literal := startupHintTarget(words[index])
			if !literal || prefix != "builtin" && prefix != "command" && prefix != "then" && prefix != "do" && prefix != "else" {
				break
			}
			index++
		}
		if index >= len(words) {
			continue
		}
		command, _ := startupHintTarget(words[index])
		targets := words[index+1:]
		switch command {
		case "export", "unset", "read", "declare", "typeset", "local", "readonly":
			for _, word := range targets {
				if (command == "declare" || command == "typeset" || command == "local") && word == "-n" {
					invalidate("$indirect")
					continue
				}
				if strings.HasPrefix(word, "-") {
					continue
				}
				target, literal := startupHintTarget(word)
				if command == "export" && literal && establish && target == name && strings.HasPrefix(original, "export "+name+"=") {
					hints[name] = resolved
					seen[name] = true
					establish = false
					continue
				}
				invalidate(word)
			}
		case "printf":
			if len(targets) >= 2 && targets[0] == "-v" {
				invalidate(targets[1])
			}
		}
	}
	return !seen["HOME"]
}

func startupGuardedOpaqueSource(original, analysis, shell, home, nativePath string, hints map[string]string) bool {
	index := strings.Index(analysis, "&&")
	if index < 0 || strings.Count(analysis, "&&") != 1 || strings.ContainsAny(analysis[:index], ";|") {
		return false
	}
	left, right := strings.TrimSpace(original[:index]), strings.TrimSpace(original[index+2:])
	prefix := ""
	for _, candidate := range []string{"[ -f ", "[ -s "} {
		if strings.HasPrefix(left, candidate) {
			prefix = candidate
		}
	}
	if prefix == "" || !strings.HasSuffix(left, " ]") {
		return false
	}
	operand := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(left, prefix), " ]"))
	fields := strings.Fields(right)
	if len(fields) < 2 || fields[0] != "source" && fields[0] != "." && fields[0] != "\\." {
		return false
	}
	sourceOperand := strings.TrimSpace(strings.TrimPrefix(right, fields[0]))
	leftValue, leftTail, leftOK := startupHintValue(operand, home, hints)
	rightValue, _, rightOK := startupHintValue(sourceOperand, home, hints)
	if !leftOK || leftTail != "" || !rightOK || leftValue != rightValue {
		return false
	}
	return startupStaticOtherSource("source "+QuoteShadow(rightValue), shell, home, nativePath)
}

func startupUnprovenFunctionHeader(line string) bool {
	fields := strings.Fields(line)
	if len(fields) > 0 && fields[0] == "function" {
		return true
	}
	end := strings.IndexAny(line, " \t(")
	if end <= 0 || strings.ContainsAny(line[:end], "=;|&") {
		return false
	}
	rest := strings.TrimSpace(line[end:])
	if !strings.HasPrefix(rest, "(") {
		return false
	}
	rest = strings.TrimSpace(rest[1:])
	if !strings.HasPrefix(rest, ")") {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(rest[1:]), "{")
}
