package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestBrowserAndStatsShareThemeCanvasByColorDepth(t *testing.T) {
	theme := builtInTheme("dracula")
	previousRenderer := lipgloss.DefaultRenderer()
	t.Cleanup(func() {
		lipgloss.SetDefaultRenderer(previousRenderer)
		applyTheme(defaultTheme())
	})
	for _, test := range []struct {
		name    string
		profile termenv.Profile
		noColor string
		paint   bool
	}{
		{name: "truecolor", profile: termenv.TrueColor, paint: true},
		{name: "ansi256", profile: termenv.ANSI256, paint: true},
		{name: "ansi", profile: termenv.ANSI},
		{name: "no_color", profile: termenv.TrueColor, noColor: "1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", test.noColor)
			renderer := lipgloss.NewRenderer(os.Stdout)
			renderer.SetColorProfile(test.profile)
			lipgloss.SetDefaultRenderer(renderer)
			applyTheme(theme)
			browser := model{
				aliases: []Alias{{Name: "gt", Category: "git", Command: "git tag", Description: "Run a Git command"}},
				width:   120,
				height:  30,
				theme:   theme,
			}
			stats := statsModel{width: 120, height: 30, theme: theme, now: time.Now()}
			wantBackground := ""
			if test.paint {
				wantBackground = theme.Background
			}
			if got := browser.TerminalBackground(); got != wantBackground {
				t.Fatalf("browser terminal background = %q, want %q", got, wantBackground)
			}
			if got := stats.TerminalBackground(); got != wantBackground {
				t.Fatalf("stats terminal background = %q, want %q", got, wantBackground)
			}
			for _, view := range []struct {
				name string
				text string
			}{
				{name: "browser", text: browser.View()},
				{name: "stats", text: stats.View()},
			} {
				lines := strings.Split(view.text, "\n")
				last := lines[len(lines)-1]
				if got := lipgloss.Width(last); got != 120 {
					t.Fatalf("%s last row width = %d, want 120", view.name, got)
				}
				if test.paint {
					marker := lipgloss.NewStyle().Background(lipgloss.Color(theme.Background)).Render("x")
					prefix := marker[:strings.IndexByte(marker, 'x')]
					if !strings.Contains(last, prefix) {
						t.Fatalf("%s last row omitted the theme canvas", view.name)
					}
				} else if strings.Contains(last, "\x1b[4") || strings.Contains(last, "\x1b[10") || strings.Contains(last, "\x1b[48;") {
					t.Fatalf("%s last row paints a background in low-color mode", view.name)
				}
			}
			card := browser.aliasDetailCard(browser.aliases[0], 72, 20)
			panelMarker := lipgloss.NewStyle().Background(lipgloss.Color(theme.Panel)).Render("x")
			panelPrefix := panelMarker[:strings.IndexByte(panelMarker, 'x')]
			if test.paint && !strings.Contains(card, panelPrefix) {
				t.Fatal("selected alias card omitted its panel background")
			}
			if !test.paint && strings.Contains(card, panelPrefix) && panelPrefix != "" {
				t.Fatal("selected alias card painted a low-color panel background")
			}
		})
	}
}
