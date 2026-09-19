package main

import (
	"bytes"
	"fmt"
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
	if err != nil || !strings.Contains(string(contents), fmt.Sprintf(`"version": %d`, currentConfigVersion)) {
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

func TestLoadConfigMigratesVersionOneForShortcutProfiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".config", "alias-lens", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	versionOne := []byte("{\n  \"version\": 1,\n  \"alias_file\": \".bash_aliases\",\n  \"shell\": \"bash\"\n}\n")
	if err := os.WriteFile(path, versionOne, 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Version != 2 || config.ShortcutProfile != "" {
		t.Fatalf("migrated config = %#v", config)
	}
	backup, err := os.ReadFile(path + ".alias-lens.bak")
	if err != nil || !bytes.Equal(backup, versionOne) {
		t.Fatalf("version 1 backup = %q, %v", backup, err)
	}
}

func TestLoadConfigRejectsVersionOneWithVersionTwoFields(t *testing.T) {
	for _, field := range []string{`"profiles": ["work"]`, `"shortcut_profile": "macos"`, `"footer": {"message": "x", "icon": "none"}`} {
		t.Run(field, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			path := filepath.Join(home, ".config", "alias-lens", "config.json")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			contents := []byte("{\"version\":1,\"alias_file\":\".bash_aliases\",\"shell\":\"bash\"," + field + "}\n")
			if err := os.WriteFile(path, contents, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadConfig(); err == nil || !strings.Contains(err.Error(), "cannot contain") {
				t.Fatalf("version 1 field error = %v", err)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(after, contents) {
				t.Fatalf("invalid version 1 config changed: %q, %v", after, err)
			}
		})
	}
}

func TestValidateAppConfigChecksProfiles(t *testing.T) {
	tests := [][]string{
		{"Work"},
		{"work", "work"},
		{"work", "laptop"},
	}
	tooMany := make([]string, 33)
	for index := range tooMany {
		tooMany[index] = fmt.Sprintf("p%02d", index)
	}
	tests = append(tests, tooMany)
	for _, profiles := range tests {
		config := defaultConfig()
		config.Profiles = profiles
		if err := validateAppConfig(config); err == nil {
			t.Errorf("invalid profiles were accepted: %#v", profiles)
		}
	}
	config := defaultConfig()
	config.Profiles = []string{"laptop", "work"}
	if err := validateAppConfig(config); err != nil {
		t.Fatalf("valid profiles were rejected: %v", err)
	}
}

func TestLoadConfigDoesNotWriteDetectedShortcutDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".config", "alias-lens", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	contents := []byte("{\n  \"version\": 2,\n  \"alias_file\": \".bash_aliases\",\n  \"shell\": \"bash\",\n  \"footer\": {\"message\": \"Made with {icon} by Naqi\", \"icon\": \"heart\"}\n}\n")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.ShortcutProfile != "" {
		t.Fatalf("shortcut profile was persisted as %q", config.ShortcutProfile)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, contents) {
		t.Fatalf("reading detected default changed config: %q, %v", after, err)
	}
}

func TestLoadConfigRejectsNewerSchemaWithoutChangingFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".config", "alias-lens", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	newer := []byte(fmt.Sprintf("{\n  \"version\": %d,\n  \"shell\": \"bash\"\n}\n", currentConfigVersion+1))
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
	if err != nil || !strings.Contains(string(contents), fmt.Sprintf(`"version": %d`, currentConfigVersion)) {
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
