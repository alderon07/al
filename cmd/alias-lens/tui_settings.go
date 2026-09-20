package main

import (
	"strings"

	tea "alias-lens/cmd/alias-lens/internal/tea"
	"github.com/rivo/uniseg"
)

const (
	settingsBrand = iota
	settingsArtStyle
	settingsMarkerStyle
	settingsMessage
	settingsIcon
	settingsAlignment
)

type appearanceSettings struct {
	Appearance AppearanceConfig
	Footer     FooterConfig
}

func (m *model) openFooterSettings() {
	config, err := loadConfig()
	if err != nil {
		m.status = "Could not open appearance settings: " + err.Error()
		return
	}
	m.settingsOpen = true
	m.settingsField = 0
	m.settingsBefore = appearanceSettings{Appearance: config.Appearance, Footer: config.Footer}
	m.settingsForm = [6]string{config.Appearance.Brand, config.Appearance.ArtStyle, config.Appearance.MarkerStyle, config.Footer.Message, config.Footer.Icon, config.Footer.Alignment}
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
		applyAppearanceConfig(m.settingsBefore.Appearance)
		applyFooterConfig(m.settingsBefore.Footer)
		m.settingsOpen = false
		m.settingsBefore = appearanceSettings{}
		m.settingsForm = [6]string{}
		m.settingsCursor = [6]int{}
		m.status = "Appearance unchanged"
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
	case tea.KeyLeft:
		if choices := settingsFieldChoices(m.settingsField); len(choices) > 0 {
			m.settingsForm[m.settingsField] = cycleSettingChoice(m.settingsForm[m.settingsField], choices, -1)
		} else {
			m.settingsCursor[m.settingsField] = max(0, m.settingsCursor[m.settingsField]-1)
		}
	case tea.KeyRight:
		if choices := settingsFieldChoices(m.settingsField); len(choices) > 0 {
			m.settingsForm[m.settingsField] = cycleSettingChoice(m.settingsForm[m.settingsField], choices, 1)
		} else {
			m.settingsCursor[m.settingsField] = min(footerGraphemeCount(m.settingsForm[m.settingsField]), m.settingsCursor[m.settingsField]+1)
		}
	case tea.KeyHome:
		if choices := settingsFieldChoices(m.settingsField); len(choices) > 0 {
			m.settingsForm[m.settingsField] = choices[0]
		} else {
			m.settingsCursor[m.settingsField] = 0
		}
	case tea.KeyEnd:
		if choices := settingsFieldChoices(m.settingsField); len(choices) > 0 {
			m.settingsForm[m.settingsField] = choices[len(choices)-1]
		} else {
			m.settingsCursor[m.settingsField] = footerGraphemeCount(m.settingsForm[m.settingsField])
		}
	case tea.KeyCtrlU:
		if len(settingsFieldChoices(m.settingsField)) > 0 {
			return m, nil
		}
		m.settingsForm[m.settingsField] = ""
		m.settingsCursor[m.settingsField] = 0
	case tea.KeyBackspace:
		if len(settingsFieldChoices(m.settingsField)) > 0 {
			return m, nil
		}
		m.settingsForm[m.settingsField], m.settingsCursor[m.settingsField] = deleteFooterGrapheme(
			m.settingsForm[m.settingsField], m.settingsCursor[m.settingsField], true,
		)
	case tea.KeyDelete:
		if len(settingsFieldChoices(m.settingsField)) > 0 {
			return m, nil
		}
		m.settingsForm[m.settingsField], m.settingsCursor[m.settingsField] = deleteFooterGrapheme(
			m.settingsForm[m.settingsField], m.settingsCursor[m.settingsField], false,
		)
	case tea.KeySpace:
		if !acceptsTextInput(message) || len(settingsFieldChoices(m.settingsField)) > 0 {
			return m, nil
		}
		m.settingsForm[m.settingsField], m.settingsCursor[m.settingsField] = insertFooterText(
			m.settingsForm[m.settingsField], m.settingsCursor[m.settingsField], " ",
		)
	case tea.KeyRunes:
		if !acceptsTextInput(message) || len(settingsFieldChoices(m.settingsField)) > 0 {
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

func settingsFieldChoices(field int) []string {
	switch field {
	case settingsArtStyle:
		return []string{"full", "compact", "text", "none"}
	case settingsMarkerStyle:
		return []string{"symbols", "ascii", "none"}
	case settingsAlignment:
		return []string{"left", "center", "right"}
	default:
		return nil
	}
}

func cycleSettingChoice(current string, choices []string, direction int) string {
	index := 0
	for choiceIndex, choice := range choices {
		if choice == current {
			index = choiceIndex
			break
		}
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
		return m, nil
	}
	if err := validateFooterConfig(footer); err != nil {
		m.status = err.Error()
		return m, nil
	}
	config, err := loadConfig()
	if err == nil {
		config.Appearance = appearance
		config.Footer = footer
		err = saveConfig(config)
	}
	if err != nil {
		m.status = "Could not save appearance settings: " + err.Error()
		return m, nil
	}
	applyAppearanceConfig(appearance)
	applyFooterConfig(footer)
	m.settingsOpen = false
	m.settingsBefore = appearanceSettings{}
	m.settingsForm = [6]string{}
	m.settingsCursor = [6]int{}
	m.status = "Appearance saved"
	return m, nil
}

func (m model) appearanceCandidates() (AppearanceConfig, FooterConfig) {
	appearance := AppearanceConfig{Brand: m.settingsForm[settingsBrand], ArtStyle: m.settingsForm[settingsArtStyle], MarkerStyle: m.settingsForm[settingsMarkerStyle]}
	footer := FooterConfig{Message: m.settingsForm[settingsMessage], Icon: m.settingsForm[settingsIcon], Alignment: m.settingsForm[settingsAlignment]}
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
