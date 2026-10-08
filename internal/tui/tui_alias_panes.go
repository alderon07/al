package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const wideAliasDetailMaxWidth = 72

func (m model) wideAliasBrowser(aliases []aliasEntry, cursor, width, height int) string {
	listWidth := min(50, max(38, width*2/5))
	detailWidth := width - listWidth - 3
	rowsVisible := max(1, height-3)
	start := max(0, min(cursor-rowsVisible+1, len(aliases)-rowsVisible))
	end := min(len(aliases), start+rowsVisible)

	listLines := []string{titleStyle.Render("Aliases"), ""}
	for index := start; index < end; index++ {
		listLines = append(listLines, m.wideAliasRow(aliases[index], index == cursor, listWidth))
	}
	for len(listLines) < height-1 {
		listLines = append(listLines, strings.Repeat(" ", listWidth))
	}
	summary := fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(aliases))
	if strings.TrimSpace(m.query) == "" && len(aliases) < len(m.aliases) {
		summary += " suggestions"
	}
	listLines = append(listLines, dimStyle.Render(padRight(summary, listWidth)))

	detailLines := strings.Split(m.aliasDetailCard(aliases[cursor], detailWidth, height), "\n")
	if len(detailLines) > height {
		detailLines = append(detailLines[:height-1], dimStyle.Render("More details hidden; enlarge terminal"))
	}
	for len(detailLines) < height {
		detailLines = append(detailLines, "")
	}
	rows := make([]string, height)
	for index := range rows {
		rows[index] = padRight(listLines[index], listWidth) + "   " + padRight(detailLines[index], detailWidth)
	}
	return strings.Join(rows, "\n")
}

func (m model) wideAliasRow(raw aliasEntry, active bool, width int) string {
	alias := terminalSafeAlias(raw)
	marker := "  "
	if active {
		marker = "▶ "
	}

	nameWidth := min(10, max(8, width/4))
	suffix := ""
	if alias.Category != "" {
		suffix += "  " + categoryBadge(ansi.Truncate(alias.Category, 6, "…"))
	}
	var status []string
	if alias.Favorite {
		if marker := interfaceMarker(iconFavorite); marker != "" {
			status = append(status, lipgloss.NewStyle().Bold(true).Foreground(amberColor).Render(marker))
		}
	}
	if m.context.Match(raw) > 0 {
		marker := interfaceMarker(iconContext)
		if marker == "" {
			marker = "LOCAL"
		}
		status = append(status, lipgloss.NewStyle().Bold(true).Foreground(cyanColor).Render(marker))
	}
	if len(alias.Issues) > 0 {
		status = append(status, lipgloss.NewStyle().Bold(true).Foreground(coralColor).Render("ISSUE"))
	}
	if len(status) > 0 {
		suffix += "  " + strings.Join(status, " ")
	}
	name := padRight(ansi.Truncate(alias.Name, nameWidth, "…"), nameWidth)
	content := ansi.Truncate(marker+aliasStyle.Render(name)+suffix, width-1, "…")
	border := lineColor
	if active {
		border = acidColor
	}
	return lipgloss.NewStyle().Width(max(1, width-1)).Border(lipgloss.ThickBorder(), false, false, false, true).BorderForeground(border).Render(content)
}

func (m model) aliasDetailCard(raw aliasEntry, width, height int) string {
	width = min(width, wideAliasDetailMaxWidth)
	content := m.aliasDetailContent(raw, max(8, width-4), height)
	content = fillUnstyledBackground(content, panelColor)
	style := lipgloss.NewStyle().Width(max(1, width-1)).Padding(0, 1).Border(lipgloss.ThickBorder(), false, false, false, true).BorderForeground(acidColor)
	if themeCanvasAvailable() {
		style = style.Background(panelColor)
	}
	card := style.Render(content)
	return titleStyle.Render("Selected alias") + "\n\n" + card
}

func (m model) aliasDetailContent(raw aliasEntry, width, height int) string {
	alias := terminalSafeAlias(raw)
	nameStyle := aliasStyle
	gapStyle := lipgloss.NewStyle()
	if themeCanvasAvailable() {
		nameStyle = nameStyle.Background(panelColor)
		gapStyle = gapStyle.Background(panelColor)
	}
	name := nameStyle.Render(alias.Name)
	if alias.Category != "" {
		name += gapStyle.Render("  ") + categoryBadge(alias.Category)
	}
	if height < 8 {
		return name + "\n" + lipgloss.NewStyle().Foreground(cyanColor).Render(wrapText(alias.Command, width))
	}

	kind := "Alias"
	if alias.Type == "function" {
		kind = "Function"
	}
	if alias.Favorite {
		kind += "  ·  Favorite"
	}
	context := "No local mark"
	switch m.context.Match(raw) {
	case 1:
		context = "Marked for this project"
	case 2:
		context = "Marked for this folder"
	}

	lines := []string{name, dimStyle.Render(kind + "  ·  " + context)}
	if len(alias.Issues) > 0 {
		lines = append(lines, "", lipgloss.NewStyle().Bold(true).Foreground(coralColor).Render("NEEDS ATTENTION"))
		appendDetailText(&lines, lipgloss.NewStyle().Foreground(coralColor).Render(strings.Join(alias.Issues, " · ")), width)
	}
	lines = append(lines, "", dimStyle.Render("COMMAND"))
	appendDetailText(&lines, lipgloss.NewStyle().Foreground(cyanColor).Render(alias.Command), width)
	if alias.Description != "" {
		lines = append(lines, "", dimStyle.Render("DESCRIPTION"))
		appendDetailText(&lines, lipgloss.NewStyle().Foreground(inkColor).Render(alias.Description), width)
	}
	if height >= 15 {
		lines = append(lines, m.aliasUsageDetail(raw)...)
	}
	if len(alias.Tags) > 0 {
		lines = append(lines, "", dimStyle.Render("TAGS"))
		appendDetailText(&lines, lipgloss.NewStyle().Foreground(amberColor).Render("#"+strings.Join(alias.Tags, "  #")), width)
	}
	if len(alias.Platforms) > 0 {
		lines = append(lines, "", dimStyle.Render("PLATFORMS"))
		appendDetailText(&lines, lipgloss.NewStyle().Foreground(violetColor).Render(strings.Join(alias.Platforms, "  ·  ")), width)
	}
	return strings.Join(lines, "\n")
}

func (m model) aliasUsageDetail(alias aliasEntry) []string {
	lines := []string{"", dimStyle.Render("USAGE  ·  SHELL HISTORY")}
	if !m.browserUsageReady || m.browserUsageState != "" {
		state := m.browserUsageState
		if state == "" {
			state = "Shell history unavailable"
		}
		return append(lines, dimStyle.Render(state))
	}
	summary := m.browserUsage[alias.Name]
	lines = append(lines, lipgloss.NewStyle().Foreground(inkColor).Render(fmt.Sprintf("All time  %d   Today  %d   Last 7 days  %d", summary.All, summary.Today, summary.Week)))
	lastUsed := "never"
	if summary.All > 0 {
		lastUsed = lastRunLabel(summary.LastRun, time.Now())
	}
	lines = append(lines, dimStyle.Render("Last used  ")+lipgloss.NewStyle().Foreground(inkColor).Render(lastUsed))
	lines = append(lines, dimStyle.Render("Exact command matches  ")+lipgloss.NewStyle().Foreground(inkColor).Render(fmt.Sprint(alias.Usage)))
	return lines
}

func categoryBadge(category string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(pageColor).Background(colorForCategory(category)).Padding(0, 1).Render(strings.ToUpper(category))
}

func appendDetailText(lines *[]string, value string, width int) {
	*lines = append(*lines, strings.Split(ansi.Hardwrap(value, width, true), "\n")...)
}
