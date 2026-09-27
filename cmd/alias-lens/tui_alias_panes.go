package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (m model) wideAliasBrowser(aliases []Alias, cursor, width, height int) string {
	listWidth := min(50, max(38, width*2/5))
	detailWidth := width - listWidth - 3
	rowsVisible := max(1, height-3)
	start := max(0, min(cursor-rowsVisible+1, len(aliases)-rowsVisible))
	end := min(len(aliases), start+rowsVisible)

	listLines := []string{titleStyle.Render(padRight("Aliases", listWidth)), ""}
	for index := start; index < end; index++ {
		alias := terminalSafeAlias(aliases[index])
		flags := ""
		if alias.Favorite {
			flags += " *"
		}
		if len(alias.Issues) > 0 {
			flags += " !"
		}
		if m.context.match(aliases[index]) > 0 {
			flags += " @"
		}
		category := ""
		if alias.Category != "" {
			category = "  " + ansi.Truncate(alias.Category, 14, "…")
		}
		nameWidth := max(4, listWidth-3-lipgloss.Width(flags)-lipgloss.Width(category))
		marker := "  "
		if index == cursor {
			marker = "▶ "
		}
		row := padRight(marker+ansi.Truncate(alias.Name, nameWidth, "…")+category+flags, listWidth)
		style := lipgloss.NewStyle().Foreground(inkColor)
		if index == cursor {
			style = style.Foreground(acidColor).Background(activeColor).Bold(true)
		}
		listLines = append(listLines, style.Render(row))
	}
	for len(listLines) < height-1 {
		listLines = append(listLines, strings.Repeat(" ", listWidth))
	}
	summary := fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(aliases))
	if strings.TrimSpace(m.query) == "" && len(aliases) < len(m.aliases) {
		summary += " suggestions"
	}
	listLines = append(listLines, dimStyle.Render(padRight(summary, listWidth)))

	detailLines := m.aliasDetailLines(aliases[cursor], detailWidth, height)
	if len(detailLines) > height {
		detailLines = append(detailLines[:height-1], dimStyle.Render("More details hidden; enlarge terminal"))
	}
	for len(detailLines) < height {
		detailLines = append(detailLines, "")
	}
	rows := make([]string, height)
	for index := range rows {
		rows[index] = listLines[index] + dimStyle.Render(" │ ") + padRight(detailLines[index], detailWidth)
	}
	return strings.Join(rows, "\n")
}

func (m model) aliasDetailLines(raw Alias, width, height int) []string {
	alias := terminalSafeAlias(raw)
	if height < 8 {
		lines := []string{titleStyle.Render("Selected alias")}
		appendDetailText(&lines, aliasStyle.Render(alias.Name), width)
		lines = append(lines, dimStyle.Render("Command"))
		appendDetailText(&lines, lipgloss.NewStyle().Foreground(cyanColor).Render(alias.Command), width)
		return lines
	}
	lines := []string{titleStyle.Render("Selected alias"), ""}
	appendDetailText(&lines, aliasStyle.Render(alias.Name), width)
	kind := "Alias"
	if alias.Type == "function" {
		kind = "Function"
	}
	if alias.Category != "" {
		kind += "  ·  " + alias.Category
	}
	if alias.Favorite {
		kind += "  ·  Favorite"
	}
	appendDetailText(&lines, dimStyle.Render(kind), width)
	context := "No local mark"
	switch m.context.match(raw) {
	case 1:
		context = "Marked for this project"
	case 2:
		context = "Marked for this folder"
	}
	appendDetailText(&lines, dimStyle.Render(context), width)
	if len(alias.Issues) > 0 {
		lines = append(lines, "", lipgloss.NewStyle().Foreground(coralColor).Render("Needs attention"))
		appendDetailText(&lines, strings.Join(alias.Issues, " · "), width)
	}
	lines = append(lines, "", dimStyle.Render("Command"))
	appendDetailText(&lines, lipgloss.NewStyle().Foreground(cyanColor).Render(alias.Command), width)
	if alias.Description != "" {
		lines = append(lines, "", dimStyle.Render("Description"))
		appendDetailText(&lines, alias.Description, width)
	}
	if len(alias.Tags) > 0 {
		lines = append(lines, "", dimStyle.Render("Tags"))
		appendDetailText(&lines, strings.Join(alias.Tags, ", "), width)
	}
	if len(alias.Platforms) > 0 {
		lines = append(lines, "", dimStyle.Render("Platforms"))
		appendDetailText(&lines, strings.Join(alias.Platforms, ", "), width)
	}
	return lines
}

func appendDetailText(lines *[]string, value string, width int) {
	*lines = append(*lines, strings.Split(ansi.Hardwrap(value, width, true), "\n")...)
}
