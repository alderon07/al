package tui

import (
	tea "alias-lens/internal/tea"
	"testing"
)

func TestTabSelectsWithoutExecuting(t *testing.T) {
	initial := model{services: applicationServices(), aliases: []aliasEntry{{Name: "gs", Command: "git status"}}, width: 80, height: 24, executeMode: true}
	updated, command := initial.Update(tea.KeyMsg{Type: tea.KeyTab})
	selected := updated.(model)
	if selected.selected == nil || selected.selected.Name != "gs" || !selected.editSelection || command == nil {
		t.Fatalf("tab selection = %#v, command nil = %t", selected.selected, command == nil)
	}
}
