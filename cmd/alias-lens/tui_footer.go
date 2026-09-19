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
	Message string `json:"message"`
	Icon    string `json:"icon"`
}

var activeFooter = defaultFooterConfig()

func defaultFooterConfig() FooterConfig {
	return FooterConfig{Message: "Made with {icon} by Naqi", Icon: "heart"}
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
	return nil
}

func applyFooterConfig(config FooterConfig) {
	if err := validateFooterConfig(config); err != nil {
		activeFooter = defaultFooterConfig()
		return
	}
	activeFooter = config
}

func (m model) footerSettingsView(width, height, contentWidth int, header string) string {
	labels := [2]string{"Message", "Icon"}
	hints := [2]string{"Use {icon} where the icon should appear.", "Try heart, spark, none, emoji:🚀, or #..#/.##."}
	var fields strings.Builder
	for index := range labels {
		marker := "  "
		style := lipgloss.NewStyle().Width(contentWidth-2).Padding(0, 1).Foreground(mutedColor)
		if index == m.settingsField {
			marker = "▶ "
			style = style.Foreground(inkColor).Background(activeColor)
		}
		value := m.settingsForm[index]
		if value == "" {
			value = "(empty)"
		}
		line := fmt.Sprintf("%s%s: %s", marker, labels[index], value)
		fields.WriteString(style.Render(truncate(line, contentWidth-4)))
		fields.WriteString("\n" + dimStyle.Render("  "+hints[index]))
		if index == 0 {
			fields.WriteString("\n\n")
		}
	}
	title := pixelIconLabel(iconEdit, "Customize the footer", titleStyle) + "\n" + dimStyle.Render("Your preview stays at the bottom of this screen. Nothing is saved until you choose Save.")
	footer := dimStyle.Render("tab/enter next  ·  ") + cyanStyle(shortcutLabel(m.shortcutProfile, shortcutSave)) + dimStyle.Render(" save  ·  esc cancel")
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
			return "", fmt.Errorf("footer emoji must contain one visible terminal grapheme after emoji:")
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
	return lipgloss.NewStyle().Width(max(1, width)).Align(lipgloss.Center).Render(credit)
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
