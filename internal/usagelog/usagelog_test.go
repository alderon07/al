package usagelog

import (
	"alias-lens/internal/transaction"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecordAndLoad(t *testing.T) {
	home := t.TempDir()
	if e := os.Chmod(home, 0o700); e != nil {
		t.Fatal(e)
	}
	at := time.Unix(1789448000, 0)
	if err := Record(home, "gs", at); err != nil {
		t.Fatal(err)
	}
	events, err := Load(home)
	if err != nil || len(events) != 1 || events[0].Name != "gs" || !events[0].Time.Equal(at) {
		t.Fatalf("unexpected usage events: %#v, %v", events, err)
	}
	info, err := os.Stat(Path(home))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("usage database mode = %v", info.Mode().Perm())
	}
}

func TestUsageRecordRefusesSharedLockAndLeafLinks(t *testing.T) {
	home := t.TempDir()
	if e := os.Chmod(home, 0o700); e != nil {
		t.Fatal(e)
	}
	if e := Record(home, "fixture", time.Unix(1, 0)); e != nil {
		t.Fatal(e)
	}
	state := filepath.Join(home, ".local", "state", "alias-lens")
	lock, e := transaction.AcquireLock(state, filepath.Join(state, "mutation.lock"))
	if e != nil {
		t.Fatal(e)
	}
	if e := Record(home, "fixture", time.Unix(2, 0)); !errors.Is(e, transaction.ErrLocked) {
		t.Fatalf("usage ignored shared lock: %v", e)
	}
	lock.Close()
	outside := filepath.Join(home, "outside")
	os.WriteFile(outside, []byte("retained"), 0o600)
	os.Remove(Path(home))
	os.Symlink(outside, Path(home))
	if e := Record(home, "fixture", time.Unix(3, 0)); e == nil {
		t.Fatal("usage followed a leaf link")
	}
	if b, e := os.ReadFile(outside); e != nil || string(b) != "retained" {
		t.Fatal("usage altered external file")
	}
}
