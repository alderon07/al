package main

import (
	"os"
	"path/filepath"
	"testing"

	"alias-lens/cmd/alias-lens/internal/usagelog"
)

func TestLocalDataPathsDoesNotCreateState(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")

	paths, err := localDataPaths(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) < 10 {
		t.Fatalf("local data paths = %#v", paths)
	}
	if _, err := os.Stat(filepath.Join(home, ".config")); !os.IsNotExist(err) {
		t.Fatalf("listing paths created configuration state: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".local")); !os.IsNotExist(err) {
		t.Fatalf("listing paths created local state: %v", err)
	}
}

func TestDataClearCommandsRemoveOnlyRequestedData(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	usagePath := usagelog.Path(home)
	revisionPath := filepath.Join(home, ".local", "share", "alias-lens", "revisions", "one.bash_aliases")
	backupPath := filepath.Join(home, ".bash_aliases.alias-lens.bak")
	for path, value := range map[string]string{usagePath: "usage\n", revisionPath: "revision\n", backupPath: "backup\n"} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := runDataCommand([]string{"clear-usage"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(usagePath); !os.IsNotExist(err) {
		t.Fatalf("usage file still exists: %v", err)
	}
	if _, err := os.Stat(revisionPath); err != nil {
		t.Fatalf("usage clear removed revision: %v", err)
	}
	if err := runDataCommand([]string{"clear-revisions"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(revisionPath); !os.IsNotExist(err) {
		t.Fatalf("revision still exists: %v", err)
	}
	if contents, err := os.ReadFile(backupPath); err != nil || string(contents) != "backup\n" {
		t.Fatalf("backup changed: %q, %v", contents, err)
	}
}
