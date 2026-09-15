package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAliasCheckAcceptsValidAliasesFunctionsAndMetadata(t *testing.T) {
	contents := []byte(`# Show a value
# al: tags=daily,work category=utility favorite=true platforms=linux,wsl
alias show='printf value'

helper() {
  printf helper
}
`)
	if findings := checkAliasContents(contents); len(findings) != 0 {
		t.Fatalf("valid alias file returned findings: %#v", findings)
	}
}

func TestAliasCheckReportsMalformedDuplicateAndMetadataLines(t *testing.T) {
	contents := []byte(`# al: tags=git,,daily mystery=yes favorite=maybe
alias gs='printf status'
alias broken
alias gs='printf duplicate'
`)
	findings := checkAliasContents(contents)
	messages := make([]string, 0, len(findings))
	for _, finding := range findings {
		messages = append(messages, finding.Message)
	}
	joined := strings.Join(messages, "\n")
	for _, expected := range []string{"comma-separated", "unknown metadata", "favorite must", "malformed alias", "duplicate entry"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("checker did not report %q: %#v", expected, findings)
		}
	}
}

func TestAliasCheckRejectsUnquotedCommandsWithSpaces(t *testing.T) {
	findings := checkAliasContents([]byte("alias gs=git status\n"))
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "must be quoted") {
		t.Fatalf("unsafe unquoted alias was accepted: %#v", findings)
	}
}

func TestAliasCheckHidesSecretValues(t *testing.T) {
	secret := "ghp_" + "ABCDEFGHIJKLMNOPQRSTUVWXYZ123456"
	findings := checkAliasContents([]byte("alias leak='printf " + secret + "'\n"))
	if len(findings) == 0 {
		t.Fatal("secret warning was not reported")
	}
	for _, finding := range findings {
		if strings.Contains(finding.Message, secret) {
			t.Fatalf("checker exposed a secret: %q", finding.Message)
		}
	}
}

func TestNativeShellCheckReportsLineWithoutSourceText(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".bash_aliases")
	secret := "never-print-this-value"
	if err := os.WriteFile(path, []byte("alias ok='printf ok'\nalias broken='"+secret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	finding := checkNativeShellSyntax(path, "bash")
	if finding == nil || finding.Severity != checkError || strings.Contains(finding.Message, secret) {
		t.Fatalf("unsafe native syntax finding: %#v", finding)
	}
	if finding.Line == 0 {
		t.Fatalf("native syntax finding omitted the line: %#v", finding)
	}
}

func TestNativeBashCheckDoesNotLoadBashEnv(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, ".bash_aliases")
	marker := filepath.Join(directory, "startup-ran")
	startup := filepath.Join(directory, "bash-env")
	if err := os.WriteFile(path, []byte("alias ok='printf ok'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(startup, []byte("touch "+shellQuote(marker)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BASH_ENV", startup)
	if finding := checkNativeShellSyntax(path, "bash"); finding != nil {
		t.Fatalf("valid file failed native check: %#v", finding)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("native syntax check executed BASH_ENV: %v", err)
	}
}

func TestRunAliasCheckStrictFailsWarnings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	path := filepath.Join(home, ".bash_aliases")
	if err := os.WriteFile(path, []byte("alias missing='alias-lens-command-that-does-not-exist'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, err := runAliasCheck(nil); err != nil || code != 0 {
		t.Fatalf("warning failed a normal check: code=%d err=%v", code, err)
	}
	if code, err := runAliasCheck([]string{"--strict"}); err != nil || code != 1 {
		t.Fatalf("strict check accepted a warning: code=%d err=%v", code, err)
	}
}
