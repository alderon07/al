package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAutoSyncStatusReportsInvalidState(t *testing.T) {
	t.Run("primary", func(t *testing.T) {
		t.Setenv("HOME", privateTestHome(t))
		path, err := syncStatePathFixture("sync-state.json")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		err = runAutoSyncCommand([]string{"status"})
		if err == nil || !strings.Contains(err.Error(), "read automatic sync status") || !strings.Contains(err.Error(), path) {
			t.Fatalf("invalid primary status error = %v", err)
		}
	})

	t.Run("tracked", func(t *testing.T) {
		home := privateTestHome(t)
		t.Setenv("HOME", home)
		tracked := TrackedFileConfig{Source: filepath.Join(home, "settings.toml"), RepositoryPath: "settings.toml"}
		config := defaultConfig()
		config.TrackedFiles = []TrackedFileConfig{tracked}
		if err := saveConfig(config); err != nil {
			t.Fatal(err)
		}
		path, err := trackedStatePathFixture(tracked)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		err = runAutoSyncCommand([]string{"status"})
		if err == nil || !strings.Contains(err.Error(), "read tracked sync status") || !strings.Contains(err.Error(), path) {
			t.Fatalf("invalid tracked status error = %v", err)
		}
	})
}
