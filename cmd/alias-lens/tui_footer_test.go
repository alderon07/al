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
	if icon != "♥" || icon == renderPixelIcon(iconHeart) {
		t.Fatalf("named heart icon = %q, want symbol", icon)
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
