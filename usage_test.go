package main

import (
	"strings"
	"testing"
)

func TestUsageDescribesImportantCommandEffects(t *testing.T) {
	checks := []string{
		"pick       Select an alias and print its name or command; never executes it",
		"repo       Choose or clone a Git repository and enable automatic sync",
		"sync       Copy and commit aliases locally, or explicitly push or pull",
		"setup      Back up shell files and install the Bash integration",
		`Run "al help COMMAND" or "al COMMAND --help"`,
	}
	for _, check := range checks {
		if !strings.Contains(usageText, check) {
			t.Errorf("usage is missing %q", check)
		}
	}
}

func TestEveryDocumentedCommandHasDetailedUsage(t *testing.T) {
	commands := []string{
		"pick", "use", "suggest", "meta", "scan", "history", "undo", "doctor",
		"setup", "repo", "config", "track", "untrack", "sync", "diff",
		"autosync", "watch", "shell-init", "--web", "--version",
	}
	for _, command := range commands {
		text, ok := commandUsage[command]
		if !ok {
			t.Errorf("%s has no detailed usage", command)
			continue
		}
		if !strings.HasPrefix(text, "Usage:") {
			t.Errorf("%s help does not start with Usage", command)
		}
	}
}
