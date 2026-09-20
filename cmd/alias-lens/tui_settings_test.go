package main

import (
	"strings"
	"testing"

	tea "alias-lens/cmd/alias-lens/internal/tea"
)

func TestFooterSettingsPageSavesValidChanges(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := saveConfig(defaultConfig()); err != nil {
		t.Fatal(err)
	}
	applyFooterConfig(defaultFooterConfig())
	t.Cleanup(func() { applyFooterConfig(defaultFooterConfig()) })

	m := model{width: 90, height: 24, shortcutProfile: shortcutLinux}
	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyF3})
	settings := opened.(model)
	if !settings.settingsOpen || !strings.Contains(settings.View(), "Compose your footer") {
		t.Fatalf("settings page did not open: %#v", settings)
	}
	settings.settingsForm = [settingsFieldCount]string{"Built {icon} by Sam", "heart", "right", "accent", "dots"}
	saved, _ := settings.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	result := saved.(model)
	if result.settingsOpen || result.status != "Footer saved" {
		t.Fatalf("settings were not saved: %#v", result)
	}
	config, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Footer.Message != "Built {icon} by Sam" || config.Footer.Icon != "heart" {
		t.Fatalf("saved footer = %#v", config.Footer)
	}
	if config.Appearance != defaultAppearanceConfig() || config.Footer.Alignment != "right" || config.Footer.Tone != "accent" || config.Footer.Rule != "dots" {
		t.Fatalf("saved appearance = %#v footer=%#v", config.Appearance, config.Footer)
	}
}

func TestFooterSettingsPageKeepsInvalidInputOpen(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := saveConfig(defaultConfig()); err != nil {
		t.Fatal(err)
	}
	m := model{
		settingsOpen:   true,
		settingsField:  settingsIcon,
		settingsForm:   [settingsFieldCount]string{"Made with {icon}", "emoji:🚀🚀", "center", "quiet", "none"},
		settingsBefore: defaultAppearanceSettings(),
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	result := updated.(model)
	if !result.settingsOpen || !strings.Contains(result.status, "one visible emoji") {
		t.Fatalf("invalid footer settings were accepted: %#v", result)
	}
	if result.settingsField != settingsIcon {
		t.Fatalf("invalid icon did not receive focus: field=%d", result.settingsField)
	}
}

func TestFooterSettingsCancelRestoresSavedPreview(t *testing.T) {
	saved := defaultFooterConfig()
	preview := FooterConfig{Message: "Preview {icon}", Icon: "spark", Alignment: "right"}
	applyFooterConfig(preview)
	t.Cleanup(func() { applyFooterConfig(defaultFooterConfig()) })
	m := model{settingsOpen: true, settingsBefore: appearanceSettings{Appearance: defaultAppearanceConfig(), Footer: saved}, settingsForm: appearanceForm(defaultAppearanceConfig(), preview)}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	result := updated.(model)
	if result.settingsOpen || activeFooter != saved {
		t.Fatalf("cancel did not restore saved footer: model=%#v footer=%#v", result, activeFooter)
	}
}

func TestFooterSettingsPageSwitchRestoresSavedPreview(t *testing.T) {
	saved := defaultFooterConfig()
	preview := FooterConfig{Message: "Preview {icon}", Icon: "spark", Alignment: "right"}
	applyFooterConfig(preview)
	t.Cleanup(func() { applyFooterConfig(defaultFooterConfig()) })
	m := model{
		settingsOpen:    true,
		settingsBefore:  appearanceSettings{Appearance: defaultAppearanceConfig(), Footer: saved},
		settingsForm:    appearanceForm(defaultAppearanceConfig(), preview),
		shortcutProfile: shortcutLinux,
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyF2})
	result := updated.(model)
	if result.currentPage() != pageStats || activeFooter != saved {
		t.Fatalf("page switch did not restore saved footer: page=%v footer=%#v", result.currentPage(), activeFooter)
	}
}

func TestFooterSettingsFieldsSupportNormalEditing(t *testing.T) {
	resetAppearanceAfterTest(t)
	m := model{
		settingsOpen:   true,
		settingsField:  settingsMessage,
		settingsForm:   appearanceForm(defaultAppearanceConfig(), defaultFooterConfig()),
		settingsCursor: [settingsFieldCount]int{footerGraphemeCount("Made by Naqi"), 0, 0, 0, 0},
		settingsBefore: defaultAppearanceSettings(),
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	m = updated.(model)
	if m.settingsForm[settingsMessage] != "" || m.settingsCursor[settingsMessage] != 0 {
		t.Fatalf("clear left message=%q cursor=%d", m.settingsForm[settingsMessage], m.settingsCursor[settingsMessage])
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Hello world")})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("!")})
	m = updated.(model)
	if m.settingsForm[settingsMessage] != "Hello worl!d" {
		t.Fatalf("insert at cursor = %q", m.settingsForm[settingsMessage])
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = updated.(model)
	if m.settingsForm[settingsMessage] != "Hello world" {
		t.Fatalf("backspace at cursor = %q", m.settingsForm[settingsMessage])
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDelete})
	m = updated.(model)
	if m.settingsForm[settingsMessage] != "ello world" || m.settingsCursor[settingsMessage] != 0 {
		t.Fatalf("delete at cursor = %q cursor=%d", m.settingsForm[settingsMessage], m.settingsCursor[settingsMessage])
	}
}

func TestFooterSettingsBackspaceRemovesOneVisibleGrapheme(t *testing.T) {
	resetAppearanceAfterTest(t)
	value := "Family 👨‍👩‍👧‍👦"
	m := model{
		settingsOpen:   true,
		settingsField:  settingsMessage,
		settingsForm:   [settingsFieldCount]string{value, "none", "center", "quiet", "none"},
		settingsCursor: [settingsFieldCount]int{footerGraphemeCount(value), 0, 0, 0, 0},
		settingsBefore: defaultAppearanceSettings(),
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	result := updated.(model)
	if result.settingsForm[settingsMessage] != "Family " {
		t.Fatalf("combined emoji backspace = %q", result.settingsForm[settingsMessage])
	}
}

func TestFooterSettingsAcceptsQuestionMarkAsText(t *testing.T) {
	resetAppearanceAfterTest(t)
	m := model{
		settingsOpen:   true,
		settingsField:  settingsMessage,
		settingsForm:   appearanceForm(defaultAppearanceConfig(), defaultFooterConfig()),
		settingsCursor: [settingsFieldCount]int{footerGraphemeCount("Made by Naqi"), 0, 0, 0, 0},
		settingsBefore: defaultAppearanceSettings(),
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	result := updated.(model)
	if result.helpVisible || !result.settingsOpen || result.settingsForm[settingsMessage] != "Made by Naqi?" {
		t.Fatalf("question mark left the editor or was not inserted: %#v", result)
	}
}

func TestFooterChoicesUseArrowKeys(t *testing.T) {
	resetAppearanceAfterTest(t)
	m := model{
		settingsOpen:   true,
		settingsField:  settingsTone,
		settingsForm:   appearanceForm(defaultAppearanceConfig(), defaultFooterConfig()),
		settingsBefore: defaultAppearanceSettings(),
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	result := updated.(model)
	if result.settingsForm[settingsTone] != "accent" || activeFooter.Tone != "accent" {
		t.Fatalf("right arrow did not preview the next tone: %#v", result)
	}
	updated, _ = result.Update(tea.KeyMsg{Type: tea.KeyLeft})
	result = updated.(model)
	if result.settingsForm[settingsTone] != "quiet" || activeFooter.Tone != "quiet" {
		t.Fatalf("left arrow did not preview the previous tone: %#v", result)
	}
}

func TestAppearanceUsesFixedProductName(t *testing.T) {
	resetAppearanceAfterTest(t)
	legacy := defaultAppearanceConfig()
	legacy.Brand = "MY ALIASES"
	applyAppearanceConfig(legacy)
	if activeAppearance.Brand != "ALIAS LENS" {
		t.Fatalf("legacy brand override is active: %q", activeAppearance.Brand)
	}

	view := model{
		width:         90,
		height:        30,
		settingsOpen:  true,
		settingsField: settingsMessage,
		settingsForm:  appearanceForm(legacy, defaultFooterConfig()),
	}.View()
	if strings.Contains(view, "Brand art") || strings.Contains(view, "Markers:") || strings.Contains(view, "MY ALIASES") {
		t.Fatalf("appearance editor still exposes the product name:\n%s", view)
	}
}

func TestAppearanceChoicesShowOptionsAndUseSpace(t *testing.T) {
	resetAppearanceAfterTest(t)
	m := model{
		width:          90,
		height:         30,
		settingsOpen:   true,
		settingsField:  settingsIcon,
		settingsForm:   appearanceForm(defaultAppearanceConfig(), defaultFooterConfig()),
		settingsBefore: defaultAppearanceSettings(),
	}
	view := m.View()
	for _, expected := range []string{"[none]", "heart", "spark", "brand", "alias", "command", "stats", "sync", "theme"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("icon picker is missing %q:\n%s", expected, view)
		}
	}
	if strings.Contains(view, "#..#") || strings.Contains(view, "bitmap") {
		t.Fatalf("icon picker still advertises bitmap input:\n%s", view)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	result := updated.(model)
	if result.settingsForm[settingsIcon] != "heart" {
		t.Fatalf("space selected %q, want heart", result.settingsForm[settingsIcon])
	}
}

func TestFooterSettingsActionRowsSaveAndCancel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := saveConfig(defaultConfig()); err != nil {
		t.Fatal(err)
	}
	m := model{
		settingsOpen:   true,
		settingsField:  settingsSave,
		settingsForm:   [settingsFieldCount]string{"Made by me", "none", "left", "bright", "thin"},
		settingsBefore: defaultAppearanceSettings(),
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	saved := updated.(model)
	if saved.settingsOpen || saved.status != "Footer saved" {
		t.Fatalf("save row did not save: %#v", saved)
	}

	preview := FooterConfig{Message: "Preview", Icon: "none", Alignment: "right"}
	applyFooterConfig(preview)
	m = model{
		settingsOpen:   true,
		settingsField:  settingsCancel,
		settingsForm:   appearanceForm(defaultAppearanceConfig(), preview),
		settingsBefore: defaultAppearanceSettings(),
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	cancelled := updated.(model)
	if cancelled.settingsOpen || cancelled.status != "Footer unchanged" || activeFooter != defaultFooterConfig() {
		t.Fatalf("cancel row did not restore settings: model=%#v footer=%#v", cancelled, activeFooter)
	}
}

func TestFooterSettingsShowsCursorAndControlsAtSupportedSizes(t *testing.T) {
	resetAppearanceAfterTest(t)
	for _, size := range []struct{ width, height int }{{48, 18}, {160, 40}} {
		m := model{
			width:          size.width,
			height:         size.height,
			settingsOpen:   true,
			settingsField:  settingsMessage,
			settingsForm:   appearanceForm(defaultAppearanceConfig(), defaultFooterConfig()),
			settingsCursor: [settingsFieldCount]int{4, 0, 0, 0, 0},
		}
		view := m.View()
		for _, expected := range []string{"Made| by Naqi", "Ctrl+U clear", "Esc", "cancel"} {
			if !strings.Contains(view, expected) {
				t.Errorf("%dx%d editor is missing %q:\n%s", size.width, size.height, expected, view)
			}
		}
	}
}

func defaultAppearanceSettings() appearanceSettings {
	return appearanceSettings{Appearance: defaultAppearanceConfig(), Footer: defaultFooterConfig()}
}

func appearanceForm(appearance AppearanceConfig, footer FooterConfig) [settingsFieldCount]string {
	return [settingsFieldCount]string{footer.Message, footer.Icon, footer.Alignment, footer.Tone, footer.Rule}
}

func resetAppearanceAfterTest(t *testing.T) {
	t.Helper()
	applyAppearanceConfig(defaultAppearanceConfig())
	applyFooterConfig(defaultFooterConfig())
	t.Cleanup(func() {
		applyAppearanceConfig(defaultAppearanceConfig())
		applyFooterConfig(defaultFooterConfig())
	})
}
