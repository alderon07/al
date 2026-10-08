package tui

import (
	"fmt"
	"os"
	"path/filepath"

	tea "alias-lens/internal/tea"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"alias-lens/internal/app"
)

func TestStatsDashboardFitsTerminalWidth(t *testing.T) {
	theme := builtInTheme("phosphor")
	view := (statsModel{services: applicationServices(), data: app.StatsData{Aliases: []aliasEntry{{Name: "ll", Command: "ls -al"}}, Events: []app.UsageEvent{{Name: "ll", Time: time.Now()}}},
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

func TestStatsOverviewShowsLastRunAtNarrowAndWideWidths(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.Local)
	data := app.StatsData{
		Aliases: []aliasEntry{{Name: "dated", Command: "echo dated"}, {Name: "undated", Command: "echo undated"}},
		Events: []app.UsageEvent{
			{Name: "dated", Time: now.Add(-3 * time.Hour)},
			{Name: "dated", Time: now.Add(-2 * time.Hour)},
			{Name: "undated"},
		},
	}
	for _, width := range []int{48, 80, 120} {
		view := (statsModel{services: applicationServices(), data: data, width: width, height: 24, theme: defaultTheme(), now: now}).View()
		for _, label := range []string{"LAST RUN", "2h ago", "unknown"} {
			if !strings.Contains(view, label) {
				t.Fatalf("%d-column overview missing %q:\n%s", width, label, view)
			}
		}
		for _, line := range strings.Split(view, "\n") {
			if lipgloss.Width(line) > width {
				t.Fatalf("%d-column overview rendered a %d-cell line:\n%s", width, lipgloss.Width(line), view)
			}
		}
	}
}

func TestStatsDashboardFillsWideTerminal(t *testing.T) {
	view := (statsModel{services: applicationServices(), data: app.StatsData{Aliases: []aliasEntry{{Name: "ll", Command: "ls -al"}}, Events: []app.UsageEvent{{Name: "ll", Time: time.Now()}}},
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

func TestStatsDashboardLeavesLastViewportRowAtTerminalBackground(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	previousRenderer := lipgloss.DefaultRenderer()
	renderer := lipgloss.NewRenderer(os.Stdout)
	renderer.SetColorProfile(termenv.TrueColor)
	lipgloss.SetDefaultRenderer(renderer)
	defer lipgloss.SetDefaultRenderer(previousRenderer)

	background := builtInTheme("xcode")
	marker := lipgloss.NewStyle().Background(lipgloss.Color(background.Background)).Render("x")
	backgroundPrefix := marker[:strings.IndexByte(marker, 'x')]
	for _, embedded := range []bool{false, true} {
		for _, size := range []struct {
			width  int
			height int
		}{{48, 18}, {80, 24}, {120, 30}} {
			model := statsModel{services: applicationServices(), data: app.StatsData{Aliases: []aliasEntry{{Name: "ll"}}},
				width:  size.width,
				height: size.height,
				theme:  background,
				now:    time.Now(),
			}
			if embedded {
				model.appHeader = "ALIAS LENS"
			}
			if got := model.TerminalBackground(); got != "" {
				t.Fatalf("embedded=%t: terminal background = %q, want no terminal mutation", embedded, got)
			}
			lines := strings.Split(model.View(), "\n")
			last := lines[len(lines)-1]
			if got := lipgloss.Width(last); got != size.width {
				t.Fatalf("embedded=%t size=%dx%d: last row width = %d", embedded, size.width, size.height, got)
			}
			if strings.Contains(last, backgroundPrefix) {
				t.Fatalf("embedded=%t size=%dx%d: last row paints a separate page background: %q", embedded, size.width, size.height, last)
			}
		}
	}
	if got := (model{services: applicationServices(), theme: background, statsOpen: true}).TerminalBackground(); got != "" {
		t.Fatalf("main TUI terminal background = %q, want no terminal mutation", got)
	}
}

func TestStatsDashboardShowsExactAliasCoverageMap(t *testing.T) {
	view := (statsModel{services: applicationServices(), data: app.StatsData{
		Aliases: []aliasEntry{{Name: "ll"}, {Name: "gs"}, {Name: "unused"}},
		Events:  []app.UsageEvent{{Name: "ll", Time: time.Now()}, {Name: "gs", Time: time.Now()}},
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

func TestAliasCoverageMapHasOneDotPerAliasAtSeventy(t *testing.T) {
	view := renderCoverageMap(15, 70, builtInTheme("phosphor"))
	if got := strings.Count(view, "●"); got != 16 {
		t.Fatalf("filled dot count = %d, want 15 chart dots and one legend dot", got)
	}
	if got := strings.Count(view, "○"); got != 56 {
		t.Fatalf("empty dot count = %d, want 55 chart dots and one legend dot", got)
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
	days := sevenDayActivity([]app.UsageEvent{
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
	rows := []app.StatsRow{{Alias: aliasEntry{Name: "ll"}, Count: 8}, {Alias: aliasEntry{Name: "gs"}, Count: 2}}
	if chart := renderConcentration(rows, styles); !strings.Contains(chart, "Top-five share  100%") || !strings.Contains(chart, "10 of 10 runs") {
		t.Fatalf("unexpected concentration chart:\n%s", chart)
	}
	data := app.StatsData{
		Aliases: []aliasEntry{
			{Name: "active", Category: "files"},
			{Name: "old", Tags: []string{"git"}},
			{Name: "ancient"},
			{Name: "fresh-zero"},
			{Name: "unknown"},
		},
		Events: []app.UsageEvent{
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
	groups := renderGroupShare(applicationServices(),

		data, "all", now, 100, styles)
	for _, expected := range []string{"files", "git", "untagged", "Uses category first"} {
		if !strings.Contains(groups, expected) {
			t.Fatalf("group chart is missing %q:\n%s", expected, groups)
		}
	}
	if !strings.Contains(groups, "●") || strings.Contains(groups, "━") {
		t.Fatalf("groups view did not use a dotted pie chart:\n%s", groups)
	}
}

func TestGroupPieCollapsesSmallSlices(t *testing.T) {
	groups := []groupUsage{{name: "one", count: 6}, {name: "two", count: 5}, {name: "three", count: 4}, {name: "four", count: 3}, {name: "five", count: 2}, {name: "six", count: 1}}
	collapsed := collapseGroupSlices(groups)
	if len(collapsed) != 5 || collapsed[4].name != "other" || collapsed[4].count != 3 {
		t.Fatalf("unexpected collapsed slices: %#v", collapsed)
	}
}

func TestStatsViewKeysOpenEachChart(t *testing.T) {
	m := statsModel{services: applicationServices(), data: app.StatsData{Aliases: []aliasEntry{{Name: "ll"}}, Events: []app.UsageEvent{{Name: "ll", Time: time.Now()}}},
		width:  100,
		height: 30,
		theme:  builtInTheme("phosphor"),
		now:    time.Now(),
	}
	if view := m.View(); !strings.Contains(view, "g categories") || strings.Contains(view, "g groups") {
		t.Fatalf("stats tab did not use the categories label:\n%s", view)
	}
	for key, expected := range map[string]string{"a": "Seven-day activity", "c": "Unused for a month", "g": "Usage by category", "o": "Alias coverage"} {
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
	events := []app.UsageEvent{
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
		view := (statsModel{services: applicationServices(), data: app.StatsData{Aliases: []aliasEntry{{Name: "ll"}}, Events: []app.UsageEvent{{Name: "ll", Time: time.Now()}}},
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
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv("ALIAS_LENS_SHELL", "bash")
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), []byte("alias ll='ls -al'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".bash_history"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	m := model{services: applicationServices(), statsOpen: true, statsViewIndex: 2, statsPeriod: 3}
	updated, _ := m.updateStatsView(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	refreshed := updated.(model)
	if refreshed.statsViewIndex != 2 || refreshed.statsPeriod != 3 {
		t.Fatalf("refresh changed view=%d period=%d", refreshed.statsViewIndex, refreshed.statsPeriod)
	}
}

func TestStatsDashboardUsesTerminalCanvasForEveryTheme(t *testing.T) {
	t.Setenv("NO_COLOR", "")
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
		model := statsModel{services: applicationServices(), data: app.StatsData{Aliases: []aliasEntry{{Name: "ll"}}, Events: []app.UsageEvent{{Name: "ll", Time: time.Now()}}},
			width:  100,
			height: 28,
			theme:  theme,
			now:    time.Now(),
		}
		view := model.View()
		if strings.Contains(view, backgroundPrefix(theme.Panel)) {
			t.Fatalf("%s stats still paint the panel background", theme.Name)
		}
		if strings.Contains(view, backgroundPrefix(theme.Background)) {
			t.Fatalf("%s stats paint a separate canvas background", theme.Name)
		}
		if lipgloss.Width(strings.Split(view, "\n")[0]) != 100 {
			t.Fatalf("%s canvas background fix changed the dashboard width", theme.Name)
		}
	}
}

func TestMainTUIOpensStatsAndReturnsToAliases(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv("ALIAS_LENS_SHELL", "bash")
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), []byte("alias ll='ls -al'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	history := fmt.Sprintf("#%d\nll\n", time.Now().Unix())
	if err := os.WriteFile(filepath.Join(home, ".bash_history"), []byte(history), 0o600); err != nil {
		t.Fatal(err)
	}

	updated, _ := (model{services: applicationServices(), aliases: []aliasEntry{{Name: "ll", Command: "ls -al"}}, width: 90, height: 24, theme: builtInTheme("phosphor")}).Update(tea.KeyMsg{Type: tea.KeyF2})
	stats := updated.(model)
	if !stats.statsOpen {
		t.Fatal("F2 did not open stats inside the main TUI")
	}
	view := stats.View()
	for _, expected := range []string{"ALIAS LENS", "Alias rhythm", "ll", "s/esc return"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("embedded stats view is missing %q:\n%s", expected, view)
		}
	}
	returned, command := stats.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if returned.(model).statsOpen || command != nil {
		t.Fatal("Esc did not return from stats to the alias browser")
	}
}

func TestStatsViewExplainsUntimestampedBashHistory(t *testing.T) {
	view := (statsModel{services: applicationServices(), data: app.StatsData{Aliases: []aliasEntry{{Name: "ll"}}, Events: []app.UsageEvent{{Name: "ll"}}},
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
