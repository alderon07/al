package tui

import (
	tea "alias-lens/internal/tea"

	"testing"
)

func TestAliasFormAcceptsSpacesInCommandsAndDescriptions(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	m := model{services: applicationServices(), adding: true, field: 1}
	m.form[1] = "git"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	command := updated.(model)
	if command.form[1] != "git " {
		t.Fatalf("command field lost its space: %q", command.form[1])
	}

	command.field = 2
	command.form[2] = "Show"
	updated, _ = command.Update(tea.KeyMsg{Type: tea.KeySpace})
	description := updated.(model)
	if description.form[2] != "Show " {
		t.Fatalf("description field lost its space: %q", description.form[2])
	}
}

func TestAliasFormRejectsSpacesInAliasName(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	m := model{services: applicationServices(), adding: true, field: 0}
	m.form[0] = "git"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	result := updated.(model)
	if result.form[0] != "git" || result.status != "Alias names cannot contain spaces" {
		t.Fatalf("alias name accepted a space or lacked guidance: value=%q status=%q", result.form[0], result.status)
	}
}
