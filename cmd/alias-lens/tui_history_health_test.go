package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRevisionDrawerRestoresSelectionAndPreservesCurrentFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	path := filepath.Join(home, ".bash_aliases")
	current := []byte("# Current\nalias gs='git status -sb'\n")
	previous := []byte("# Previous\nalias gs='git status'\n")
	if err := os.WriteFile(path, current, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := saveRevision(path, previous); err != nil {
		t.Fatal(err)
	}

	m := model{aliases: []Alias{{Name: "gs", Command: "git status -sb"}}, width: 100, height: 24}
	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlZ})
	drawer := opened.(model)
	if !drawer.revisionOpen || len(drawer.revisions) != 1 {
		t.Fatalf("Ctrl+Z did not open revision history: %#v", drawer)
	}
	if view := drawer.View(); !strings.Contains(view, "Restore an earlier alias file") || strings.Contains(view, "git status") {
		t.Fatalf("revision drawer is missing guidance or leaked alias contents:\n%s", view)
	}

	confirming, _ := drawer.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !confirming.(model).revisionConfirm {
		t.Fatal("Enter did not request restore confirmation")
	}
	restored, _ := confirming.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	result := restored.(model)
	if result.revisionOpen || !strings.Contains(result.status, "previous version saved") {
		t.Fatalf("restore did not close with a recoverability message: %#v", result)
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != string(previous) {
		t.Fatalf("revision was not restored: %q, %v", contents, err)
	}
	revisions, err := listRevisions(path)
	if err != nil || len(revisions) < 2 {
		t.Fatalf("current file was not saved before restore: %#v, %v", revisions, err)
	}
}

func TestHealthHeaderDistinguishesIssueTypes(t *testing.T) {
	aliases := []Alias{
		{Name: "missing", Issues: []string{"missing executable: docker"}},
		{Name: "duplicate", Issues: []string{"duplicate definition"}},
		{Name: "wipe", Issues: []string{"review before running"}},
	}
	if got := healthHeaderSummary(aliases); got != "1 missing, 1 broken, 1 risky ^h" {
		t.Fatalf("health summary = %q", got)
	}
	if got := healthHeaderSummary([]Alias{{Name: "gs"}}); got != "healthy" {
		t.Fatalf("healthy summary = %q", got)
	}
}
