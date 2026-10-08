package app

import (
	"alias-lens/internal/shell"
	"fmt"
	"os"
	"os/exec"
	"regexp"

	"strings"
)

type checkSeverity = shell.CheckSeverity

const (
	CheckError   = shell.CheckError
	checkWarning = shell.CheckWarning
)

type aliasCheckFinding = shell.CheckFinding

var executableName = regexp.MustCompile(`^[A-Za-z0-9_.+-]+$`)

func checkAliasContents(contents []byte) []aliasCheckFinding {
	lines := strings.Split(string(contents), "\n")
	seen := make(map[string]int)
	var findings []aliasCheckFinding
	for index, line := range lines {
		lineNumber := index + 1
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(trimmed), "# al:") {
			findings = append(findings, checkMetadataSyntax(line, lineNumber)...)
			continue
		}
		if strings.HasPrefix(trimmed, "alias ") {
			if strings.HasSuffix(trimmed, "\\") {
				findings = append(findings, aliasCheckFinding{Line: lineNumber, Severity: CheckError, Message: "multiline alias definitions are not supported"})
				continue
			}
			if valueFinding := checkAliasValueSyntax(line, lineNumber); valueFinding != nil {
				findings = append(findings, *valueFinding)
				continue
			}
			name, command, ok := parseAliasDefinition(line)
			if !ok {
				findings = append(findings, aliasCheckFinding{Line: lineNumber, Severity: CheckError, Message: "malformed alias definition"})
				continue
			}
			findings = append(findings, duplicateNameFinding(seen, name, lineNumber)...)
			findings = append(findings, executableFinding(command, lineNumber)...)
			continue
		}
		if match := functionStart.FindStringSubmatch(line); match != nil {
			findings = append(findings, duplicateNameFinding(seen, match[1], lineNumber)...)
		}
	}
	for _, secret := range findSecretFindings(contents) {
		findings = append(findings, aliasCheckFinding{Line: secret.Line, Severity: checkWarning, Message: "possible " + secret.Kind + "; run al scan"})
	}
	return findings
}

func checkMetadataSyntax(line string, lineNumber int) []aliasCheckFinding {
	trimmed := strings.TrimSpace(line)
	body := strings.TrimSpace(trimmed[len("# al:"):])
	if body == "" {
		return []aliasCheckFinding{{Line: lineNumber, Severity: CheckError, Message: "metadata comment has no fields"}}
	}
	validKeys := map[string]bool{"tags": true, "collections": true, "platforms": true, "favorite": true, "category": true}
	seenKeys := make(map[string]bool)
	var findings []aliasCheckFinding
	for _, field := range strings.Fields(body) {
		key, value, found := strings.Cut(field, "=")
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if !found || key == "" || value == "" {
			findings = append(findings, aliasCheckFinding{Line: lineNumber, Severity: CheckError, Message: "metadata fields must use key=value"})
			continue
		}
		if !validKeys[key] {
			findings = append(findings, aliasCheckFinding{Line: lineNumber, Severity: CheckError, Message: fmt.Sprintf("unknown metadata field %q", key)})
			continue
		}
		if seenKeys[key] {
			findings = append(findings, aliasCheckFinding{Line: lineNumber, Severity: CheckError, Message: fmt.Sprintf("metadata field %q is repeated", key)})
			continue
		}
		seenKeys[key] = true
		switch key {
		case "favorite":
			if !validBooleanMetadata(value) {
				findings = append(findings, aliasCheckFinding{Line: lineNumber, Severity: CheckError, Message: "favorite must be true, false, yes, no, 1, or 0"})
			}
		case "category":
			if !aliasName.MatchString(value) {
				findings = append(findings, aliasCheckFinding{Line: lineNumber, Severity: CheckError, Message: "category may only use letters, numbers, dot, dash, and underscore"})
			}
		case "tags", "collections", "platforms":
			for _, item := range strings.Split(value, ",") {
				if item == "" || !aliasName.MatchString(item) {
					findings = append(findings, aliasCheckFinding{Line: lineNumber, Severity: CheckError, Message: key + " must be a comma-separated list of names"})
					break
				}
			}
		}
	}
	return findings
}

func checkAliasValueSyntax(line string, lineNumber int) *aliasCheckFinding {
	definition := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "alias "))
	_, value, found := strings.Cut(definition, "=")
	value = strings.TrimSpace(value)
	if !found || value == "" {
		return nil
	}
	if strings.HasPrefix(value, "'") && !strings.HasSuffix(value, "'") || strings.HasPrefix(value, `"`) && !strings.HasSuffix(value, `"`) {
		return &aliasCheckFinding{Line: lineNumber, Severity: CheckError, Message: "quoted alias command must end on the same line"}
	}
	if !strings.HasPrefix(value, "'") && !strings.HasPrefix(value, `"`) && strings.ContainsAny(value, " \t") {
		return &aliasCheckFinding{Line: lineNumber, Severity: CheckError, Message: "alias commands that contain spaces must be quoted"}
	}
	return nil
}

func validBooleanMetadata(value string) bool {
	switch strings.ToLower(value) {
	case "true", "false", "yes", "no", "1", "0":
		return true
	default:
		return false
	}
}

func duplicateNameFinding(seen map[string]int, name string, line int) []aliasCheckFinding {
	first, exists := seen[name]
	if !exists {
		seen[name] = line
		return nil
	}
	return []aliasCheckFinding{{Line: line, Severity: CheckError, Message: fmt.Sprintf("duplicate entry %q; first defined on line %d", name, first)}}
}

func executableFinding(command string, line int) []aliasCheckFinding {
	fields := strings.Fields(command)
	if len(fields) == 0 || !executableName.MatchString(fields[0]) || !shouldCheckExecutable(fields[0]) {
		return nil
	}
	if _, err := exec.LookPath(fields[0]); err != nil {
		return []aliasCheckFinding{{Line: line, Severity: checkWarning, Message: "alias references a missing executable"}}
	}
	return nil
}

func appendNativeSyntaxFinding(findings []aliasCheckFinding, native *aliasCheckFinding) []aliasCheckFinding {
	if native == nil {
		return findings
	}
	for _, finding := range findings {
		if native.Line > 0 && finding.Line == native.Line && finding.Severity == CheckError {
			return findings
		}
	}
	return append(findings, *native)
}

func (svc *Services) checkNativeShellSyntax(path, shell string) *aliasCheckFinding {
	adapter, err := svc.shellAdapter(shell)
	if err != nil {
		return &aliasCheckFinding{Severity: checkWarning, Message: err.Error()}
	}
	return adapter.CheckSyntax(path)
}

func shellCheckEnvironment() []string { return shell.LegacyCheckEnvironment() }

func shellSyntaxLine(output []byte) int { return shell.SyntaxLine(output) }

func (svc *Services) aliasSyntaxDoctorCheck(path, shell string) DoctorCheck {
	contents, err := os.ReadFile(path)
	if err != nil {
		return DoctorCheck{Name: "alias syntax", OK: false, Message: "run al check"}
	}
	findings := checkAliasContents(contents)
	native := svc.checkNativeShellSyntax(path, shell)
	if native != nil && native.Severity == checkWarning {
		return DoctorCheck{Name: "alias syntax", OK: false, Message: native.Message}
	}
	findings = appendNativeSyntaxFinding(findings, native)
	errors := 0
	for _, finding := range findings {
		if finding.Severity == CheckError {
			errors++
		}
	}
	if errors > 0 {
		return DoctorCheck{Name: "alias syntax", OK: false, Message: fmt.Sprintf("%d errors; run al check", errors)}
	}
	return DoctorCheck{Name: "alias syntax", OK: true, Message: "valid for " + shell}
}
