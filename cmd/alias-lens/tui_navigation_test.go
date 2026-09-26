package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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
		{"help", tea.KeyMsg{Type: tea.KeyF1}, pageHelp},
		{"stats", tea.KeyMsg{Type: tea.KeyF2}, pageStats},
		{"settings", tea.KeyMsg{Type: tea.KeyF3}, pageSettings},
		{"themes", tea.KeyMsg{Type: tea.KeyF4}, pageThemes},
		{"revisions", tea.KeyMsg{Type: tea.KeyF8}, pageRevisions},
		{"sync", tea.KeyMsg{Type: tea.KeyF6}, pageSync},
		{"health", tea.KeyMsg{Type: tea.KeyF7}, pageHealth},
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

func TestHeldPageShortcutDoesNotToggleBetweenPages(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	current := navigationTestModel(pageAliases)
	current = pressPageShortcut(t, current, tea.KeyMsg{Type: tea.KeyF2})
	if current.currentPage() != pageStats {
		t.Fatalf("first shortcut opened %v, want stats", current.currentPage())
	}
	for range 12 {
		current = pressPageShortcut(t, current, tea.KeyMsg{Type: tea.KeyF2, Repeat: true})
		current = pressPageShortcut(t, current, tea.KeyMsg{Type: tea.KeyF2})
		if current.currentPage() != pageStats {
			t.Fatal("held shortcut toggled away from stats")
		}
	}

	released, _ := current.Update(tea.KeyReleaseMsg{Key: tea.KeyMsg{Type: tea.KeyF2}})
	current = pressPageShortcut(t, released.(model), tea.KeyMsg{Type: tea.KeyF2})
	if current.currentPage() != pageAliases {
		t.Fatalf("press after release opened %v, want aliases", current.currentPage())
	}
}

func TestDifferentPageShortcutInterruptsRepeatGuard(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	current := pressPageShortcut(t, navigationTestModel(pageAliases), tea.KeyMsg{Type: tea.KeyF2})
	current = pressPageShortcut(t, current, tea.KeyMsg{Type: tea.KeyF1})
	if current.currentPage() != pageHelp {
		t.Fatalf("different shortcut opened %v, want help", current.currentPage())
	}
	current = pressPageShortcut(t, current, tea.KeyMsg{Type: tea.KeyDown})
	current = pressPageShortcut(t, current, tea.KeyMsg{Type: tea.KeyF1})
	if current.currentPage() != pageAliases {
		t.Fatalf("shortcut after another key opened %v, want aliases", current.currentPage())
	}
}

func TestLegacyPageShortcutCanToggleAfterQuietPeriod(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	current := pressPageShortcut(t, navigationTestModel(pageAliases), tea.KeyMsg{Type: tea.KeyF2})
	current.lastShortcutAt = time.Now().Add(-2 * pageShortcutRepeatWindow)
	current = pressPageShortcut(t, current, tea.KeyMsg{Type: tea.KeyF2})
	if current.currentPage() != pageAliases {
		t.Fatalf("shortcut after quiet period opened %v, want aliases", current.currentPage())
	}
}

func pressPageShortcut(t *testing.T, current model, key tea.KeyMsg) model {
	t.Helper()
	updated, _ := current.Update(key)
	return updated.(model)
}

func TestPageShortcutRestoresUnsavedThemePreview(t *testing.T) {
	original := builtInTheme("tokyo-night")
	preview := builtInTheme("vercel")
	applyTheme(preview)
	defer applyTheme(defaultTheme())

	m := model{theme: preview, themeBefore: original, themePicker: true}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyF2})
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

	confirmation := model{revisionOpen: true, diff: &terminalDiff{confirmRestore: true}}
	updated, _ = confirmation.Update(tea.KeyMsg{Type: tea.KeyF2})
	if got := updated.(model); !got.revisionOpen || got.diff == nil || !got.diff.confirmRestore || got.statsOpen {
		t.Fatalf("stats shortcut interrupted revision confirmation: %#v", got)
	}
}

func TestSelectModeKeepsPageRestrictions(t *testing.T) {
	m := model{selectMode: true}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyF2})
	if got := updated.(model); got.statsOpen {
		t.Fatalf("select mode opened stats: %#v", got)
	}
	updated, _ = updated.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if got := updated.(model); !got.helpVisible {
		t.Fatalf("select mode did not open help: %#v", got)
	}
}

func navigationTestModel(page tuiPage) model {
	m := model{width: 100, height: 30, theme: builtInTheme("tokyo-night"), settingsForm: appearanceForm(defaultAppearanceConfig(), defaultFooterConfig())}
	switch page {
	case pageHelp:
		m.helpVisible = true
	case pageStats:
		m.statsOpen = true
	case pageSettings:
		m.settingsOpen = true
		m.settingsBefore = appearanceSettings{Appearance: defaultAppearanceConfig(), Footer: defaultFooterConfig()}
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
