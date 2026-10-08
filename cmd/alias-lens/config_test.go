package main

import (
	"os"
	"path/filepath"

	"strings"

	"testing"
)

func TestTrackRejectsExplicitEmptyRepositoryPath(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	source := filepath.Join(home, "preferences.txt")
	if err := os.WriteFile(source, []byte("synthetic preferences\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runTrackCommand([]string{source, ""}, false); err == nil || !strings.Contains(err.Error(), "tracked repository path must name a file") {
		t.Fatalf("empty repository path error = %v", err)
	}
	path, err := configPath()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("rejected tracking wrote config: %v", err)
	}
	config, err := loadConfig()
	if err != nil || len(config.TrackedFiles) != 0 {
		t.Fatalf("rejected tracking changed registry: %#v, %v", config.TrackedFiles, err)
	}
}
