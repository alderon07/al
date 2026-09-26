package main

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestSyncLockHeartbeatKeepsWorkerExclusive(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	lock, err := acquireSyncLock()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		releaseSyncLock(lock)
		lock.Close()
	}()
	stale := time.Now().Add(-3 * time.Minute)
	if err := os.Chtimes(lock.Name(), stale, stale); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	errs := make(chan error, 1)
	go func() {
		defer close(done)
		maintainSyncLock(lock, stop, errs, 10*time.Millisecond)
	}()
	defer func() { close(stop); <-done }()
	deadline := time.Now().Add(time.Second)
	for {
		info, err := os.Stat(lock.Name())
		if err != nil {
			t.Fatal(err)
		}
		if time.Since(info.ModTime()) < time.Minute {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("sync lock heartbeat did not refresh the lock")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if another, err := acquireSyncLock(); !errors.Is(err, os.ErrExist) {
		if another != nil {
			another.Close()
		}
		t.Fatalf("second worker acquired a live lock: %v", err)
	}
	select {
	case err := <-errs:
		t.Fatalf("heartbeat failed: %v", err)
	default:
	}
}

func TestOldWorkerCannotRemoveReplacementSyncLock(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old, err := acquireSyncLock()
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	if err := os.Remove(old.Name()); err != nil {
		t.Fatal(err)
	}
	replacement, err := os.OpenFile(old.Name(), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := replacement.Close(); err != nil {
		t.Fatal(err)
	}
	releaseSyncLock(old)
	if _, err := os.Stat(old.Name()); err != nil {
		t.Fatalf("old worker removed replacement lock: %v", err)
	}
}
