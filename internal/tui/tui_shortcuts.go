package tui

import (
	"fmt"
	"github.com/alderon07/al/internal/shortcuts"
	"strings"
	"unicode/utf8"

	tea "github.com/alderon07/al/internal/tea"
	"github.com/charmbracelet/lipgloss"
)

type shortcutEditorRow struct {
	name  string
	scope string
	key   string
	set   bool
}

func (m model) shortcutEditorRows() []shortcutEditorRow {
	rows := []shortcutEditorRow{{name: "launcher", scope: "shell", key: m.shortcutLauncher, set: !strings.EqualFold(m.shortcutLauncher, "Ctrl+G")}}
	base := baseShortcutProfile(m.shortcutProfile.String())
	for _, definition := range shortcuts.Descriptors() {
		name := shortcutActionName(definition.Action)
		if m.shortcutFilter != "" && !strings.Contains(strings.ToLower(name+" "+definition.Scope), strings.ToLower(m.shortcutFilter)) {
			continue
		}
		key := shortcutLabel(m.shortcutProfile, definition.Action)
		rows = append(rows, shortcutEditorRow{name: name, scope: definition.Scope, key: key, set: key != shortcutLabel(base, definition.Action)})
	}
	if m.shortcutFilter != "" && !strings.Contains("launcher shell", strings.ToLower(m.shortcutFilter)) {
		rows = rows[1:]
	}
	return rows
}

func (m model) updateShortcutEditor(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if message.Type == tea.KeyCtrlC && !m.shortcutCapture {
		return m, tea.Quit
	}
	if m.shortcutCapture {
		if message.Type == tea.KeyEsc && m.shortcutPending != "" {
			m.shortcutCapture = false
			m.shortcutPending = ""
			m.status = "Key change canceled"
			return m, nil
		}
		if message.Paste || message.Repeat {
			return m, nil
		}
		rows := m.shortcutEditorRows()
		if len(rows) == 0 {
			m.shortcutCapture = false
			m.shortcutPending = ""
			return m, nil
		}
		if message.Type == tea.KeyEnter && m.shortcutPending != "" {
			label := m.shortcutPending
			m.shortcutCapture = false
			m.shortcutPending = ""
			return m.saveShortcutEditorChoice(rows[m.shortcutCursor].name, label, false), nil
		}
		label := message.String()
		if message.Type == tea.KeySpace {
			label = "Space"
		}
		m.shortcutPending = label
		m.status = "Press Enter to save " + label + "; Esc cancels"
		return m, nil
	}
	if m.shortcutFiltering {
		switch message.Type {
		case tea.KeyEsc, tea.KeyEnter:
			m.shortcutFiltering = false
		case tea.KeyBackspace:
			if m.shortcutFilter != "" {
				_, size := utf8.DecodeLastRuneInString(m.shortcutFilter)
				m.shortcutFilter = m.shortcutFilter[:len(m.shortcutFilter)-size]
				m.shortcutCursor = 0
			}
		case tea.KeyRunes:
			if acceptsTextInput(message) && !message.Paste {
				m.shortcutFilter = appendSearchQuery(m.shortcutFilter, string(message.Runes))
				m.shortcutCursor = 0
			}
		}
		return m, nil
	}
	rows := m.shortcutEditorRows()
	if len(rows) > 0 {
		m.shortcutCursor = min(m.shortcutCursor, len(rows)-1)
	}
	switch message.Type {
	case tea.KeyEsc:
		m.shortcutsOpen = false
		m.helpVisible = true
		m.shortcutFilter = ""
		m.status = ""
	case tea.KeyUp:
		m.shortcutCursor = max(0, m.shortcutCursor-1)
	case tea.KeyDown:
		m.shortcutCursor = min(len(rows)-1, m.shortcutCursor+1)
	case tea.KeyPgUp:
		m.shortcutCursor = max(0, m.shortcutCursor-m.shortcutEditorVisibleCount())
	case tea.KeyPgDown:
		m.shortcutCursor = min(len(rows)-1, m.shortcutCursor+m.shortcutEditorVisibleCount())
	case tea.KeyHome:
		m.shortcutCursor = 0
	case tea.KeyEnd:
		m.shortcutCursor = max(0, len(rows)-1)
	case tea.KeyEnter:
		if len(rows) > 0 {
			m.shortcutCapture = true
			m.shortcutPending = ""
			m.status = "Press the new key for " + rows[m.shortcutCursor].name + "; press Esc twice to cancel"
		}
	case tea.KeyDelete, tea.KeyBackspace:
		if len(rows) > 0 {
			return m.saveShortcutEditorChoice(rows[m.shortcutCursor].name, "", true), nil
		}
	case tea.KeyRunes:
		if acceptsTextInput(message) && string(message.Runes) == "/" {
			m.shortcutFiltering = true
			m.status = ""
		}
	}
	return m, nil
}

func (m model) saveShortcutEditorChoice(name, label string, reset bool) model {
	config, err := m.service().Settings.Load()
	if err != nil {
		m.status = "Could not load shortcuts: " + err.Error()
		return m
	}
	if config.Shortcuts == nil {
		config.Shortcuts = make(map[string]string)
	}
	if reset {
		delete(config.Shortcuts, name)
	} else {
		config.Shortcuts[name] = label
		for _, definition := range shortcuts.Descriptors() {
			if shortcutActionName(definition.Action) != name {
				continue
			}
			defaults := shortcuts.Labels(sharedShortcutProfile(baseShortcutProfile(m.shortcutProfile.String())), definition.Action)
			if len(defaults) == 1 && strings.EqualFold(defaults[0], label) {
				delete(config.Shortcuts, name)
			}
			break
		}
	}
	if err := m.service().Settings.Save(config); err != nil {
		m.status = "Could not save " + name + ": " + err.Error()
		return m
	}
	m.shortcutProfile = resolvedShortcutProfile(config)
	m.shortcutLauncher = launcherLabel(config)
	m.status = name + " saved"
	if name == "launcher" {
		m.status += "; reload your shell integration to use it"
	}
	return m
}

func (m model) shortcutEditorVisibleCount() int {
	return max(1, m.height-12)
}

func (m model) shortcutEditorView(frame tuiFrame, header string) string {
	width := frame.contentWidth
	rows := m.shortcutEditorRows()
	title := pixelIconLabel(iconHelp, "Configure shortcuts", titleStyle)
	subtitle := dimStyle.Render("Select an action, then press Enter and your new key. Changes save immediately.")
	if width < 72 {
		subtitle = dimStyle.Render("Enter changes a key. Changes save immediately.")
	}
	controls := "↑↓ select  ·  enter change  ·  delete reset  ·  / filter  ·  esc back"
	if m.shortcutFiltering {
		controls = "Filter: " + m.shortcutFilter + "_  ·  enter finish  ·  esc finish"
	}
	if m.shortcutCapture {
		controls = "Press a key, then Enter to save  ·  Esc cancels after a key"
	}
	footer := dimStyle.Render(wrapText(controls, width))
	if m.status != "" {
		footer += "\n" + lipgloss.NewStyle().Foreground(amberColor).Render(truncate(m.status, width))
	}
	rowBudget := max(1, frame.contentHeight()-frame.measureHeight(header)-frame.measureHeight(title)-frame.measureHeight(subtitle)-frame.measureHeight(footer)-frame.makerHeight()-5)
	start := max(0, m.shortcutCursor-rowBudget+1)
	if start > 0 && m.shortcutCursor < start+rowBudget/2 {
		start = max(0, m.shortcutCursor-rowBudget/2)
	}
	end := min(len(rows), start+rowBudget)
	var body strings.Builder
	for index := start; index < end; index++ {
		row := rows[index]
		marker := "  "
		style := dimStyle
		if index == m.shortcutCursor {
			marker = "› "
			style = aliasStyle
		}
		changed := ""
		if row.set {
			changed = " *"
		}
		nameWidth := max(10, min(31, width/2))
		name := fmt.Sprintf("%-*s", nameWidth, truncate(row.name, nameWidth))
		key := truncate(row.key+changed, max(8, width-nameWidth-6))
		body.WriteString(style.Render(marker + name + "  " + key))
		if index < end-1 {
			body.WriteByte('\n')
		}
	}
	if len(rows) == 0 {
		body.WriteString(dimStyle.Render("No actions match " + fmt.Sprintf("%q", m.shortcutFilter)))
	}
	position := dimStyle.Render(fmt.Sprintf("%d of %d  ·  * custom", min(len(rows), m.shortcutCursor+1), len(rows)))
	page := lipgloss.JoinVertical(lipgloss.Left, header, "", title, subtitle, "", position, body.String())
	return frame.renderWithFooter(page, footer)
}
