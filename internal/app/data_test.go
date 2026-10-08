package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalDataPathsDoesNotCreateState(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")

	paths, err := DefaultServices().localDataPaths(home)
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
