package main

import (
	"os"
	"strings"
	"testing"

	"alias-lens/internal/app"
	tea "alias-lens/internal/tea"
)

func TestShellIntegrationUsesConfiguredLauncher(t *testing.T) {
	config := defaultConfig()
	config.Shortcuts = map[string]string{"launcher": "Ctrl+K"}
	for _, adapter := range []app.ShellAdapter{testShellAdapter("bash"), testShellAdapter("zsh")} {
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
	t.Setenv("HOME", privateTestHome(t))
	t.Setenv("ALIAS_LENS_SHELL", "bash")
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
