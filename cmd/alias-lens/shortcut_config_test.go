package main

import (
	"os"
	"strings"
	"testing"

	tea "alias-lens/cmd/alias-lens/internal/tea"
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
	m := model{aliases: []Alias{{Name: "a"}, {Name: "b"}}, cursor: 1, shortcutProfile: profile}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}, Alt: true})
	if updated.(model).cursor != 0 {
		t.Fatal("custom up shortcut did not move selection")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if updated.(model).cursor != 1 {
		t.Fatal("old up shortcut remained active")
	}
}

func TestShortcutOverridesRejectConflictsAndInvalidKeys(t *testing.T) {
	for _, binding := range []string{"F1", "s", "Ctrl+I", "Alt+"} {
		config := defaultConfig()
		config.Shortcuts = map[string]string{"add": binding}
		if err := validateAppConfig(config); err == nil {
			t.Errorf("accepted add=%q", binding)
		}
	}
}

func TestShellIntegrationUsesConfiguredLauncher(t *testing.T) {
	config := defaultConfig()
	config.Shortcuts = map[string]string{"launcher": "Ctrl+K"}
	for _, adapter := range []ShellAdapter{bashShellAdapter{}, zshShellAdapter{}} {
		integration, err := shellIntegrationForConfig(adapter, config)
		if err != nil {
			t.Fatal(err)
		}
		if adapter.Name() == "bash" {
			if !strings.Contains(integration, `bind '"\C-k":"\C-x\C-k\C-x\C-a"'`) {
				t.Fatal("Bash custom binding missing")
			}
		} else if !strings.Contains(integration, "bindkey '^K' _alias_lens_launch") {
			t.Fatal("Zsh custom binding missing")
		}
	}
}

func TestShortcutCLISetAndReset(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(activeShellEnvironment, "bash")
	if err := runShortcutsCommand([]string{"set", "add", "Alt+N"}); err != nil {
		t.Fatal(err)
	}
	if err := runShortcutsCommand([]string{"set", "launcher", "Ctrl+K"}); err != nil {
		t.Fatal(err)
	}
	config, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Shortcuts["add"] != "Alt+N" || config.Shortcuts["launcher"] != "Ctrl+K" {
		t.Fatalf("saved shortcuts = %#v", config.Shortcuts)
	}
	if err := runShortcutsCommand([]string{"set", "add", "F1"}); err == nil {
		t.Fatal("accepted conflicting binding")
	}
	config, err = loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Shortcuts["add"] != "Alt+N" {
		t.Fatal("failed update changed config")
	}
	if err := runShortcutsCommand([]string{"reset", "add"}); err != nil {
		t.Fatal(err)
	}
	if err := runShortcutsCommand([]string{"reset-all"}); err != nil {
		t.Fatal(err)
	}
	config, err = loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Shortcuts) != 0 {
		t.Fatalf("reset-all retained %#v", config.Shortcuts)
	}
	path, err := configPath()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %04o", info.Mode().Perm())
	}
}

func TestConfiguredConfirmationUsesOnlyNewKey(t *testing.T) {
	config := defaultConfig()
	config.Shortcuts = map[string]string{"confirm": "Alt+Y"}
	if err := validateAppConfig(config); err != nil {
		t.Fatal(err)
	}
	selected := Alias{Name: "sample", Command: "printf sample"}
	m := model{runConfirm: &selected, shortcutProfile: resolvedShortcutProfile(config)}
	old, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if old.(model).selected != nil {
		t.Fatal("old confirmation key still selected alias")
	}
	newKey, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}, Alt: true})
	if newKey.(model).selected == nil {
		t.Fatal("custom confirmation key did not select alias")
	}
}

func TestControlLetterOverrideMatchesTerminalKey(t *testing.T) {
	config := defaultConfig()
	config.Shortcuts = map[string]string{"add": "Ctrl+Y"}
	profile := resolvedShortcutProfile(config)
	if !matchesShortcut(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}, Ctrl: true}, profile, shortcutAdd) {
		t.Fatal("Ctrl+Y did not match terminal key")
	}
	config.Shortcuts["add"] = "Ctrl+R"
	if err := validateAppConfig(config); err == nil {
		t.Fatal("Ctrl+R conflict with refresh was accepted")
	}
}

func TestShortcutConflictDetectsShiftTolerantDefault(t *testing.T) {
	config := defaultConfig()
	config.Shortcuts = map[string]string{"confirm": "Shift+?"}
	if err := validateAppConfig(config); err != nil {
		t.Fatalf("confirmation key conflicts with a key used outside confirmations: %v", err)
	}
}

func TestChangingUseKeyDoesNotBreakFormEnter(t *testing.T) {
	config := defaultConfig()
	config.Shortcuts = map[string]string{"use": "Alt+U"}
	m := model{adding: true, field: 0, form: [5]string{"sample", "printf sample"}, shortcutProfile: resolvedShortcutProfile(config)}
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
	m := model{statsOpen: true, shortcutProfile: profile}
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
	confirm := model{diff: &terminalDiff{confirmRestore: true}, shortcutProfile: profile}
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
	for _, m := range []model{{statsOpen: true, shortcutProfile: profile}, {diff: &terminalDiff{}, shortcutProfile: profile}} {
		if got := translateShortcut(up, profile, m.activeTranslatedShortcuts()...); got.Type != tea.KeyUp {
			t.Fatalf("scoped Up became %v", got.Type)
		}
	}
	if got := translateShortcut(up, profile, (model{shortcutProfile: profile}).activeTranslatedShortcuts()...); got.Type != tea.KeyNull {
		t.Fatal("old Up key remained active in alias list")
	}
}

func TestDisjointViewBindingsCanShareKey(t *testing.T) {
	config := defaultConfig()
	config.Shortcuts = map[string]string{"stats.next-view": "Alt+V", "diff.next-change": "Alt+V"}
	if err := validateAppConfig(config); err != nil {
		t.Fatal(err)
	}
}
