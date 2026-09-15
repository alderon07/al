package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

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

func TestStatsDashboardFillsWideTerminal(t *testing.T) {
	view := (statsModel{
		data:   statsData{Aliases: []Alias{{Name: "ll", Command: "ls -al"}}, Events: []usageEvent{{Name: "ll", Time: time.Now()}}},
		width:  160,
		height: 36,
		theme:  builtInTheme("phosphor"),
		now:    time.Now(),
	}).View()
	lines := strings.Split(view, "\n")
	if lipgloss.Width(lines[0]) != 160 {
		t.Fatalf("wide dashboard uses %d of 160 columns", lipgloss.Width(lines[0]))
	}
	if lipgloss.Height(view) != 36 {
		t.Fatalf("dashboard uses %d of 36 rows", lipgloss.Height(view))
	}
}

func TestStatsIgnorePrivateUsageDatabase(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), []byte("alias gs='git status'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".bash_history"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := recordAliasUse("gs"); err != nil {
		t.Fatal(err)
	}
	data, err := loadStatsData()
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

func TestMainTUIOpensStatsAndReturnsToAliases(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), []byte("alias ll='ls -al'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	history := fmt.Sprintf("#%d\nll\n", time.Now().Unix())
	if err := os.WriteFile(filepath.Join(home, ".bash_history"), []byte(history), 0o600); err != nil {
		t.Fatal(err)
	}

	updated, _ := (model{aliases: []Alias{{Name: "ll", Command: "ls -al"}}, width: 90, height: 24, theme: builtInTheme("phosphor")}).Update(tea.KeyMsg{Type: tea.KeyF2})
	stats := updated.(model)
	if !stats.statsOpen {
		t.Fatal("F2 did not open stats inside the main TUI")
	}
	view := stats.View()
	for _, expected := range []string{"ALIAS LENS", "Alias rhythm", "ll", "F2/esc return"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("embedded stats view is missing %q:\n%s", expected, view)
		}
	}
	returned, command := stats.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if returned.(model).statsOpen || command != nil {
		t.Fatal("Esc did not return from stats to the alias browser")
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

func TestStatsViewExplainsUntimestampedBashHistory(t *testing.T) {
	view := (statsModel{
		data:        statsData{Aliases: []Alias{{Name: "ll"}}, Events: []usageEvent{{Name: "ll"}}},
		periodIndex: 1,
		width:       100,
		height:      24,
		theme:       defaultTheme(),
		now:         time.Now(),
	}).View()
	if !strings.Contains(view, "history has no dates") || !strings.Contains(view, "Start a new shell") {
		t.Fatalf("stats view did not explain untimestamped Bash history:\n%s", view)
	}
}
