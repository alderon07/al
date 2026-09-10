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
	for _, want := range []string{"Keyboard guide", "Run the selected alias", "Choose a theme with live preview"} {
		if !strings.Contains(view, want) {
			t.Fatalf("keyboard guide does not contain %q:\n%s", want, view)
		}
	}

	closed, _ := help.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if closed.(model).helpVisible {
		t.Fatal("second ? did not close the keyboard guide")
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
