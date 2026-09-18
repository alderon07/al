package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigRejectsCorruptionWithoutChangingFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".config", "alias-lens", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	corrupt := []byte(`{"version":1,"repository":`)
	if err := os.WriteFile(path, corrupt, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := loadConfig(); err == nil || !strings.Contains(err.Error(), "parse") {
		t.Fatalf("corrupt config error = %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, corrupt) {
		t.Fatalf("corrupt config changed: %q, %v", after, err)
	}
	if _, err := os.Stat(path + ".alias-lens.bak"); !os.IsNotExist(err) {
		t.Fatalf("corrupt config created backup: %v", err)
	}
}

func TestLoadConfigMigratesLegacyFileWithPrivateBackup(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".config", "alias-lens", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := []byte("{\n  \"shell\": \"zsh\",\n  \"alias_file\": \".zsh_aliases\"\n}\n")
	if err := os.WriteFile(path, legacy, 0o644); err != nil {
		t.Fatal(err)
	}

	config, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Version != currentConfigVersion || config.Shell != "zsh" || config.AliasFile != ".zsh_aliases" {
		t.Fatalf("legacy configuration was not migrated: %+v", config)
	}
	contents, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(contents), `"version": 1`) {
		t.Fatalf("migrated configuration was not saved: %s, %v", contents, err)
	}
	backup, err := os.ReadFile(path + ".alias-lens.bak")
	if err != nil || string(backup) != string(legacy) {
		t.Fatalf("migration backup = %q, %v", backup, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("migrated configuration mode = %v, want 0600", info.Mode().Perm())
	}
	directoryInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if directoryInfo.Mode().Perm() != 0o700 {
		t.Fatalf("configuration directory mode = %v, want 0700", directoryInfo.Mode().Perm())
	}
}

func TestLoadConfigRejectsNewerSchemaWithoutChangingFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".config", "alias-lens", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	newer := []byte("{\n  \"version\": 2,\n  \"shell\": \"bash\"\n}\n")
	if err := os.WriteFile(path, newer, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := loadConfig()
	if err == nil || !strings.Contains(err.Error(), "newer than this Alias Lens supports") {
		t.Fatalf("newer configuration returned %v", err)
	}
	contents, readErr := os.ReadFile(path)
	if readErr != nil || string(contents) != string(newer) {
		t.Fatalf("newer configuration was changed: %q, %v", contents, readErr)
	}
	if _, statErr := os.Stat(path + ".alias-lens.bak"); !os.IsNotExist(statErr) {
		t.Fatalf("newer configuration unexpectedly created a backup: %v", statErr)
	}
}

func TestSaveConfigWritesCurrentVersionPrivately(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	config := defaultConfig()
	config.Version = 0
	if err := saveConfig(config); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".config", "alias-lens", "config.json")
	contents, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(contents), `"version": 1`) {
		t.Fatalf("saved configuration = %s, %v", contents, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("saved configuration mode = %v, want 0600", info.Mode().Perm())
	}
}
