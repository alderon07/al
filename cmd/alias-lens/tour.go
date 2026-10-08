package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	tea "alias-lens/internal/tea"
	"github.com/charmbracelet/lipgloss"
)

const (
	tourPending = "pending"
	tourSeen    = "seen"
)

func tourStatePath() (string, error) {
	home, e := os.UserHomeDir()
	if e != nil {
		return "", e
	}
	return filepath.Join(home, ".local", "state", "alias-lens", "tour-state"), nil
}

func scheduleTour() error { return withMutation(scheduleTourInSession) }
func scheduleTourInSession(session *mutationSession) error {
	path, e := tourStatePath()
	if e != nil {
		return e
	}
	state, e := readManagedPrivateFile(path, 1024)
	if e == nil && strings.TrimSpace(string(state)) == tourSeen {
		return nil
	}
	if e != nil && !errors.Is(e, os.ErrNotExist) {
		return e
	}
	return session.writePrivate(path, []byte(tourPending+"\n"))
}

func tourShouldShow() bool {
	path, err := tourStatePath()
	if err != nil {
		return false
	}
	state, err := readManagedPrivateFile(path, 1024)
	return err == nil && strings.TrimSpace(string(state)) == tourPending
}

func markTourSeen() error {
	return withMutation(func(session *mutationSession) error {
		path, e := tourStatePath()
		if e != nil {
			return e
		}
		return session.writePrivate(path, []byte(tourSeen+"\n"))
	})
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
