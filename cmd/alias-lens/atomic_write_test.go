package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestInterruptedAliasWriteKeepsOriginalAndBackup(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, ".bash_aliases")
	original := []byte("alias old='true'\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	previousHook := atomicWriteBeforeRename
	atomicWriteBeforeRename = func(target string) error {
		if target == path {
			return errors.New("simulated termination before rename")
		}
		return nil
	}
	t.Cleanup(func() { atomicWriteBeforeRename = previousHook })

	err := writeAliasFile(path, original, []byte("alias new='true'\n"), 0o600)
	if err == nil {
		t.Fatal("interrupted write succeeded")
	}
	contents, readErr := os.ReadFile(path)
	if readErr != nil || string(contents) != string(original) {
		t.Fatalf("live file = %q, %v", contents, readErr)
	}
	backup, backupErr := os.ReadFile(path + ".alias-lens.bak")
	if backupErr != nil || string(backup) != string(original) {
		t.Fatalf("backup = %q, %v", backup, backupErr)
	}
}

func TestEditingEmptyAliasFileSavesBackupAndRevision(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".bash_aliases")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeAliasFile(path, nil, []byte("alias gs='git status'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	backup := path + ".alias-lens.bak"
	if info, err := os.Stat(backup); err != nil || info.Size() != 0 || info.Mode().Perm() != 0o600 {
		t.Fatalf("empty backup = %v, %v", info, err)
	}
	revisions, err := listRevisions(path)
	if err != nil || len(revisions) != 1 || revisions[0].Size != 0 {
		t.Fatalf("empty revision = %v, %v", revisions, err)
	}
}

func TestCreatingAliasFileDoesNotSavePriorRevision(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".bash_aliases")
	if err := writeAliasFile(path, nil, []byte("alias gs='git status'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".alias-lens.bak"); !os.IsNotExist(err) {
		t.Fatalf("missing file gained a backup: %v", err)
	}
	revisions, err := listRevisions(path)
	if err != nil || len(revisions) != 0 {
		t.Fatalf("missing file gained a prior revision: %v, %v", revisions, err)
	}
}

func TestAliasWriteRejectsEditBeforeFinalRename(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".bash_aliases")
	original := []byte("alias old='true'\n")
	concurrent := []byte("alias newer='true'\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	previousHook := atomicWriteBeforeRename
	atomicWriteBeforeRename = func(target string) error {
		return os.WriteFile(target, concurrent, 0o600)
	}
	t.Cleanup(func() { atomicWriteBeforeRename = previousHook })
	if err := writeAliasFile(path, original, []byte("alias restored='true'\n"), 0o600); err == nil {
		t.Fatal("alias write replaced a concurrent edit")
	}
	if contents, err := os.ReadFile(path); err != nil || string(contents) != string(concurrent) {
		t.Fatalf("concurrent edit was overwritten: %q, %v", contents, err)
	}
}

func TestInterruptedTrackedReplacementKeepsOriginalAndBackup(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "settings")
	original := []byte("old\n")
	if err := os.WriteFile(path, original, 0o640); err != nil {
		t.Fatal(err)
	}
	previousHook := atomicWriteBeforeRename
	atomicWriteBeforeRename = func(target string) error {
		if target == path {
			return errors.New("simulated termination before rename")
		}
		return nil
	}
	t.Cleanup(func() { atomicWriteBeforeRename = previousHook })

	if err := replaceTrackedFile(path, []byte("new\n")); err == nil {
		t.Fatal("interrupted replacement succeeded")
	}
	contents, readErr := os.ReadFile(path)
	if readErr != nil || string(contents) != string(original) {
		t.Fatalf("live file = %q, %v", contents, readErr)
	}
	backup, backupErr := os.ReadFile(path + ".alias-lens.bak")
	if backupErr != nil || string(backup) != string(original) {
		t.Fatalf("backup = %q, %v", backup, backupErr)
	}
}
