package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSetupTourAppearsOnceAndCanOpenHelp(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := scheduleTour(); err != nil {
		t.Fatal(err)
	}
	if !tourShouldShow() {
		t.Fatal("scheduled tour was not visible")
	}
	m := model{tourVisible: true, width: 80, height: 24}
	view := m.View()
	for _, want := range []string{"Alias Lens is ready.", "Type", "Enter", "Ctrl+T", "searchable keyboard guide"} {
		if !strings.Contains(view, want) {
			t.Fatalf("tour does not contain %q:\n%s", want, view)
		}
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	result := updated.(model)
	if result.tourVisible || !result.helpVisible || tourShouldShow() {
		t.Fatalf("tour did not dismiss into help: %#v", result)
	}
	if err := scheduleTour(); err != nil {
		t.Fatal(err)
	}
	if tourShouldShow() {
		t.Fatal("completed tour was rescheduled")
	}
}
