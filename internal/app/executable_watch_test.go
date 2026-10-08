package app

import (
	"os"
	"path/filepath"

	"testing"
)

func TestExecutableWatchDetectsAtomicReplacement(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "alias-lens")
	if err := os.WriteFile(path, []byte("first build"), 0o700); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	watch := ExecutableWatch{path: path, info: info}
	if watch.Changed() {
		t.Fatal("unchanged executable was reported as replaced")
	}
	replacement := filepath.Join(directory, "replacement")
	if err := os.WriteFile(replacement, []byte("second build"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	if !watch.Changed() {
		t.Fatal("replaced executable was reported as unchanged")
	}
}
