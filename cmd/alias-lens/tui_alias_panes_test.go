package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	tea "alias-lens/cmd/alias-lens/internal/tea"
)

func TestWideAliasBrowserShowsSelectedDetailsAndKeepsListVisible(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	current := model{
		aliases: []Alias{
			{Name: "first", Command: "printf first", Category: "tools"},
			{Name: "second", Command: "printf second argument", Description: "Shows the second item", Category: "tools", Tags: []string{"daily"}, Platforms: []string{"linux"}, Issues: []string{"review command"}, Favorite: true},
		},
		query:           "second",
		width:           132,
		height:          36,
		shortcutProfile: shortcutLinux,
	}
	view := ansi.Strip(current.View())
	for _, want := range []string{"Selected alias", "▶", "TOOLS", "FAV", "ISSUE", "COMMAND", "DESCRIPTION", "TAGS", "PLATFORMS", "second", "printf second argument", "Shows the second item", "daily", "linux", "review command", "No local mark"} {
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

func TestWideAliasRowsAlignCategoryBadges(t *testing.T) {
	current := model{}
	aliases := []Alias{
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
	t.Setenv("HOME", t.TempDir())
	current := model{
		aliases: []Alias{
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
	t.Setenv("HOME", t.TempDir())
	current := model{aliases: []Alias{{Name: "sample", Command: "printf sample"}}, width: 120, height: 18}
	view := ansi.Strip(current.View())
	if !strings.Contains(view, "Selected alias") || !strings.Contains(view, "printf sample") {
		t.Fatalf("short wide browser hid the command:\n%s", view)
	}
	if got := lipgloss.Height(current.View()); got != 18 {
		t.Fatalf("short wide view height = %d, want 18", got)
	}
}

func TestWideAliasBrowserEscapesCommandControlText(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	current := model{aliases: []Alias{{Name: "safe", Command: "printf '\x1b[2J'"}}, width: 132, height: 36}
	view := current.View()
	if strings.Contains(view, "\x1b[2J'") || !strings.Contains(view, `\x1b[2J`) {
		t.Fatalf("wide detail rendered raw control text: %q", view)
	}
}

func TestWideAliasBrowserAlignsUnicodeContent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	alias := Alias{
		Name:        "部署e\u0301🚀",
		Command:     "printf '界界界界界界界界界界界界界界界界界界界界界界界界'",
		Description: "Inspect a multilingual command without breaking cell alignment",
		Category:    "工具",
	}
	current := model{aliases: []Alias{alias}, width: 132, height: 36, shortcutProfile: shortcutLinux}
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
