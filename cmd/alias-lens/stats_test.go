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
	"github.com/muesli/termenv"
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

func TestStatsDashboardShowsExactAliasCoverageMap(t *testing.T) {
	view := (statsModel{
		data: statsData{
			Aliases: []Alias{{Name: "ll"}, {Name: "gs"}, {Name: "unused"}},
			Events:  []usageEvent{{Name: "ll", Time: time.Now()}, {Name: "gs", Time: time.Now()}},
		},
		width:  100,
		height: 28,
		theme:  builtInTheme("phosphor"),
		now:    time.Now(),
	}).View()
	for _, expected := range []string{"Alias coverage  67%", "● 2 used", "○ 1 unused", "1 dot = 1 alias"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("coverage map is missing %q:\n%s", expected, view)
		}
	}
	if strings.Contains(view, "██") {
		t.Fatalf("coverage map fell back to the old block style:\n%s", view)
	}
}

func TestAliasCoverageMapScalesLargeCollections(t *testing.T) {
	view := renderCoverageMap(75, 150, builtInTheme("phosphor"))
	if !strings.Contains(view, "Alias coverage  50%") || !strings.Contains(view, "scaled to 100 dots") {
		t.Fatalf("large coverage map did not explain its scale:\n%s", view)
	}
}

func TestSevenDayActivityBucketsDatedHistory(t *testing.T) {
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.Local)
	days := sevenDayActivity([]usageEvent{
		{Name: "ll", Time: now.AddDate(0, 0, -6)},
		{Name: "ll", Time: now},
		{Name: "gs", Time: now},
		{Name: "old", Time: now.AddDate(0, 0, -7)},
		{Name: "unknown"},
	}, now)
	if len(days) != 7 || days[0].count != 1 || days[6].count != 2 {
		t.Fatalf("unexpected seven-day buckets: %#v", days)
	}
}

func TestStatsChartsShowConcentrationStalenessAndGroups(t *testing.T) {
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.Local)
	styles := statsChartStyles{
		accent: lipgloss.NewStyle(),
		text:   lipgloss.NewStyle(),
		muted:  lipgloss.NewStyle(),
		theme:  builtInTheme("phosphor"),
	}
	rows := []statsRow{{Alias: Alias{Name: "ll"}, Count: 8}, {Alias: Alias{Name: "gs"}, Count: 2}}
	if chart := renderConcentration(rows, styles); !strings.Contains(chart, "Top-five share  100%") || !strings.Contains(chart, "10 of 10 executions") {
		t.Fatalf("unexpected concentration chart:\n%s", chart)
	}
	data := statsData{
		Aliases: []Alias{
			{Name: "active", Category: "files"},
			{Name: "old", Tags: []string{"git"}},
			{Name: "ancient"},
			{Name: "fresh-zero"},
			{Name: "unknown"},
		},
		Events: []usageEvent{
			{Name: "active", Time: now},
			{Name: "old", Time: now.AddDate(0, 0, -100)},
			{Name: "ancient", Time: now.AddDate(-2, 0, 0)},
			{Name: "unknown"},
		},
	}
	stale := renderStaleAliases(data, "month", now, 100, 30, styles)
	for _, expected := range []string{"Unused for a month", "old", "ancient", "3mo", "2y"} {
		if !strings.Contains(stale, expected) {
			t.Fatalf("stale chart is missing %q:\n%s", expected, stale)
		}
	}
	never := renderStaleAliases(data, "never", now, 100, 30, styles)
	if !strings.Contains(never, "Never-used aliases") || !strings.Contains(never, "fresh-zero") || strings.Contains(never, "unknown  ") {
		t.Fatalf("never-used chart mixed in unknown dates:\n%s", never)
	}
	yearly := renderStaleAliases(data, "year", now, 100, 30, styles)
	if !strings.Contains(yearly, "ancient") || strings.Contains(yearly, "old") {
		t.Fatalf("year cleanup filter used the wrong threshold:\n%s", yearly)
	}
	groups := renderGroupShare(data, "all", now, 100, styles)
	for _, expected := range []string{"files", "git", "untagged", "Uses category first"} {
		if !strings.Contains(groups, expected) {
			t.Fatalf("group chart is missing %q:\n%s", expected, groups)
		}
	}
}

func TestStatsViewKeysOpenEachChart(t *testing.T) {
	m := statsModel{
		data:   statsData{Aliases: []Alias{{Name: "ll"}}, Events: []usageEvent{{Name: "ll", Time: time.Now()}}},
		width:  100,
		height: 30,
		theme:  builtInTheme("phosphor"),
		now:    time.Now(),
	}
	for key, expected := range map[string]string{"a": "Seven-day activity", "c": "Unused for a month", "g": "Usage by group", "o": "Alias coverage"} {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		m = updated.(statsModel)
		if view := m.View(); !strings.Contains(view, expected) {
			t.Fatalf("%q did not open %q:\n%s", key, expected, view)
		}
	}
}

func TestActivityViewChangesGranularityWithPeriod(t *testing.T) {
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.Local)
	styles := statsChartStyles{accent: lipgloss.NewStyle(), text: lipgloss.NewStyle(), muted: lipgloss.NewStyle(), theme: builtInTheme("phosphor")}
	for period, expected := range map[string]string{
		"all":   "All-time activity by year",
		"today": "Today's activity",
		"week":  "Seven-day activity",
		"year":  "Twelve-month activity",
	} {
		view := renderActivityHistogram(nil, period, now, 100, styles)
		if !strings.Contains(view, expected) {
			t.Fatalf("%s activity view is missing %q:\n%s", period, expected, view)
		}
	}
}

func TestActivityBucketsMatchSelectedTimeframe(t *testing.T) {
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.Local)
	events := []usageEvent{
		{Name: "ll", Time: time.Date(2026, 9, 15, 1, 0, 0, 0, time.Local)},
		{Name: "ll", Time: time.Date(2026, 9, 15, 22, 0, 0, 0, time.Local)},
		{Name: "gs", Time: now.AddDate(0, -1, 0)},
		{Name: "old", Time: now.AddDate(-1, 0, 0)},
	}
	_, today := activityBuckets(events, "today", now)
	if len(today) != 8 || today[0].count != 1 || today[7].count != 1 {
		t.Fatalf("today did not use three-hour buckets: %#v", today)
	}
	_, week := activityBuckets(events, "week", now)
	if len(week) != 7 || week[6].count != 2 {
		t.Fatalf("week did not use daily buckets: %#v", week)
	}
	_, year := activityBuckets(events, "year", now)
	if len(year) != 12 || year[10].count != 1 || year[11].count != 2 {
		t.Fatalf("year did not use monthly buckets: %#v", year)
	}
	_, all := activityBuckets(events, "all", now)
	if len(all) != 2 || all[0].count != 1 || all[1].count != 3 {
		t.Fatalf("all did not use yearly buckets: %#v", all)
	}
}

func TestStatsViewsKeepNarrowFooterOnOneLine(t *testing.T) {
	for viewIndex := range statsViews {
		view := (statsModel{
			data:      statsData{Aliases: []Alias{{Name: "ll"}}, Events: []usageEvent{{Name: "ll", Time: time.Now()}}},
			viewIndex: viewIndex,
			width:     60,
			height:    36,
			theme:     builtInTheme("phosphor"),
			now:       time.Now(),
		}).View()
		if strings.Contains(view, "\n  close") {
			t.Fatalf("stats view %d wrapped the close hint:\n%s", viewIndex, view)
		}
	}
}

func TestEmbeddedStatsRefreshPreservesViewAndPeriod(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), []byte("alias ll='ls -al'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".bash_history"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	m := model{statsOpen: true, statsViewIndex: 2, statsPeriod: 3}
	updated, _ := m.updateStatsView(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	refreshed := updated.(model)
	if refreshed.statsViewIndex != 2 || refreshed.statsPeriod != 3 {
		t.Fatalf("refresh changed view=%d period=%d", refreshed.statsViewIndex, refreshed.statsPeriod)
	}
}

func TestStatsDashboardUsesEachThemeCanvasInsteadOfPanel(t *testing.T) {
	previousRenderer := lipgloss.DefaultRenderer()
	renderer := lipgloss.NewRenderer(os.Stdout)
	renderer.SetColorProfile(termenv.TrueColor)
	lipgloss.SetDefaultRenderer(renderer)
	defer lipgloss.SetDefaultRenderer(previousRenderer)

	backgroundPrefix := func(color string) string {
		marker := lipgloss.NewStyle().Background(lipgloss.Color(color)).Render("x")
		return marker[:strings.IndexByte(marker, 'x')]
	}
	for _, theme := range availableThemes() {
		if theme.Panel == theme.Background {
			continue
		}
		model := statsModel{
			data:   statsData{Aliases: []Alias{{Name: "ll"}}, Events: []usageEvent{{Name: "ll", Time: time.Now()}}},
			width:  100,
			height: 28,
			theme:  theme,
			now:    time.Now(),
		}
		view := model.View()
		if strings.Contains(view, backgroundPrefix(theme.Panel)) {
			t.Fatalf("%s stats still paint the panel background", theme.Name)
		}
		if count := strings.Count(view, backgroundPrefix(theme.Background)); count < 3 {
			t.Fatalf("%s canvas background was restored only %d times", theme.Name, count)
		}
		if lipgloss.Width(strings.Split(view, "\n")[0]) != 100 {
			t.Fatalf("%s canvas background fix changed the dashboard width", theme.Name)
		}
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
