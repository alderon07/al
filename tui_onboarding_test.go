package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestEmptyAliasFileShowsOnboardingAndEnterStartsAdd(t *testing.T) {
	t.Setenv(activeShellEnvironment, "bash")
	m := model{width: 80, height: 24, theme: builtInTheme("phosphor"), executeMode: true}

	view := m.View()
	for _, want := range []string{"No aliases yet.", "~/.bash_aliases", "Enter", "Create your first alias"} {
		if !strings.Contains(view, want) {
			t.Fatalf("empty state does not contain %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, `No alias matched ""`) {
		t.Fatalf("empty state still looks like a failed search:\n%s", view)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !updated.(model).adding {
		t.Fatal("Enter did not open the add form from the empty state")
	}
}

func TestQuestionMarkOpensAndClosesKeyboardGuide(t *testing.T) {
	m := model{
		aliases:     []Alias{{Name: "gs", Command: "git status", Description: "Show working tree status"}},
		width:       100,
		height:      24,
		theme:       builtInTheme("phosphor"),
		executeMode: true,
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	help := updated.(model)
	if !help.helpVisible {
		t.Fatal("? did not open the keyboard guide")
	}
	view := help.View()
	for _, want := range []string{"Keyboard guide", "Run the selected alias", "Edit description, command, or name"} {
		if !strings.Contains(view, want) {
			t.Fatalf("keyboard guide does not contain %q:\n%s", want, view)
		}
	}
	filtered, _ := help.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("theme")})
	if view := filtered.(model).View(); !strings.Contains(view, "Choose a theme with live preview") || strings.Contains(view, "Add an alias") {
		t.Fatalf("keyboard guide did not filter theme shortcuts:\n%s", view)
	}

	closed, _ := help.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if closed.(model).helpVisible {
		t.Fatal("second ? did not close the keyboard guide")
	}
}

func TestEditAliasStartsOnDescription(t *testing.T) {
	m := model{
		aliases: []Alias{{Name: "gs", Command: "git status", Description: "Show status"}},
		query:   "gs",
		width:   90,
		height:  24,
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	edit := updated.(model)
	if !edit.adding || edit.editingName != "gs" || edit.field != 2 || edit.form[2] != "Show status" {
		t.Fatalf("Ctrl+E did not focus the existing description: %#v", edit)
	}
	if view := edit.View(); !strings.Contains(view, "Update its description, command, or name") {
		t.Fatalf("edit form does not explain description editing:\n%s", view)
	}
}

func TestQuestionMarkRemainsTextInsideAliasForm(t *testing.T) {
	m := model{adding: true, width: 80, height: 24}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	got := updated.(model)
	if got.helpVisible || got.form[0] != "?" {
		t.Fatalf("? should be form input, got help=%v value=%q", got.helpVisible, got.form[0])
	}
}
