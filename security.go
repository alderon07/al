package main

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
)

type SecretFinding struct {
	Line int
	Kind string
}

var secretPatterns = []struct {
	name    string
	pattern *regexp.Regexp
}{
	{"GitHub token", regexp.MustCompile(`(?:gh[pousr]_[A-Za-z0-9_]{20,}|github_pat_[A-Za-z0-9_]{20,})`)},
	{"AWS access key", regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{"private key", regexp.MustCompile(`BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY`)},
	{"credential in URL", regexp.MustCompile(`https?://[^\s/:]+:[^\s/@]+@`)},
	{"assigned secret", regexp.MustCompile(`(?i)(?:password|passwd|secret|api[_-]?key|access[_-]?token)\s*=\s*['"]?[^'$"\s][^\s;]*`)},
}

func findSecretFindings(contents []byte) []SecretFinding {
	var findings []SecretFinding
	for index, line := range bytes.Split(contents, []byte("\n")) {
		for _, candidate := range secretPatterns {
			if candidate.pattern.Match(line) {
				findings = append(findings, SecretFinding{Line: index + 1, Kind: candidate.name})
			}
		}
	}
	return findings
}

func secretFindingsError(findings []SecretFinding) error {
	if len(findings) == 0 {
		return nil
	}
	labels := make([]string, 0, len(findings))
	for _, finding := range findings {
		labels = append(labels, fmt.Sprintf("line %d: %s", finding.Line, finding.Kind))
	}
	return fmt.Errorf("push blocked because %s may contain a secret (%s); run al scan", aliasDisplayPath(), strings.Join(labels, ", "))
}

func runSecretScan() error {
	path, err := aliasesPath()
	if err != nil {
		return err
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	findings := findSecretFindings(contents)
	if len(findings) == 0 {
		fmt.Printf("No likely secrets found in %s.\n", aliasDisplayPath())
		return nil
	}
	fmt.Println("Review these lines before syncing. Secret values are hidden:")
	for _, finding := range findings {
		fmt.Printf("  line %d  %s\n", finding.Line, finding.Kind)
	}
	return fmt.Errorf("found %d possible secrets", len(findings))
}
