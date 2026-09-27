package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	mainTUIHorizontalPadding = 3
	mainTUIContentInset      = 8
	mainTUIMaxContentWidth   = 108
	mainTUISearchMaxWidth    = 72
)

type tuiFrame struct {
	width             int
	height            int
	contentWidth      int
	horizontalPadding int
}

func newMainTUIFrame(width, height int) tuiFrame {
	width = max(48, width)
	height = max(18, height)
	return tuiFrame{
		width:             width,
		height:            height,
		contentWidth:      max(40, min(width-mainTUIContentInset, mainTUIMaxContentWidth)),
		horizontalPadding: mainTUIHorizontalPadding,
	}
}

func newFullWidthTUIFrame(width, height, horizontalPadding int) tuiFrame {
	width = max(48, width)
	height = max(18, height)
	return tuiFrame{
		width:             width,
		height:            height,
		contentWidth:      max(1, width-horizontalPadding*2),
		horizontalPadding: horizontalPadding,
	}
}

func (frame tuiFrame) contentHeight() int {
	return max(1, frame.height-2)
}

func (frame tuiFrame) footerWidth() int {
	// Keep the last column free so terminals that wrap at the right edge do not
	// push the maker line below the viewport.
	return max(1, frame.width-frame.horizontalPadding*2-1)
}

func (frame tuiFrame) makerHeight() int {
	return lipgloss.Height(lipgloss.NewStyle().Width(frame.footerWidth()).Render(makerCredit(frame.footerWidth())))
}

func (frame tuiFrame) measureHeight(content string) int {
	return lipgloss.Height(lipgloss.NewStyle().Width(frame.contentWidth).Render(content))
}

func (frame tuiFrame) measureFooterHeight(content string) int {
	return lipgloss.Height(lipgloss.NewStyle().Width(frame.footerWidth()).Render(content))
}

func (frame tuiFrame) render(page string) string {
	return frame.renderStyled(page, lipgloss.NewStyle())
}

func (frame tuiFrame) renderStyled(page string, style lipgloss.Style) string {
	return style.
		Width(frame.width).
		Height(frame.height).
		Padding(1, frame.horizontalPadding).
		Render(page)
}

func (frame tuiFrame) renderWithMaker(page string) string {
	return frame.render(pageWithMakerWidths(page, frame.contentWidth, frame.footerWidth(), frame.contentHeight()))
}

func (frame tuiFrame) renderWithFooter(body, footer string) string {
	credit := makerCredit(frame.footerWidth())
	footerHeight := frame.measureFooterHeight(footer)
	creditHeight := 0
	if credit != "" {
		creditHeight = frame.makerHeight()
	}
	availableBodyHeight := max(1, frame.contentHeight()-footerHeight-creditHeight)
	if frame.measureHeight(body) > availableBodyHeight {
		body = lipgloss.NewStyle().Width(frame.contentWidth).MaxHeight(availableBodyHeight).Render(body)
	}
	gap := max(0, frame.contentHeight()-frame.measureHeight(body)-footerHeight-creditHeight)
	page := body + strings.Repeat("\n", gap+1) + footer
	if credit != "" {
		page += "\n" + credit
	}
	return frame.render(page)
}

func searchFieldWidth(contentWidth int) int {
	return min(contentWidth, mainTUISearchMaxWidth)
}
