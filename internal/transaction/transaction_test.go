//go:build !windows

package transaction

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func privateTestRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{"transactions", "temporary", "backups"} {
		if err := os.Mkdir(filepath.Join(root, directory), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestLockIsKernelBackedAndNeverStolen(t *testing.T) {
	root := privateTestRoot(t)
	path := filepath.Join(root, "mutation.lock")
	first, err := AcquireLock(root, path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if err := os.Chtimes(path, testOldTime(), testOldTime()); err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireLock(root, path); !errors.Is(err, ErrLocked) {
		t.Fatalf("second lock = %v, want ErrLocked", err)
	}
}

func TestReadJournalBindsManifestToRequestedPrivateRoot(t *testing.T) {
	firstRoot := privateTestRoot(t)
	secondRoot := privateTestRoot(t)
	target := filepath.Join(firstRoot, "config.json")
	planned := []byte("new\n")
	manifest := testManifest(firstRoot, target, FileIdentity{Exists: false}, planned)
	firstPath := filepath.Join(firstRoot, "transactions", "root.journal")
	journal, err := CreateJournal(firstRoot, firstPath, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := journal.Close(); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatal(err)
	}
	secondPath := filepath.Join(secondRoot, "transactions", "root.journal")
	if err := os.WriteFile(secondPath, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := ReadJournal(secondRoot, secondPath); !errors.Is(err, ErrJournalCorrupt) {
		t.Fatalf("ReadJournal accepted a manifest for another root: %v", err)
	}
}

func TestReplacePrivateJournalsBacksUpAndFinalizes(t *testing.T) {
	root := privateTestRoot(t)
	target := filepath.Join(root, "config.json")
	oldBytes := []byte("old\n")
	newBytes := []byte("new\n")
	if err := os.WriteFile(target, oldBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	expected, err := InspectPrivateFile(root, target, MaxPrivateFileSize)
	if err != nil {
		t.Fatal(err)
	}
	manifest := testManifest(root, target, expected, newBytes)
	journalPath := filepath.Join(root, "transactions", "replace.journal")
	journal, err := CreateJournal(root, journalPath, manifest)
	if err != nil {
		t.Fatal(err)
	}
	defer journal.Close()
	if err := journal.ReplacePrivate(1, newBytes); err != nil {
		t.Fatal(err)
	}
	if err := journal.Commit(); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != string(newBytes) {
		t.Fatalf("target = %q, %v", got, err)
	}
	if got, err := os.ReadFile(manifest.Actions[0].BackupPath); err != nil || string(got) != string(oldBytes) {
		t.Fatalf("backup = %q, %v", got, err)
	}
	if err := journal.Finalize(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(journalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("journal remains after finalize: %v", err)
	}
}

func TestRemovePrivateIsRecoverableAndCommitsAbsentTarget(t *testing.T) {
	root := privateTestRoot(t)
	target := filepath.Join(root, "completion.bash")
	original := []byte("complete code\n")
	if err := os.WriteFile(target, original, 0o600); err != nil {
		t.Fatal(err)
	}
	expected, err := InspectPrivateFile(root, target, MaxPrivateFileSize)
	if err != nil {
		t.Fatal(err)
	}
	manifest := testManifest(root, target, expected, nil)
	manifest.Actions[0].Remove = true
	journal, err := CreateJournal(root, filepath.Join(root, "transactions", "remove.journal"), manifest)
	if err != nil {
		t.Fatal(err)
	}
	defer journal.Close()
	if err := journal.RemovePrivate(1); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("target still exists after removal: %v", err)
	}
	if err := journal.Rollback(); err != nil {
		t.Fatal(err)
	}
	if restored, err := os.ReadFile(target); err != nil || string(restored) != string(original) {
		t.Fatalf("rollback restored %q, %v", restored, err)
	}
}

func TestReplacePrivateRejectsChangedInput(t *testing.T) {
	root := privateTestRoot(t)
	target := filepath.Join(root, "config.json")
	if err := os.WriteFile(target, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	expected, err := InspectPrivateFile(root, target, MaxPrivateFileSize)
	if err != nil {
		t.Fatal(err)
	}
	manifest := testManifest(root, target, expected, []byte("new\n"))
	journal, err := CreateJournal(root, filepath.Join(root, "transactions", "stale.journal"), manifest)
	if err != nil {
		t.Fatal(err)
	}
	defer journal.Close()
	if err := os.WriteFile(target, []byte("changed elsewhere\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := journal.ReplacePrivate(1, []byte("new\n")); !errors.Is(err, ErrIdentityChanged) {
		t.Fatalf("ReplacePrivate = %v, want ErrIdentityChanged", err)
	}
	if got, _ := os.ReadFile(target); string(got) != "changed elsewhere\n" {
		t.Fatalf("stale target was overwritten: %q", got)
	}
}

func TestReadJournalIgnoresOnlyTruncatedFinalFrame(t *testing.T) {
	root := privateTestRoot(t)
	target := filepath.Join(root, "new.json")
	manifest := testManifest(root, target, FileIdentity{}, []byte("new\n"))
	path := filepath.Join(root, "transactions", "truncated.journal")
	journal, err := CreateJournal(root, path, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := journal.Close(); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte{0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	_, _, truncated, err := ReadJournal(root, path)
	if err != nil || !truncated {
		t.Fatalf("ReadJournal = truncated %v, err %v", truncated, err)
	}
}

func TestRollbackRestoresOldBytesAfterACompletedReplacement(t *testing.T) {
	root := privateTestRoot(t)
	target := filepath.Join(root, "config.json")
	if err := os.WriteFile(target, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	expected, err := InspectPrivateFile(root, target, MaxPrivateFileSize)
	if err != nil {
		t.Fatal(err)
	}
	manifest := testManifest(root, target, expected, []byte("new\n"))
	journal, err := CreateJournal(root, filepath.Join(root, "transactions", "rollback.journal"), manifest)
	if err != nil {
		t.Fatal(err)
	}
	defer journal.Close()
	if err := journal.ReplacePrivate(1, []byte("new\n")); err != nil {
		t.Fatal(err)
	}
	if err := journal.Rollback(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(target); string(got) != "old\n" {
		t.Fatalf("rollback target = %q", got)
	}
}

func TestRecoverJournalRollsBackUncommittedNewState(t *testing.T) {
	root := privateTestRoot(t)
	target := filepath.Join(root, "config.json")
	if err := os.WriteFile(target, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	expected, _ := InspectPrivateFile(root, target, MaxPrivateFileSize)
	manifest := testManifest(root, target, expected, []byte("new\n"))
	path := filepath.Join(root, "transactions", "recover.journal")
	journal, err := CreateJournal(root, path, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := journal.ReplacePrivate(1, []byte("new\n")); err != nil {
		t.Fatal(err)
	}
	if err := journal.Close(); err != nil {
		t.Fatal(err)
	}
	if err := RecoverJournal(root, path); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(target); string(got) != "old\n" {
		t.Fatalf("recovered target = %q", got)
	}
}

func testManifest(root, target string, expected FileIdentity, planned []byte) Manifest {
	return Manifest{SchemaVersion: SchemaVersion, OperationID: "test-operation", PrivateRoot: root, Actions: []Action{{
		Sequence: 1, TargetPath: target,
		TemporaryPath: filepath.Join(root, "temporary", "test.tmp"),
		BackupPath: func() string {
			if expected.Exists {
				return filepath.Join(root, "backups", "test.bak")
			}
			return ""
		}(),
		Expected: expected, PlannedSHA256: hashBytes(planned), Mode: 0o600,
	}}}
}
