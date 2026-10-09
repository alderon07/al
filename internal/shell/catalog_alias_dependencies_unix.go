//go:build !windows

package shell

import (
	"bytes"
	"fmt"
	"strings"
)

type catalogDependencyScanner struct {
	shell, name string
	aliases     map[string]bool
}

func rejectCatalogAliasDependencies(shell, name, text string, aliases map[string]bool, replacement bool) error {
	if err := validateSource([]byte(text)); err != nil {
		return fmt.Errorf("alias dependency inspection exceeds source bounds; simplify it before al catalog enable")
	}
	if replacement && len(text) > 0 && (text[len(text)-1] == ' ' || text[len(text)-1] == '\t') {
		return fmt.Errorf("trailing-blank alias replacement is unsupported; remove the trailing blank before al catalog enable")
	}
	wrapped := append(append([]byte{}, text...), '\n', '}')
	if end, reason := (nativeBoundaryScanner{source: wrapped}).region(0, '}', 0); reason != "" || end != len(wrapped)-1 {
		return dependencySyntaxError()
	}
	return (catalogDependencyScanner{shell: shell, name: name, aliases: aliases}).commands([]byte(text), 0, replacement)
}

func dependencySyntaxError() error {
	return fmt.Errorf("alias dependency syntax is unsupported; simplify it before al catalog enable")
}

func (scanner catalogDependencyScanner) substitution(source []byte, position, depth int) (int, error) {
	end, reason := (nativeBoundaryScanner{source: source}).expansion(position, depth)
	if reason != "" {
		return 0, dependencySyntaxError()
	}
	if end == position {
		return position, nil
	}
	if source[position+1] == '(' {
		if err := scanner.commands(source[position+2:end], depth+1, false); err != nil {
			return 0, err
		}
	} else {
		for index := position + 2; index < end; index++ {
			if source[index] == '$' {
				next, err := scanner.substitution(source, index, depth+1)
				if err != nil {
					return 0, err
				}
				index = next
			}
		}
	}
	return end, nil
}

func (scanner catalogDependencyScanner) word(source []byte, position, depth int) (int, string, bool, error) {
	start := position
	literal := true
	for position < len(source) {
		character := source[position]
		if bytes.ContainsRune([]byte(" \t\n;|&<>(){}"), rune(character)) {
			break
		}
		switch character {
		case '\\':
			if position+1 < len(source) && source[position+1] == '\n' {
				return 0, "", false, dependencySyntaxError()
			}
			literal = false
			if position+1 >= len(source) {
				return 0, "", false, dependencySyntaxError()
			}
			position += 2
		case '\'':
			literal = false
			end := bytes.IndexByte(source[position+1:], '\'')
			if end < 0 {
				return 0, "", false, dependencySyntaxError()
			}
			position += end + 2
		case '"':
			literal = false
			position++
			for position < len(source) && source[position] != '"' {
				if source[position] == '\\' {
					position += 2
					continue
				}
				if source[position] == '$' {
					end, err := scanner.substitution(source, position, depth)
					if err != nil {
						return 0, "", false, err
					}
					position = end
				}
				if source[position] == '`' {
					return 0, "", false, dependencySyntaxError()
				}
				position++
			}
			if position >= len(source) {
				return 0, "", false, dependencySyntaxError()
			}
			position++
		case '$':
			literal = false
			end, err := scanner.substitution(source, position, depth)
			if err != nil {
				return 0, "", false, err
			}
			position = end + 1
		case '`':
			return 0, "", false, dependencySyntaxError()
		default:
			position++
		}
	}
	if start == position {
		return 0, "", false, dependencySyntaxError()
	}
	return position, string(source[start:position]), literal, nil
}

func catalogAssignmentWord(word string) bool {
	prefix, _, ok := strings.Cut(word, "=")
	if !ok {
		return false
	}
	prefix = strings.TrimSuffix(prefix, "+")
	return shadowFunctionName.MatchString(prefix)
}

func (scanner catalogDependencyScanner) commands(source []byte, depth int, self bool) error {
	if depth > 64 {
		return dependencySyntaxError()
	}
	commandPosition, redirectTarget, firstCommand := true, false, true
	prefix := ""
	for position := 0; position < len(source); {
		character := source[position]
		if character == ' ' || character == '\t' {
			position++
			continue
		}
		if character == '\\' && position+1 < len(source) && source[position+1] == '\n' {
			position += 2
			continue
		}
		if character == '#' {
			for position < len(source) && source[position] != '\n' {
				position++
			}
			continue
		}
		if character == '\n' || character == ';' || character == '|' || character == '&' {
			if redirectTarget {
				return dependencySyntaxError()
			}
			if position+1 < len(source) && source[position+1] == character {
				if character == ';' {
					return dependencySyntaxError()
				}
				position++
			}
			commandPosition = true
			prefix = ""
			position++
			continue
		}
		if character == '<' || character == '>' {
			if redirectTarget {
				return dependencySyntaxError()
			}
			if position+1 < len(source) && (source[position+1] == '(' || character == '<' && source[position+1] == '<') {
				return dependencySyntaxError()
			}
			if position+1 < len(source) && source[position+1] == character {
				position++
			}
			redirectTarget = true
			position++
			continue
		}
		if character == '(' || character == ')' || character == '{' || character == '}' {
			return dependencySyntaxError()
		}
		end, word, literal, err := scanner.word(source, position, depth)
		if err != nil {
			return err
		}
		position = end
		if literal && position < len(source) && (source[position] == '<' || source[position] == '>') {
			digits := true
			for _, value := range word {
				if value < '0' || value > '9' {
					digits = false
				}
			}
			if digits {
				continue
			}
		}
		if redirectTarget {
			redirectTarget = false
			continue
		}
		if !commandPosition {
			continue
		}
		if catalogAssignmentWord(word) {
			continue
		}
		if literal && strings.Contains(word, "=") {
			return dependencySyntaxError()
		}
		if prefix != "" && literal && strings.HasPrefix(word, "-") {
			if (prefix == "command" && (word == "-p" || word == "-v" || word == "-V")) || (prefix == "exec" && (word == "-c" || word == "-l")) || word == "--" {
				continue
			}
			return dependencySyntaxError()
		}
		if literal {
			if scanner.aliases[word] && !(self && scanner.shell == "bash" && firstCommand && word == scanner.name) {
				return fmt.Errorf("%s has an ambiguous dependency on alias %s; migrate it manually before al catalog enable", scanner.name, word)
			}
			switch word {
			case "if", "then", "else", "elif", "fi", "for", "while", "until", "do", "done", "case", "esac", "in", "function", "select", "time", "coproc", "!", "[[", "eval", "source", ".":
				return dependencySyntaxError()
			}
			if scanner.shell == "zsh" {
				switch word {
				case "builtin", "command", "exec", "noglob", "nocorrect":
					firstCommand = false
					prefix = word
					continue
				}
			}
		}
		commandPosition = false
		firstCommand = false
	}
	if redirectTarget {
		return dependencySyntaxError()
	}
	return nil
}
