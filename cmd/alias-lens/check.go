package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type checkSeverity string

const (
	checkError   checkSeverity = "ERROR"
	checkWarning checkSeverity = "WARN"
)

type aliasCheckFinding struct {
	Line     int
	Severity checkSeverity
	Message  string
}

var shellSyntaxLinePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)line\s+([0-9]+)`),
	regexp.MustCompile(`:([0-9]+):`),
}

var executableName = regexp.MustCompile(`^[A-Za-z0-9_.+-]+$`)

func runAliasCheck(arguments []string) (int, error) {
	strict := false
	if len(arguments) == 1 && arguments[0] == "--strict" {
		strict = true
	} else if len(arguments) != 0 {
		return 2, fmt.Errorf("usage: al check [--strict]")
	}
	path, err := aliasesPath()
	if err != nil {
		return 2, err
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return 2, err
	}
	adapter := activeShellAdapter()
	findings := checkAliasContents(contents)
	findings = appendNativeSyntaxFinding(findings, checkNativeShellSyntax(path, adapter.Name()))
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Line == findings[j].Line {
			return findings[i].Severity < findings[j].Severity
		}
		return findings[i].Line < findings[j].Line
	})
	errors, warnings := 0, 0
	for _, finding := range findings {
		if finding.Severity == checkError {
			errors++
		} else {
			warnings++
		}
		location := ""
		if finding.Line > 0 {
			location = fmt.Sprintf(" line %d", finding.Line)
		}
		fmt.Printf("%-5s%s  %s\n", finding.Severity, location, finding.Message)
	}
	if len(findings) == 0 {
		fmt.Printf("OK  %s is valid for %s.\n", aliasDisplayPath(), adapter.DisplayName())
		return 0, nil
	}
	fmt.Printf("Found %s and %s in %s.\n", findingCount(errors, "error"), findingCount(warnings, "warning"), aliasDisplayPath())
	if errors > 0 || strict && warnings > 0 {
		return 1, nil
	}
	return 0, nil
}

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
				findings = append(findings, aliasCheckFinding{Line: lineNumber, Severity: checkError, Message: "multiline alias definitions are not supported"})
				continue
			}
			if valueFinding := checkAliasValueSyntax(line, lineNumber); valueFinding != nil {
				findings = append(findings, *valueFinding)
				continue
			}
			name, command, ok := parseAliasDefinition(line)
			if !ok {
				findings = append(findings, aliasCheckFinding{Line: lineNumber, Severity: checkError, Message: "malformed alias definition"})
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
		return []aliasCheckFinding{{Line: lineNumber, Severity: checkError, Message: "metadata comment has no fields"}}
	}
	validKeys := map[string]bool{"tags": true, "collections": true, "platforms": true, "favorite": true, "category": true}
	seenKeys := make(map[string]bool)
	var findings []aliasCheckFinding
	for _, field := range strings.Fields(body) {
		key, value, found := strings.Cut(field, "=")
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if !found || key == "" || value == "" {
			findings = append(findings, aliasCheckFinding{Line: lineNumber, Severity: checkError, Message: "metadata fields must use key=value"})
			continue
		}
		if !validKeys[key] {
			findings = append(findings, aliasCheckFinding{Line: lineNumber, Severity: checkError, Message: fmt.Sprintf("unknown metadata field %q", key)})
			continue
		}
		if seenKeys[key] {
			findings = append(findings, aliasCheckFinding{Line: lineNumber, Severity: checkError, Message: fmt.Sprintf("metadata field %q is repeated", key)})
			continue
		}
		seenKeys[key] = true
		switch key {
		case "favorite":
			if !validBooleanMetadata(value) {
				findings = append(findings, aliasCheckFinding{Line: lineNumber, Severity: checkError, Message: "favorite must be true, false, yes, no, 1, or 0"})
			}
		case "category":
			if !aliasName.MatchString(value) {
				findings = append(findings, aliasCheckFinding{Line: lineNumber, Severity: checkError, Message: "category may only use letters, numbers, dot, dash, and underscore"})
			}
		case "tags", "collections", "platforms":
			for _, item := range strings.Split(value, ",") {
				if item == "" || !aliasName.MatchString(item) {
					findings = append(findings, aliasCheckFinding{Line: lineNumber, Severity: checkError, Message: key + " must be a comma-separated list of names"})
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
		return &aliasCheckFinding{Line: lineNumber, Severity: checkError, Message: "quoted alias command must end on the same line"}
	}
	if !strings.HasPrefix(value, "'") && !strings.HasPrefix(value, `"`) && strings.ContainsAny(value, " \t") {
		return &aliasCheckFinding{Line: lineNumber, Severity: checkError, Message: "alias commands that contain spaces must be quoted"}
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
	return []aliasCheckFinding{{Line: line, Severity: checkError, Message: fmt.Sprintf("duplicate entry %q; first defined on line %d", name, first)}}
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
		if native.Line > 0 && finding.Line == native.Line && finding.Severity == checkError {
			return findings
		}
	}
	return append(findings, *native)
}

func findingCount(count int, noun string) string {
	if count == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", count, noun)
}

func checkNativeShellSyntax(path, shell string) *aliasCheckFinding {
	if _, err := exec.LookPath(shell); err != nil {
		return &aliasCheckFinding{Severity: checkWarning, Message: shell + " is not installed; native syntax was not checked"}
	}
	arguments := []string{"-n", path}
	if shell == "bash" {
		arguments = []string{"--noprofile", "--norc", "-n", path}
	} else if shell == "zsh" {
		arguments = []string{"-f", "-n", path}
	}
	command := exec.Command(shell, arguments...)
	command.Env = shellCheckEnvironment()
	output, err := command.CombinedOutput()
	if err == nil {
		return nil
	}
	line := shellSyntaxLine(output)
	return &aliasCheckFinding{Line: line, Severity: checkError, Message: shell + " reports invalid shell syntax"}
}

func shellCheckEnvironment() []string {
	environment := make([]string, 0, len(os.Environ())+2)
	for _, variable := range os.Environ() {
		if strings.HasPrefix(variable, "BASH_ENV=") || strings.HasPrefix(variable, "ENV=") {
			continue
		}
		environment = append(environment, variable)
	}
	return append(environment, "BASH_ENV=/dev/null", "ENV=/dev/null")
}

func shellSyntaxLine(output []byte) int {
	for _, pattern := range shellSyntaxLinePatterns {
		match := pattern.FindSubmatch(output)
		if len(match) == 2 {
			line, _ := strconv.Atoi(string(match[1]))
			return line
		}
	}
	return 0
}

func aliasSyntaxDoctorCheck(path, shell string) DoctorCheck {
	contents, err := os.ReadFile(path)
	if err != nil {
		return DoctorCheck{Name: "alias syntax", OK: false, Message: "run al check"}
	}
	findings := checkAliasContents(contents)
	native := checkNativeShellSyntax(path, shell)
	if native != nil && native.Severity == checkWarning {
		return DoctorCheck{Name: "alias syntax", OK: false, Message: native.Message}
	}
	findings = appendNativeSyntaxFinding(findings, native)
	errors := 0
	for _, finding := range findings {
		if finding.Severity == checkError {
			errors++
		}
	}
	if errors > 0 {
		return DoctorCheck{Name: "alias syntax", OK: false, Message: fmt.Sprintf("%d errors; run al check", errors)}
	}
	return DoctorCheck{Name: "alias syntax", OK: true, Message: "valid for " + shell}
}
