package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alderon07/al/internal/usagelog"
)

func TestDataClearCommandsRemoveOnlyRequestedData(t *testing.T) {
	home := privateTestHome(t)
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

func TestClearRevisionsKeepsCatalogRollbackArtifacts(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	roots := []string{filepath.Join(home, ".local", "share", "alias-lens", "revisions"), filepath.Join(home, ".local", "state", "alias-lens", "catalog-revisions")}
	retained := []string{filepath.Join(home, ".local", "state", "alias-lens", "rollback", "baseline.json"), filepath.Join(home, ".local", "state", "alias-lens", "catalog-snapshots", "referenced.json"), filepath.Join(home, ".local", "state", "alias-lens", "native-snapshots", "referenced.json")}
	for _, p := range append([]string{filepath.Join(roots[0], "one"), filepath.Join(roots[1], "two")}, retained...) {
		if e := os.MkdirAll(filepath.Dir(p), 0o700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(p, []byte("synthetic"), 0o600); e != nil {
			t.Fatal(e)
		}
	}
	if e := runDataCommand([]string{"clear-revisions"}); e != nil {
		t.Fatal(e)
	}
	for _, root := range roots {
		entries, e := os.ReadDir(root)
		if e != nil || len(entries) != 0 {
			t.Fatal("revision history retained", e)
		}
	}
	for _, p := range retained {
		b, e := os.ReadFile(p)
		if e != nil || string(b) != "synthetic" {
			t.Fatal("rollback artifact changed", e)
		}
	}
}
