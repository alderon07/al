package main

import (
	"errors"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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
	openHelp := message.Type == tea.KeyRunes && len(message.Runes) == 1 && message.Runes[0] == '?'
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

func (m model) tourView(width, height, contentWidth int, header string) string {
	steps := []string{
		aliasStyle.Render("Type") + dimStyle.Render("      Search aliases, commands, and descriptions"),
		aliasStyle.Render("Enter") + dimStyle.Render("     Run the selected alias by name"),
		aliasStyle.Render("Ctrl+T") + dimStyle.Render("    Preview and save a dark theme"),
		aliasStyle.Render("?") + dimStyle.Render("         Open the searchable keyboard guide"),
	}
	body := titleStyle.Render("Alias Lens is ready.") +
		"\n" + dimStyle.Render("Four keys are enough to get started.") +
		"\n\n" + strings.Join(steps, "\n\n")
	footer := aliasStyle.Render("enter") + dimStyle.Render(" start  ·  ") + aliasStyle.Render("?") + dimStyle.Render(" full guide  ·  esc dismiss")
	page := lipgloss.JoinVertical(lipgloss.Left, header, "", body, "", footer)
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 3).Render(page)
}
