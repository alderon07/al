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
	if !settings.settingsOpen || !strings.Contains(settings.View(), "Customize appearance") {
		t.Fatalf("settings page did not open: %#v", settings)
	}
	settings.settingsForm = [6]string{"MY ALIASES", "compact", "ascii", "Built {icon} by Sam", "emoji:🚀", "right"}
	saved, _ := settings.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	result := saved.(model)
	if result.settingsOpen || result.status != "Appearance saved" {
		t.Fatalf("settings were not saved: %#v", result)
	}
	config, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Footer.Message != "Built {icon} by Sam" || config.Footer.Icon != "emoji:🚀" {
		t.Fatalf("saved footer = %#v", config.Footer)
	}
	if config.Appearance.Brand != "MY ALIASES" || config.Appearance.ArtStyle != "compact" || config.Appearance.MarkerStyle != "ascii" || config.Footer.Alignment != "right" {
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
		settingsForm:   [6]string{"ALIAS LENS", "full", "symbols", "Made with {icon}", "emoji:🚀🚀", "center"},
		settingsBefore: defaultAppearanceSettings(),
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	result := updated.(model)
	if !result.settingsOpen || !strings.Contains(result.status, "one visible emoji") {
		t.Fatalf("invalid footer settings were accepted: %#v", result)
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
		settingsCursor: [6]int{0, 0, 0, footerGraphemeCount("Made by Naqi"), footerGraphemeCount("none"), 0},
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
	value := "emoji:👨‍👩‍👧‍👦"
	m := model{
		settingsOpen:   true,
		settingsField:  settingsIcon,
		settingsForm:   [6]string{"ALIAS LENS", "full", "symbols", "Family {icon}", value, "center"},
		settingsCursor: [6]int{0, 0, 0, 0, footerGraphemeCount(value), 0},
		settingsBefore: defaultAppearanceSettings(),
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	result := updated.(model)
	if result.settingsForm[settingsIcon] != "emoji:" {
		t.Fatalf("combined emoji backspace = %q", result.settingsForm[settingsIcon])
	}
}

func TestFooterSettingsAcceptsQuestionMarkAsText(t *testing.T) {
	resetAppearanceAfterTest(t)
	m := model{
		settingsOpen:   true,
		settingsField:  settingsMessage,
		settingsForm:   appearanceForm(defaultAppearanceConfig(), defaultFooterConfig()),
		settingsCursor: [6]int{0, 0, 0, footerGraphemeCount("Made by Naqi"), 0, 0},
		settingsBefore: defaultAppearanceSettings(),
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	result := updated.(model)
	if result.helpVisible || !result.settingsOpen || result.settingsForm[settingsMessage] != "Made by Naqi?" {
		t.Fatalf("question mark left the editor or was not inserted: %#v", result)
	}
}

func TestAppearanceChoicesUseArrowKeys(t *testing.T) {
	resetAppearanceAfterTest(t)
	m := model{
		settingsOpen:   true,
		settingsField:  settingsMarkerStyle,
		settingsForm:   appearanceForm(defaultAppearanceConfig(), defaultFooterConfig()),
		settingsBefore: defaultAppearanceSettings(),
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	result := updated.(model)
	if result.settingsForm[settingsMarkerStyle] != "ascii" || activeAppearance.MarkerStyle != "ascii" {
		t.Fatalf("right arrow did not preview the next marker choice: %#v", result)
	}
	updated, _ = result.Update(tea.KeyMsg{Type: tea.KeyLeft})
	result = updated.(model)
	if result.settingsForm[settingsMarkerStyle] != "symbols" || activeAppearance.MarkerStyle != "symbols" {
		t.Fatalf("left arrow did not preview the previous marker choice: %#v", result)
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
			settingsCursor: [6]int{0, 0, 0, 4, 0, 0},
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

func appearanceForm(appearance AppearanceConfig, footer FooterConfig) [6]string {
	return [6]string{appearance.Brand, appearance.ArtStyle, appearance.MarkerStyle, footer.Message, footer.Icon, footer.Alignment}
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
