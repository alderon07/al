package tui

import (
	"strings"
	"testing"

	tea "github.com/alderon07/al/internal/tea"
)

func TestSetupTourAppearsOnceAndCanOpenHelp(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	if err := seedTourFixture(); err != nil {
		t.Fatal(err)
	}
	if !applicationServices().TourShouldShow() {
		t.Fatal("scheduled tour was not visible")
	}
	m := model{services: applicationServices(), tourVisible: true, width: 80, height: 24}
	view := m.View()
	for _, want := range []string{"Alias Lens is ready.", "/", "Enter", "t", "searchable keyboard guide"} {
		if !strings.Contains(view, want) {
			t.Fatalf("tour does not contain %q:\n%s", want, view)
		}
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	result := updated.(model)
	if result.tourVisible || !result.helpVisible || applicationServices().TourShouldShow() {
		t.Fatalf("tour did not dismiss into help: %#v", result)
	}

	if applicationServices().TourShouldShow() {
		t.Fatal("completed tour was rescheduled")
	}
}
