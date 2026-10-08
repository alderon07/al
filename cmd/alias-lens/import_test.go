package main

import (
	"bytes"
	"os"
	"path/filepath"

	"testing"
)

func TestImportApplyWritesOnceAndKeepsMetadata(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv("ALIAS_LENS_SHELL", "bash")
	aliasPath := filepath.Join(home, ".bash_aliases")
	original := []byte("alias ll='ls -la'\n")
	if err := os.WriteFile(aliasPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	importPath := filepath.Join(home, "incoming.sh")
	imported := []byte("# Deploy staging\n# al: tags=deploy favorite=true\nalias ds='deploy staging'\n")
	if err := os.WriteFile(importPath, imported, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := runImportCommand([]string{importPath, "--apply"}); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(aliasPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"alias ll='ls -la'", "# Deploy staging", "# al: tags=deploy favorite=true", "alias ds='deploy staging'"} {
		if !bytes.Contains(contents, []byte(expected)) {
			t.Errorf("imported file missing %q:\n%s", expected, contents)
		}
	}
	if backup, err := os.ReadFile(aliasPath + ".alias-lens.bak"); err != nil || !bytes.Equal(backup, original) {
		t.Fatalf("backup = %q, %v", backup, err)
	}
	revisions, err := applicationServices().RevisionList()
	if err != nil || len(revisions) != 1 {
		t.Fatalf("revisions = %#v, %v", revisions, err)
	}
}
