package usagelog

import (
	"os"
	"testing"
	"time"
)

func TestRecordAndLoad(t *testing.T) {
	home := t.TempDir()
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
