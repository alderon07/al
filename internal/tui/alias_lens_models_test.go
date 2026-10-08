package tui

import (
	"fmt"

	"path/filepath"

	"alias-lens/internal/app"
	tea "alias-lens/internal/tea"
	"strings"
	"testing"
)

func TestWrapTextKeepsDescriptionWithinWidth(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	wrapped := wrapText("Long listing view with permissions owner size and date", 18)
	for _, line := range strings.Split(wrapped, "\n") {
		if len([]rune(line)) > 18 {
			t.Fatalf("line %q exceeds width", line)
		}
	}
}

func TestTrackedFilesViewShowsSourceDestinationAndState(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	t.Setenv("HOME", privateTestHome(t))
	applyTheme(builtInTheme("phosphor"))
	m := model{services: applicationServices(), width: 100,
		height:          36,
		trackedOnly:     true,
		trackedRepo:     "/tmp/dotfiles",
		autoSyncEnabled: true,
		syncInterval:    15,
		primarySync: trackedFileItem{
			Config: trackedFileConfig{Source: "/tmp/.bash_aliases", RepositoryPath: ".bash_aliases"},
			State:  app.SyncState{Status: "pushed", Message: "aliases match"},
		},
		tracked: []trackedFileItem{{
			Config: trackedFileConfig{Source: "/tmp/starship.toml", RepositoryPath: "shell/starship.toml"},
			State:  app.SyncState{Status: "synced", Message: "files match"},
		}},
	}
	view := m.View()
	for _, expected := range []string{"SYNC STATUS", "AUTO ON", "every 15s", "PRIMARY ALIAS FILE", "/tmp/.bash_aliases", "PUSHED", "aliases match", "EXTRA TRACKED FILES", "/tmp/starship.toml", "repo/shell/starship.toml", "SYNCED", "files match"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("tracked-files view does not contain %q:\n%s", expected, view)
		}
	}
}

func TestTrackedFilesViewExplainsEmptyRegistry(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	t.Setenv("HOME", privateTestHome(t))
	applyTheme(builtInTheme("phosphor"))
	view := (model{services: applicationServices(), width: 90, height: 24, trackedOnly: true}).View()
	if !strings.Contains(view, "SYNC STATUS") || !strings.Contains(view, "AUTO OFF") || !strings.Contains(view, "PRIMARY ALIAS FILE") || !strings.Contains(view, "None. Add one with") || !strings.Contains(view, "al track PATH") {
		t.Fatalf("empty tracked-files view is not actionable:\n%s", view)
	}
}

func TestF6LoadsPrimarySyncStatus(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv("ALIAS_LENS_SHELL", "bash")
	config := defaultConfig()
	config.Repository = filepath.Join(home, "dotfiles")
	config.AutoSync = autoSyncConfig{Enabled: true, IntervalSeconds: 30}
	if err := saveConfig(config); err != nil {
		t.Fatal(err)
	}
	if err := seedSyncStateFixture("synced", "files match", "local", "remote"); err != nil {
		t.Fatal(err)
	}

	updated, _ := (model{services: applicationServices(), width: 100, height: 30}).Update(tea.KeyMsg{Type: tea.KeyF6})
	result := updated.(model)
	if !result.trackedOnly || !result.autoSyncEnabled || result.syncInterval != 30 || result.primarySync.State.Status != "synced" {
		t.Fatalf("sync status was not loaded: %#v", result)
	}
	view := result.View()
	for _, expected := range []string{"AUTO ON", "every 30s", "SYNCED", "files match", ".bash_aliases"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("sync view is missing %q:\n%s", expected, view)
		}
	}
}

func TestEnterSelectsAliasAndQuitsTheTUI(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	m := model{services: applicationServices(), aliases: []aliasEntry{{Name: "gc", Command: "git commit"}}, query: "g"}
	updated, command := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result := updated.(model)
	if command == nil {
		t.Fatal("Enter did not request that the TUI exit")
	}
	if result.selected == nil || result.selected.Name != "gc" {
		t.Fatalf("Enter selected %#v, want gc", result.selected)
	}
}

func TestExecuteModeExplainsThatEnterRunsTheAlias(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	applyTheme(builtInTheme("phosphor"))
	m := model{services: applicationServices(), aliases: []aliasEntry{{Name: "gs", Command: "git status", Description: "Show status", Category: "git"}},
		width:       90,
		height:      24,
		executeMode: true,
	}
	view := m.View()
	for _, expected := range []string{"Choose an alias to use.", "Press Enter to use it.", "enter use"} {
		if !strings.Contains(view, expected) {
			t.Errorf("execute-mode view is missing %q:\n%s", expected, view)
		}
	}
}

func TestTallTerminalShowsMoreSuggestedAliases(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	applyTheme(builtInTheme("phosphor"))
	if count := (model{services: applicationServices(), height: 40}).visibleCount(); count != 6 {
		t.Fatalf("tall terminal pages by %d aliases, want 6", count)
	}
	aliases := make([]aliasEntry, 10)
	for index := range aliases {
		aliases[index] = aliasEntry{Name: fmt.Sprintf("a%d", index), Command: fmt.Sprintf("echo %d", index)}
	}
	if suggestions := suggestedAliases(applicationServices(),

		aliases); len(suggestions) != 10 {
		t.Fatalf("suggestion cap returned %d aliases, want 10", len(suggestions))
	}
	start, end := aliasWindow(aliases, 0, 82, 26)
	if start != 0 || end != 10 {
		t.Fatalf("tall terminal rendered aliases %d through %d, want all 10", start, end)
	}
	_, shortEnd := aliasWindow(aliases, 0, 82, 12)
	if shortEnd >= end {
		t.Fatalf("short terminal rendered %d aliases, want fewer than %d", shortEnd, end)
	}
}
