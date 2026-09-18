package main

import (
	"strings"
	"testing"

	tea "alias-lens/cmd/alias-lens/internal/tea"
	"github.com/charmbracelet/lipgloss"
)

func TestSearchCursorBlinksAndReschedules(t *testing.T) {
	m := model{}
	updated, command := m.Update(cursorBlinkMsg{})
	result := updated.(model)
	if !result.cursorHidden {
		t.Fatal("blink did not hide the search cursor")
	}
	if command == nil {
		t.Fatal("blink did not schedule the next cursor update")
	}
}

func TestSearchCursorDoesNotBlinkWithoutFocus(t *testing.T) {
	states := []model{
		{helpVisible: true},
		{adding: true},
		{trackedOnly: true},
		{statsOpen: true},
		{terminalBlurred: true},
	}
	for _, state := range states {
		updated, command := state.Update(cursorBlinkMsg{})
		result := updated.(model)
		if !result.cursorHidden {
			t.Fatalf("unfocused cursor became visible: %#v", state)
		}
		updated, _ = result.Update(cursorBlinkMsg{})
		if !updated.(model).cursorHidden {
			t.Fatalf("unfocused cursor blinked on the next tick: %#v", state)
		}
		if command == nil {
			t.Fatal("focus polling was not rescheduled")
		}
	}
}

func TestTerminalFocusControlsSearchCursor(t *testing.T) {
	m := model{}
	blurred, _ := m.Update(tea.BlurMsg{})
	if result := blurred.(model); !result.terminalBlurred || !result.cursorHidden || result.searchFocused() {
		t.Fatalf("terminal blur did not hide the cursor: %#v", result)
	}
	focused, _ := blurred.(model).Update(tea.FocusMsg{})
	if result := focused.(model); result.terminalBlurred || result.cursorHidden || !result.searchFocused() {
		t.Fatalf("terminal focus did not restore the cursor: %#v", result)
	}
}

func TestKeyPressShowsSearchCursor(t *testing.T) {
	m := model{cursorHidden: true}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	result := updated.(model)
	if result.cursorHidden || result.query != "g" {
		t.Fatalf("key press did not restore the cursor: %#v", result)
	}
}

func TestSearchCursorKeepsItsWidthWhenHidden(t *testing.T) {
	visible := searchTextCursor("git", true)
	hidden := searchTextCursor("git", false)
	if !strings.Contains(visible, "█") || strings.Contains(hidden, "█") {
		t.Fatalf("unexpected cursor rendering: visible=%q hidden=%q", visible, hidden)
	}
	if lipgloss.Width(visible) != lipgloss.Width(hidden) {
		t.Fatalf("cursor blink changed input width: visible=%d hidden=%d", lipgloss.Width(visible), lipgloss.Width(hidden))
	}
}
