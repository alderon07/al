package shell

import (
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type CheckSeverity string

const (
	CheckError   CheckSeverity = "ERROR"
	CheckWarning CheckSeverity = "WARN"
)

type CheckFinding struct {
	Line     int
	Severity CheckSeverity
	Message  string
}

var shellSyntaxLinePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)line\s+([0-9]+)`),
	regexp.MustCompile(`:([0-9]+):`),
}

func (bashShellAdapter) CheckSyntax(path string) *CheckFinding {
	return legacySyntaxCheckCommand("bash", []string{"--noprofile", "--norc", "-n", path})
}

func (zshShellAdapter) CheckSyntax(path string) *CheckFinding {
	return legacySyntaxCheckCommand("zsh", []string{"-f", "-n", path})
}

func legacySyntaxCheckCommand(shell string, arguments []string) *CheckFinding {
	if _, err := exec.LookPath(shell); err != nil {
		return &CheckFinding{Severity: CheckWarning, Message: shell + " is not installed; native syntax was not checked"}
	}
	command := exec.Command(shell, arguments...)
	command.Env = LegacyCheckEnvironment()
	output, err := command.CombinedOutput()
	if err == nil {
		return nil
	}
	line := SyntaxLine(output)
	return &CheckFinding{Line: line, Severity: CheckError, Message: shell + " reports invalid shell syntax"}
}

func LegacyCheckEnvironment() []string {
	environment := make([]string, 0, len(os.Environ())+2)
	for _, variable := range os.Environ() {
		if strings.HasPrefix(variable, "BASH_ENV=") || strings.HasPrefix(variable, "ENV=") {
			continue
		}
		environment = append(environment, variable)
	}
	return append(environment, "BASH_ENV=/dev/null", "ENV=/dev/null")
}

func SyntaxLine(output []byte) int {
	for _, pattern := range shellSyntaxLinePatterns {
		match := pattern.FindSubmatch(output)
		if len(match) == 2 {
			line, _ := strconv.Atoi(string(match[1]))
			return line
		}
	}
	return 0
}
