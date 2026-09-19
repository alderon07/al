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
	if !settings.settingsOpen || !strings.Contains(settings.View(), "Customize the footer") {
		t.Fatalf("settings page did not open: %#v", settings)
	}
	settings.settingsForm = [2]string{"Built {icon} by Sam", "emoji:🚀"}
	saved, _ := settings.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	result := saved.(model)
	if result.settingsOpen || result.status != "Footer saved" {
		t.Fatalf("settings were not saved: %#v", result)
	}
	config, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Footer.Message != "Built {icon} by Sam" || config.Footer.Icon != "emoji:🚀" {
		t.Fatalf("saved footer = %#v", config.Footer)
	}
}

func TestFooterSettingsPageKeepsInvalidInputOpen(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := saveConfig(defaultConfig()); err != nil {
		t.Fatal(err)
	}
	m := model{
		settingsOpen:   true,
		settingsField:  1,
		settingsForm:   [2]string{"Made with {icon}", "emoji:🚀🚀"},
		settingsBefore: defaultFooterConfig(),
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	result := updated.(model)
	if !result.settingsOpen || !strings.Contains(result.status, "one visible terminal grapheme") {
		t.Fatalf("invalid footer settings were accepted: %#v", result)
	}
}

func TestFooterSettingsCancelRestoresSavedPreview(t *testing.T) {
	saved := defaultFooterConfig()
	preview := FooterConfig{Message: "Preview {icon}", Icon: "spark"}
	applyFooterConfig(preview)
	t.Cleanup(func() { applyFooterConfig(defaultFooterConfig()) })
	m := model{settingsOpen: true, settingsBefore: saved, settingsForm: [2]string{preview.Message, preview.Icon}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	result := updated.(model)
	if result.settingsOpen || activeFooter != saved {
		t.Fatalf("cancel did not restore saved footer: model=%#v footer=%#v", result, activeFooter)
	}
}

func TestFooterSettingsPageSwitchRestoresSavedPreview(t *testing.T) {
	saved := defaultFooterConfig()
	preview := FooterConfig{Message: "Preview {icon}", Icon: "spark"}
	applyFooterConfig(preview)
	t.Cleanup(func() { applyFooterConfig(defaultFooterConfig()) })
	m := model{
		settingsOpen:    true,
		settingsBefore:  saved,
		settingsForm:    [2]string{preview.Message, preview.Icon},
		shortcutProfile: shortcutLinux,
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyF2})
	result := updated.(model)
	if result.currentPage() != pageStats || activeFooter != saved {
		t.Fatalf("page switch did not restore saved footer: page=%v footer=%#v", result.currentPage(), activeFooter)
	}
}
