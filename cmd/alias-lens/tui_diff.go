package main

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"

	udiff "github.com/aymanbagabas/go-udiff"
	"github.com/aymanbagabas/go-udiff/myers"
	"github.com/charmbracelet/lipgloss"
	"github.com/rivo/uniseg"

	tea "alias-lens/cmd/alias-lens/internal/tea"
)

type diffRow struct {
	kind      byte
	oldNumber int
	newNumber int
	oldText   string
	newText   string
	header    string
}

type terminalDiff struct {
	title          string
	oldLabel       string
	newLabel       string
	summary        string
	unified        []diffRow
	split          []diffRow
	aliasUnified   []diffRow
	aliasSplit     []diffRow
	scroll         int
	horizontal     int
	preferSplit    bool
	showAliases    bool
	revisionID     string
	oldHash        string
	newHash        string
	confirmRestore bool
	err            string
	fromCLI        bool
}

func buildTerminalDiff(title, oldLabel, newLabel string, old, new []byte) (*terminalDiff, error) {
	view := &terminalDiff{
		title: title, oldLabel: oldLabel, newLabel: newLabel,
		preferSplit: true, oldHash: contentHash(old), newHash: contentHash(new),
	}
	removed, added, changed := compareAliasFiles(old, new)
	view.summary = fmt.Sprintf("Commands: %d added · %d removed · %d changed", len(added), len(removed), len(changed))
	view.buildAliasRows(old, new)
	if bytes.Equal(old, new) {
		return view, nil
	}
	edits := myers.ComputeEdits(string(old), string(new))
	patch, err := udiff.ToUnifiedDiff(oldLabel, newLabel, string(old), edits, 3)
	if err != nil {
		return nil, fmt.Errorf("compare alias files: %w", err)
	}
	for _, hunk := range patch.Hunks {
		header := fmt.Sprintf("@@ -%d +%d @@", hunk.FromLine, hunk.ToLine)
		view.unified = append(view.unified, diffRow{kind: '@', header: header})
		view.split = append(view.split, diffRow{kind: '@', header: header})
		oldNumber, newNumber := hunk.FromLine, hunk.ToLine
		var deleted, inserted []diffRow
		flush := func() {
			for index := 0; index < max(len(deleted), len(inserted)); index++ {
				row := diffRow{kind: '~'}
				if index < len(deleted) {
					row.oldNumber = deleted[index].oldNumber
					row.oldText = deleted[index].oldText
				}
				if index < len(inserted) {
					row.newNumber = inserted[index].newNumber
					row.newText = inserted[index].newText
				}
				view.split = append(view.split, row)
			}
			deleted, inserted = nil, nil
		}
		for _, line := range hunk.Lines {
			text := strings.TrimSuffix(line.Content, "\n")
			switch line.Kind {
			case udiff.Delete:
				row := diffRow{kind: '-', oldNumber: oldNumber, oldText: text}
				view.unified = append(view.unified, row)
				deleted = append(deleted, row)
				oldNumber++
			case udiff.Insert:
				row := diffRow{kind: '+', newNumber: newNumber, newText: text}
				view.unified = append(view.unified, row)
				inserted = append(inserted, row)
				newNumber++
			case udiff.Equal:
				flush()
				row := diffRow{kind: ' ', oldNumber: oldNumber, newNumber: newNumber, oldText: text, newText: text}
				view.unified = append(view.unified, row)
				view.split = append(view.split, row)
				oldNumber++
				newNumber++
			}
		}
		flush()
	}
	if len(view.unified) == 0 {
		return nil, fmt.Errorf("compare alias files: no line changes found")
	}
	return view, nil
}

func (view *terminalDiff) buildAliasRows(old, new []byte) {
	oldCommands, newCommands := aliasCommandMap(old), aliasCommandMap(new)
	names := make([]string, 0, len(oldCommands)+len(newCommands))
	seen := make(map[string]bool)
	for name := range oldCommands {
		names = append(names, name)
		seen[name] = true
	}
	for name := range newCommands {
		if !seen[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		oldCommand, hadOld := oldCommands[name]
		newCommand, hasNew := newCommands[name]
		if hadOld && hasNew && oldCommand == newCommand {
			continue
		}
		header := diffRow{kind: '@', header: name}
		view.aliasUnified = append(view.aliasUnified, header)
		view.aliasSplit = append(view.aliasSplit, header)
		pair := diffRow{kind: '~'}
		if hadOld {
			pair.oldText = oldCommand
			view.aliasUnified = append(view.aliasUnified, diffRow{kind: '-', oldText: oldCommand})
		}
		if hasNew {
			pair.newText = newCommand
			view.aliasUnified = append(view.aliasUnified, diffRow{kind: '+', newText: newCommand})
		}
		view.aliasSplit = append(view.aliasSplit, pair)
	}
}

func (m *model) openRepositoryDiff() error {
	_, source, target, err := repositoryPaths()
	if err != nil {
		return err
	}
	current, err := readFileLimited(source, aliasFileLimit)
	if err != nil {
		return fmt.Errorf("read current aliases: %w", err)
	}
	tracked, err := readFileLimited(target, aliasFileLimit)
	if os.IsNotExist(err) {
		tracked = nil
	} else if err != nil {
		return fmt.Errorf("read tracked aliases: %w", err)
	}
	view, err := buildTerminalDiff("Repository comparison", "Tracked copy", "Current file", tracked, current)
	if err != nil {
		return err
	}
	m.diff = view
	return nil
}

func (m *model) openSelectedRevisionDiff() error {
	if len(m.revisions) == 0 {
		return nil
	}
	revision := m.revisions[min(max(0, m.revisionCursor), len(m.revisions)-1)]
	path, err := aliasesPath()
	if err != nil {
		return err
	}
	current, err := readFileLimited(path, aliasFileLimit)
	if err != nil {
		return fmt.Errorf("read current aliases: %w", err)
	}
	previous, err := readFileLimited(revision.Path, aliasFileLimit)
	if err != nil {
		return fmt.Errorf("read selected revision: %w", err)
	}
	view, err := buildTerminalDiff("Preview restore", "Current file", "Selected revision", current, previous)
	if err != nil {
		return err
	}
	view.revisionID = revision.ID
	m.diff = view
	return nil
}

func (m model) updateDiff(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if message.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	view := m.diff
	if view.confirmRestore {
		switch message.String() {
		case "y":
			return m.restoreSelectedRevision()
		case "n":
			view.confirmRestore = false
			return m, nil
		}
		if message.Type == tea.KeyEsc {
			view.confirmRestore = false
		}
		return m, nil
	}
	rows := view.rows(m.width)
	visible := m.diffVisibleRows()
	switch message.Type {
	case tea.KeyEsc:
		if view.fromCLI {
			return m, tea.Quit
		}
		m.diff = nil
	case tea.KeyUp:
		view.scroll--
	case tea.KeyDown:
		view.scroll++
	case tea.KeyPgUp:
		view.scroll -= visible
	case tea.KeyPgDown:
		view.scroll += visible
	case tea.KeyHome:
		view.scroll = 0
	case tea.KeyEnd:
		view.scroll = len(rows) - visible
	case tea.KeyLeft:
		view.horizontal = max(0, view.horizontal-8)
	case tea.KeyRight:
		view.horizontal += 8
	case tea.KeyRunes:
		switch message.String() {
		case "q":
			if view.fromCLI {
				return m, tea.Quit
			}
			m.diff = nil
		case "j":
			view.scroll++
		case "k":
			view.scroll--
		case "n":
			view.scroll = nextDiffHunk(rows, view.scroll, 1)
		case "p":
			view.scroll = nextDiffHunk(rows, view.scroll, -1)
		case "s":
			view.preferSplit = !view.preferSplit
			view.scroll = 0
		case "a":
			view.showAliases = !view.showAliases
			view.scroll = 0
		case "r":
			if view.revisionID != "" {
				view.confirmRestore = true
			}
		}
	}
	if m.diff != nil {
		view.scroll = min(max(0, view.scroll), max(0, len(view.rows(m.width))-visible))
	}
	return m, nil
}

func (view *terminalDiff) rows(width int) []diffRow {
	if view.showAliases {
		if view.preferSplit && newMainTUIFrame(width, 24).contentWidth >= 94 {
			return view.aliasSplit
		}
		return view.aliasUnified
	}
	if view.preferSplit && newMainTUIFrame(width, 24).contentWidth >= 94 {
		return view.split
	}
	return view.unified
}

func nextDiffHunk(rows []diffRow, current, direction int) int {
	for index := current + direction; index >= 0 && index < len(rows); index += direction {
		if rows[index].kind == '@' {
			return index
		}
	}
	return current
}

func (m model) diffVisibleRows() int {
	return max(1, m.height-12)
}

func (m model) diffView(frame tuiFrame, header string) string {
	view := m.diff
	width := frame.contentWidth
	rows := view.rows(m.width)
	split := view.preferSplit && width >= 94
	var body strings.Builder
	body.WriteString(titleStyle.Render(view.title))
	body.WriteString("\n" + dimStyle.Render(view.oldLabel+"  →  "+view.newLabel))
	mode := "Full file"
	if view.showAliases {
		mode = "Changed commands"
	}
	body.WriteString("\n" + dimStyle.Render(view.summary+" · "+mode))
	if view.err != "" {
		body.WriteString("\n" + lipgloss.NewStyle().Foreground(coralColor).Render(wrapText(view.err, width)))
	}
	body.WriteString("\n\n")
	if len(rows) == 0 {
		if view.showAliases {
			body.WriteString(dimStyle.Render("No parsed commands changed. Press a to inspect the full file."))
		} else {
			body.WriteString(dimStyle.Render("Files match exactly. No changes to show."))
		}
	} else {
		if split {
			pane := (width - 3) / 2
			body.WriteString(dimStyle.Render(padDiffCell(clipDiffText(view.oldLabel, 0, pane), pane) + " │ " + clipDiffText(view.newLabel, 0, pane)))
		} else {
			body.WriteString(dimStyle.Render("OLD   NEW   CHANGE"))
		}
		visible := m.diffVisibleRows()
		start := min(max(0, view.scroll), max(0, len(rows)-visible))
		for _, row := range rows[start:min(len(rows), start+visible)] {
			body.WriteByte('\n')
			if split {
				body.WriteString(renderSplitDiffRow(row, width, view.horizontal))
			} else {
				body.WriteString(renderUnifiedDiffRow(row, width, view.horizontal))
			}
		}
	}
	footerText := "↑↓/jk scroll · n/p change · ←→ pan · s layout · a commands · esc back"
	if view.revisionID != "" {
		footerText = "↑↓/jk scroll · n/p change · s layout · a commands · r restore · esc back"
	}
	if view.confirmRestore {
		footerText = "Restore this revision? y confirm · n or esc cancel"
	}
	footer := dimStyle.Render(footerText)
	page := lipgloss.JoinVertical(lipgloss.Left, header, "", body.String())
	return frame.renderWithFooter(page, footer)
}

func renderUnifiedDiffRow(row diffRow, width, horizontal int) string {
	if row.kind == '@' {
		return lipgloss.NewStyle().Foreground(cyanColor).Render(clipDiffText(row.header, 0, width))
	}
	text := row.oldText
	if row.kind == '+' {
		text = row.newText
	}
	prefix := fmt.Sprintf("%4s %4s %c ", diffNumber(row.oldNumber), diffNumber(row.newNumber), row.kind)
	style := diffStyle(row.kind)
	return style.Render(prefix + clipDiffText(text, horizontal, max(1, width-lipgloss.Width(prefix))))
}

func renderSplitDiffRow(row diffRow, width, horizontal int) string {
	if row.kind == '@' {
		return lipgloss.NewStyle().Foreground(cyanColor).Render(clipDiffText(row.header, 0, width))
	}
	pane := (width - 3) / 2
	leftMark, rightMark := byte(' '), byte(' ')
	if row.kind == '~' {
		if row.oldNumber > 0 || row.oldText != "" {
			leftMark = '-'
		}
		if row.newNumber > 0 || row.newText != "" {
			rightMark = '+'
		}
	}
	left := diffPane(row.oldNumber, leftMark, row.oldText, pane, horizontal)
	right := diffPane(row.newNumber, rightMark, row.newText, pane, horizontal)
	return diffStyle(leftMark).Render(left) + dimStyle.Render(" │ ") + diffStyle(rightMark).Render(right)
}

func diffPane(number int, marker byte, value string, width, horizontal int) string {
	prefix := fmt.Sprintf("%4s %c ", diffNumber(number), marker)
	if number == 0 && value == "" {
		return strings.Repeat(" ", width)
	}
	return padDiffCell(prefix+clipDiffText(value, horizontal, max(1, width-lipgloss.Width(prefix))), width)
}

func diffNumber(number int) string {
	if number == 0 {
		return ""
	}
	return fmt.Sprint(number)
}

func diffStyle(kind byte) lipgloss.Style {
	switch kind {
	case '+':
		return lipgloss.NewStyle().Foreground(acidColor)
	case '-':
		return lipgloss.NewStyle().Foreground(coralColor)
	default:
		return lipgloss.NewStyle().Foreground(inkColor)
	}
}

func padDiffCell(value string, width int) string {
	return value + strings.Repeat(" ", max(0, width-lipgloss.Width(value)))
}

func clipDiffText(value string, offset, width int) string {
	if width <= 0 {
		return ""
	}
	safe := terminalSafeText(value)
	graphemes := uniseg.NewGraphemes(safe)
	var output strings.Builder
	for index := 0; index < offset && graphemes.Next(); index++ {
	}
	used := 0
	if offset > 0 {
		output.WriteRune('‹')
		used++
	}
	for graphemes.Next() {
		cluster := graphemes.Str()
		cellWidth := graphemes.Width()
		if used+cellWidth > width {
			return truncate(output.String()+"…", width)
		}
		output.WriteString(cluster)
		used += cellWidth
	}
	return output.String()
}
