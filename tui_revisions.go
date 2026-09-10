package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m *model) openRevisionDrawer() {
	m.revisionOpen = true
	m.revisionCursor = 0
	m.revisionConfirm = false
	m.revisionErr = ""
	path, err := aliasesPath()
	if err != nil {
		m.revisionErr = err.Error()
		return
	}
	m.revisions, err = listRevisions(path)
	if err != nil {
		m.revisionErr = err.Error()
	}
}

func (m model) updateRevisionDrawer(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if message.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	if m.revisionConfirm {
		if message.Type == tea.KeyEsc || message.String() == "n" {
			m.revisionConfirm = false
			m.status = "Restore canceled"
			return m, nil
		}
		if message.String() != "y" {
			return m, nil
		}
		return m.restoreSelectedRevision()
	}

	switch message.Type {
	case tea.KeyEsc, tea.KeyCtrlZ:
		m.revisionOpen = false
		m.revisions = nil
		m.revisionErr = ""
	case tea.KeyCtrlR:
		m.openRevisionDrawer()
	case tea.KeyUp:
		m.revisionCursor = max(0, m.revisionCursor-1)
	case tea.KeyDown:
		m.revisionCursor = min(max(0, len(m.revisions)-1), m.revisionCursor+1)
	case tea.KeyPgUp:
		m.revisionCursor = max(0, m.revisionCursor-m.revisionVisibleCount())
	case tea.KeyPgDown:
		m.revisionCursor = min(max(0, len(m.revisions)-1), m.revisionCursor+m.revisionVisibleCount())
	case tea.KeyHome:
		m.revisionCursor = 0
	case tea.KeyEnd:
		m.revisionCursor = max(0, len(m.revisions)-1)
	case tea.KeyEnter:
		if len(m.revisions) > 0 && m.revisionErr == "" {
			m.revisionConfirm = true
			m.status = ""
		}
	}
	return m, nil
}

func (m model) restoreSelectedRevision() (tea.Model, tea.Cmd) {
	if len(m.revisions) == 0 {
		return m, nil
	}
	index := min(max(0, m.revisionCursor), len(m.revisions)-1)
	selected := m.revisions[index]
	path, err := aliasesPath()
	if err == nil {
		err = restoreRevisionFile(path, selected)
	}
	if err != nil {
		m.revisionConfirm = false
		m.revisionErr = err.Error()
		return m, nil
	}
	aliases, err := loadAliases()
	if err != nil {
		m.revisionConfirm = false
		m.revisionErr = "Restored, but reload failed: " + err.Error()
		return m, nil
	}
	m.aliases = aliases
	m.query = ""
	m.cursor = 0
	m.revisionOpen = false
	m.revisionConfirm = false
	m.revisions = nil
	m.status = "Restored " + selected.Time.Local().Format("Jan 2, 15:04") + " · previous version saved"
	return m, nil
}

func (m model) revisionVisibleCount() int {
	return max(1, (m.height-14)/3)
}

func (m model) revisionDrawerView(width, height, contentWidth int, header string) string {
	var body strings.Builder
	body.WriteString(titleStyle.Render("Restore an earlier alias file"))
	body.WriteString("\n" + dimStyle.Render("Alias Lens saves the current file before restoring your choice."))
	body.WriteString("\n\n")

	if m.revisionErr != "" {
		body.WriteString(lipgloss.NewStyle().Foreground(coralColor).Render(wrapText(m.revisionErr, contentWidth)))
	} else if len(m.revisions) == 0 {
		body.WriteString(dimStyle.Render("No private revisions exist yet. They appear after you add, edit, delete, or restore an alias."))
	} else {
		cursor := min(max(0, m.revisionCursor), len(m.revisions)-1)
		visible := min(len(m.revisions), m.revisionVisibleCount())
		start := max(0, cursor-visible/2)
		start = min(start, max(0, len(m.revisions)-visible))
		end := min(len(m.revisions), start+visible)
		for index := start; index < end; index++ {
			revision := m.revisions[index]
			marker := "  "
			style := lipgloss.NewStyle().Width(contentWidth-2).Padding(0, 1).Foreground(mutedColor)
			if index == cursor {
				marker = "▶ "
				style = style.Bold(true).Foreground(inkColor).Background(activeColor)
			}
			label := fmt.Sprintf("%s%s   %s", marker, revision.Time.Local().Format("Jan 2, 2006 15:04:05"), formatByteSize(revision.Size))
			body.WriteString(style.Render(label))
			if index < end-1 {
				body.WriteString("\n\n")
			}
		}
		if len(m.revisions) > visible {
			body.WriteString("\n" + dimStyle.Render(matchSummary(start, end, len(m.revisions))))
		}
	}

	footer := dimStyle.Render("↑↓ move  ·  enter restore  ·  ctrl+r refresh  ·  ctrl+z or esc close")
	if m.revisionConfirm && len(m.revisions) > 0 {
		selected := m.revisions[min(max(0, m.revisionCursor), len(m.revisions)-1)]
		footer = lipgloss.NewStyle().Foreground(coralColor).Render("Restore " + selected.Time.Local().Format("Jan 2, 15:04") + "?  y confirm  ·  n or esc cancel")
	}
	page := lipgloss.JoinVertical(lipgloss.Left, header, "", body.String(), "", footer)
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 3).Render(page)
}

func formatByteSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	return fmt.Sprintf("%.1f KB", float64(size)/1024)
}
