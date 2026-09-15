package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitHubConnectSignsInAndVerifiesAuthentication(t *testing.T) {
	directory := t.TempDir()
	logPath := filepath.Join(directory, "commands.log")
	markerPath := filepath.Join(directory, "authenticated")
	ghPath := filepath.Join(directory, "gh")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$GH_TEST_LOG"
if [ "$1 $2" = "auth status" ]; then
  test -f "$GH_TEST_MARKER"
  exit $?
fi
if [ "$1 $2" = "auth login" ]; then
  : > "$GH_TEST_MARKER"
  exit 0
fi
exit 2
`
	if err := os.WriteFile(ghPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory)
	t.Setenv("GH_TEST_LOG", logPath)
	t.Setenv("GH_TEST_MARKER", markerPath)

	provider := githubProvider{host: "github.com", protocol: "auto"}
	if err := provider.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	commands := strings.Split(strings.TrimSpace(string(contents)), "\n")
	want := []string{
		"auth status --hostname github.com",
		"auth login --hostname github.com --web --git-protocol https",
		"auth status --hostname github.com",
	}
	if len(commands) != len(want) {
		t.Fatalf("commands = %#v, want %#v", commands, want)
	}
	for index := range want {
		if commands[index] != want[index] {
			t.Fatalf("command %d = %q, want %q", index, commands[index], want[index])
		}
	}
}

func TestGitHubConnectExplainsMissingCLI(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	err := (githubProvider{host: "github.com"}).Connect(context.Background())
	if err == nil || !strings.Contains(err.Error(), "GitHub CLI is required") || !strings.Contains(err.Error(), "al repo github") {
		t.Fatalf("unexpected error: %v", err)
	}
}
