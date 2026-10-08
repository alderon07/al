package presentation

import (
	"regexp"
	"strings"
	"testing"
)

func TestAllCodexDarkThemesAreAvailable(t *testing.T) {
	if len(AvailableThemes()) != 29 {
		t.Fatalf("got %d themes, want 27 Codex dark themes plus Phosphor and Darcula", len(AvailableThemes()))
	}
	wanted := []string{
		"absolutely", "ayu", "catppuccin", "codex", "dracula", "everforest", "github",
		"gruvbox", "linear", "lobster", "material", "matrix", "monokai", "night-owl",
		"nord", "notion", "one", "oscurange", "raycast", "rose-pine", "sentry",
		"solarized", "temple", "tokyo-night", "vercel", "vscode-plus", "xcode",
	}
	for _, name := range wanted {
		if canonical, ok := CanonicalThemeName(name); !ok || canonical != name {
			t.Errorf("Codex dark theme %q is unavailable", name)
		}
	}
}

func TestTokyoNightMatchesCodexPalette(t *testing.T) {
	theme := BuiltInTheme("tokyo-night")
	if theme.Accent != "#3D59A1" || theme.Text != "#A9B1D6" || theme.Background != "#1A1B26" || theme.Panel != "#16161E" {
		t.Fatalf("Tokyo Night drifted from the Codex palette: %+v", theme)
	}
	if theme.Git != "#FF9E64" || theme.Docker != "#7AA2F7" || theme.Files != "#BB9AF7" || theme.Dev != "#E0AF68" {
		t.Fatalf("Tokyo Night semantic colors drifted: %+v", theme)
	}
}

func TestEveryThemeHasUsableColors(t *testing.T) {
	hexColor := regexp.MustCompile(`^#[0-9A-F]{6}$`)
	for _, theme := range AvailableThemes() {
		colors := []string{theme.Accent, theme.Secondary, theme.Git, theme.Docker, theme.Files, theme.Dev, theme.Text, theme.Muted, theme.Background, theme.Panel, theme.Selected, theme.Border}
		for _, color := range colors {
			if !hexColor.MatchString(strings.ToUpper(color)) {
				t.Errorf("theme %s has invalid color %q", theme.Preset, color)
			}
		}
		if strings.EqualFold(theme.Selected, theme.Background) || strings.EqualFold(theme.Selected, theme.Text) {
			t.Errorf("theme %s has an unreadable selected-row color", theme.Preset)
		}
	}
}

func TestThemeCycleWrapsToBeginning(t *testing.T) {
	if got := NextTheme(themeOrder[len(themeOrder)-1]); got.Preset != themeOrder[0] {
		t.Fatalf("theme cycle wrapped to %q, want %q", got.Preset, themeOrder[0])
	}
}

func TestDefaultThemeMeetsContrastThresholds(t *testing.T) {
	theme := DefaultTheme()
	if got := themeContrast(theme.Text, theme.Background); got < 4.5 {
		t.Fatalf("default text contrast = %.2f", got)
	}
	if got := themeContrast(theme.Accent, theme.Background); got < 3 {
		t.Fatalf("default control contrast = %.2f", got)
	}
}
