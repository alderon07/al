package tui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"testing"
	"time"

	"github.com/alderon07/al/internal/app"
	tea "github.com/alderon07/al/internal/tea"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestWideAliasDetailCardKeepsPanelBounded(t *testing.T) {
	alias := aliasEntry{
		Name:        "gs",
		Category:    "git",
		Command:     "git status --short --branch --show-stash",
		Description: "Show changed files and the current branch with enough words to wrap inside the selected alias card",
	}
	for _, width := range []int{63, 102} {
		card := (model{services: applicationServices()}).aliasDetailCard(alias, width, 20)
		limit := min(width, wideAliasDetailMaxWidth)
		for _, line := range strings.Split(card, "\n") {
			if got := lipgloss.Width(line); got > limit {
				t.Fatalf("detail card at %d cells has %d-cell line, limit %d", width, got, limit)
			}
		}
		if plain := ansi.Strip(card); width == 102 && (!strings.Contains(plain, "Show changed files and the current branch") || !strings.Contains(plain, "inside the selected alias card")) {
			t.Fatalf("wide card lost the selected description: %q", plain)
		}
	}
}

func TestWideAliasDetailNameGapUsesPanelBackground(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	previousRenderer := lipgloss.DefaultRenderer()
	renderer := lipgloss.NewRenderer(os.Stdout)
	renderer.SetColorProfile(termenv.TrueColor)
	lipgloss.SetDefaultRenderer(renderer)
	defer lipgloss.SetDefaultRenderer(previousRenderer)

	marker := lipgloss.NewStyle().Background(panelColor).Render("x")
	panelPrefix := marker[:strings.IndexByte(marker, 'x')]
	content := (model{services: applicationServices()}).aliasDetailContent(aliasEntry{Name: "gs", Category: "git", Command: "git status"}, 60, 20)
	if !strings.Contains(content, panelPrefix+"  ") {
		t.Fatalf("name/category gap does not use the panel background: %q", content)
	}
	if !strings.Contains(ansi.Strip(content), "gs   GIT") {
		t.Fatalf("name/category spacing changed: %q", ansi.Strip(content))
	}
}

func TestWideAliasBrowserShowsSelectedDetailsAndKeepsListVisible(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	current := model{services: applicationServices(), aliases: []aliasEntry{
		{Name: "first", Command: "printf first", Category: "tools"},
		{Name: "second", Command: "printf second argument", Description: "Shows the second item", Category: "tools", Tags: []string{"daily"}, Platforms: []string{"linux"}, Issues: []string{"review command"}, Favorite: true},
	},
		query:           "second",
		width:           132,
		height:          36,
		shortcutProfile: shortcutLinux,
	}
	view := ansi.Strip(current.View())
	for _, want := range []string{"Selected alias", "▶", "TOOLS", "♥︎", "ISSUE", "COMMAND", "DESCRIPTION", "TAGS", "PLATFORMS", "second", "printf second argument", "Shows the second item", "daily", "linux", "review command", "No local mark"} {
		if !strings.Contains(view, want) {
			t.Errorf("wide view missing %q:\n%s", want, view)
		}
	}
	if got := lipgloss.Width(current.View()); got != 132 {
		t.Fatalf("wide view width = %d, want 132", got)
	}
	if got := lipgloss.Height(current.View()); got != 36 {
		t.Fatalf("wide view height = %d, want 36", got)
	}
}

func TestWideAliasBrowserKeepsOverviewAndShowsSelectedUsage(t *testing.T) {
	current := model{services: applicationServices(), aliases: []aliasEntry{
		{Name: "daily", Command: "printf daily", Usage: 8},
		{Name: "sometimes", Command: "printf sometimes", Usage: 3, Issues: []string{"review command"}},
		{Name: "unused", Command: "printf unused"},
	},
		browserUsageReady: true,
		browserUsage: map[string]app.AliasUsageSummary{
			"daily": {All: 4, Today: 1, Week: 3, LastRun: time.Now().Add(-2 * time.Hour)},
		},
		width: 132, height: 36, shortcutProfile: shortcutLinux,
	}
	view := ansi.Strip(current.View())
	for _, want := range []string{"0 favorites", "0 categories", "1 needs attention", "USAGE", "All time  4", "Today  1", "Last 7 days  3", "Last used  2h ago", "Exact command matches  8"} {
		if !strings.Contains(view, want) {
			t.Errorf("wide view missing %q:\n%s", want, view)
		}
	}
	for _, unwanted := range []string{"seen in history", "11 history matches"} {
		if strings.Contains(view, unwanted) {
			t.Errorf("wide overview still shows %q:\n%s", unwanted, view)
		}
	}

	current.query = "unused"
	if view := ansi.Strip(current.View()); !strings.Contains(view, "All time  0") || !strings.Contains(view, "Last used  never") || !strings.Contains(view, "Exact command matches  0") {
		t.Fatalf("changing selection did not update usage:\n%s", view)
	}

	current.width = 80
	view = ansi.Strip(current.View())
	if strings.Contains(view, "USAGE  ·  SHELL HISTORY") {
		t.Fatalf("narrow cards gained the selected usage section:\n%s", view)
	}
}

func TestSelectedAliasUsageHandlesMissingUndatedAndUnreadableHistory(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv("ALIAS_LENS_SHELL", "bash")
	t.Setenv("ALIAS_LENS_HISTORY_FILE", "")
	current := model{services: applicationServices(), aliases: []aliasEntry{{Name: "gs", Command: "git status"}}, width: 132, height: 36}
	current.refreshBrowserUsage()
	if view := ansi.Strip(current.View()); !strings.Contains(view, "No shell history file yet") || strings.Contains(view, "All time  0") {
		t.Fatalf("missing history looked like a measured zero:\n%s", view)
	}

	history := filepath.Join(home, ".bash_history")
	if err := os.WriteFile(history, []byte("gs\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	current.refreshBrowserUsage()
	if view := ansi.Strip(current.View()); !strings.Contains(view, "All time  1") || !strings.Contains(view, "Last used  unknown") {
		t.Fatalf("undated history lost its use or invented a date:\n%s", view)
	}

	t.Setenv("ALIAS_LENS_HISTORY_FILE", home)
	current.refreshBrowserUsage()
	if view := ansi.Strip(current.View()); !strings.Contains(view, "Shell history unavailable") || strings.Contains(view, "All time  0") {
		t.Fatalf("unreadable history looked like a measured zero:\n%s", view)
	}
}

func TestAliasStatusMarkersFollowAppearance(t *testing.T) {
	t.Cleanup(func() { applyAppearanceConfig(defaultAppearanceConfig()) })
	alias := aliasEntry{Name: "gs", Command: "git status", Category: "git", Favorite: true, Issues: []string{"review command"}}
	t.Setenv("HOME", privateTestHome(t))
	project := t.TempDir()
	seedContextFixture(t, alias, app.ContextDirectory, project)
	ranking := contextRankingFixture(t, project, "")
	current := model{services: applicationServices(), context: ranking}
	for _, test := range []struct {
		style   string
		heart   string
		context string
	}{
		{style: "symbols", heart: "♥︎", context: "⌖"},
		{style: "ascii", heart: "*", context: "@"},
		{style: "none", context: "LOCAL"},
	} {
		t.Run(test.style, func(t *testing.T) {
			appearance := defaultAppearanceConfig()
			appearance.MarkerStyle = test.style
			applyAppearanceConfig(appearance)
			for _, width := range []int{38, 50} {
				row := ansi.Strip(current.wideAliasRow(alias, true, width))
				if !strings.Contains(row, test.context) || !strings.Contains(row, "ISSUE") || strings.Contains(row, "FAV") || strings.Contains(row, "HERE") {
					t.Fatalf("%s wide row at %d cells lost a status mark: %q", test.style, width, row)
				}
				if test.heart != "" && !strings.Contains(row, test.heart) {
					t.Fatalf("%s wide row at %d cells lost the favorite mark: %q", test.style, width, row)
				}
				if got := lipgloss.Width(current.wideAliasRow(alias, true, width)); got != width {
					t.Fatalf("%s wide row width = %d, want %d", test.style, got, width)
				}
			}
			card := ansi.Strip(renderAlias(alias, true, 80, true))
			if !strings.Contains(card, "LOCAL") || strings.Contains(card, "HERE") {
				t.Fatalf("%s narrow card lost the context explanation: %q", test.style, card)
			}
			if test.heart != "" && !strings.Contains(card, test.heart) {
				t.Fatalf("%s narrow card lost the favorite mark: %q", test.style, card)
			}
		})
	}
	if detail := ansi.Strip(current.aliasDetailCard(alias, 60, 20)); !strings.Contains(detail, "Favorite") || !strings.Contains(detail, "Marked for this folder") {
		t.Fatalf("selected detail lost the full status labels: %q", detail)
	}
}

func TestWideAliasRowsAlignCategoryBadges(t *testing.T) {
	current := model{services: applicationServices()}
	aliases := []aliasEntry{
		{Name: "gs", Command: "git status", Category: "git"},
		{Name: "gchanged", Command: "git diff", Category: "git", Favorite: true},
		{Name: "部署e\u0301🚀", Command: "git branch", Category: "git", Issues: []string{"review command"}},
		{Name: "long-alias-name-beyond-the-list-column", Command: "git log", Category: "git"},
	}
	for _, width := range []int{38, 50} {
		categoryColumn := -1
		for index, alias := range aliases {
			row := current.wideAliasRow(alias, index == 1, width)
			plain := ansi.Strip(row)
			badge := strings.Index(plain, "GIT")
			if badge < 0 {
				t.Fatalf("width %d row %q lost its category: %q", width, alias.Name, plain)
			}
			column := lipgloss.Width(plain[:badge])
			if categoryColumn < 0 {
				categoryColumn = column
			} else if column != categoryColumn {
				t.Errorf("width %d row %q category starts at %d, want %d", width, alias.Name, column, categoryColumn)
			}
			if got := lipgloss.Width(row); got != width {
				t.Errorf("width %d row %q occupies %d cells", width, alias.Name, got)
			}
		}
	}
	long := aliases[len(aliases)-1]
	if row := ansi.Strip(current.wideAliasRow(long, false, 38)); !strings.Contains(row, "…") || strings.Contains(row, long.Name) {
		t.Fatalf("long list name was not truncated: %q", row)
	}
	if detail := ansi.Strip(current.aliasDetailCard(long, 60, 20)); !strings.Contains(detail, long.Name) {
		t.Fatalf("selected detail lost the full alias name: %q", detail)
	}
}

func TestAliasBrowserReturnsToCardsWhenNarrowed(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	current := model{services: applicationServices(), aliases: []aliasEntry{
		{Name: "first", Command: "printf first"},
		{Name: "second", Command: "printf second"},
	},
		width: 132, height: 36, shortcutProfile: shortcutLinux,
	}
	if !strings.Contains(current.View(), "Selected alias") {
		t.Fatal("wide browser did not show details")
	}
	updated, _ := current.Update(tea.KeyMsg{Type: tea.KeyDown})
	current = updated.(model)
	selected := current.currentAliases()[current.cursor].Name
	if command := current.currentAliases()[current.cursor].Command; !strings.Contains(ansi.Strip(current.View()), command) {
		t.Fatalf("moving the cursor did not update the detail command to %q", command)
	}
	updated, _ = current.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	current = updated.(model)
	view := ansi.Strip(current.View())
	if strings.Contains(view, "Selected alias") || !strings.Contains(view, "Showing") {
		t.Fatalf("narrow browser did not return to cards:\n%s", view)
	}
	if current.currentAliases()[current.cursor].Name != selected {
		t.Fatal("resize changed the selected alias")
	}
	if got := lipgloss.Width(current.View()); got != 80 {
		t.Fatalf("narrow view width = %d, want 80", got)
	}
}

func TestShortWideAliasBrowserKeepsCommandVisible(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	current := model{services: applicationServices(), aliases: []aliasEntry{{Name: "sample", Command: "printf sample"}}, width: 120, height: 18}
	view := ansi.Strip(current.View())
	if !strings.Contains(view, "Selected alias") || !strings.Contains(view, "printf sample") {
		t.Fatalf("short wide browser hid the command:\n%s", view)
	}
	if got := lipgloss.Height(current.View()); got != 18 {
		t.Fatalf("short wide view height = %d, want 18", got)
	}
}

func TestWideAliasBrowserEscapesCommandControlText(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	current := model{services: applicationServices(), aliases: []aliasEntry{{Name: "safe", Command: "printf '\x1b[2J'"}}, width: 132, height: 36}
	view := current.View()
	if strings.Contains(view, "\x1b[2J'") || !strings.Contains(view, `\x1b[2J`) {
		t.Fatalf("wide detail rendered raw control text: %q", view)
	}
}

func TestWideAliasBrowserAlignsUnicodeContent(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	alias := aliasEntry{
		Name:        "部署e\u0301🚀",
		Command:     "printf '界界界界界界界界界界界界界界界界界界界界界界界界'",
		Description: "Inspect a multilingual command without breaking cell alignment",
		Category:    "工具",
	}
	current := model{services: applicationServices(), aliases: []aliasEntry{alias}, width: 132, height: 36, shortcutProfile: shortcutLinux}
	view := ansi.Strip(current.View())
	for _, want := range []string{alias.Name, alias.Command, strings.ToUpper(alias.Category)} {
		if !strings.Contains(view, want) {
			t.Errorf("wide Unicode view missing %q:\n%s", want, view)
		}
	}
	if got := lipgloss.Width(current.View()); got != 132 {
		t.Fatalf("wide Unicode view width = %d, want 132", got)
	}
	if got := lipgloss.Height(current.View()); got != 36 {
		t.Fatalf("wide Unicode view height = %d, want 36", got)
	}
}
