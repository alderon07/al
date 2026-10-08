package main

import (
	"os"
	"path/filepath"

	"testing"
)

func privateTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if e := os.Chmod(home, 0o700); e != nil {
		t.Fatal(e)
	}
	return home
}

func TestRevisionClearRefusesLinks(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	outside := filepath.Join(home, "outside")
	os.WriteFile(outside, []byte("retained"), 0o600)
	directory := filepath.Join(home, ".local", "share", "alias-lens", "revisions")
	os.MkdirAll(directory, 0o700)
	os.Symlink(outside, filepath.Join(directory, "revision"))
	if e := runDataCommand([]string{"clear-revisions"}); e == nil {
		t.Fatal("revision symlink accepted")
	}
	if b, e := os.ReadFile(outside); e != nil || string(b) != "retained" {
		t.Fatal("revision clear changed external file")
	}
}
