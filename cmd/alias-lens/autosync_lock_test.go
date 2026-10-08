package main

import (
	"alias-lens/internal/transaction"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWorkerLifetimeLockNeverUsesAgeForTakeover(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	if e := withMutation(func(*mutationSession) error { return nil }); e != nil {
		t.Fatal(e)
	}
	root := filepath.Join(home, ".local", "state", "alias-lens")
	path := filepath.Join(root, "watch-worker.lock")
	first, e := transaction.AcquireLock(root, path)
	if e != nil {
		t.Fatal(e)
	}
	old := time.Now().Add(-24 * time.Hour)
	if e := os.Chtimes(path, old, old); e != nil {
		t.Fatal(e)
	}
	if second, e := transaction.AcquireLock(root, path); !errors.Is(e, transaction.ErrLocked) {
		if second != nil {
			second.Close()
		}
		t.Fatal("old active worker lock taken over", e)
	}
	if e := first.Close(); e != nil {
		t.Fatal(e)
	}
	second, e := transaction.AcquireLock(root, path)
	if e != nil {
		t.Fatal("released worker lock not reusable", e)
	}
	second.Close()
	if _, e := os.Stat(path); e != nil {
		t.Fatal("persistent worker lock deleted", e)
	}
}
func TestSyncDataPathIsObservational(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	path, e := syncDataPath("sync-state.json")
	if e != nil || path != filepath.Join(home, ".local", "state", "alias-lens", "sync-state.json") {
		t.Fatal(path, e)
	}
	entries, e := os.ReadDir(home)
	if e != nil || len(entries) != 0 {
		t.Fatal("state path lookup wrote directories", e)
	}
}
