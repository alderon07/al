package main

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/rivo/uniseg"
	"strings"
	"unicode"
)

const footerIconToken = "{icon}"

type FooterConfig struct {
	Message   string `json:"message"`
	Icon      string `json:"icon"`
	Alignment string `json:"alignment,omitempty"`
	Tone      string `json:"tone,omitempty"`
	Rule      string `json:"rule,omitempty"`
}

var activeFooter = defaultFooterConfig()

type AppearanceConfig struct {
	Brand       string `json:"brand"`
	ArtStyle    string `json:"art_style"`
	MarkerStyle string `json:"marker_style"`
}

var activeAppearance = defaultAppearanceConfig()

func defaultAppearanceConfig() AppearanceConfig {
	return AppearanceConfig{Brand: "ALIAS LENS", ArtStyle: "full", MarkerStyle: "symbols"}
}

func defaultFooterConfig() FooterConfig {
	return FooterConfig{Message: "Made by Naqi", Icon: "none", Alignment: "center", Tone: "quiet", Rule: "none"}
}

func validateAppearanceConfig(config AppearanceConfig) error {
	if config.ArtStyle != "full" && config.ArtStyle != "compact" && config.ArtStyle != "text" && config.ArtStyle != "none" {
		return fmt.Errorf("choose full, compact, text, or none for Brand art")
	}
	if config.MarkerStyle != "symbols" && config.MarkerStyle != "ascii" && config.MarkerStyle != "none" {
		return fmt.Errorf("choose symbols, ascii, or none for Markers")
	}
	return nil
}

func validateFooterConfig(config FooterConfig) error {
	if lipgloss.Width(config.Message) > 80 {
		return fmt.Errorf("footer message must be 80 terminal cells or fewer")
	}
	for _, character := range config.Message {
		if unicode.IsControl(character) {
			return fmt.Errorf("footer message cannot contain control characters or newlines")
		}
	}
	if strings.Count(config.Message, footerIconToken) > 1 {
		return fmt.Errorf("footer message may contain {icon} once")
	}
	if _, err := renderFooterIcon(config.Icon); err != nil {
		return err
	}
	if config.Alignment != "" && config.Alignment != "left" && config.Alignment != "center" && config.Alignment != "right" {
		return fmt.Errorf("choose left, center, or right for footer alignment")
	}
	if config.Tone != "" && config.Tone != "quiet" && config.Tone != "accent" && config.Tone != "bright" {
		return fmt.Errorf("choose quiet, accent, or bright for footer tone")
	}
	if config.Rule != "" && config.Rule != "none" && config.Rule != "thin" && config.Rule != "dots" {
		return fmt.Errorf("choose none, thin, or dots for the footer rule")
	}
	return nil
}

func applyAppearanceConfig(config AppearanceConfig) {
	if err := validateAppearanceConfig(config); err != nil {
		activeAppearance = defaultAppearanceConfig()
		return
	}
	config.Brand = defaultAppearanceConfig().Brand
	activeAppearance = config
}

func applyFooterConfig(config FooterConfig) {
	if err := validateFooterConfig(config); err != nil {
		activeFooter = defaultFooterConfig()
		return
	}
	if config.Tone == "" {
		config.Tone = defaultFooterConfig().Tone
	}
	if config.Rule == "" {
		config.Rule = defaultFooterConfig().Rule
	}
	activeFooter = config
}

func (m model) footerSettingsView(frame tuiFrame, header string) string {
	width, height, contentWidth := frame.width, frame.height, frame.contentWidth
	title := pixelIconLabel(iconEdit, "Compose your footer", titleStyle)
	subtitle := dimStyle.Render("A small signature for the bottom of Alias Lens.")
	compact := width < 76 || height < 24
	var workspace string
	if compact {
		workspace = m.compactAppearanceEditor(contentWidth)
	} else {
		formWidth := max(48, min(62, contentWidth*3/5))
		previewWidth := max(28, contentWidth-formWidth-2)
		workspace = lipgloss.JoinHorizontal(
			lipgloss.Top,
			m.appearanceControlPanel(formWidth),
			"  ",
			m.appearancePreviewPanel(previewWidth),
		)
	}
	footer := dimStyle.Render(wrapText(appearanceEditorControls(m.settingsField), contentWidth))
	page := lipgloss.JoinVertical(lipgloss.Left, header, "", title, subtitle, "", workspace, "", footer)
	if compact {
		footer = dimStyle.Render(appearanceEditorCompactControls(m.settingsField))
		page = lipgloss.JoinVertical(lipgloss.Left, header, "", title, "", workspace, "", footer)
	}
	return frame.render(page)
}

var appearanceFieldLabels = [settingsFieldCount]string{"Message", "Icon", "Alignment", "Tone", "Rule"}

var appearanceFieldHints = [settingsFieldCount]string{
	"Use {icon} to place the selected icon.",
	"Named icons work without a Nerd Font.",
	"Choose where the maker line sits.",
	"Quiet recedes, accent adds color, and bright adds contrast.",
	"Add a thin or dotted divider above the footer.",
}

func (m model) appearanceControlPanel(width int) string {
	innerWidth := max(20, width-4)
	contentWidth := max(18, innerWidth-2)
	var body strings.Builder
	groups := []struct {
		label  string
		fields []int
	}{
		{label: "CONTENT", fields: []int{settingsMessage, settingsIcon}},
		{label: "PRESENTATION", fields: []int{settingsAlignment, settingsTone, settingsRule}},
	}
	for groupIndex, group := range groups {
		if groupIndex > 0 {
			body.WriteString("\n")
		}
		body.WriteString(lipgloss.NewStyle().Bold(true).Foreground(cyanColor).Render(group.label))
		body.WriteByte('\n')
		for _, field := range group.fields {
			body.WriteString(m.appearanceFieldRow(field, contentWidth))
			body.WriteByte('\n')
		}
	}
	body.WriteString("\n" + m.appearanceActionBar(contentWidth))
	return lipgloss.NewStyle().
		Width(innerWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lineColor).
		Padding(1).
		Render(strings.TrimSuffix(body.String(), "\n"))
}

func (m model) appearanceFieldRow(field, width int) string {
	focused := m.settingsField == field
	marker := "  "
	labelStyle := lipgloss.NewStyle().Foreground(mutedColor)
	if focused {
		marker = acidStyle("● ")
		labelStyle = lipgloss.NewStyle().Bold(true).Foreground(inkColor)
	}
	label := labelStyle.Render(appearanceFieldLabels[field])
	valueWidth := max(8, width-lipgloss.Width(appearanceFieldLabels[field])-5)
	value := footerFieldValue(m.settingsForm[field], m.settingsCursor[field], focused, valueWidth)
	if choices := settingsFieldChoices(field); len(choices) > 0 {
		value = "‹ " + m.settingsForm[field] + " ›"
		if !containsSettingChoice(m.settingsForm[field], choices) {
			value = "‹ custom ›"
		}
	}
	row := marker + label + strings.Repeat(" ", max(1, width-lipgloss.Width(marker+label)-lipgloss.Width(value)-1)) + value
	if !focused {
		return row
	}
	detail := appearanceFieldHints[field]
	if choices := settingsFieldChoices(field); len(choices) > 0 {
		detail = settingsChoiceList(m.settingsForm[field], choices)
	}
	row += "\n" + dimStyle.Render(wrapText("  "+detail, width))
	if m.status != "" {
		row += "\n" + statusStyle.Render(wrapText("  "+m.status, width))
	}
	return row
}

func (m model) appearanceActionBar(width int) string {
	save := " Save changes "
	cancel := " Cancel "
	if m.settingsField == settingsSave {
		save = lipgloss.NewStyle().Bold(true).Foreground(pageColor).Background(acidColor).Render(save)
	} else {
		save = dimStyle.Render("[" + save + "]")
	}
	if m.settingsField == settingsCancel {
		cancel = lipgloss.NewStyle().Bold(true).Foreground(pageColor).Background(amberColor).Render(cancel)
	} else {
		cancel = dimStyle.Render(cancel)
	}
	bar := save + "   " + cancel
	if m.status != "" && m.settingsField >= settingsFieldCount {
		bar += "\n" + statusStyle.Render(wrapText(m.status, width))
	}
	return bar
}

func (m model) appearancePreviewPanel(width int) string {
	innerWidth := max(20, width-4)
	contentWidth := max(18, innerWidth-2)
	_, footer := m.appearanceCandidates()
	var preview strings.Builder
	preview.WriteString(lipgloss.NewStyle().Bold(true).Foreground(cyanColor).Render("LIVE PREVIEW"))
	preview.WriteString("\n\n")
	headerDetails := "~/.bash_aliases  •  69 aliases"
	renderedBrand := brandStyle.Render("ALIAS LENS")
	detailsWidth := max(4, contentWidth-lipgloss.Width(renderedBrand)-2)
	headerLine := renderedBrand + "  " + dimStyle.Render(truncate(headerDetails, detailsWidth))
	preview.WriteString(headerLine)
	preview.WriteString("\n\n")
	preview.WriteString(dimStyle.Render("$ deploy  →  deploy staging") + "\n")
	preview.WriteString(dimStyle.Render("$ logs    →  tail service logs") + "\n")
	preview.WriteString(dimStyle.Render("$ clean   →  remove build output") + "\n\n")
	preview.WriteString(renderFooterPreview(footer, contentWidth))
	return lipgloss.NewStyle().
		Width(innerWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lineColor).
		Padding(1).
		Render(preview.String())
}

func (m model) compactAppearanceEditor(width int) string {
	innerWidth := max(20, width-4)
	contentWidth := max(18, innerWidth-2)
	position := fmt.Sprintf("%d / %d", m.settingsField+1, settingsRowCount)
	cardTitle := "ACTION"
	content := m.appearanceActionBar(contentWidth)
	if m.settingsField < settingsFieldCount {
		cardTitle = appearanceFieldGroup(m.settingsField)
		content = m.appearanceFieldRow(m.settingsField, contentWidth)
	}
	card := lipgloss.NewStyle().
		Width(innerWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lineColor).
		Padding(0, 1).
		Render(lipgloss.NewStyle().Bold(true).Foreground(cyanColor).Render(cardTitle) +
			strings.Repeat(" ", max(1, contentWidth-lipgloss.Width(cardTitle)-lipgloss.Width(position)-1)) +
			dimStyle.Render(position) + "\n\n" + content)
	_, footer := m.appearanceCandidates()
	preview := dimStyle.Render("PREVIEW  ") + renderFooterPreview(footer, max(12, width-9))
	return card + "\n" + preview
}

func appearanceFieldGroup(field int) string {
	switch field {
	case settingsMessage, settingsIcon:
		return "CONTENT"
	default:
		return "PRESENTATION"
	}
}

func appearanceEditorControls(field int) string {
	if field == settingsSave || field == settingsCancel {
		return "Tab/↑↓ move  ·  ←→ switch action  ·  Enter choose  ·  Esc cancel"
	}
	if len(settingsFieldChoices(field)) > 0 {
		return "Tab/↑↓ move  ·  ←→ or Space choose  ·  Enter next  ·  Esc cancel"
	}
	return "Tab/↑↓ move  ·  ←→ edit  ·  Ctrl+U clear  ·  Enter next  ·  Esc cancel"
}

func appearanceEditorCompactControls(field int) string {
	if field == settingsSave || field == settingsCancel {
		return "Tab/↑↓ move · ←→ switch · Enter choose\nEsc cancel"
	}
	if len(settingsFieldChoices(field)) > 0 {
		return "Tab/↑↓ move · ←→/Space choose\nEnter next · Esc cancel"
	}
	return "Tab/↑↓ move · ←→ edit · Enter next\nCtrl+U clear · Esc cancel"
}

func renderFooterPreview(config FooterConfig, width int) string {
	message := config.Message
	if message == "" {
		message = "(maker line hidden)"
	} else {
		message = renderFooterMessage(message, config.Icon)
	}
	alignment := lipgloss.Center
	if config.Alignment == "left" {
		alignment = lipgloss.Left
	} else if config.Alignment == "right" {
		alignment = lipgloss.Right
	}
	credit := footerToneStyle(config.Tone).Width(max(1, width)).Align(alignment).Render(truncate(message, width))
	if rule := footerRule(config.Rule, width); rule != "" {
		return rule + "\n" + credit
	}
	return credit
}

func footerToneStyle(tone string) lipgloss.Style {
	switch tone {
	case "accent":
		return lipgloss.NewStyle().Foreground(acidColor)
	case "bright":
		return lipgloss.NewStyle().Bold(true).Foreground(inkColor)
	default:
		return dimStyle
	}
}

func footerRule(rule string, width int) string {
	character := ""
	switch rule {
	case "thin":
		character = "─"
	case "dots":
		character = "·"
	}
	if character == "" {
		return ""
	}
	return dimStyle.Render(strings.Repeat(character, max(1, width)))
}

func containsSettingChoice(value string, choices []string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}

func settingsChoiceList(current string, choices []string) string {
	parts := make([]string, 0, len(choices))
	for _, choice := range choices {
		if choice == current {
			parts = append(parts, "["+choice+"]")
		} else {
			parts = append(parts, choice)
		}
	}
	return "Choose: " + strings.Join(parts, "  ")
}

func footerFieldValue(value string, cursor int, focused bool, width int) string {
	if !focused {
		if value == "" {
			return "(empty)"
		}
		return truncate(value, width)
	}
	graphemes := footerGraphemes(value)
	cursor = min(max(0, cursor), len(graphemes))
	start, end := 0, len(graphemes)
	for lipgloss.Width(strings.Join(graphemes[start:cursor], "")+"|"+strings.Join(graphemes[cursor:end], "")) > width {
		leftCount := cursor - start
		rightCount := end - cursor
		if leftCount >= rightCount && start < cursor {
			start++
		} else if end > cursor {
			end--
		} else {
			break
		}
	}
	return strings.Join(graphemes[start:cursor], "") + "|" + strings.Join(graphemes[cursor:end], "")
}

func renderFooterIcon(value string) (string, error) {
	if value == "none" {
		return "", nil
	}
	if strings.HasPrefix(value, "emoji:") {
		glyph := strings.TrimPrefix(value, "emoji:")
		graphemes := uniseg.NewGraphemes(glyph)
		count := 0
		for graphemes.Next() {
			count++
		}
		if count != 1 || lipgloss.Width(glyph) < 1 || lipgloss.Width(glyph) > 2 {
			return "", fmt.Errorf("enter one visible emoji after emoji:, for example emoji:🚀")
		}
		for _, character := range glyph {
			if unicode.IsControl(character) {
				return "", fmt.Errorf("footer emoji cannot contain control characters")
			}
		}
		return glyph, nil
	}
	if icon, ok := namedPixelIcon(value); ok {
		return icon.symbol, nil
	}
	icon, ok := parsePixelIcon(value)
	if !ok {
		return "", fmt.Errorf("choose a named footer icon, none, or one emoji written as emoji:VALUE")
	}
	return renderPixelIcon(icon), nil
}

func makerCredit(width int) string {
	if activeFooter.Message == "" {
		return ""
	}
	credit := renderFooterMessage(activeFooter.Message, activeFooter.Icon)
	alignment := lipgloss.Center
	if activeFooter.Alignment == "left" {
		alignment = lipgloss.Left
	} else if activeFooter.Alignment == "right" {
		alignment = lipgloss.Right
	}
	credit = footerToneStyle(activeFooter.Tone).Width(max(1, width)).Align(alignment).Render(credit)
	if rule := footerRule(activeFooter.Rule, width); rule != "" {
		return rule + "\n" + credit
	}
	return credit
}

func renderFooterMessage(message, iconName string) string {
	parts := strings.SplitN(message, footerIconToken, 2)
	if len(parts) != 2 {
		return message
	}
	icon, err := renderFooterIcon(iconName)
	if err != nil {
		return message
	}
	suffix := parts[1]
	if strings.EqualFold(iconName, "heart") && strings.HasPrefix(suffix, " ") {
		suffix = "\u00a0" + strings.TrimPrefix(suffix, " ")
	}
	return parts[0] + icon + suffix
}

func footerWithMaker(footer string, width int) string {
	credit := makerCredit(width)
	if credit == "" {
		return footer
	}
	return footer + "\n" + credit
}

func footerWithNavigation(footer string, width int, profiles ...ShortcutProfile) string {
	navigation := pageNavigationHint(width, profiles...)
	if navigation == "" {
		return footer
	}
	return footer + "\n\n" + dimStyle.Render(navigation)
}

func pageWithMaker(page string, width, height int) string {
	credit := makerCredit(width)
	if credit == "" {
		return page
	}
	wrappedPage := lipgloss.NewStyle().Width(max(1, width)).Render(page)
	gap := max(0, height-lipgloss.Height(wrappedPage)-lipgloss.Height(credit))
	return page + strings.Repeat("\n", gap) + "\n" + credit
}
