package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUsageLogStoresOnlyTimeAndAliasNamePrivately(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := recordAliasUse("gs"); err != nil {
		t.Fatal(err)
	}
	events, err := loadUsageEvents()
	if err != nil || len(events) != 1 || events[0].Name != "gs" {
		t.Fatalf("unexpected usage events: %#v, %v", events, err)
	}
	info, err := os.Stat(filepath.Join(home, ".local", "share", "alias-lens", "usage.tsv"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("usage log is not private: %v, %v", info, err)
	}
}

func TestUsageCountsRespectPeriodBoundary(t *testing.T) {
	now := time.Now()
	events := []usageEvent{
		{Name: "gs", Time: now.Add(-time.Hour)},
		{Name: "gs", Time: now.Add(-8 * 24 * time.Hour)},
		{Name: "gp", Time: now.Add(-time.Hour)},
	}
	counts := usageCountsSince(events, now.Add(-7*24*time.Hour))
	if counts["gs"] != 1 || counts["gp"] != 1 {
		t.Fatalf("unexpected weekly counts: %#v", counts)
	}
}
