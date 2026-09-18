package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestBuildImportPlanReportsDuplicatesConflictsAndSyntax(t *testing.T) {
	current := []byte("alias gs='git status'\nalias ll='ls -la'\n")
	source := []byte("alias gs='git switch'\nalias status='git status'\nalias one='echo ok'\nalias two='echo ok'\nalias broken='unterminated\n")
	plan := buildImportPlan(source, current, bashShellAdapter{})

	joined := ""
	for _, issue := range plan.Issues {
		joined += issue.Kind + ":" + issue.Message + "\n"
	}
	for _, expected := range []string{"conflict:alias \"gs\"", "duplicate command:alias \"status\"", "duplicate command:aliases \"one\" and \"two\"", "error:quoted alias command"} {
		if !strings.Contains(joined, expected) {
			t.Errorf("issues missing %q:\n%s", expected, joined)
		}
	}
}

func TestTabSelectsWithoutExecuting(t *testing.T) {
	initial := model{aliases: []Alias{{Name: "gs", Command: "git status"}}, width: 80, height: 24, executeMode: true}
	updated, command := initial.Update(tea.KeyMsg{Type: tea.KeyTab})
	selected := updated.(model)
	if selected.selected == nil || selected.selected.Name != "gs" || !selected.editSelection || command == nil {
		t.Fatalf("tab selection = %#v, command nil = %t", selected.selected, command == nil)
	}
}

func TestImportApplyWritesOnceAndKeepsMetadata(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	aliasPath := filepath.Join(home, ".bash_aliases")
	original := []byte("alias ll='ls -la'\n")
	if err := os.WriteFile(aliasPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	importPath := filepath.Join(home, "incoming.sh")
	imported := []byte("# Deploy staging\n# al: tags=deploy favorite=true\nalias ds='deploy staging'\n")
	if err := os.WriteFile(importPath, imported, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := runImportCommand([]string{importPath, "--apply"}); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(aliasPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"alias ll='ls -la'", "# Deploy staging", "# al: tags=deploy favorite=true", "alias ds='deploy staging'"} {
		if !bytes.Contains(contents, []byte(expected)) {
			t.Errorf("imported file missing %q:\n%s", expected, contents)
		}
	}
	if backup, err := os.ReadFile(aliasPath + ".alias-lens.bak"); err != nil || !bytes.Equal(backup, original) {
		t.Fatalf("backup = %q, %v", backup, err)
	}
	revisions, err := listRevisions(aliasPath)
	if err != nil || len(revisions) != 1 {
		t.Fatalf("revisions = %#v, %v", revisions, err)
	}
}
