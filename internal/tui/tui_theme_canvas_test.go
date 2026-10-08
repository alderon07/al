package tui

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestBrowserStatsAndThemePickerLeaveTerminalCanvasByColorDepth(t *testing.T) {
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
			browser := model{services: applicationServices(), aliases: []aliasEntry{{Name: "gt", Category: "git", Command: "git tag", Description: "Run a Git command"}},
				width:  120,
				height: 30,
				theme:  theme,
			}
			stats := statsModel{services: applicationServices(), width: 120, height: 30, theme: theme, now: time.Now()}
			picker := browser
			picker.themePicker = true
			if got := browser.TerminalBackground(); got != "" {
				t.Fatalf("browser terminal background = %q, want no terminal mutation", got)
			}
			if got := stats.TerminalBackground(); got != "" {
				t.Fatalf("stats terminal background = %q, want no terminal mutation", got)
			}
			for _, view := range []struct {
				name string
				text string
			}{
				{name: "browser", text: browser.View()},
				{name: "stats", text: stats.View()},
				{name: "theme picker", text: picker.View()},
			} {
				lines := strings.Split(view.text, "\n")
				last := lines[len(lines)-1]
				if got := lipgloss.Width(last); got != 120 {
					t.Fatalf("%s last row width = %d, want 120", view.name, got)
				}
				if strings.Contains(last, "\x1b[4") || strings.Contains(last, "\x1b[10") || strings.Contains(last, "\x1b[48;") {
					t.Fatalf("%s last row paints a separate background", view.name)
				}
			}
			card := browser.aliasDetailCard(browser.aliases[0], 72, 20)
			panelMarker := lipgloss.NewStyle().Background(terminalBackgroundColor(theme.Panel)).Render("x")
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

func TestANSI256DarkBackgroundsKeepTheirDarkValue(t *testing.T) {
	previousRenderer := lipgloss.DefaultRenderer()
	renderer := lipgloss.NewRenderer(os.Stdout)
	renderer.SetColorProfile(termenv.ANSI256)
	lipgloss.SetDefaultRenderer(renderer)
	t.Cleanup(func() { lipgloss.SetDefaultRenderer(previousRenderer) })
	for _, test := range []struct {
		color string
		want  string
	}{
		{color: "#282A36", want: "236"},
		{color: "#272822", want: "235"},
		{color: "#1E1F1C", want: "234"},
	} {
		if got := terminalBackgroundColor(test.color); got != lipgloss.Color(test.want) {
			t.Errorf("%s mapped to %q, want dark ANSI color %s", test.color, got, test.want)
		}
	}
}

func TestNestedStyleResetRestoresBoundedBackground(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	previousRenderer := lipgloss.DefaultRenderer()
	renderer := lipgloss.NewRenderer(os.Stdout)
	renderer.SetColorProfile(termenv.TrueColor)
	lipgloss.SetDefaultRenderer(renderer)
	t.Cleanup(func() { lipgloss.SetDefaultRenderer(previousRenderer) })
	for _, test := range []struct {
		name       string
		background string
	}{
		{name: "canvas", background: "#282A36"},
		{name: "panel", background: "#21222C"},
	} {
		t.Run(test.name, func(t *testing.T) {
			color := lipgloss.Color(test.background)
			marker := lipgloss.NewStyle().Background(color).Render("x")
			prefix := marker[:strings.IndexByte(marker, 'x')]
			accent := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF79C6")).Render("accent")
			badge := lipgloss.NewStyle().Background(lipgloss.Color("#FF5555")).Render("badge")
			got := fillUnstyledBackground("before "+accent+" between "+badge+" after", color)
			if !strings.Contains(got, "\x1b[0m"+prefix+" between ") || !strings.Contains(got, "\x1b[0m"+prefix+" after") {
				t.Fatalf("nested style reset exposed the terminal background: %q", got)
			}
		})
	}
}
