package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestMakerCreditIsCenteredAtTheRequestedWidth(t *testing.T) {
	applyTheme(builtInTheme("phosphor"))
	applyFooterConfig(defaultFooterConfig())
	t.Cleanup(func() {
		applyTheme(defaultTheme())
		applyFooterConfig(defaultFooterConfig())
	})

	credit := makerCredit(40)
	if width := lipgloss.Width(credit); width != 40 {
		t.Fatalf("maker credit width = %d, want 40: %q", width, credit)
	}
	assertMakerCredit(t, credit)
}

func TestMakerCreditIsPinnedToTheLastPageRow(t *testing.T) {
	page := pageWithMaker("top", 40, 10)
	if height := lipgloss.Height(page); height != 9 {
		t.Fatalf("page height = %d, want 9:\n%s", height, page)
	}
	lines := strings.Split(page, "\n")
	assertMakerCredit(t, lines[len(lines)-1])
}

func TestFooterControlsAndNavigationHaveOneBlankRowBetweenThem(t *testing.T) {
	footer := footerWithNavigation("controls", 100, shortcutLinux)
	lines := strings.Split(footer, "\n")
	if len(lines) != 3 || lines[0] != "controls" || lines[1] != "" || !strings.Contains(lines[2], " help") {
		t.Fatalf("footer rows are not separated by one blank row:\n%s", footer)
	}
}

func TestAliasAndStatsViewsPinMakerCreditToTheSameRow(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	applyTheme(builtInTheme("phosphor"))
	applyFooterConfig(defaultFooterConfig())
	t.Cleanup(func() {
		applyTheme(defaultTheme())
		applyFooterConfig(defaultFooterConfig())
	})

	base := model{width: 120, height: 36, theme: builtInTheme("phosphor"), shortcutProfile: shortcutLinux}
	aliasView := base.View()
	base.statsOpen = true
	statsView := base.View()

	aliasRow := lineContaining(aliasView, "Made by Naqi")
	statsRow := lineContaining(statsView, "Made by Naqi")
	if aliasRow < 0 || statsRow < 0 {
		t.Fatalf("maker credit missing: alias row=%d stats row=%d", aliasRow, statsRow)
	}
	if aliasRow != statsRow {
		t.Fatalf("maker credit rows differ: aliases=%d/%d stats=%d/%d", aliasRow, lipgloss.Height(aliasView), statsRow, lipgloss.Height(statsView))
	}
}

func TestPlainAliasListOmitsMakerCredit(t *testing.T) {
	var output bytes.Buffer
	printPlainAliasList(&output, []Alias{{Name: "gs", Command: "git status"}}, "")
	if strings.Contains(output.String(), "Made with") || strings.Contains(output.String(), "♥") {
		t.Fatalf("plain alias output contains the TUI maker credit: %q", output.String())
	}
}

func TestMakerCreditFitsTheMinimumInteractiveTerminal(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	applyTheme(builtInTheme("phosphor"))
	t.Cleanup(func() { applyTheme(defaultTheme()) })

	view := (model{width: 50, height: 18}).View()
	assertMakerCredit(t, view)
	if !strings.Contains(view, "esc quit") {
		t.Fatalf("minimum-size view lost its control footer:\n%s", view)
	}
}

func TestFooterConfigurationRendersCustomMessageAndBitmap(t *testing.T) {
	custom := FooterConfig{Message: "Built {icon} here", Icon: "#..#/.##."}
	applyFooterConfig(custom)
	t.Cleanup(func() { applyFooterConfig(defaultFooterConfig()) })

	credit := makerCredit(40)
	if !strings.Contains(credit, "Built ") || !strings.Contains(credit, " here") {
		t.Fatalf("custom footer message was not rendered: %q", credit)
	}
	icon, _ := parsePixelIcon(custom.Icon)
	if !strings.Contains(credit, renderPixelIcon(icon)) {
		t.Fatalf("custom footer bitmap was not rendered: %q", credit)
	}
}

func TestFooterConfigurationRendersOneEmojiGrapheme(t *testing.T) {
	for _, value := range []string{"emoji:🚀", "emoji:👍🏽", "emoji:❤️"} {
		t.Run(value, func(t *testing.T) {
			config := FooterConfig{Message: "Built {icon} here", Icon: value}
			if err := validateFooterConfig(config); err != nil {
				t.Fatal(err)
			}
			applyFooterConfig(config)
			glyph := strings.TrimPrefix(value, "emoji:")
			if credit := makerCredit(40); !strings.Contains(credit, glyph) {
				t.Fatalf("emoji footer icon was not rendered: %q", credit)
			}
		})
	}
	applyFooterConfig(defaultFooterConfig())
}

func TestNamedFooterIconUsesSymbolInsteadOfBitmap(t *testing.T) {
	icon, err := renderFooterIcon("heart")
	if err != nil {
		t.Fatal(err)
	}
	if icon != "♥︎" || icon == renderPixelIcon(iconHeart) {
		t.Fatalf("named heart icon = %q, want symbol", icon)
	}
	if width := lipgloss.Width(icon); width != 1 {
		t.Fatalf("heart icon width = %d, want 1", width)
	}
}

func TestHeartFooterPreservesFollowingSpace(t *testing.T) {
	config := FooterConfig{Message: "Made with {icon} by Naqi", Icon: "heart", Alignment: "center", Tone: "quiet", Rule: "none"}
	preview := renderFooterPreview(config, 40)
	if !strings.Contains(preview, "♥︎\u00a0by") {
		t.Fatalf("preview lost the space after the heart: %q", preview)
	}

	applyFooterConfig(config)
	t.Cleanup(func() { applyFooterConfig(defaultFooterConfig()) })
	if credit := makerCredit(40); !strings.Contains(credit, "♥︎\u00a0by") {
		t.Fatalf("saved footer lost the space after the heart: %q", credit)
	}
}

func TestFooterSpacingCompensationOnlyAppliesToHeart(t *testing.T) {
	if got := renderFooterMessage("Made with {icon} by Naqi", "spark"); got != "Made with ✦ by Naqi" {
		t.Fatalf("spark message = %q", got)
	}
	if got := renderFooterMessage("Made with {icon}by Naqi", "heart"); got != "Made with ♥︎by Naqi" {
		t.Fatalf("unspaced heart message = %q", got)
	}
}

func TestFooterConfigurationValidation(t *testing.T) {
	tests := []FooterConfig{
		{Message: "bad\nmessage", Icon: "heart"},
		{Message: strings.Repeat("x", 81), Icon: "heart"},
		{Message: "two {icon} tokens {icon}", Icon: "heart"},
		{Message: "bad {icon}", Icon: "###/...."},
		{Message: "bad {icon}", Icon: "####/..x."},
		{Message: "bad {icon}", Icon: "emoji:"},
		{Message: "bad {icon}", Icon: "emoji:🚀🚀"},
		{Message: "bad tone", Icon: "none", Tone: "loud"},
		{Message: "bad rule", Icon: "none", Rule: "double"},
	}
	for _, config := range tests {
		if err := validateFooterConfig(config); err == nil {
			t.Errorf("invalid footer config was accepted: %#v", config)
		}
	}
}

func TestFooterToneAndRuleRender(t *testing.T) {
	applyFooterConfig(FooterConfig{Message: "Built here", Icon: "none", Alignment: "right", Tone: "bright", Rule: "dots"})
	t.Cleanup(func() { applyFooterConfig(defaultFooterConfig()) })

	credit := makerCredit(24)
	if !strings.Contains(credit, strings.Repeat("·", 24)) || !strings.Contains(credit, "Built here") {
		t.Fatalf("styled footer was not rendered:\n%s", credit)
	}
	if lipgloss.Height(credit) != 2 || lipgloss.Width(credit) != 24 {
		t.Fatalf("styled footer size = %dx%d, want 24x2:\n%s", lipgloss.Width(credit), lipgloss.Height(credit), credit)
	}
}

func TestFooterAccentToneFollowsActiveTheme(t *testing.T) {
	t.Cleanup(func() { applyTheme(defaultTheme()) })
	for _, name := range []string{"tokyo-night", "dracula"} {
		theme := builtInTheme(name)
		applyTheme(theme)
		if got, want := footerToneStyle("accent").GetForeground(), lipgloss.Color(theme.Accent); got != want {
			t.Errorf("%s footer accent = %v, want %v", name, got, want)
		}
	}
}

func TestFooterRuleStaysInsidePageHeight(t *testing.T) {
	applyFooterConfig(FooterConfig{Message: "Built here", Icon: "none", Alignment: "center", Tone: "quiet", Rule: "thin"})
	t.Cleanup(func() { applyFooterConfig(defaultFooterConfig()) })

	page := pageWithMaker("top", 40, 10)
	if height := lipgloss.Height(page); height != 9 {
		t.Fatalf("page with footer rule height = %d, want 9:\n%s", height, page)
	}
}

func TestFooterConfigCommandsWriteAndResetSettings(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := runConfigCommand([]string{"footer-message", "Built {icon} by Sam"}); err != nil {
		t.Fatal(err)
	}
	if err := runConfigCommand([]string{"footer-icon", "spark"}); err != nil {
		t.Fatal(err)
	}
	config, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Footer.Message != "Built {icon} by Sam" || config.Footer.Icon != "spark" {
		t.Fatalf("saved footer config = %#v", config.Footer)
	}
	if err := runConfigCommand([]string{"footer-reset"}); err != nil {
		t.Fatal(err)
	}
	config, err = loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Footer != defaultFooterConfig() {
		t.Fatalf("reset footer config = %#v", config.Footer)
	}
}

func assertMakerCredit(t *testing.T, view string) {
	t.Helper()
	if !strings.Contains(view, "Made by Naqi") {
		t.Fatalf("view lacks the default maker credit:\n%s", view)
	}
	if strings.Contains(view, renderPixelIcon(iconHeart)) {
		t.Fatalf("default maker credit contains bitmap art:\n%s", view)
	}
}

func lineContaining(view, text string) int {
	for index, line := range strings.Split(view, "\n") {
		if strings.Contains(line, text) {
			return index
		}
	}
	return -1
}
