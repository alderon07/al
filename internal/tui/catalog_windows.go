//go:build windows

package tui

import "github.com/alderon07/al/internal/app"

import (
	tea "github.com/alderon07/al/internal/tea"
)

type catalogTUIView struct {
	Shell   string
	Loading bool
	Err     string
}
type catalogTUILoadedMsg struct{ View *catalogTUIView }
type catalogTUIAppliedMsg struct{ Err error }

func loadCatalogTUI(services *app.Services, shell string) tea.Cmd {
	return func() tea.Msg {
		return catalogTUILoadedMsg{&catalogTUIView{Shell: shell, Err: "Catalog activation needs Bash or Zsh on Linux, WSL, or macOS"}}
	}
}
func (m model) updateCatalogTUI(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if message.Type == tea.KeyEsc {
		m.catalogView = nil
	}
	return m, nil
}
func (m model) catalogTUIView(frame tuiFrame, header string) string {
	return frame.renderWithFooter(header+"\n"+m.catalogView.Err, "esc back")
}
