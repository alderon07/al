package shortcuts

import (
	tea "alias-lens/internal/tea"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestDetectShortcutProfile(t *testing.T) {
	noEnvironment := func(string) string { return "" }
	missingFile := func(string) ([]byte, error) { return nil, errors.New("missing") }
	tests := []struct {
		name     string
		goos     string
		getenv   func(string) string
		readFile func(string) ([]byte, error)
		want     Profile
	}{
		{name: "windows", goos: "windows", getenv: noEnvironment, readFile: missingFile, want: shortcutWindows},
		{name: "macos", goos: "darwin", getenv: noEnvironment, readFile: missingFile, want: shortcutMacOS},
		{name: "linux", goos: "linux", getenv: noEnvironment, readFile: missingFile, want: shortcutLinux},
		{name: "wsl environment", goos: "linux", getenv: func(name string) string {
			if name == "WSL_DISTRO_NAME" {
				return "Ubuntu"
			}
			return ""
		}, readFile: missingFile, want: shortcutWindows},
		{name: "wsl kernel", goos: "linux", getenv: noEnvironment, readFile: func(path string) ([]byte, error) {
			if path == "/proc/sys/kernel/osrelease" {
				return []byte("6.6.0-microsoft-standard-WSL2"), nil
			}
			return nil, errors.New("missing")
		}, want: shortcutWindows},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := detectProfile(test.goos, test.getenv, test.readFile); got != test.want {
				t.Fatalf("detectShortcutProfile(%q) = %q, want %q", test.goos, got, test.want)
			}
		})
	}
}

func TestShortcutProfilesUsePlatformConventions(t *testing.T) {
	tests := []struct {
		profile Profile
		action  Action
		want    string
	}{
		{profile: shortcutWindows, action: Add, want: "a / Ctrl+N"},
		{profile: shortcutWindows, action: Refresh, want: "r / F5 / Ctrl+R"},
		{profile: shortcutLinux, action: Help, want: "h / F1 / Ctrl+? / ?"},
		{profile: shortcutLinux, action: Settings, want: "o / F3 / Ctrl+,"},
		{profile: shortcutLinux, action: Refresh, want: "r / Ctrl+R / F5"},
		{profile: shortcutMacOS, action: Add, want: "a / Cmd+N / Ctrl+N"},
		{profile: shortcutMacOS, action: Edit, want: "e / Cmd+Shift+E / Ctrl+E"},
		{profile: shortcutMacOS, action: Themes, want: "t / Cmd+4 / F4"},
	}
	for _, test := range tests {
		if got := Label(test.profile, test.action); got != test.want {
			t.Errorf("%s action %d label = %q, want %q", test.profile, test.action, got, test.want)
		}
	}
}

func TestProfilesDoNotRepurposeCommonShortcuts(t *testing.T) {
	tests := []struct {
		name    string
		profile Profile
		action  Action
		key     tea.KeyMsg
	}{
		{name: "select all", profile: shortcutLinux, action: Add, key: tea.KeyMsg{Type: tea.KeyCtrlA}},
		{name: "find", profile: shortcutWindows, action: Sync, key: tea.KeyMsg{Type: tea.KeyCtrlF}},
		{name: "replace", profile: shortcutLinux, action: Health, key: tea.KeyMsg{Type: tea.KeyCtrlH}},
		{name: "new tab", profile: shortcutMacOS, action: Themes, key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}, Super: true}},
		{name: "hide app", profile: shortcutMacOS, action: Health, key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}, Super: true}},
		{name: "use selection for find", profile: shortcutMacOS, action: Edit, key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}, Super: true}},
		{name: "save as", profile: shortcutMacOS, action: Sync, key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}, Super: true, Shift: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if Matches(test.key, test.profile, test.action) {
				t.Fatalf("%s profile repurposed %s", test.profile, test.name)
			}
		})
	}
}

func TestMacShortcutsUseCommandWithPortableFallbacks(t *testing.T) {
	tests := []struct {
		name   string
		key    tea.KeyMsg
		action Action
	}{
		{name: "command new", key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}, Super: true}, action: Add},
		{name: "control add fallback", key: tea.KeyMsg{Type: tea.KeyCtrlN}, action: Add},
		{name: "command sync", key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'6'}, Super: true}, action: Sync},
		{name: "function sync fallback", key: tea.KeyMsg{Type: tea.KeyF6}, action: Sync},
		{name: "command stats", key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}, Super: true}, action: Stats},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if !Matches(test.key, shortcutMacOS, test.action) {
				t.Fatalf("%s did not match its macOS action", test.name)
			}
		})
	}
	if Matches(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}, shortcutMacOS, Add) {
		t.Fatal("plain n was treated as a shortcut and would block search input")
	}
}

func TestEveryDisplayedShortcutComesFromAnAcceptedBinding(t *testing.T) {
	for _, profile := range []Profile{shortcutWindows, shortcutLinux, shortcutMacOS} {
		for _, definition := range shortcutDefinitions {
			choice := shortcutChoiceForProfile(definition, profile)
			if Label(profile, definition.action) == "" || len(choice.bindings) == 0 {
				t.Fatalf("%s action %d has no displayed or accepted key", profile, definition.action)
			}
			for _, binding := range choice.bindings {
				key := binding.key
				message := tea.KeyMsg{Type: key.typeCode, Alt: key.alt, Ctrl: key.ctrl, Meta: key.meta, Super: key.super, Shift: key.shift}
				if key.typeCode == tea.KeyRunes {
					message.Runes = []rune{key.runeCode}
				}
				if !Matches(message, profile, definition.action) {
					t.Fatalf("%s action %d rejected binding %#v", profile, definition.action, binding)
				}
			}
		}
	}
}

func TestEveryShortcutActionHasTerminalSafeBinding(t *testing.T) {
	for _, profile := range []Profile{shortcutWindows, shortcutLinux, shortcutMacOS} {
		for _, definition := range shortcutDefinitions {
			choice := shortcutChoiceForProfile(definition, profile)
			if !slices.ContainsFunc(choice.bindings, func(binding shortcutBinding) bool { return binding.terminalSafe }) {
				t.Errorf("%s action %d has no terminal-safe binding", profile, definition.action)
			}
		}
	}
}

func TestShortcutProfilesDoNotAssignOneKeyToMultipleActions(t *testing.T) {
	for _, profile := range []Profile{shortcutWindows, shortcutLinux, shortcutMacOS} {
		seen := make(map[string]map[shortcutKey]Action)
		for _, definition := range shortcutDefinitions {
			if seen[definition.scope] == nil {
				seen[definition.scope] = make(map[shortcutKey]Action)
			}
			for _, binding := range shortcutChoiceForProfile(definition, profile).bindings {
				if action, exists := seen[definition.scope][binding.key]; exists {
					t.Errorf("%s binding %s is assigned to actions %d and %d", profile, binding.label, action, definition.action)
				}
				seen[definition.scope][binding.key] = definition.action
			}
		}
	}
}

func TestSelectModeGuideOnlyShowsAvailableActions(t *testing.T) {
	rows := Guide(shortcutMacOS, true)
	for _, row := range rows {
		if strings.Contains(row[1], "stats") || strings.Contains(row[1], "theme") || strings.Contains(row[1], "sync") {
			t.Fatalf("select-mode guide advertised a blocked action: %#v", row)
		}
	}
}
