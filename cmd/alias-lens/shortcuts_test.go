package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSavedShortcutProfileWinsOverComputerDefault(t *testing.T) {
	config := defaultConfig()
	config.ShortcutProfile = "macos"
	if got := resolvedShortcutProfile(config); got != shortcutMacOS {
		t.Fatalf("resolved shortcut profile = %q, want macos", got)
	}
}

func TestShortcutCommandSavesChoicePrivately(t *testing.T) {
	home := privateTestHome(t)
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
	home := privateTestHome(t)
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
	home := privateTestHome(t)
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
