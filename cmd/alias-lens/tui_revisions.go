package main

import (
	"fmt"
	"os"
	"strings"

	tea "alias-lens/cmd/alias-lens/internal/tea"
	"github.com/charmbracelet/lipgloss"
)

func (m *model) openRevisionDrawer() {
	m.revisionOpen = true
	m.diff = nil
	m.revisionCursor = 0
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
	if matchesShortcut(message, m.shortcutProfile, shortcutRefresh) {
		m.openRevisionDrawer()
		return m, nil
	}

	switch message.Type {
	case tea.KeyEsc, tea.KeyCtrlZ:
		m.revisionOpen = false
		m.revisions = nil
		m.revisionErr = ""
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
			if err := m.openSelectedRevisionDiff(); err != nil {
				m.revisionErr = err.Error()
			}
		}
	}
	return m, nil
}

func (m model) restoreSelectedRevision() (tea.Model, tea.Cmd) {
	if len(m.revisions) == 0 || m.diff == nil || m.diff.revisionID == "" {
		return m, nil
	}
	index := min(max(0, m.revisionCursor), len(m.revisions)-1)
	selected := m.revisions[index]
	if selected.ID != m.diff.revisionID {
		m.diff.err = "The selected revision changed. Close this preview and open it again."
		m.diff.confirmRestore = false
		return m, nil
	}
	path, err := aliasesPath()
	if err == nil {
		var current, previous []byte
		var mode os.FileMode
		current, mode, _, err = readAliasFile(path)
		if err == nil {
			previous, err = readFileLimited(selected.Path, aliasFileLimit)
		}
		if err == nil && (contentHash(current) != m.diff.oldHash || contentHash(previous) != m.diff.newHash) {
			err = fmt.Errorf("aliases or this revision changed since preview; reopen the diff before restoring")
		}
		if err == nil {
			err = writeAliasFile(path, current, previous, mode)
		}
	}
	if err != nil {
		m.diff.confirmRestore = false
		m.diff.err = err.Error()
		return m, nil
	}
	aliases, err := loadAliases()
	if err != nil {
		m.diff.confirmRestore = false
		m.diff.err = "Restored, but reload failed: " + err.Error()
		return m, nil
	}
	m.aliases = aliases
	m.query = ""
	m.cursor = 0
	m.revisionOpen = false
	m.revisions = nil
	m.diff = nil
	m.status = "Restored " + selected.Time.Local().Format("Jan 2, 15:04") + " · previous version saved"
	return m, nil
}

func (m model) revisionVisibleCount() int {
	return max(1, (m.height-14)/3)
}

func (m model) revisionDrawerView(frame tuiFrame, header string) string {
	contentWidth := frame.contentWidth
	var body strings.Builder
	body.WriteString(pixelIconLabel(iconHistory, "Restore an earlier alias file", titleStyle))
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

	footer := dimStyle.Render("↑↓ move  ·  enter preview  ·  " + strings.ToLower(shortcutLabel(m.shortcutProfile, shortcutRefresh)) + " refresh  ·  " + strings.ToLower(shortcutLabel(m.shortcutProfile, shortcutRevisions)) + " or esc close")
	footer = footerWithNavigation(footer, contentWidth, m.shortcutProfile)
	page := lipgloss.JoinVertical(lipgloss.Left, header, "", body.String())
	return frame.renderWithFooter(page, footer)
}

func formatByteSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	return fmt.Sprintf("%.1f KB", float64(size)/1024)
}
