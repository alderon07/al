package tui

import (
	"testing"

	tea "alias-lens/internal/tea"
)

func TestCustomShortcutReplacesProfileBinding(t *testing.T) {
	config := defaultConfig()
	config.ShortcutProfile = "linux"
	config.Shortcuts = map[string]string{"add": "Alt+N", "up": "Alt+K"}
	if err := validateAppConfig(config); err != nil {
		t.Fatal(err)
	}
	profile := resolvedShortcutProfile(config)
	if !matchesShortcut(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}, Alt: true}, profile, shortcutAdd) {
		t.Fatal("custom add shortcut did not match")
	}
	if matchesShortcut(tea.KeyMsg{Type: tea.KeyCtrlN}, profile, shortcutAdd) {
		t.Fatal("old add shortcut remained active")
	}
	if got := shortcutLabel(profile, shortcutAdd); got != "Alt+N" {
		t.Fatalf("label = %q", got)
	}
	m := model{services: applicationServices(), aliases: []aliasEntry{{Name: "a"}, {Name: "b"}}, cursor: 1, shortcutProfile: profile}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}, Alt: true})
	if updated.(model).cursor != 0 {
		t.Fatal("custom up shortcut did not move selection")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if updated.(model).cursor != 1 {
		t.Fatal("old up shortcut remained active")
	}
}

func TestConfiguredConfirmationUsesOnlyNewKey(t *testing.T) {
	config := defaultConfig()
	config.Shortcuts = map[string]string{"confirm": "Alt+Y"}
	if err := validateAppConfig(config); err != nil {
		t.Fatal(err)
	}
	selected := aliasEntry{Name: "sample", Command: "printf sample"}
	m := model{services: applicationServices(), runConfirm: &selected, shortcutProfile: resolvedShortcutProfile(config)}
	old, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if old.(model).selected != nil {
		t.Fatal("old confirmation key still selected alias")
	}
	newKey, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}, Alt: true})
	if newKey.(model).selected == nil {
		t.Fatal("custom confirmation key did not select alias")
	}
}

func TestChangingUseKeyDoesNotBreakFormEnter(t *testing.T) {
	config := defaultConfig()
	config.Shortcuts = map[string]string{"use": "Alt+U"}
	m := model{services: applicationServices(), adding: true, field: 0, form: [5]string{"sample", "printf sample"}, shortcutProfile: resolvedShortcutProfile(config)}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if updated.(model).field != 1 {
		t.Fatal("rebound use key suppressed form Enter")
	}
}

func TestScopedStatsAndDiffShortcuts(t *testing.T) {
	config := defaultConfig()
	config.Shortcuts = map[string]string{"stats.next-view": "Alt+V", "diff.next-change": "Alt+N", "sync.diff": "Alt+D"}
	if err := validateAppConfig(config); err != nil {
		t.Fatal(err)
	}
	profile := resolvedShortcutProfile(config)
	m := model{services: applicationServices(), statsOpen: true, shortcutProfile: profile}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}, Alt: true})
	if updated.(model).statsViewIndex != 1 {
		t.Fatal("custom stats view key did not advance")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if updated.(model).statsViewIndex != 0 {
		t.Fatal("old stats view key remained active")
	}
	key := translateShortcut(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}, Alt: true}, profile, diffTranslatedShortcutActions...)
	if key.String() != "n" {
		t.Fatalf("custom diff key translated to %q", key.String())
	}
	if matchesShortcut(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}, profile, shortcutDiffNext) {
		t.Fatal("old diff key remained active")
	}
	if !matchesShortcut(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}, Alt: true}, profile, shortcutOpenDiff) {
		t.Fatal("custom sync diff key did not match")
	}
	confirm := model{services: applicationServices(), diff: &terminalDiff{services: applicationServices(), confirmRestore: true}, shortcutProfile: profile}
	canceled, _ := confirm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if canceled.(model).diff.confirmRestore {
		t.Fatal("diff next-change override blocked restore cancellation")
	}
}

func TestGlobalNavigationOverrideDoesNotDisableScopedArrows(t *testing.T) {
	config := defaultConfig()
	config.Shortcuts = map[string]string{"up": "Alt+K"}
	profile := resolvedShortcutProfile(config)
	up := tea.KeyMsg{Type: tea.KeyUp}
	for _, m := range []model{{statsOpen: true, shortcutProfile: profile}, {diff: &terminalDiff{services: applicationServices()}, shortcutProfile: profile}} {
		if got := translateShortcut(up, profile, m.activeTranslatedShortcuts()...); got.Type != tea.KeyUp {
			t.Fatalf("scoped Up became %v", got.Type)
		}
	}
	if got := translateShortcut(up, profile, (model{services: applicationServices(), shortcutProfile: profile}).activeTranslatedShortcuts()...); got.Type != tea.KeyNull {
		t.Fatal("old Up key remained active in alias list")
	}
}
