package main

import (
	"strings"
	"testing"

	tea "alias-lens/internal/tea"
)

func TestShortcutEditorSavesAndResetsBindings(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	t.Setenv(activeShellEnvironment, "bash")
	m := model{helpVisible: true, width: 90, height: 28, shortcutProfile: shortcutLinux, shortcutLauncher: "Ctrl+G"}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(model)
	if !m.shortcutsOpen || m.helpVisible || !strings.Contains(m.View(), "Configure shortcuts") {
		t.Fatal("keyboard guide did not open the shortcut editor")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}, Ctrl: true})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	config, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Shortcuts["launcher"] != "ctrl+k" || !strings.Contains(m.status, "reload") {
		t.Fatalf("launcher was not saved: %#v, %q", config.Shortcuts, m.status)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDelete})
	m = updated.(model)
	config, err = loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, set := config.Shortcuts["launcher"]; set || m.shortcutLauncher != "Ctrl+G" {
		t.Fatalf("launcher reset failed: %#v", config.Shortcuts)
	}
}

func TestShortcutEditorRejectsConflictsWithoutChangingConfig(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	t.Setenv(activeShellEnvironment, "bash")
	m := model{shortcutsOpen: true, shortcutCursor: 1, width: 90, height: 28, shortcutProfile: shortcutLinux, shortcutLauncher: "Ctrl+G"}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyF2})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if !strings.Contains(m.status, "conflicts") || !m.shortcutsOpen {
		t.Fatalf("conflict was not shown: %q", m.status)
	}
	config, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Shortcuts) != 0 {
		t.Fatalf("conflict changed config: %#v", config.Shortcuts)
	}
}

func TestShortcutEditorAppliesTUIBindingImmediately(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	t.Setenv(activeShellEnvironment, "bash")
	m := model{shortcutsOpen: true, shortcutCursor: 1, width: 90, height: 28, shortcutProfile: shortcutLinux, shortcutLauncher: "Ctrl+G"}
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyEnter},
		{Type: tea.KeyRunes, Runes: []rune{'n'}, Alt: true},
		{Type: tea.KeyEnter},
	} {
		updated, _ := m.Update(key)
		m = updated.(model)
	}
	if m.shortcutCapture || !strings.Contains(m.status, "help saved") {
		t.Fatalf("editor did not save TUI binding: %q", m.status)
	}
	for _, key := range []tea.KeyMsg{{Type: tea.KeyEsc}, {Type: tea.KeyEsc}, {Type: tea.KeyRunes, Runes: []rune{'n'}, Alt: true}} {
		updated, _ := m.Update(key)
		m = updated.(model)
	}
	if !m.helpVisible {
		t.Fatal("new help key did not open the guide in the same TUI session")
	}
}

func TestShortcutEditorCanCaptureEscape(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	t.Setenv(activeShellEnvironment, "bash")
	m := model{shortcutsOpen: true, shortcutFilter: "diff.close", width: 90, height: 28, shortcutProfile: shortcutLinux, shortcutLauncher: "Ctrl+G"}
	for _, key := range []tea.KeyMsg{{Type: tea.KeyEnter}, {Type: tea.KeyEsc}, {Type: tea.KeyEnter}} {
		updated, _ := m.Update(key)
		m = updated.(model)
	}
	if m.shortcutCapture {
		t.Fatal("escape capture did not complete")
	}
	config, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.status, "diff.close saved") || config.Shortcuts["diff.close"] != "esc" {
		t.Fatalf("escape was not accepted: %q, %#v", m.status, config.Shortcuts)
	}
}

func TestShortcutEditorAppliesAliasActionImmediately(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	t.Setenv(activeShellEnvironment, "bash")
	m := model{shortcutsOpen: true, shortcutFilter: "add", width: 90, height: 28, aliasMode: aliasModeCommand, shortcutProfile: shortcutLinux, shortcutLauncher: "Ctrl+G"}
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyEnter},
		{Type: tea.KeyRunes, Runes: []rune{'n'}, Alt: true},
		{Type: tea.KeyEnter},
		{Type: tea.KeyEsc},
		{Type: tea.KeyEsc},
		{Type: tea.KeyRunes, Runes: []rune{'n'}, Alt: true},
	} {
		updated, _ := m.Update(key)
		m = updated.(model)
	}
	if !m.adding {
		t.Fatal("new Add binding did not open the form in the same TUI session")
	}
}
