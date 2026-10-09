//go:build !windows

package shell

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	neutralcatalog "github.com/alderon07/al/internal/catalog"
)

type CatalogNativeUnit struct {
	Name      string
	Kind      string
	StartByte int
	EndByte   int
	StartLine int
	EndLine   int
	Body      string
	Entry     *neutralcatalog.Entry
}

type nativeBoundaryScanner struct{ source []byte }

func nativeBoundaryError(start, end int, reason string) error {
	return fmt.Errorf("native lines %d-%d: %s; keep a complete supported native declaration before al catalog enable", start, end, reason)
}

func InspectCatalogNativeSource(shell string, source []byte) ([]CatalogNativeUnit, error) {
	if shell != "bash" && shell != "zsh" {
		return nil, fmt.Errorf("native inspection requires bash or zsh")
	}
	if err := validateSource(source); err != nil || SourceLineCount(source) > shadowMaxLines {
		return nil, nativeBoundaryError(1, max(1, SourceLineCount(source)), "source encoding or bounds are unsupported")
	}
	lines := SourceLines(source)
	units := []CatalogNativeUnit{}
	kinds := map[string]string{}
	pending := []SourceLine{}
	scanner := nativeBoundaryScanner{source: source}
	for index := 0; index < len(lines); {
		line := lines[index]
		text := strings.TrimSpace(string(line.Bytes))
		if text == "" {
			pending = nil
			index++
			continue
		}
		if strings.HasPrefix(text, "#") {
			pending = append(pending, line)
			index++
			continue
		}
		start, startLine := line.Start, line.Number
		comments := ownedShadowComments(pending)
		if len(comments) > 0 && comments[len(comments)-1].End == line.Start {
			start = comments[0].Start
			startLine = comments[0].Number
		}
		pending = nil
		if _, _, ok := parseShadowAlias(text); ok {
			parsed := ImportShadowSource(shell, source[start:line.End])
			if len(parsed) != 1 || parsed[0].Entry == nil {
				return nil, nativeBoundaryError(startLine, line.Number, "alias metadata is outside the supported grammar")
			}
			result := parsed[0]
			units = append(units, CatalogNativeUnit{Name: result.Name, Kind: result.Kind, StartByte: start, EndByte: line.End, StartLine: startLine, EndLine: line.Number, Entry: result.Entry})
			index++
		} else {
			name, open, ok := nativeFunctionHeader(source, line.Start)
			if !ok {
				return nil, nativeBoundaryError(startLine, line.Number, "top-level statement has unproven boundaries or masks catalog handoff control")
			}
			close, reason := scanner.region(open+1, '}', 0)
			if reason != "" {
				return nil, nativeBoundaryError(startLine, lines[len(lines)-1].Number, reason)
			}
			endIndex := sort.Search(len(lines), func(i int) bool { return lines[i].End > close })
			if endIndex >= len(lines) {
				return nil, nativeBoundaryError(startLine, line.Number, "function boundary is unproven")
			}
			tail := source[close+1 : lines[endIndex].End]
			if len(tail) > 0 && tail[0] != ' ' && tail[0] != '\t' && tail[0] != '\n' {
				return nil, nativeBoundaryError(startLine, lines[endIndex].Number, "function declaration has an unsupported tail")
			}
			trimmed := bytes.TrimSpace(tail)
			if len(trimmed) > 0 && trimmed[0] != '#' {
				return nil, nativeBoundaryError(startLine, lines[endIndex].Number, "function declaration has trailing commands or redirections")
			}
			unit := CatalogNativeUnit{Name: name, Kind: "function", StartByte: start, EndByte: lines[endIndex].End, StartLine: startLine, EndLine: lines[endIndex].Number, Body: string(source[open+1 : close])}
			parsed := ImportShadowSource(shell, source[start:unit.EndByte])
			if len(parsed) == 1 && parsed[0].Entry != nil && parsed[0].Name == name && parsed[0].Kind == "function" {
				unit.Entry = parsed[0].Entry
			}
			units = append(units, unit)
			index = endIndex + 1
		}
		if len(units) > shadowMaxResults {
			return nil, nativeBoundaryError(1, len(lines), "native declaration count exceeds inspection bounds")
		}
		unit := units[len(units)-1]
		if previous, ok := kinds[unit.Name]; ok && previous != unit.Kind {
			return nil, nativeBoundaryError(unit.StartLine, unit.EndLine, "alias and function headers share an ambiguous name")
		}
		kinds[unit.Name] = unit.Kind
	}
	return units, nil
}

func nativeFunctionHeader(source []byte, start int) (string, int, bool) {
	position := start
	space := func() {
		for position < len(source) && (source[position] == ' ' || source[position] == '\t') {
			position++
		}
	}
	word := func() string {
		begin := position
		for position < len(source) && (source[position] >= 'A' && source[position] <= 'Z' || source[position] >= 'a' && source[position] <= 'z' || source[position] == '_' || position > begin && source[position] >= '0' && source[position] <= '9') {
			position++
		}
		return string(source[begin:position])
	}
	space()
	name := word()
	keyword := name == "function"
	if keyword {
		if position >= len(source) || source[position] != ' ' && source[position] != '\t' {
			return "", 0, false
		}
		space()
		name = word()
	}
	if !shadowFunctionName.MatchString(name) {
		return "", 0, false
	}
	for _, reserved := range strings.Fields("if then else elif fi for while until do done case esac in function select time coproc") {
		if name == reserved {
			return "", 0, false
		}
	}
	space()
	parentheses := position+1 < len(source) && source[position] == '(' && source[position+1] == ')'
	if parentheses {
		position += 2
		space()
	}
	if !keyword && !parentheses {
		return "", 0, false
	}
	if position < len(source) && source[position] == '\n' {
		position++
		space()
	}
	if position >= len(source) || source[position] != '{' {
		return "", 0, false
	}
	if position+1 < len(source) && !bytes.ContainsRune([]byte(" \t\n}"), rune(source[position+1])) {
		return "", 0, false
	}
	return name, position, true
}

func (scanner nativeBoundaryScanner) expansion(position, depth int) (int, string) {
	source := scanner.source
	if position+1 >= len(source) {
		return position, ""
	}
	switch source[position+1] {
	case '(':
		if position+2 < len(source) && source[position+2] == '(' {
			return 0, "arithmetic expansion is outside retained-function inspection"
		}
		return scanner.region(position+2, ')', depth+1)
	case '{':
		return scanner.region(position+2, 'p', depth+1)
	case '[', '\'', '"':
		return 0, "special dollar expansion is outside retained-function inspection"
	}
	return position, ""
}

func (scanner nativeBoundaryScanner) region(position int, mode byte, depth int) (int, string) {
	if depth > 64 {
		return 0, "substitution nesting exceeds inspection bounds"
	}
	source := scanner.source
	wordStart, commandStart := true, true
	hasCommand, pendingOperator := false, false
	sawCommand, pendingRedirect := false, false
	for index := position; index < len(source); index++ {
		character := source[index]
		if character == '\\' {
			if index+1 >= len(source) {
				return 0, "escape sequence is incomplete"
			}
			index++
			if source[index] == '\n' && mode != '"' && mode != 'p' {
				if index < 2 || source[index-2] != ' ' && source[index-2] != '\t' || index+1 >= len(source) || source[index+1] != ' ' && source[index+1] != '\t' {
					return 0, "unquoted continuation joins unproven tokens"
				}
			}
			if source[index] != '\n' {
				wordStart, commandStart = false, false
				hasCommand, pendingOperator = true, false
				sawCommand, pendingRedirect = true, false
			}
			continue
		}
		if character == '\'' && mode != '"' && mode != 'p' {
			end := bytes.IndexByte(source[index+1:], '\'')
			if end < 0 {
				return 0, "single-quoted string is incomplete"
			}
			index += end + 1
			wordStart, commandStart = false, false
			hasCommand, pendingOperator = true, false
			sawCommand, pendingRedirect = true, false
			continue
		}
		if character == '"' {
			if mode == '"' {
				return index, ""
			}
			if mode == 'p' {
				return 0, "quoted parameter operands are outside retained-function inspection"
			}
			end, reason := scanner.region(index+1, '"', depth+1)
			if reason != "" {
				return 0, reason
			}
			index = end
			wordStart, commandStart = false, false
			hasCommand, pendingOperator = true, false
			sawCommand, pendingRedirect = true, false
			continue
		}
		if mode == 'p' && character == '\'' {
			return 0, "quoted parameter operands are outside retained-function inspection"
		}
		if character == '$' {
			end, reason := scanner.expansion(index, depth)
			if reason != "" {
				return 0, reason
			}
			index = end
			wordStart, commandStart = false, false
			hasCommand, pendingOperator = true, false
			sawCommand, pendingRedirect = true, false
			continue
		}
		if character == '`' {
			return 0, "backtick substitution is outside retained-function inspection"
		}
		if character == '\r' || character == 0 {
			return 0, "control character is outside retained-function inspection"
		}
		if mode == '"' {
			continue
		}
		if mode == 'p' {
			if character == '}' {
				if index == position {
					return 0, "parameter expansion is empty"
				}
				return index, ""
			}
			if character == '{' || character == '(' || character == ')' || character == '[' || character == ']' || character == '\n' {
				return 0, "parameter operand has unsupported structural syntax"
			}
			continue
		}
		if character == '#' && wordStart {
			for index < len(source) && source[index] != '\n' {
				index++
			}
			wordStart, commandStart = true, true
			hasCommand = false
			if pendingRedirect {
				return 0, "redirection target is unproven"
			}
			continue
		}
		if character == '<' && index+1 < len(source) && (source[index+1] == '<' || source[index+1] == '(') || character == '>' && index+1 < len(source) && source[index+1] == '(' {
			return 0, "heredoc or process substitution is outside retained-function inspection"
		}
		if character == '!' && commandStart {
			return 0, "command negation is outside retained-function inspection"
		}
		if commandStart && wordStart && character == '[' && index+1 < len(source) && source[index+1] == '[' {
			return 0, "compound control syntax is outside retained-function inspection"
		}
		if character == '(' {
			return 0, "parenthesized grouping is outside retained-function inspection"
		}
		if character == ')' {
			if mode == ')' {
				if pendingOperator || pendingRedirect || !sawCommand {
					return 0, "control flow is incomplete before substitution boundary"
				}
				return index, ""
			}
			return 0, "unmatched closing parenthesis in function body"
		}
		if character == '{' {
			return 0, "nested brace grouping is outside retained-function inspection"
		}
		if character == '}' {
			if mode != '}' || !wordStart || !commandStart {
				return 0, "closing brace is not at a proven function boundary"
			}
			if pendingOperator || pendingRedirect || !sawCommand {
				return 0, "control flow is incomplete before function boundary"
			}
			return index, ""
		}
		if commandStart && wordStart && character >= 'a' && character <= 'z' {
			end := index
			for end < len(source) && source[end] >= 'a' && source[end] <= 'z' {
				end++
			}
			token := string(source[index:end])
			if end == len(source) || bytes.ContainsRune([]byte(" \t\n;|&<>(){}[]"), rune(source[end])) {
				switch token {
				case "if", "then", "else", "elif", "fi", "for", "while", "until", "do", "done", "case", "esac", "in", "function", "select", "time", "coproc":
					return 0, "compound control syntax is outside retained-function inspection"
				}
			}
		}
		switch character {
		case ' ', '\t':
			wordStart = true
		case '\n', ';', '|', '&':
			if pendingRedirect {
				return 0, "redirection target is unproven"
			}
			if character == '|' || character == '&' {
				if !hasCommand {
					return 0, "command list operator has no proven left command"
				}
				if index+1 < len(source) && source[index+1] == character {
					index++
				}
				pendingOperator = true
			} else if character == ';' && (!hasCommand || pendingOperator) {
				return 0, "command list separator has no proven command"
			}
			hasCommand = false
			wordStart, commandStart = true, true
		case '<', '>':
			if pendingRedirect {
				return 0, "redirection target is unproven"
			}
			pendingRedirect = true
			if index+1 < len(source) && source[index+1] == character {
				index++
			}
			wordStart = true
		default:
			hasCommand, pendingOperator = true, false
			sawCommand, pendingRedirect = true, false
			wordStart, commandStart = false, false
		}
	}
	return 0, "function quote or substitution boundary is incomplete"
}
