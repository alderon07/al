package main

import (
	"os"
	"path/filepath"
	"testing"

	tea "alias-lens/cmd/alias-lens/internal/tea"
)

func TestPageShortcutsWorkFromEveryPage(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".bash_history"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	sources := []struct {
		name string
		page tuiPage
	}{
		{"aliases", pageAliases},
		{"help", pageHelp},
		{"stats", pageStats},
		{"settings", pageSettings},
		{"themes", pageThemes},
		{"revisions", pageRevisions},
		{"sync", pageSync},
		{"health", pageHealth},
	}
	destinations := []struct {
		name string
		key  tea.KeyMsg
		page tuiPage
	}{
		{"help", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}, pageHelp},
		{"stats", tea.KeyMsg{Type: tea.KeyF2}, pageStats},
		{"settings", tea.KeyMsg{Type: tea.KeyF3}, pageSettings},
		{"themes", tea.KeyMsg{Type: tea.KeyCtrlT}, pageThemes},
		{"revisions", tea.KeyMsg{Type: tea.KeyCtrlZ}, pageRevisions},
		{"sync", tea.KeyMsg{Type: tea.KeyCtrlF}, pageSync},
		{"health", tea.KeyMsg{Type: tea.KeyCtrlH}, pageHealth},
	}

	for _, source := range sources {
		for _, destination := range destinations {
			t.Run(source.name+"_to_"+destination.name, func(t *testing.T) {
				m := navigationTestModel(source.page)
				updated, _ := m.Update(destination.key)
				got := updated.(model)
				want := destination.page
				if source.page == destination.page {
					want = pageAliases
				}
				if got.currentPage() != want {
					t.Fatalf("current page = %v, want %v: %#v", got.currentPage(), want, got)
				}
			})
		}
	}
}

func TestPageShortcutRestoresUnsavedThemePreview(t *testing.T) {
	original := builtInTheme("tokyo-night")
	preview := builtInTheme("vercel")
	applyTheme(preview)
	defer applyTheme(defaultTheme())

	m := model{theme: preview, themeBefore: original, themePicker: true}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	got := updated.(model)
	if got.currentPage() != pageStats || got.theme.Preset != original.Preset {
		t.Fatalf("theme switch did not restore preview: %#v", got)
	}
}

func TestPageShortcutsDoNotInterruptEditingOrConfirmation(t *testing.T) {
	form := model{adding: true, field: 1, form: [5]string{"ll", "ls"}}
	updated, _ := form.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if got := updated.(model); !got.adding || got.helpVisible || got.form[1] != "ls?" {
		t.Fatalf("help shortcut interrupted the form: %#v", got)
	}

	confirmation := model{revisionOpen: true, revisionConfirm: true}
	updated, _ = confirmation.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if got := updated.(model); !got.revisionOpen || !got.revisionConfirm || got.statsOpen {
		t.Fatalf("stats shortcut interrupted revision confirmation: %#v", got)
	}
}

func TestSelectModeKeepsPageRestrictions(t *testing.T) {
	m := model{selectMode: true}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if got := updated.(model); got.statsOpen {
		t.Fatalf("select mode opened stats: %#v", got)
	}
	updated, _ = updated.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if got := updated.(model); !got.helpVisible {
		t.Fatalf("select mode did not open help: %#v", got)
	}
}

func navigationTestModel(page tuiPage) model {
	m := model{width: 100, height: 30, theme: builtInTheme("tokyo-night")}
	switch page {
	case pageHelp:
		m.helpVisible = true
	case pageStats:
		m.statsOpen = true
	case pageSettings:
		m.settingsOpen = true
		m.settingsBefore = defaultFooterConfig()
	case pageThemes:
		m.themePicker = true
		m.themeBefore = m.theme
	case pageRevisions:
		m.revisionOpen = true
	case pageSync:
		m.trackedOnly = true
	case pageHealth:
		m.healthOnly = true
	}
	return m
}
