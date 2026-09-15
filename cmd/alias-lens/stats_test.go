package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
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

func TestStatsDashboardFitsTerminalWidth(t *testing.T) {
	theme := builtInTheme("phosphor")
	view := (statsModel{
		data:   statsData{Aliases: []Alias{{Name: "ll", Command: "ls -al"}}, Events: []usageEvent{{Name: "ll", Time: time.Now()}}},
		width:  80,
		height: 24,
		theme:  theme,
		now:    time.Now(),
	}).View()
	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > 80 {
			t.Fatalf("dashboard line is %d cells wide:\n%s", lipgloss.Width(line), line)
		}
	}
	if strings.Contains(view, "\n    1                                                                   \n") {
		t.Fatal("usage count wrapped onto its own line")
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

func TestZshHistoryUsageIncludesDirectAliasesAndTimestamps(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".zsh_history")
	contents := ": 1789441200:0;ll -a\n: 1789441210:0;git status\n: 1789441220:0;gs\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	events, err := historyUsageEventsFromShell(path, "zsh", []Alias{{Name: "ll"}, {Name: "gs"}})
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
	events, err := historyUsageEventsFromShell(path, "bash", []Alias{{Name: "ll"}, {Name: "gs"}})
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
