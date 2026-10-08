package app

import (
	"os"
	"path/filepath"

	"testing"

	"time"
)

func TestStatsLastRunRespectsSelectedPeriod(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.Local)
	data := StatsData{
		Aliases: []Alias{{Name: "ll"}},
		Events: []UsageEvent{
			{Name: "ll"},
			{Name: "ll", Time: now.AddDate(0, 0, -2)},
			{Name: "ll", Time: now.Add(-time.Hour)},
		},
	}
	all, err := rankedStatsRows(data, "all", now)
	if err != nil || len(all) != 1 || all[0].Count != 3 || !all[0].LastRun.Equal(now.Add(-time.Hour)) {
		t.Fatalf("all-time row = %#v, %v", all, err)
	}
	today, err := rankedStatsRows(data, "today", now)
	if err != nil || len(today) != 1 || today[0].Count != 1 || !today[0].LastRun.Equal(now.Add(-time.Hour)) {
		t.Fatalf("today row = %#v, %v", today, err)
	}
}

func TestStatsIgnorePrivateUsageDatabase(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), []byte("alias gs='git status'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".bash_history"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := DefaultServices().recordAliasUse("gs"); err != nil {
		t.Fatal(err)
	}
	data, err := DefaultServices().loadStatsData()
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Events) != 0 {
		t.Fatalf("private usage database affected history-only stats: %#v", data.Events)
	}
	info, err := os.Stat(filepath.Join(home, ".local", "share", "alias-lens", "usage.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("private usage database mode = %v", info.Mode().Perm())
	}
}

func TestUsageCountsRespectPeriodBoundary(t *testing.T) {
	now := time.Now()
	events := []UsageEvent{
		{Name: "gs", Time: now.Add(-time.Hour)},
		{Name: "gs", Time: now.Add(-8 * 24 * time.Hour)},
		{Name: "gp", Time: now.Add(-time.Hour)},
	}
	counts := usageCountsSince(events, now.Add(-7*24*time.Hour))
	if counts["gs"] != 1 || counts["gp"] != 1 {
		t.Fatalf("unexpected weekly counts: %#v", counts)
	}
}

func TestSelectedAliasUsageUsesDatedHistoryWindows(t *testing.T) {
	now := time.Date(2026, time.September, 27, 15, 0, 0, 0, time.UTC)
	events := []UsageEvent{
		{Name: "gs", Time: now.Add(-time.Hour)},
		{Name: "gs", Time: now.Add(-6 * 24 * time.Hour)},
		{Name: "gs", Time: now.Add(-8 * 24 * time.Hour)},
		{Name: "gs"},
		{Name: "gl", Time: now.Add(-time.Hour)},
	}
	summaries := summarizeAliasUses(events, now)
	if got := summaries["gs"]; got.All != 4 || got.Today != 1 || got.Week != 2 || !got.LastRun.Equal(now.Add(-time.Hour)) {
		t.Fatalf("selected alias usage = %#v", got)
	}
	if got := summaries["gl"]; got.All != 1 || got.Today != 1 || got.Week != 1 {
		t.Fatalf("other alias usage = %#v", got)
	}
}

func TestZshHistoryUsageIncludesDirectAliasesAndTimestamps(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".zsh_history")
	contents := ": 1789441200:0;ll -a\n: 1789441210:0;git status\n: 1789441220:0;gs\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	events, err := DefaultServices().historyUsageEventsFromShell(path, "zsh", []Alias{{Name: "ll"}, {Name: "gs"}})
	if err != nil || len(events) != 2 {
		t.Fatalf("unexpected direct alias events: %#v, %v", events, err)
	}
	if events[0].Name != "ll" || events[0].Time.Unix() != 1789441200 || events[1].Name != "gs" {
		t.Fatalf("Zsh history metadata was not preserved: %#v", events)
	}
}

func TestBashHistoryUsageKeepsUntimestampedAliasesForAllTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".bash_history")
	contents := "ll\n#1789441200\ngs --short\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	events, err := DefaultServices().historyUsageEventsFromShell(path, "bash", []Alias{{Name: "ll"}, {Name: "gs"}})
	if err != nil || len(events) != 2 {
		t.Fatalf("unexpected direct alias events: %#v, %v", events, err)
	}
	if !events[0].Time.IsZero() || events[1].Time.Unix() != 1789441200 {
		t.Fatalf("Bash history timestamps were not handled: %#v", events)
	}
	if counts := usageCountsSince(events, time.Unix(1789441100, 0)); counts["ll"] != 0 || counts["gs"] != 1 {
		t.Fatalf("untimestamped history leaked into a dated period: %#v", counts)
	}
}
