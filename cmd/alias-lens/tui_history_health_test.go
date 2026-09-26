package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "alias-lens/cmd/alias-lens/internal/tea"
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

	previewing, _ := drawer.Update(tea.KeyMsg{Type: tea.KeyEnter})
	preview := previewing.(model)
	if preview.diff == nil || !strings.Contains(preview.View(), "git status -sb") || !strings.Contains(preview.View(), "git status'") {
		t.Fatalf("Enter did not preview the current and selected revision:\n%s", preview.View())
	}
	if contents, err := os.ReadFile(path); err != nil || string(contents) != string(current) {
		t.Fatalf("preview changed the live file: %q, %v", contents, err)
	}
	confirming, _ := preview.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if !confirming.(model).diff.confirmRestore {
		t.Fatal("r did not request restore confirmation")
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

func TestMainHeaderKeepsOnlyCurrentFileStatus(t *testing.T) {
	applyTheme(builtInTheme("darcula"))
	view := (model{
		aliases: []Alias{
			{Name: "gs"},
			{Name: "docker", Issues: []string{"missing executable: docker"}},
		},
		width:  120,
		height: 24,
		theme:  builtInTheme("darcula"),
	}).View()
	header := ""
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "ALIAS LENS") {
			header = line
			break
		}
	}
	for _, expected := range []string{"ALIAS LENS", aliasDisplayPath(), "2 aliases", "1 issue"} {
		if !strings.Contains(header, expected) {
			t.Fatalf("header is missing %q:\n%s", expected, header)
		}
	}
	for _, clutter := range []string{"Darcula", "sync off", "^h", "loaded"} {
		if strings.Contains(header, clutter) {
			t.Fatalf("header still contains %q:\n%s", clutter, header)
		}
	}
}
