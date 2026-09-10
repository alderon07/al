package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestAllCodexDarkThemesAreAvailable(t *testing.T) {
	if len(availableThemes()) != 29 {
		t.Fatalf("got %d themes, want 27 Codex dark themes plus Phosphor and Darcula", len(availableThemes()))
	}
	wanted := []string{
		"absolutely", "ayu", "catppuccin", "codex", "dracula", "everforest", "github",
		"gruvbox", "linear", "lobster", "material", "matrix", "monokai", "night-owl",
		"nord", "notion", "one", "oscurange", "raycast", "rose-pine", "sentry",
		"solarized", "temple", "tokyo-night", "vercel", "vscode-plus", "xcode",
	}
	for _, name := range wanted {
		if canonical, ok := canonicalThemeName(name); !ok || canonical != name {
			t.Errorf("Codex dark theme %q is unavailable", name)
		}
	}
}

func TestTokyoNightMatchesCodexPalette(t *testing.T) {
	theme := builtInTheme("tokyo-night")
	if theme.Accent != "#3D59A1" || theme.Text != "#A9B1D6" || theme.Background != "#1A1B26" || theme.Panel != "#16161E" {
		t.Fatalf("Tokyo Night drifted from the Codex palette: %+v", theme)
	}
	if theme.Git != "#FF9E64" || theme.Docker != "#7AA2F7" || theme.Files != "#BB9AF7" || theme.Dev != "#E0AF68" {
		t.Fatalf("Tokyo Night semantic colors drifted: %+v", theme)
	}
}

func TestEveryThemeHasUsableColors(t *testing.T) {
	hexColor := regexp.MustCompile(`^#[0-9A-F]{6}$`)
	for _, theme := range availableThemes() {
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
	if got := nextTheme(themeOrder[len(themeOrder)-1]); got.Preset != themeOrder[0] {
		t.Fatalf("theme cycle wrapped to %q, want %q", got.Preset, themeOrder[0])
	}
}

func TestLegacyThemeAliasLoadsCanonicalPreset(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	directory := filepath.Join(home, ".config", "alias-lens")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "theme.json"), []byte("{\"preset\":\"catppuccin-mocha\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	theme, err := loadTheme()
	if err != nil {
		t.Fatal(err)
	}
	if theme.Preset != "catppuccin" || theme.Name != "Catppuccin" {
		t.Fatalf("legacy theme alias loaded as %+v", theme)
	}
}

func TestEveryThemeRendersTheAliasBrowser(t *testing.T) {
	for _, theme := range availableThemes() {
		applyTheme(theme)
		view := (model{
			aliases: []Alias{{Name: "gs", Command: "git status", Description: "Show repository status"}},
			width:   120,
			height:  24,
			theme:   theme,
		}).View()
		if !strings.Contains(view, theme.Name) || !strings.Contains(view, "gs") {
			t.Errorf("theme %s did not render a complete browser", theme.Preset)
		}
	}
}
