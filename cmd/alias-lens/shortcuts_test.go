package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "alias-lens/cmd/alias-lens/internal/tea"
)

func TestDetectShortcutProfile(t *testing.T) {
	noEnvironment := func(string) string { return "" }
	missingFile := func(string) ([]byte, error) { return nil, errors.New("missing") }
	tests := []struct {
		name     string
		goos     string
		getenv   func(string) string
		readFile func(string) ([]byte, error)
		want     ShortcutProfile
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
			if got := detectShortcutProfile(test.goos, test.getenv, test.readFile); got != test.want {
				t.Fatalf("detectShortcutProfile(%q) = %q, want %q", test.goos, got, test.want)
			}
		})
	}
}

func TestSavedShortcutProfileWinsOverComputerDefault(t *testing.T) {
	config := defaultConfig()
	config.ShortcutProfile = "macos"
	if got := resolvedShortcutProfile(config); got != shortcutMacOS {
		t.Fatalf("resolved shortcut profile = %q, want macos", got)
	}
}

func TestShortcutProfilesUsePlatformConventions(t *testing.T) {
	tests := []struct {
		profile ShortcutProfile
		action  shortcutAction
		want    string
	}{
		{profile: shortcutWindows, action: shortcutAdd, want: "Ctrl+N"},
		{profile: shortcutWindows, action: shortcutRefresh, want: "F5 / Ctrl+R"},
		{profile: shortcutLinux, action: shortcutHelp, want: "Ctrl+? / F1 / ?"},
		{profile: shortcutLinux, action: shortcutSettings, want: "Ctrl+, / F3"},
		{profile: shortcutLinux, action: shortcutRefresh, want: "Ctrl+R / F5"},
		{profile: shortcutMacOS, action: shortcutAdd, want: "Cmd+N / Ctrl+N"},
		{profile: shortcutMacOS, action: shortcutEdit, want: "Cmd+Shift+E / Ctrl+E"},
		{profile: shortcutMacOS, action: shortcutThemes, want: "Cmd+4 / F4"},
	}
	for _, test := range tests {
		if got := shortcutLabel(test.profile, test.action); got != test.want {
			t.Errorf("%s action %d label = %q, want %q", test.profile, test.action, got, test.want)
		}
	}
}

func TestProfilesDoNotRepurposeCommonShortcuts(t *testing.T) {
	tests := []struct {
		name    string
		profile ShortcutProfile
		action  shortcutAction
		key     tea.KeyMsg
	}{
		{name: "select all", profile: shortcutLinux, action: shortcutAdd, key: tea.KeyMsg{Type: tea.KeyCtrlA}},
		{name: "find", profile: shortcutWindows, action: shortcutSync, key: tea.KeyMsg{Type: tea.KeyCtrlF}},
		{name: "replace", profile: shortcutLinux, action: shortcutHealth, key: tea.KeyMsg{Type: tea.KeyCtrlH}},
		{name: "new tab", profile: shortcutMacOS, action: shortcutThemes, key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}, Super: true}},
		{name: "hide app", profile: shortcutMacOS, action: shortcutHealth, key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}, Super: true}},
		{name: "use selection for find", profile: shortcutMacOS, action: shortcutEdit, key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}, Super: true}},
		{name: "save as", profile: shortcutMacOS, action: shortcutSync, key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}, Super: true, Shift: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if matchesShortcut(test.key, test.profile, test.action) {
				t.Fatalf("%s profile repurposed %s", test.profile, test.name)
			}
		})
	}
}

func TestShortcutCommandSavesChoicePrivately(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := runShortcutsCommand([]string{"macos"}); err != nil {
		t.Fatal(err)
	}
	config, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.ShortcutProfile != "macos" {
		t.Fatalf("shortcut profile = %q, want macos", config.ShortcutProfile)
	}
	path := filepath.Join(home, ".config", "alias-lens", "config.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestShortcutCommandRestoresAutomaticSelection(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	config := defaultConfig()
	config.ShortcutProfile = "macos"
	if err := saveConfig(config); err != nil {
		t.Fatal(err)
	}

	if err := runShortcutsCommand([]string{"auto"}); err != nil {
		t.Fatal(err)
	}
	config, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.ShortcutProfile != "" {
		t.Fatalf("shortcut profile = %q, want automatic selection", config.ShortcutProfile)
	}
	path := filepath.Join(home, ".config", "alias-lens", "config.json")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), `"shortcut_profile"`) {
		t.Fatalf("automatic selection remained pinned in config: %s", contents)
	}

	config.AutoSync.Enabled = false
	if err := saveConfig(config); err != nil {
		t.Fatal(err)
	}
	config, err = loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.ShortcutProfile != "" {
		t.Fatalf("later config write restored shortcut profile %q", config.ShortcutProfile)
	}
}

func TestShortcutTestKeepsSavedChoice(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	config := defaultConfig()
	config.ShortcutProfile = "macos"
	if err := saveConfig(config); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".config", "alias-lens", "config.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := runShortcutsCommand([]string{"test"}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("shortcut test changed config:\nbefore: %s\nafter: %s", before, after)
	}
}

func TestMacShortcutsUseCommandWithPortableFallbacks(t *testing.T) {
	tests := []struct {
		name   string
		key    tea.KeyMsg
		action shortcutAction
	}{
		{name: "command new", key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}, Super: true}, action: shortcutAdd},
		{name: "control add fallback", key: tea.KeyMsg{Type: tea.KeyCtrlN}, action: shortcutAdd},
		{name: "command sync", key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'6'}, Super: true}, action: shortcutSync},
		{name: "function sync fallback", key: tea.KeyMsg{Type: tea.KeyF6}, action: shortcutSync},
		{name: "command stats", key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}, Super: true}, action: shortcutStats},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if !matchesShortcut(test.key, shortcutMacOS, test.action) {
				t.Fatalf("%s did not match its macOS action", test.name)
			}
		})
	}
	if matchesShortcut(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}, shortcutMacOS, shortcutAdd) {
		t.Fatal("plain n was treated as a shortcut and would block search input")
	}
}

func TestEveryDisplayedShortcutComesFromAnAcceptedBinding(t *testing.T) {
	for _, profile := range []ShortcutProfile{shortcutWindows, shortcutLinux, shortcutMacOS} {
		for _, definition := range shortcutDefinitions {
			choice := shortcutChoiceForProfile(definition, profile)
			if shortcutLabel(profile, definition.action) == "" || len(choice.bindings) == 0 {
				t.Fatalf("%s action %d has no displayed or accepted key", profile, definition.action)
			}
			for _, binding := range choice.bindings {
				key := binding.key
				message := tea.KeyMsg{Type: key.typeCode, Alt: key.alt, Ctrl: key.ctrl, Meta: key.meta, Super: key.super, Shift: key.shift}
				if key.typeCode == tea.KeyRunes {
					message.Runes = []rune{key.runeCode}
				}
				if !matchesShortcut(message, profile, definition.action) {
					t.Fatalf("%s action %d rejected binding %#v", profile, definition.action, binding)
				}
			}
		}
	}
}

func TestEveryShortcutActionHasTerminalSafeBinding(t *testing.T) {
	for _, profile := range []ShortcutProfile{shortcutWindows, shortcutLinux, shortcutMacOS} {
		for _, definition := range shortcutDefinitions {
			choice := shortcutChoiceForProfile(definition, profile)
			if !slices.ContainsFunc(choice.bindings, func(binding shortcutBinding) bool { return binding.terminalSafe }) {
				t.Errorf("%s action %d has no terminal-safe binding", profile, definition.action)
			}
		}
	}
}

func TestShortcutProfilesDoNotAssignOneKeyToMultipleActions(t *testing.T) {
	for _, profile := range []ShortcutProfile{shortcutWindows, shortcutLinux, shortcutMacOS} {
		seen := make(map[shortcutKey]shortcutAction)
		for _, definition := range shortcutDefinitions {
			for _, binding := range shortcutChoiceForProfile(definition, profile).bindings {
				if action, exists := seen[binding.key]; exists {
					t.Errorf("%s binding %s is assigned to actions %d and %d", profile, binding.label, action, definition.action)
				}
				seen[binding.key] = definition.action
			}
		}
	}
}

func TestDeleteShortcutProtectsSearchText(t *testing.T) {
	aliases := []Alias{{Name: "gs", Command: "git status"}}

	updated, _ := (model{aliases: aliases, shortcutProfile: shortcutLinux}).Update(tea.KeyMsg{Type: tea.KeyDelete})
	if got := updated.(model).deleteName; got != "gs" {
		t.Fatalf("Delete on an empty search selected %q for deletion, want gs", got)
	}

	updated, _ = (model{aliases: aliases, query: "g", shortcutProfile: shortcutLinux}).Update(tea.KeyMsg{Type: tea.KeyDelete})
	result := updated.(model)
	if result.query != "" || result.deleteName != "" {
		t.Fatalf("Delete with search text produced query %q and deletion %q", result.query, result.deleteName)
	}

	updated, _ = (model{aliases: aliases, shortcutProfile: shortcutMacOS}).Update(tea.KeyMsg{Type: tea.KeyBackspace, Super: true})
	if got := updated.(model).deleteName; got != "gs" {
		t.Fatalf("Cmd+Backspace selected %q for deletion, want gs", got)
	}
}

func TestShortcutGuideUsesFriendlyMacLabels(t *testing.T) {
	rows := shortcutGuide(shortcutMacOS, false)
	var text strings.Builder
	for _, row := range rows {
		text.WriteString(row[0])
		text.WriteString(" ")
		text.WriteString(row[1])
		text.WriteString("\n")
	}
	guide := text.String()
	for _, want := range []string{"Cmd+N / Ctrl+N", "Add an alias", "Cmd+6 / F6", "saved versions"} {
		if !strings.Contains(guide, want) {
			t.Fatalf("macOS shortcut guide is missing %q:\n%s", want, guide)
		}
	}
	view := (model{width: 120, height: 36, helpVisible: true, shortcutProfile: shortcutMacOS}).View()
	for _, want := range []string{"Cmd+N / Ctrl+N", "Cmd+Backspace / Delete", "Cmd+6 / F6", "Cmd+, / F3", "Cmd+? / F1 / ? / Esc"} {
		if !strings.Contains(view, want) {
			t.Fatalf("rendered macOS keyboard guide is missing %q:\n%s", want, view)
		}
	}
}

func TestWideNavigationUsesReadableModifierNames(t *testing.T) {
	hint := pageNavigationHint(120, shortcutLinux)
	for _, want := range []string{"Ctrl+? help", "Ctrl+, settings", "Ctrl+Z versions"} {
		if !strings.Contains(hint, want) {
			t.Fatalf("wide navigation hint is missing %q: %s", want, hint)
		}
	}
}

func TestPreferredSettingsShortcutTogglesSettingsPage(t *testing.T) {
	tests := []struct {
		profile ShortcutProfile
		key     tea.KeyMsg
	}{
		{profile: shortcutLinux, key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}, Ctrl: true}},
		{profile: shortcutWindows, key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}, Ctrl: true}},
		{profile: shortcutMacOS, key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}, Super: true}},
	}
	for _, test := range tests {
		m := model{settingsOpen: true, shortcutProfile: test.profile}
		updated, _ := m.Update(test.key)
		if updated.(model).settingsOpen {
			t.Errorf("%s settings shortcut did not return to aliases", test.profile)
		}
	}
}

func TestPreferredControlPunctuationOpensPages(t *testing.T) {
	tests := []struct {
		name string
		key  tea.KeyMsg
		page tuiPage
	}{
		{name: "help", key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}, Ctrl: true, Shift: true}, page: pageHelp},
		{name: "settings", key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}, Ctrl: true}, page: pageSettings},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page, ok := pageForShortcut(test.key, shortcutLinux)
			if !ok || page != test.page {
				t.Fatalf("pageForShortcut(%s) = %d, %t, want %d, true", test.key.String(), page, ok, test.page)
			}
		})
	}
}

func TestUnassignedCommandKeysDoNotBecomeText(t *testing.T) {
	key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}, Super: true}
	browser, _ := (model{query: "git", shortcutProfile: shortcutMacOS}).Update(key)
	if got := browser.(model).query; got != "git" {
		t.Fatalf("Command+C changed search text to %q", got)
	}
	form, _ := (model{adding: true, field: 1, form: [5]string{"gs", "git"}, shortcutProfile: shortcutMacOS}).Update(key)
	if got := form.(model).form[1]; got != "git" {
		t.Fatalf("Command+C changed form text to %q", got)
	}
}

func TestPastedAndModifiedKeysDoNotTriggerActions(t *testing.T) {
	pastedQuestion := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}, Paste: true}
	updated, _ := (model{shortcutProfile: shortcutMacOS}).Update(pastedQuestion)
	result := updated.(model)
	if result.helpVisible || result.query != "?" {
		t.Fatalf("pasted question mark triggered help or was lost: %#v", result)
	}
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{'n'}, Alt: true},
		{Type: tea.KeyRunes, Runes: []rune{'n'}, Ctrl: true},
		{Type: tea.KeyRunes, Runes: []rune{'n'}, Meta: true},
	} {
		updated, _ := (model{query: "git", shortcutProfile: shortcutMacOS}).Update(key)
		if got := updated.(model).query; got != "git" {
			t.Fatalf("modified key changed search text to %q: %#v", got, key)
		}
		if matchesShortcut(key, shortcutMacOS, shortcutAdd) {
			t.Fatalf("modified key triggered add: %#v", key)
		}
	}
	form, _ := (model{adding: true, field: 1, form: [5]string{"gs", "git"}, shortcutProfile: shortcutMacOS}).Update(tea.KeyMsg{Type: tea.KeySpace, Super: true})
	if got := form.(model).form[1]; got != "git" {
		t.Fatalf("modified Space changed form text to %q", got)
	}
}

func TestSelectModeGuideOnlyShowsAvailableActions(t *testing.T) {
	rows := shortcutGuide(shortcutMacOS, true)
	for _, row := range rows {
		if strings.Contains(row[1], "stats") || strings.Contains(row[1], "theme") || strings.Contains(row[1], "sync") {
			t.Fatalf("select-mode guide advertised a blocked action: %#v", row)
		}
	}
}

func TestInvalidSettingsStopPickerBeforeAliasAccess(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".config", "alias-lens", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":2,"shell":"zsh","alias_file":".zsh_aliases","shortcut_profile":"amiga","footer":{"message":"x","icon":"none"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runAliasPicker("", false, false); err == nil || !strings.Contains(err.Error(), "shortcut style") {
		t.Fatalf("picker settings error = %v", err)
	}
	for _, name := range []string{".bash_aliases", ".zsh_aliases"} {
		if _, err := os.Stat(filepath.Join(home, name)); !os.IsNotExist(err) {
			t.Fatalf("picker touched %s before rejecting settings: %v", name, err)
		}
	}
}

func TestMacStatsDoesNotAcceptUndisclosedControlShortcut(t *testing.T) {
	m := model{statsOpen: true, shortcutProfile: shortcutMacOS}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if !updated.(model).statsOpen {
		t.Fatal("Ctrl+S closed macOS stats even though it is not in the macOS profile")
	}
}
