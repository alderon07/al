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
	return FooterConfig{Message: "Made by Naqi", Icon: "none", Alignment: "center"}
}

func validateAppearanceConfig(config AppearanceConfig) error {
	if config.ArtStyle != "full" && config.ArtStyle != "compact" && config.ArtStyle != "text" && config.ArtStyle != "none" {
		return fmt.Errorf("choose full, compact, text, or none for Brand art")
	}
	if config.MarkerStyle != "symbols" && config.MarkerStyle != "ascii" && config.MarkerStyle != "none" {
		return fmt.Errorf("choose symbols, ascii, or none for Markers")
	}
	if config.ArtStyle != "none" && strings.TrimSpace(config.Brand) == "" {
		return fmt.Errorf("brand must contain visible text unless art style is none")
	}
	if lipgloss.Width(config.Brand) > 24 {
		return fmt.Errorf("keep the brand name at 24 characters wide or fewer")
	}
	for _, character := range config.Brand {
		if unicode.IsControl(character) {
			return fmt.Errorf("brand cannot contain control characters or newlines")
		}
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
	return nil
}

func applyAppearanceConfig(config AppearanceConfig) {
	if err := validateAppearanceConfig(config); err != nil {
		activeAppearance = defaultAppearanceConfig()
		return
	}
	activeAppearance = config
}

func applyFooterConfig(config FooterConfig) {
	if err := validateFooterConfig(config); err != nil {
		activeFooter = defaultFooterConfig()
		return
	}
	activeFooter = config
}

func (m model) footerSettingsView(width, height, contentWidth int, header string) string {
	labels := [6]string{"Brand", "Brand art", "Markers", "Message", "Icon", "Alignment"}
	hints := [6]string{
		"Your short product name, up to 24 characters wide.",
		"Use Left or Right to choose full, compact, text, or none.",
		"Use Left or Right to choose symbols, ascii, or none.",
		"Use {icon} where the maker icon should appear.",
		"Enter heart, spark, none, emoji:🚀, or #..#/.##.",
		"Use Left or Right to choose left, center, or right.",
	}
	var fields strings.Builder
	start, end := 0, len(labels)
	compact := height < 24
	if compact {
		start = max(0, m.settingsField-1)
		end = min(len(labels), start+3)
		start = max(0, end-3)
	}
	for index := start; index < end; index++ {
		marker := "  "
		style := lipgloss.NewStyle().Width(contentWidth-2).Padding(0, 1).Foreground(mutedColor)
		if index == m.settingsField {
			marker = "▶ "
			style = style.Foreground(inkColor).Background(activeColor)
		}
		prefix := fmt.Sprintf("%s%s: ", marker, labels[index])
		valueWidth := max(4, contentWidth-lipgloss.Width(prefix)-4)
		value := footerFieldValue(m.settingsForm[index], m.settingsCursor[index], index == m.settingsField, valueWidth)
		if choices := settingsFieldChoices(index); len(choices) > 0 && index == m.settingsField {
			value = "← " + m.settingsForm[index] + " →"
		}
		fields.WriteString(style.Render(prefix + value))
		if index == m.settingsField {
			fields.WriteString("\n" + dimStyle.Render("  "+hints[index]))
		}
		fields.WriteByte('\n')
	}
	title := pixelIconLabel(iconEdit, "Customize appearance", titleStyle)
	if !compact {
		title += "\n" + dimStyle.Render("The header and maker line preview here. Nothing is saved until you choose Save.")
	}
	controls := "Tab or Up/Down changes fields. Left/Right moves the cursor. Ctrl+U clears the field."
	if len(settingsFieldChoices(m.settingsField)) > 0 {
		controls = "Tab or Up/Down changes fields. Left/Right changes this choice."
	}
	actions := "Enter moves or saves. " + shortcutLabel(m.shortcutProfile, shortcutSave) + " saves. Esc cancels."
	if compact {
		controls = "Tab field · ←→ move · Ctrl+U clear"
		if len(settingsFieldChoices(m.settingsField)) > 0 {
			controls = "Tab field · ←→ choose"
		}
		actions = "Enter next/save · " + shortcutLabel(m.shortcutProfile, shortcutSave) + " save · Esc cancel"
	}
	footer := dimStyle.Render(wrapText(controls, contentWidth)) + "\n" + dimStyle.Render(wrapText(actions, contentWidth))
	if m.status != "" {
		footer = statusStyle.Render(wrapText(m.status, contentWidth)) + "\n" + footer
	}
	if navigation := pageNavigationHint(contentWidth, m.shortcutProfile); navigation != "" {
		footer += "\n" + dimStyle.Render(navigation)
	}
	page := lipgloss.JoinVertical(lipgloss.Left, header, "", title, "", fields.String(), "", footer)
	page = pageWithMaker(page, contentWidth, height)
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 3).Render(page)
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
	icon, ok := parsePixelIcon(value)
	if !ok {
		return "", fmt.Errorf("footer icon must be a built-in name, none, emoji:VALUE, or a 4x2 bitmap such as #..#/.##.")
	}
	return renderPixelIcon(icon), nil
}

func makerCredit(width int) string {
	if activeFooter.Message == "" {
		return ""
	}
	parts := strings.SplitN(activeFooter.Message, footerIconToken, 2)
	credit := dimStyle.Render(parts[0])
	if len(parts) == 2 {
		if icon, err := renderFooterIcon(activeFooter.Icon); err == nil && icon != "" {
			credit += lipgloss.NewStyle().Foreground(coralColor).Render(icon)
		}
		credit += dimStyle.Render(parts[1])
	}
	alignment := lipgloss.Center
	if activeFooter.Alignment == "left" {
		alignment = lipgloss.Left
	} else if activeFooter.Alignment == "right" {
		alignment = lipgloss.Right
	}
	return lipgloss.NewStyle().Width(max(1, width)).Align(alignment).Render(credit)
}

func footerWithMaker(footer string, width int) string {
	credit := makerCredit(width)
	if credit == "" {
		return footer
	}
	return footer + "\n" + credit
}

func pageWithMaker(page string, width, height int) string {
	credit := makerCredit(width)
	if credit == "" {
		return page
	}
	innerHeight := max(1, height-1)
	wrappedPage := lipgloss.NewStyle().Width(max(1, width)).Render(page)
	gap := max(0, innerHeight-lipgloss.Height(wrappedPage)-1)
	return page + strings.Repeat("\n", gap) + "\n" + credit
}
