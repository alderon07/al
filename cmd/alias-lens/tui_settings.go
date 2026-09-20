package main

import (
	"strings"

	tea "alias-lens/cmd/alias-lens/internal/tea"
	"github.com/rivo/uniseg"
)

const (
	settingsMessage = iota
	settingsIcon
	settingsAlignment
	settingsTone
	settingsRule
	settingsFieldCount
)

const (
	settingsSave = settingsFieldCount + iota
	settingsCancel
	settingsRowCount
)

type appearanceSettings struct {
	Appearance AppearanceConfig
	Footer     FooterConfig
}

func (m *model) openFooterSettings() {
	config, err := loadConfig()
	if err != nil {
		m.status = "Could not open footer settings: " + err.Error()
		return
	}
	m.settingsOpen = true
	m.settingsField = 0
	m.settingsBefore = appearanceSettings{Appearance: config.Appearance, Footer: config.Footer}
	m.settingsForm = [settingsFieldCount]string{config.Footer.Message, config.Footer.Icon, config.Footer.Alignment, config.Footer.Tone, config.Footer.Rule}
	for index, value := range m.settingsForm {
		m.settingsCursor[index] = footerGraphemeCount(value)
	}
	m.status = ""
	applyAppearanceConfig(config.Appearance)
	applyFooterConfig(config.Footer)
}

func footerSettingsHandles(message tea.KeyMsg, profile ShortcutProfile) bool {
	if matchesShortcut(message, profile, shortcutSave) {
		return true
	}
	switch message.Type {
	case tea.KeyCtrlC, tea.KeyCtrlU, tea.KeyEsc, tea.KeyTab, tea.KeyShiftTab, tea.KeyUp, tea.KeyDown,
		tea.KeyEnter, tea.KeyBackspace, tea.KeyDelete, tea.KeyLeft, tea.KeyRight, tea.KeyHome, tea.KeyEnd,
		tea.KeySpace, tea.KeyRunes:
		return true
	default:
		return false
	}
}

func (m model) updateFooterSettings(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if matchesShortcut(message, m.shortcutProfile, shortcutSave) {
		return m.saveFooterSettings()
	}
	m.status = ""
	switch message.Type {
	case tea.KeyCtrlC:
		applyAppearanceConfig(m.settingsBefore.Appearance)
		applyFooterConfig(m.settingsBefore.Footer)
		return m, tea.Quit
	case tea.KeyEsc:
		return m.cancelFooterSettings()
	case tea.KeyTab, tea.KeyDown:
		m.settingsField = (m.settingsField + 1) % settingsRowCount
	case tea.KeyShiftTab, tea.KeyUp:
		m.settingsField = (m.settingsField + settingsRowCount - 1) % settingsRowCount
	case tea.KeyEnter:
		switch m.settingsField {
		case settingsSave:
			return m.saveFooterSettings()
		case settingsCancel:
			return m.cancelFooterSettings()
		default:
			m.settingsField++
		}
	case tea.KeyLeft:
		if m.settingsField == settingsSave || m.settingsField == settingsCancel {
			m.settingsField = settingsSave + (m.settingsField-settingsSave+1)%2
		} else if choices := settingsFieldChoices(m.settingsField); len(choices) > 0 {
			m.settingsForm[m.settingsField] = cycleSettingChoice(m.settingsForm[m.settingsField], choices, -1)
		} else if isSettingsValueField(m.settingsField) {
			m.settingsCursor[m.settingsField] = max(0, m.settingsCursor[m.settingsField]-1)
		}
	case tea.KeyRight:
		if m.settingsField == settingsSave || m.settingsField == settingsCancel {
			m.settingsField = settingsSave + (m.settingsField-settingsSave+1)%2
		} else if choices := settingsFieldChoices(m.settingsField); len(choices) > 0 {
			m.settingsForm[m.settingsField] = cycleSettingChoice(m.settingsForm[m.settingsField], choices, 1)
		} else if isSettingsValueField(m.settingsField) {
			m.settingsCursor[m.settingsField] = min(footerGraphemeCount(m.settingsForm[m.settingsField]), m.settingsCursor[m.settingsField]+1)
		}
	case tea.KeyHome:
		if !isSettingsValueField(m.settingsField) {
			return m, nil
		}
		if choices := settingsFieldChoices(m.settingsField); len(choices) > 0 {
			m.settingsForm[m.settingsField] = choices[0]
		} else {
			m.settingsCursor[m.settingsField] = 0
		}
	case tea.KeyEnd:
		if !isSettingsValueField(m.settingsField) {
			return m, nil
		}
		if choices := settingsFieldChoices(m.settingsField); len(choices) > 0 {
			m.settingsForm[m.settingsField] = choices[len(choices)-1]
		} else {
			m.settingsCursor[m.settingsField] = footerGraphemeCount(m.settingsForm[m.settingsField])
		}
	case tea.KeyCtrlU:
		if !isSettingsValueField(m.settingsField) || len(settingsFieldChoices(m.settingsField)) > 0 {
			return m, nil
		}
		m.settingsForm[m.settingsField] = ""
		m.settingsCursor[m.settingsField] = 0
	case tea.KeyBackspace:
		if !isSettingsValueField(m.settingsField) || len(settingsFieldChoices(m.settingsField)) > 0 {
			return m, nil
		}
		m.settingsForm[m.settingsField], m.settingsCursor[m.settingsField] = deleteFooterGrapheme(
			m.settingsForm[m.settingsField], m.settingsCursor[m.settingsField], true,
		)
	case tea.KeyDelete:
		if !isSettingsValueField(m.settingsField) || len(settingsFieldChoices(m.settingsField)) > 0 {
			return m, nil
		}
		m.settingsForm[m.settingsField], m.settingsCursor[m.settingsField] = deleteFooterGrapheme(
			m.settingsForm[m.settingsField], m.settingsCursor[m.settingsField], false,
		)
	case tea.KeySpace:
		if choices := settingsFieldChoices(m.settingsField); len(choices) > 0 {
			m.settingsForm[m.settingsField] = cycleSettingChoice(m.settingsForm[m.settingsField], choices, 1)
			break
		}
		if !isSettingsValueField(m.settingsField) || !acceptsTextInput(message) {
			return m, nil
		}
		m.settingsForm[m.settingsField], m.settingsCursor[m.settingsField] = insertFooterText(
			m.settingsForm[m.settingsField], m.settingsCursor[m.settingsField], " ",
		)
	case tea.KeyRunes:
		if !isSettingsValueField(m.settingsField) || !acceptsTextInput(message) || len(settingsFieldChoices(m.settingsField)) > 0 {
			return m, nil
		}
		m.settingsForm[m.settingsField], m.settingsCursor[m.settingsField] = insertFooterText(
			m.settingsForm[m.settingsField], m.settingsCursor[m.settingsField], string(message.Runes),
		)
	default:
		return m, nil
	}
	m.previewFooterSettings()
	return m, nil
}

func isSettingsValueField(field int) bool {
	return field >= settingsMessage && field < settingsFieldCount
}

func settingsFieldChoices(field int) []string {
	switch field {
	case settingsIcon:
		return []string{"none", "heart", "spark", "brand", "alias", "command", "stats", "sync", "theme"}
	case settingsAlignment:
		return []string{"left", "center", "right"}
	case settingsTone:
		return []string{"quiet", "accent", "bright"}
	case settingsRule:
		return []string{"none", "thin", "dots"}
	default:
		return nil
	}
}

func cycleSettingChoice(current string, choices []string, direction int) string {
	index := -1
	for choiceIndex, choice := range choices {
		if choice == current {
			index = choiceIndex
			break
		}
	}
	if index < 0 {
		return choices[0]
	}
	index = (index + direction + len(choices)) % len(choices)
	return choices[index]
}

func (m *model) previewFooterSettings() {
	appearance, footer := m.appearanceCandidates()
	if err := validateAppearanceConfig(appearance); err != nil {
		m.status = err.Error()
		return
	}
	if err := validateFooterConfig(footer); err != nil {
		m.status = err.Error()
		return
	}
	applyAppearanceConfig(appearance)
	applyFooterConfig(footer)
}

func (m model) saveFooterSettings() (tea.Model, tea.Cmd) {
	appearance, footer := m.appearanceCandidates()
	if err := validateAppearanceConfig(appearance); err != nil {
		m.status = err.Error()
		m.settingsField = invalidAppearanceField(appearance)
		return m, nil
	}
	if err := validateFooterConfig(footer); err != nil {
		m.status = err.Error()
		m.settingsField = invalidFooterField(footer)
		return m, nil
	}
	config, err := loadConfig()
	if err == nil {
		config.Appearance = appearance
		config.Footer = footer
		err = saveConfig(config)
	}
	if err != nil {
		m.status = "Could not save footer settings: " + err.Error()
		return m, nil
	}
	applyAppearanceConfig(appearance)
	applyFooterConfig(footer)
	m.settingsOpen = false
	m.settingsBefore = appearanceSettings{}
	m.settingsForm = [settingsFieldCount]string{}
	m.settingsCursor = [settingsFieldCount]int{}
	m.status = "Footer saved"
	return m, nil
}

func (m model) cancelFooterSettings() (tea.Model, tea.Cmd) {
	applyAppearanceConfig(m.settingsBefore.Appearance)
	applyFooterConfig(m.settingsBefore.Footer)
	m.settingsOpen = false
	m.settingsBefore = appearanceSettings{}
	m.settingsForm = [settingsFieldCount]string{}
	m.settingsCursor = [settingsFieldCount]int{}
	m.status = "Footer unchanged"
	return m, nil
}

func invalidAppearanceField(config AppearanceConfig) int {
	return settingsMessage
}

func invalidFooterField(config FooterConfig) int {
	messageOnly := config
	messageOnly.Icon = "none"
	messageOnly.Alignment = "center"
	messageOnly.Tone = "quiet"
	messageOnly.Rule = "none"
	if err := validateFooterConfig(messageOnly); err != nil {
		return settingsMessage
	}
	if _, err := renderFooterIcon(config.Icon); err != nil {
		return settingsIcon
	}
	if config.Alignment != "" && config.Alignment != "left" && config.Alignment != "center" && config.Alignment != "right" {
		return settingsAlignment
	}
	if config.Tone != "" && config.Tone != "quiet" && config.Tone != "accent" && config.Tone != "bright" {
		return settingsTone
	}
	return settingsRule
}

func (m model) appearanceCandidates() (AppearanceConfig, FooterConfig) {
	appearance := m.settingsBefore.Appearance
	if validateAppearanceConfig(appearance) != nil {
		appearance = defaultAppearanceConfig()
	}
	appearance.Brand = defaultAppearanceConfig().Brand
	footer := FooterConfig{Message: m.settingsForm[settingsMessage], Icon: m.settingsForm[settingsIcon], Alignment: m.settingsForm[settingsAlignment], Tone: m.settingsForm[settingsTone], Rule: m.settingsForm[settingsRule]}
	return appearance, footer
}

func footerGraphemes(value string) []string {
	iterator := uniseg.NewGraphemes(value)
	graphemes := make([]string, 0, len(value))
	for iterator.Next() {
		graphemes = append(graphemes, iterator.Str())
	}
	return graphemes
}

func footerGraphemeCount(value string) int {
	return len(footerGraphemes(value))
}

func insertFooterText(value string, cursor int, inserted string) (string, int) {
	graphemes := footerGraphemes(value)
	cursor = min(max(0, cursor), len(graphemes))
	addition := footerGraphemes(inserted)
	updated := make([]string, 0, len(graphemes)+len(addition))
	updated = append(updated, graphemes[:cursor]...)
	updated = append(updated, addition...)
	updated = append(updated, graphemes[cursor:]...)
	return joinFooterGraphemes(updated), cursor + len(addition)
}

func deleteFooterGrapheme(value string, cursor int, backward bool) (string, int) {
	graphemes := footerGraphemes(value)
	cursor = min(max(0, cursor), len(graphemes))
	index := cursor
	if backward {
		index--
	}
	if index < 0 || index >= len(graphemes) {
		return value, cursor
	}
	graphemes = append(graphemes[:index], graphemes[index+1:]...)
	if backward {
		cursor--
	}
	return joinFooterGraphemes(graphemes), cursor
}

func joinFooterGraphemes(graphemes []string) string {
	return strings.Join(graphemes, "")
}
