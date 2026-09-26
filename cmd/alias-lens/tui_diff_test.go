package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "alias-lens/cmd/alias-lens/internal/tea"
)

func TestTerminalDiffShowsCommandsAndNonCommandChanges(t *testing.T) {
	old := []byte("# old\nalias gs='git status'\n")
	newer := []byte("# new\nalias gs='git status -sb'\n")
	view, err := buildTerminalDiff("Review", "Tracked", "Current", old, newer)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(view.summary, "1 changed") {
		t.Fatalf("summary = %q", view.summary)
	}
	m := model{width: 80, height: 24, diff: view}
	text := m.View()
	for _, part := range []string{"# old", "# new", "git status'", "git status -sb'", "n/p change"} {
		if !strings.Contains(text, part) {
			t.Fatalf("unified diff missing %q:\n%s", part, text)
		}
	}
	m.width = 120
	text = m.View()
	if !strings.Contains(text, "Tracked") || !strings.Contains(text, "Current") || !strings.Contains(text, "│") {
		t.Fatalf("wide diff did not use labeled split columns:\n%s", text)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if got := updated.(model); !got.diff.showAliases || !strings.Contains(got.View(), "Changed commands") {
		t.Fatalf("alias-aware view did not open:\n%s", got.View())
	}
}

func TestTerminalDiffShowsCommentsOnlyAndExactMatch(t *testing.T) {
	old := []byte("# old\nalias gs='git status'\n")
	newer := []byte("# new\nalias gs='git status'\n")
	view, err := buildTerminalDiff("Review", "Old", "New", old, newer)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(view.summary, "0 changed") || len(view.unified) == 0 {
		t.Fatalf("comment-only change vanished: %#v", view)
	}
	view.showAliases = true
	if !strings.Contains((model{width: 80, height: 24, diff: view}).View(), "No parsed commands changed") {
		t.Fatal("comment-only change did not explain empty alias view")
	}
	match, err := buildTerminalDiff("Review", "Old", "New", old, old)
	if err != nil || len(match.unified) != 0 || !strings.Contains((model{width: 80, height: 24, diff: match}).View(), "Files match exactly") {
		t.Fatalf("exact match was not clear: %#v, %v", match, err)
	}
}

func TestTerminalDiffJumpsBetweenHunks(t *testing.T) {
	old := []byte("# first\n" + strings.Repeat("# unchanged\n", 12) + "# last\n")
	newer := []byte("# changed first\n" + strings.Repeat("# unchanged\n", 12) + "# changed last\n")
	view, err := buildTerminalDiff("Review", "Old", "New", old, newer)
	if err != nil {
		t.Fatal(err)
	}
	if got := nextDiffHunk(view.unified, 0, 1); got <= 0 {
		t.Fatalf("next change = %d", got)
	}
	m := model{width: 80, height: 18, diff: view}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if updated.(model).diff.scroll == 0 {
		t.Fatal("n did not move to the next hunk")
	}
	updated, _ = updated.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if updated.(model).diff.scroll != 0 {
		t.Fatal("p did not return to the previous hunk")
	}
}

func TestTerminalDiffNavigationAndReadOnlyRepositoryPreview(t *testing.T) {
	old := []byte("# tracked\nalias gs='git status'\n")
	newer := []byte("# current\nalias gs='git status -sb'\n")
	setupRepositoryDiffTest(t, newer, old)
	_, source, target, err := repositoryPaths()
	if err != nil {
		t.Fatal(err)
	}
	m := model{width: 80, height: 24, trackedOnly: true}
	opened, _ := m.refreshTrackedFiles().Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	view := opened.(model)
	if view.diff == nil || view.diff.revisionID != "" {
		t.Fatalf("sync screen did not open read-only diff: %#v", view.diff)
	}
	updated, _ := view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if updated.(model).diff.preferSplit {
		t.Fatal("layout toggle did not select unified")
	}
	updated, _ = updated.(model).Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.(model).diff != nil || !updated.(model).trackedOnly {
		t.Fatal("Escape did not return to sync status")
	}
	if contents, err := os.ReadFile(source); err != nil || string(contents) != string(newer) {
		t.Fatalf("current file changed: %q, %v", contents, err)
	}
	if contents, err := os.ReadFile(target); err != nil || string(contents) != string(old) {
		t.Fatalf("tracked file changed: %q, %v", contents, err)
	}
}

func TestRevisionPreviewRejectsChangedLiveFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	path := filepath.Join(home, ".bash_aliases")
	current := []byte("alias gs='git status'\n")
	if err := os.WriteFile(path, current, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := saveRevision(path, []byte("alias gs='git status -sb'\n")); err != nil {
		t.Fatal(err)
	}
	m := model{width: 80, height: 24}
	m.openRevisionDrawer()
	if err := m.openSelectedRevisionDiff(); err != nil {
		t.Fatal(err)
	}
	changed := []byte("alias gs='git status --short'\n")
	if err := os.WriteFile(path, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	m.diff.confirmRestore = true
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if got := updated.(model); got.diff == nil || !strings.Contains(got.diff.err, "changed since preview") {
		t.Fatalf("stale preview was accepted: %#v", got.diff)
	}
	if contents, err := os.ReadFile(path); err != nil || string(contents) != string(changed) {
		t.Fatalf("stale restore changed live aliases: %q, %v", contents, err)
	}
}

func TestDirectTerminalDiffDoesNotCreateMissingAliasFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	t.Setenv("TERM", "xterm-256color")
	repository := filepath.Join(home, "dotfiles")
	if err := os.MkdirAll(repository, 0o700); err != nil {
		t.Fatal(err)
	}
	config := defaultConfig()
	config.Repository = repository
	if err := saveConfig(config); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, config.AliasFile), []byte("alias gs='git status'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runTUIWithDiff(true); err == nil {
		t.Fatal("direct terminal diff accepted a missing local alias file")
	}
	if _, err := os.Stat(filepath.Join(home, config.AliasFile)); !os.IsNotExist(err) {
		t.Fatalf("direct terminal diff created alias file: %v", err)
	}
}
