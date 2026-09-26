package main

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	tea "alias-lens/cmd/alias-lens/internal/tea"
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
	if height := lipgloss.Height(page); height != 10 {
		t.Fatalf("page height = %d, want 10:\n%s", height, page)
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

func TestFooterRemainsInsideViewportAcrossResizes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	applyTheme(builtInTheme("phosphor"))
	applyFooterConfig(defaultFooterConfig())
	t.Cleanup(func() {
		applyTheme(defaultTheme())
		applyFooterConfig(defaultFooterConfig())
	})

	current := model{
		aliases: []Alias{
			{Name: "a", Command: "printf a", Description: "first"},
			{Name: "b", Command: "printf b", Description: "second"},
			{Name: "c", Command: "printf c", Description: "third"},
			{Name: "d", Command: "printf d", Description: "fourth"},
			{Name: "e", Command: "printf e", Description: "fifth"},
			{Name: "f", Command: "printf f", Description: "sixth"},
			{Name: "g", Command: "printf g", Description: "seventh"},
			{Name: "h", Command: "printf h", Description: "eighth"},
		},
		theme:           builtInTheme("phosphor"),
		executeMode:     true,
		shortcutProfile: shortcutLinux,
	}

	for _, size := range []struct{ width, height int }{{120, 36}, {80, 24}, {100, 30}, {48, 18}} {
		updated, _ := current.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
		current = updated.(model)
		view := current.View()
		if gotWidth, gotHeight := lipgloss.Width(view), lipgloss.Height(view); gotWidth != size.width || gotHeight != size.height {
			t.Errorf("%dx%d resize rendered %dx%d", size.width, size.height, gotWidth, gotHeight)
		}
		lines := strings.Split(view, "\n")
		visible := strings.Join(lines[:min(size.height, len(lines))], "\n")
		if !strings.Contains(visible, "Made by Naqi") {
			t.Errorf("%dx%d viewport lost maker footer: rendered height=%d\n%s", size.width, size.height, len(lines), visible)
		}
		if !strings.Contains(visible, "esc quit") {
			t.Errorf("%dx%d viewport lost control footer:\n%s", size.width, size.height, visible)
		}
	}
}

func TestEmptyMakerMessageReclaimsFooterRow(t *testing.T) {
	applyFooterConfig(FooterConfig{Message: "", Icon: "none", Alignment: "right", Tone: "quiet", Rule: "dots"})
	t.Cleanup(func() { applyFooterConfig(defaultFooterConfig()) })

	frame := newMainTUIFrame(48, 18)
	bodyRows := make([]string, frame.contentHeight()-1)
	for index := range bodyRows {
		bodyRows[index] = "body " + strconv.Itoa(index+1)
	}
	view := frame.renderWithFooter(strings.Join(bodyRows, "\n"), "shortcuts")

	if !strings.Contains(view, bodyRows[len(bodyRows)-1]) {
		t.Fatalf("empty maker message clipped the last body row:\n%s", view)
	}
	if row := lineContaining(view, "shortcuts"); row != frame.height-2 {
		t.Fatalf("shortcut row = %d, want %d:\n%s", row, frame.height-2, view)
	}
}

func TestShortcutsSitImmediatelyAboveDottedRule(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	applyTheme(builtInTheme("phosphor"))
	applyFooterConfig(FooterConfig{Message: "Made by Naqi", Icon: "none", Alignment: "right", Tone: "quiet", Rule: "dots"})
	t.Cleanup(func() {
		applyTheme(defaultTheme())
		applyFooterConfig(defaultFooterConfig())
	})

	confirmation := navigationTestModel(pageAliases)
	confirmation.runConfirm = &Alias{Name: "gs", Command: "git status"}
	pages := []struct {
		name  string
		model model
	}{
		{"aliases", navigationTestModel(pageAliases)},
		{"help", navigationTestModel(pageHelp)},
		{"themes", navigationTestModel(pageThemes)},
		{"revisions", navigationTestModel(pageRevisions)},
		{"sync", navigationTestModel(pageSync)},
		{"health", navigationTestModel(pageHealth)},
		{"tour", model{tourVisible: true, theme: builtInTheme("phosphor")}},
		{"add form", model{adding: true, theme: builtInTheme("phosphor")}},
		{"confirmation", confirmation},
	}
	for _, size := range []struct{ width, height int }{{120, 36}, {48, 18}} {
		for _, page := range pages {
			t.Run(page.name+"_"+strconv.Itoa(size.width), func(t *testing.T) {
				page.model.width, page.model.height = size.width, size.height
				view := page.model.View()
				if gotWidth, gotHeight := lipgloss.Width(view), lipgloss.Height(view); gotWidth != size.width || gotHeight != size.height {
					t.Fatalf("rendered %dx%d, want %dx%d", gotWidth, gotHeight, size.width, size.height)
				}
				lines := strings.Split(view, "\n")
				rule := strings.Repeat("·", newMainTUIFrame(size.width, size.height).contentWidth)
				ruleRow := lineContaining(view, rule)
				if ruleRow < 1 || ruleRow >= len(lines)-1 {
					t.Fatalf("dotted footer rule missing or outside viewport: row=%d\n%s", ruleRow, view)
				}
				if strings.TrimSpace(ansi.Strip(lines[ruleRow-1])) == "" {
					t.Fatalf("blank row between shortcuts and dotted rule:\n%s", view)
				}
				if page.name == "aliases" && size.width == 120 {
					if strings.TrimSpace(ansi.Strip(lines[ruleRow-2])) != "" || !strings.Contains(lines[ruleRow-3], "esc quit") {
						t.Fatalf("alias controls and navigation lost their blank separator:\n%s", view)
					}
				}
				if !strings.Contains(lines[ruleRow+1], "Made by Naqi") {
					t.Fatalf("credit is not directly below dotted rule:\n%s", view)
				}
				if size.width == 48 {
					requiredContent := map[string]string{
						"add form": "CATEGORY",
						"tour":     "Open the searchable keyboard guide",
						"sync":     "EXTRA TRACKED FILES",
					}[page.name]
					if requiredContent != "" && !strings.Contains(view, requiredContent) {
						t.Fatalf("compact layout hides %q:\n%s", requiredContent, view)
					}
				}
			})
		}
	}
}

func TestRepositoryPickerShortcutsSitAboveDottedRule(t *testing.T) {
	applyTheme(builtInTheme("phosphor"))
	applyFooterConfig(FooterConfig{Message: "Made by Naqi", Icon: "none", Alignment: "right", Tone: "quiet", Rule: "dots"})
	t.Cleanup(func() {
		applyTheme(defaultTheme())
		applyFooterConfig(defaultFooterConfig())
	})

	for _, size := range []struct{ width, height int }{{120, 36}, {48, 18}} {
		picker := repoPickerModel{width: size.width, height: size.height}
		view := picker.View()
		if gotWidth, gotHeight := lipgloss.Width(view), lipgloss.Height(view); gotWidth != size.width || gotHeight != size.height {
			t.Fatalf("%dx%d picker rendered %dx%d", size.width, size.height, gotWidth, gotHeight)
		}
		lines := strings.Split(view, "\n")
		rule := strings.Repeat("·", newMainTUIFrame(size.width, size.height).contentWidth)
		ruleRow := lineContaining(view, rule)
		if ruleRow < 1 || !strings.Contains(lines[ruleRow-1], "esc cancel") {
			t.Fatalf("%dx%d picker shortcuts are not beside the dotted rule:\n%s", size.width, size.height, view)
		}
	}
}

func TestMainPagesUseOneWideContentColumn(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	applyTheme(builtInTheme("phosphor"))
	applyFooterConfig(FooterConfig{Message: "Made by Naqi", Icon: "none", Alignment: "center", Tone: "quiet", Rule: "thin"})
	t.Cleanup(func() {
		applyTheme(defaultTheme())
		applyFooterConfig(defaultFooterConfig())
	})

	pages := []struct {
		name      string
		model     model
		makerRule bool
	}{
		{"aliases", navigationTestModel(pageAliases), true},
		{"help", navigationTestModel(pageHelp), true},
		{"stats", navigationTestModel(pageStats), true},
		{"settings", navigationTestModel(pageSettings), false},
		{"themes", navigationTestModel(pageThemes), true},
		{"revisions", navigationTestModel(pageRevisions), true},
		{"sync", navigationTestModel(pageSync), true},
		{"health", navigationTestModel(pageHealth), true},
	}
	headerColumn := -1
	for _, page := range pages {
		page.model.width = 160
		page.model.height = 40
		view := page.model.View()
		if gotWidth, gotHeight := lipgloss.Width(view), lipgloss.Height(view); gotWidth != 160 || gotHeight != 40 {
			t.Errorf("%s page rendered %dx%d, want 160x40", page.name, gotWidth, gotHeight)
		}
		pageHeaderColumn := -1
		for _, line := range strings.Split(view, "\n") {
			if column := strings.Index(line, "ALIAS LENS"); column >= 0 {
				pageHeaderColumn = column
				break
			}
		}
		if headerColumn < 0 {
			headerColumn = pageHeaderColumn
		}
		if pageHeaderColumn != headerColumn {
			t.Errorf("%s page header starts at column %d, want shared column %d", page.name, pageHeaderColumn, headerColumn)
		}
		if !page.makerRule {
			continue
		}
		longestRule := 0
		for _, line := range strings.Split(view, "\n") {
			longestRule = max(longestRule, strings.Count(line, "─"))
		}
		if longestRule != mainTUIMaxContentWidth {
			t.Errorf("%s page divider width = %d, want %d", page.name, longestRule, mainTUIMaxContentWidth)
		}
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
	if height := lipgloss.Height(page); height != 10 {
		t.Fatalf("page with footer rule height = %d, want 10:\n%s", height, page)
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
