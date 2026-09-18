package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositoryDiffClearsStaleConflictForExactMatch(t *testing.T) {
	local := []byte("# Status\nalias gs='git status'\n")
	setupRepositoryDiffTest(t, local, local)
	if err := writeSyncStatus("conflict", "both local and remote aliases changed; run al diff", "old-local", "old-remote"); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := showRepositoryDiffTo(&output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "files match") {
		t.Fatalf("diff output = %q", output.String())
	}
	state, err := loadSyncState()
	if err != nil {
		t.Fatal(err)
	}
	wantHash := contentHash(local)
	if state.Status != "synced" || state.LocalHash != wantHash || state.RemoteHash != wantHash || state.Message != "files match" {
		t.Fatalf("sync state was not refreshed: %#v", state)
	}
}

func TestRepositoryDiffReportsWholeFileDifferences(t *testing.T) {
	local := []byte("# Local description\nalias gs='git status'\n")
	remote := []byte("# Tracked description\nalias gs='git status'\n")
	setupRepositoryDiffTest(t, local, remote)
	if err := writeSyncStatus("conflict", "both local and remote aliases changed; run al diff", contentHash(local), contentHash(remote)); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := showRepositoryDiffTo(&output); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"FILES DIFFER", "comments, metadata, ordering, whitespace, or unparsed syntax differ", "Run al sync to keep the local file"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("diff output does not contain %q: %s", expected, output.String())
		}
	}
	state, err := loadSyncState()
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != "conflict" {
		t.Fatalf("non-identical files cleared conflict state: %#v", state)
	}
}

func setupRepositoryDiffTest(t *testing.T, local, remote []byte) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	repository := filepath.Join(home, "dotfiles")
	if err := os.MkdirAll(repository, 0o700); err != nil {
		t.Fatal(err)
	}
	config := defaultConfig()
	config.Repository = repository
	if err := saveConfig(config); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, config.AliasFile), local, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, config.AliasFile), remote, 0o600); err != nil {
		t.Fatal(err)
	}
}
