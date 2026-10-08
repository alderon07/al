package tui

import (
	"strings"

	tea "alias-lens/internal/tea"
	"github.com/charmbracelet/lipgloss"
)

func (m model) updateTour(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if message.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	openHelp := matchesShortcut(message, m.shortcutProfile, shortcutHelp)
	if message.Type != tea.KeyEnter && message.Type != tea.KeyEsc && !openHelp {
		return m, nil
	}
	if err := m.service().MarkTourSeen(); err != nil {
		m.status = "Could not dismiss tour: " + err.Error()
		return m, nil
	}
	m.tourVisible = false
	if openHelp {
		m.helpVisible = true
	}
	return m, nil
}

func (m model) tourView(frame tuiFrame, header string) string {
	height := frame.height
	steps := []string{
		aliasStyle.Render("/") + dimStyle.Render("         Search aliases, commands, and descriptions"),
		aliasStyle.Render("Enter") + dimStyle.Render("     Run the selected alias by name"),
		aliasStyle.Render(primaryShortcutLabel(m.shortcutProfile, shortcutThemes)) + dimStyle.Render("  Preview and save a dark theme"),
		aliasStyle.Render(primaryShortcutLabel(m.shortcutProfile, shortcutHelp)) + dimStyle.Render("  Open the searchable keyboard guide"),
	}
	body := ""
	if mark := fullBrandMark(); mark != "" && height >= 24 {
		body = mark + "\n"
	}
	stepSeparator := "\n\n"
	if height < 24 {
		stepSeparator = "\n"
	}
	body += pixelIconLabel(iconBrand, "Alias Lens is ready.", titleStyle) +
		"\n" + dimStyle.Render("Four keys are enough to get started.") +
		stepSeparator + strings.Join(steps, stepSeparator)
	footer := aliasStyle.Render("enter") + dimStyle.Render(" start  ·  ") + aliasStyle.Render(primaryShortcutLabel(m.shortcutProfile, shortcutHelp)) + dimStyle.Render(" full guide  ·  esc dismiss")
	sections := []string{header, "", body}
	if height < 24 {
		sections = []string{header, body}
	}
	page := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return frame.renderWithFooter(page, footer)
}
