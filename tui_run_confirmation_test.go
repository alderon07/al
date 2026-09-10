package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRiskyAliasRequiresExplicitConfirmation(t *testing.T) {
	risky := Alias{Name: "wipe", Command: "git reset --hard", Description: "Discard local changes"}
	m := model{aliases: []Alias{risky}, query: "wipe", width: 90, height: 24, theme: builtInTheme("phosphor"), executeMode: true}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	confirm := updated.(model)
	if cmd != nil || confirm.runConfirm == nil || confirm.selected != nil {
		t.Fatalf("risky alias ran without confirmation: confirm=%#v selected=%#v", confirm.runConfirm, confirm.selected)
	}
	view := confirm.View()
	for _, want := range []string{"Review before running", "wipe", "git reset --hard", "discards uncommitted Git changes", "y", "run alias"} {
		if !strings.Contains(view, want) {
			t.Fatalf("confirmation does not contain %q:\n%s", want, view)
		}
	}

	ignored, cmd := confirm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil || ignored.(model).selected != nil {
		t.Fatal("Enter should not confirm a risky alias")
	}

	confirmed, cmd := confirm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	result := confirmed.(model)
	if cmd == nil || result.selected == nil || result.selected.Name != "wipe" || result.runConfirm != nil {
		t.Fatalf("y did not confirm the risky alias: selected=%#v confirm=%#v", result.selected, result.runConfirm)
	}
}

func TestRiskyAliasConfirmationCanBeCanceled(t *testing.T) {
	risky := Alias{Name: "clean", Command: "rm -rf ./build", Description: "Delete build output"}
	m := model{aliases: []Alias{risky}, query: "clean", width: 90, height: 24, executeMode: true}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	canceled, cmd := updated.(model).Update(tea.KeyMsg{Type: tea.KeyEsc})
	result := canceled.(model)
	if cmd != nil || result.runConfirm != nil || result.selected != nil || result.status != "Run canceled" {
		t.Fatalf("Esc did not cancel cleanly: %#v", result)
	}
}

func TestNormalAliasStillRunsWithOneEnter(t *testing.T) {
	normal := Alias{Name: "gs", Command: "git status -sb", Description: "Show status"}
	m := model{aliases: []Alias{normal}, width: 90, height: 24, executeMode: true}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result := updated.(model)
	if cmd == nil || result.selected == nil || result.selected.Name != "gs" || result.runConfirm != nil {
		t.Fatalf("normal alias did not run directly: selected=%#v confirm=%#v", result.selected, result.runConfirm)
	}
}

func TestDangerousCommandReasonsIdentifyEachRisk(t *testing.T) {
	tests := map[string]string{
		"git push --force-with-lease": "uses a force option",
		"git clean -fd":               "deletes untracked Git files",
		"rm -rf ./build":              "recursively deletes files",
		"git branch -D old":           "force-deletes a Git branch",
	}
	for command, want := range tests {
		if got := strings.Join(dangerousCommandReasons(command), " "); !strings.Contains(got, want) {
			t.Errorf("%q reason = %q, want %q", command, got, want)
		}
	}
}
