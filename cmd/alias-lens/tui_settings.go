package main

import (
	"unicode/utf8"

	tea "alias-lens/cmd/alias-lens/internal/tea"
)

func (m *model) openFooterSettings() {
	config, err := loadConfig()
	if err != nil {
		m.status = "Could not open footer settings: " + err.Error()
		return
	}
	m.settingsOpen = true
	m.settingsField = 0
	m.settingsBefore = config.Footer
	m.settingsForm = [2]string{config.Footer.Message, config.Footer.Icon}
	m.status = ""
	applyFooterConfig(config.Footer)
}

func (m model) updateFooterSettings(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if matchesShortcut(message, m.shortcutProfile, shortcutSave) {
		return m.saveFooterSettings()
	}
	m.status = ""
	switch message.Type {
	case tea.KeyCtrlC:
		applyFooterConfig(m.settingsBefore)
		return m, tea.Quit
	case tea.KeyEsc:
		applyFooterConfig(m.settingsBefore)
		m.settingsOpen = false
		m.settingsBefore = FooterConfig{}
		m.settingsForm = [2]string{}
		m.status = "Footer unchanged"
		return m, nil
	case tea.KeyTab, tea.KeyDown:
		m.settingsField = (m.settingsField + 1) % len(m.settingsForm)
	case tea.KeyShiftTab, tea.KeyUp:
		m.settingsField = (m.settingsField + len(m.settingsForm) - 1) % len(m.settingsForm)
	case tea.KeyEnter:
		if m.settingsField == len(m.settingsForm)-1 {
			return m.saveFooterSettings()
		}
		m.settingsField++
	case tea.KeyBackspace, tea.KeyDelete:
		if value := m.settingsForm[m.settingsField]; value != "" {
			_, size := utf8.DecodeLastRuneInString(value)
			m.settingsForm[m.settingsField] = value[:len(value)-size]
		}
	case tea.KeySpace:
		if !acceptsTextInput(message) {
			return m, nil
		}
		m.settingsForm[m.settingsField] += " "
	case tea.KeyRunes:
		if !acceptsTextInput(message) {
			return m, nil
		}
		m.settingsForm[m.settingsField] += string(message.Runes)
	default:
		return m, nil
	}
	m.previewFooterSettings()
	return m, nil
}

func (m *model) previewFooterSettings() {
	candidate := FooterConfig{Message: m.settingsForm[0], Icon: m.settingsForm[1]}
	if err := validateFooterConfig(candidate); err != nil {
		m.status = err.Error()
		return
	}
	applyFooterConfig(candidate)
}

func (m model) saveFooterSettings() (tea.Model, tea.Cmd) {
	candidate := FooterConfig{Message: m.settingsForm[0], Icon: m.settingsForm[1]}
	if err := validateFooterConfig(candidate); err != nil {
		m.status = err.Error()
		return m, nil
	}
	config, err := loadConfig()
	if err == nil {
		config.Footer = candidate
		err = saveConfig(config)
	}
	if err != nil {
		m.status = "Could not save the footer: " + err.Error()
		return m, nil
	}
	applyFooterConfig(candidate)
	m.settingsOpen = false
	m.settingsBefore = FooterConfig{}
	m.settingsForm = [2]string{}
	m.status = "Footer saved"
	return m, nil
}
