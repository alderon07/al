package main

import (
	"errors"
	"os"
	"strings"

	tea "alias-lens/cmd/alias-lens/internal/tea"
	"github.com/charmbracelet/lipgloss"
)

const (
	tourPending = "pending"
	tourSeen    = "seen"
)

func tourStatePath() (string, error) {
	return syncDataPath("tour-state")
}

func scheduleTour() error {
	path, err := tourStatePath()
	if err != nil {
		return err
	}
	state, readErr := os.ReadFile(path)
	if readErr == nil && strings.TrimSpace(string(state)) == tourSeen {
		return nil
	}
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	return os.WriteFile(path, []byte(tourPending+"\n"), 0o600)
}

func tourShouldShow() bool {
	path, err := tourStatePath()
	if err != nil {
		return false
	}
	state, err := os.ReadFile(path)
	return err == nil && strings.TrimSpace(string(state)) == tourPending
}

func markTourSeen() error {
	path, err := tourStatePath()
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(tourSeen+"\n"), 0o600)
}

func (m model) updateTour(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if message.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	openHelp := matchesShortcut(message, m.shortcutProfile, shortcutHelp)
	if message.Type != tea.KeyEnter && message.Type != tea.KeyEsc && !openHelp {
		return m, nil
	}
	if err := markTourSeen(); err != nil {
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
		aliasStyle.Render("Type") + dimStyle.Render("      Search aliases, commands, and descriptions"),
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
	footer := aliasStyle.Render("enter") + dimStyle.Render(" start  ·  ") + aliasStyle.Render("?") + dimStyle.Render(" full guide  ·  esc dismiss")
	sections := []string{header, "", body}
	if height < 24 {
		sections = []string{header, body}
	}
	page := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return frame.renderWithFooter(page, footer)
}
