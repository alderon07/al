//go:build !windows

package shell

import (
	"bytes"
	"fmt"
	"strings"
)

func ExplainPreviewDiagnostics(source []byte, results []Result) {
	lines := SourceLines(source)
	for index := range results {
		result := &results[index]
		for _, diagnostic := range result.Diagnostics {
			if diagnostic.Code == "secret_detected" {
				result.Name = ""
				break
			}
		}
		for diagnosticIndex := range result.Diagnostics {
			diagnostic := &result.Diagnostics[diagnosticIndex]
			switch diagnostic.Code {
			case "unsupported_syntax", "ambiguous_syntax", "ambiguous_definition":
				start := result.StartLine - 1
				for start >= 0 && start < len(lines) && start < result.EndLine {
					text := strings.TrimSpace(string(lines[start].Bytes))
					if text != "" && (!strings.HasPrefix(text, "#") || strings.HasPrefix(text, "# al-shadow-origin:")) {
						break
					}
					start++
				}
				reason := "top_level_syntax"
				failureLine := 0
				if start >= 0 && start < len(lines) && start < result.EndLine {
					text := strings.TrimSpace(string(lines[start].Bytes))
					if strings.HasPrefix(text, "# al-shadow-origin:") {
						reason = "reserved_origin_marker"
					} else if strings.HasPrefix(text, "alias") && (len(text) == 5 || text[5] == ' ' || text[5] == '\t') {
						_, _, reason = parseShadowAliasReason(text)
					} else if shadowFunctionStart(text) {
						contents := source[lines[start].Start:]
						open := bytes.IndexByte(lines[start].Bytes, '{')
						if open < 0 {
							reason = "function_opening_brace"
						} else {
							var position int
							position, reason = matchingOuterBraceReason(contents, open)
							failureLine = start + 1 + bytes.Count(contents[:position], []byte{'\n'})
							if failureLine > len(lines) {
								failureLine = len(lines)
							}
							if reason == "" {
								reason = "function_trailing"
							}
						}
					} else if headerReason := previewFunctionHeaderReason(text); headerReason != "" {
						reason = headerReason
					} else if diagnostic.Code == "ambiguous_syntax" {
						reason = "top_level_boundary"
					}
				}
				message, next := previewExplanation(reason)
				if failureLine > 0 {
					message = fmt.Sprintf("%s Scanner stopped at line %d.", message, failureLine)
				}
				if diagnostic.Code == "ambiguous_definition" || diagnostic.Code == "ambiguous_syntax" {
					message += " The importer cannot prove a safe definition boundary, so the remaining lines are grouped and may include otherwise supported definitions."
				}
				diagnostic.Code = reason
				diagnostic.Message = message + " Next: " + next
			default:
				message, next := previewExplanation(diagnostic.Code)
				diagnostic.Message = message + " Next: " + next
			}
		}
	}
}

func previewFunctionHeaderReason(line string) string {
	quote := rune(0)
	escaped := false
	wordStart := true
	for index, character := range line {
		if escaped {
			escaped = false
			wordStart = false
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
			wordStart = false
			continue
		}
		if character == '#' && wordStart {
			line = line[:index]
			break
		}
		if character == '{' {
			line = line[:index]
			break
		}
		wordStart = character == ' ' || character == '\t' || character == ';' || character == '|' || character == '&'
	}
	fields := strings.Fields(line)
	keyword := len(fields) > 0 && fields[0] == "function"
	if keyword {
		fields = fields[1:]
	}
	if len(fields) != 1 {
		return ""
	}
	prefix := fields[0]
	parentheses := strings.HasSuffix(prefix, "()")
	if !keyword && !parentheses {
		return ""
	}
	if parentheses {
		prefix = strings.TrimSuffix(prefix, "()")
	}
	if !keyword && strings.ContainsRune(prefix, '=') {
		return ""
	}
	if !shadowFunctionName.MatchString(prefix) {
		return "function_name"
	}
	return "function_header"
}

func previewExplanation(reason string) (string, string) {
	leaveNative := "Keep this definition native, or simplify it without changing its behavior, then run al catalog preview again."
	switch reason {
	case "alias_definition", "alias_assignment":
		return "The importer requires one alias name=value assignment on a line.", leaveNative
	case "alias_name":
		return "The alias name is outside the import grammar: it must start with an ASCII letter or underscore, followed by letters, digits, underscores, dots or hyphens.", leaveNative
	case "alias_quoting":
		return "The alias value is outside the import grammar: only a complete single-quoted literal with supported escaped single quotes is accepted.", "Keep this alias native, or review expansion timing before rewriting its quoting; then run al catalog preview again."
	case "alias_trailing":
		return "Additional text follows the alias literal; only whitespace and a separated comment are accepted.", "Keep the definition native, or put additional commands on separate lines without changing their behavior; then run al catalog preview again."
	case "unsupported_metadata":
		return "The attached description or metadata is outside the import grammar. Metadata must appear once, after descriptions, with ordered tags, platforms, favorite and category fields and supported values.", "Review the attached comments and metadata format, then run al catalog preview again."
	case "reserved_origin_marker":
		return "This comment uses a reserved internal origin marker and cannot be imported as a definition.", "Keep the comment native or remove the reserved marker, then run al catalog preview again."
	case "function_name":
		return "The function name is outside the import grammar: it must start with an ASCII letter or underscore, followed by letters, digits or underscores.", leaveNative
	case "function_header":
		return "The function header is outside the import grammar: use a name followed by parentheses, or function followed by a literal space and a name without parentheses.", leaveNative
	case "function_opening_brace":
		return "The function opening brace must be on the definition's first line for import.", leaveNative
	case "function_backticks":
		return "Backtick command substitution in a function body is outside the import grammar.", leaveNative
	case "function_parameter_expansion":
		return "Braced parameter expansion in a function body is outside the import grammar.", leaveNative
	case "function_legacy_arithmetic":
		return "Legacy dollar-bracket arithmetic in a function body is outside the import grammar.", leaveNative
	case "function_dollar_single_quote":
		return "ANSI-C dollar quoting in a function body is outside the import grammar.", leaveNative
	case "function_dollar_double_quote":
		return "Localized dollar quoting in a function body is outside the import grammar.", leaveNative
	case "function_command_substitution":
		return "Command substitution in a function body is outside the import grammar.", leaveNative
	case "function_arithmetic_expression":
		return "Arithmetic containing nonnumeric or unsupported expressions in a function body is outside the import grammar.", leaveNative
	case "function_grouping":
		return "Nested braces, parentheses and process substitution in a function body are outside the import grammar.", leaveNative
	case "function_here_string":
		return "Here-strings in a function body are outside the import grammar.", leaveNative
	case "function_heredoc_delimiter":
		return "The function heredoc delimiter is outside the import grammar.", leaveNative
	case "function_heredoc_boundary":
		return "The importer cannot prove the end of the function heredoc.", leaveNative
	case "function_heredoc_continuation", "function_heredoc_quote":
		return "Heredoc quoting or continuation in the function body prevents a proven import boundary.", leaveNative
	case "function_continuation":
		return "Escaped line continuation in a function body is outside the import grammar.", leaveNative
	case "function_escape_boundary":
		return "The function body ends during an escape sequence.", leaveNative
	case "function_closing_brace":
		return "The closing brace does not appear at a supported command boundary, or a heredoc remains pending.", leaveNative
	case "function_carriage_return":
		return "Unquoted carriage returns in a function body are outside the import grammar.", leaveNative
	case "function_boundary":
		return "The importer cannot prove the function's closing boundary within its supported body grammar.", leaveNative
	case "function_trailing":
		return "Additional text follows the function closing brace; only whitespace and a separated comment are accepted.", leaveNative
	case "top_level_boundary":
		return "A top-level quote, substitution, continuation or heredoc has no boundary the importer can prove.", "Keep shell configuration native and review its boundaries before running al catalog preview again."
	case "top_level_syntax":
		return "Top-level commands and shell configuration are outside the alias and function import grammar.", "Keep shell configuration in its native file; al catalog import copies only proven equivalent definitions."
	case "secret_detected":
		return "A possible secret was detected in this source range; its value is hidden and the range is blocked.", "Remove credentials from definitions and attached comments, then run al catalog preview again."
	case "duplicate_name":
		return "More than one definition uses the same name, so the importer cannot choose safely.", "Resolve the duplicate definitions in the native file, then run al catalog preview again."
	case "invalid_candidate":
		return "The parsed definition does not satisfy the catalog schema.", leaveNative
	case "invalid_origin_marker", "reparse_failed", "comparison_failed", "round_trip_changed":
		return "Rendering and reparsing did not prove that this definition preserves its supported fields.", leaveNative
	case "native_syntax":
		return "The rendered definition failed native shell syntax validation.", "Review the native definition's syntax, then run al catalog preview again."
	case "validator_blocked":
		return "Native shell syntax validation could not complete.", "Check that the selected shell is installed and available, then run al catalog preview again."
	default:
		return "This source range could not be proven equivalent by catalog inspection.", "Review this range in the native file, then run al catalog preview again."
	}
}
