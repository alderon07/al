package main

import (
	"errors"
	"os"
	"path/filepath"
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

func TestMacShortcutsUseCommandWithControlFallbacks(t *testing.T) {
	tests := []struct {
		name   string
		key    tea.KeyMsg
		action shortcutAction
	}{
		{name: "command new", key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}, Super: true}, action: shortcutAdd},
		{name: "control add fallback", key: tea.KeyMsg{Type: tea.KeyCtrlA}, action: shortcutAdd},
		{name: "command sync", key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}, Super: true, Shift: true}, action: shortcutSync},
		{name: "control sync fallback", key: tea.KeyMsg{Type: tea.KeyCtrlF}, action: shortcutSync},
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
	for _, want := range []string{"Cmd+N / Ctrl+A", "Add an alias", "Cmd+Shift+S / Ctrl+F", "saved versions"} {
		if !strings.Contains(guide, want) {
			t.Fatalf("macOS shortcut guide is missing %q:\n%s", want, guide)
		}
	}
	view := (model{width: 120, height: 30, helpVisible: true, shortcutProfile: shortcutMacOS}).View()
	for _, want := range []string{"Cmd+N / Ctrl+A", "Cmd+Shift+S / Ctrl+F", "Cmd+, / F3"} {
		if !strings.Contains(view, want) {
			t.Fatalf("rendered macOS keyboard guide is missing %q:\n%s", want, view)
		}
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
