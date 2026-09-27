package main

import (
	"strings"
	"testing"
	"unicode/utf8"

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

func TestSearchFieldWidthAndLongQuery(t *testing.T) {
	for _, contentWidth := range []int{40, 72, 108} {
		fieldWidth := searchFieldWidth(contentWidth)
		search := lipgloss.NewStyle().Width(fieldWidth-2).Padding(0, 1).Border(lipgloss.RoundedBorder()).Render(
			markerPrefix(iconSearch) + searchTextCursorAtWidth(strings.Repeat("g", 200)+"tail", true, fieldWidth),
		)
		if got := lipgloss.Width(search); got != fieldWidth {
			t.Errorf("content width %d: search width = %d, want %d", contentWidth, got, fieldWidth)
		}
		if !strings.Contains(search, "…") || !strings.Contains(search, "tail█") {
			t.Errorf("content width %d: long query lost its visible tail or cursor: %q", contentWidth, search)
		}
	}
}

func TestSearchQueryHasBoundedLength(t *testing.T) {
	query := appendSearchQuery("", strings.Repeat("a", maxSearchQueryRunes+20))
	query = appendSearchQuery(query, "extra")
	if got := utf8.RuneCountInString(query); got != maxSearchQueryRunes {
		t.Fatalf("search query length = %d, want %d", got, maxSearchQueryRunes)
	}
}

func TestSearchInputLimitAcrossPages(t *testing.T) {
	start := strings.Repeat("a", maxSearchQueryRunes-1)
	key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("bc")}

	aliases, _ := (model{query: start}).Update(key)
	if got := aliases.(model).query; got != start+"b" {
		t.Fatalf("alias search query = %q", got)
	}
	help, _ := (model{helpVisible: true, helpQuery: start}).Update(key)
	if got := help.(model).helpQuery; got != start+"b" {
		t.Fatalf("help search query = %q", got)
	}
	repository, _ := (repoPickerModel{query: start}).Update(key)
	if got := repository.(repoPickerModel).query; got != start+"b" {
		t.Fatalf("repository search query = %q", got)
	}
}
