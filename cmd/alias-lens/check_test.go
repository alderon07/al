package main

import (
	"os"
	"path/filepath"

	"testing"
)

func TestRunAliasCheckStrictFailsWarnings(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv("ALIAS_LENS_SHELL", "bash")
	path := filepath.Join(home, ".bash_aliases")
	if err := os.WriteFile(path, []byte("alias missing='alias-lens-command-that-does-not-exist'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, err := runAliasCheck(nil); err != nil || code != 0 {
		t.Fatalf("warning failed a normal check: code=%d err=%v", code, err)
	}
	if code, err := runAliasCheck([]string{"--strict"}); err != nil || code != 1 {
		t.Fatalf("strict check accepted a warning: code=%d err=%v", code, err)
	}
}
