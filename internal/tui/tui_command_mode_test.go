package tui

import (
	"testing"

	tea "alias-lens/internal/tea"
)

func letterKey(letter rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{letter}}
}

func TestCommandAndSearchModesKeepLettersDistinct(t *testing.T) {
	m := model{services: applicationServices(), aliases: []aliasEntry{{Name: "hello", Command: "echo hello"}}, width: 90, height: 24, aliasMode: aliasModeCommand, shortcutProfile: shortcutLinux}
	updated, _ := m.Update(letterKey('h'))
	m = updated.(model)
	if !m.helpVisible {
		t.Fatal("h did not open the keyboard guide in command mode")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(model)
	updated, _ = m.Update(letterKey('/'))
	m = updated.(model)
	if m.aliasMode != aliasModeSearch || m.query != "" {
		t.Fatalf("/ did not enter search mode: %#v", m)
	}
	for _, key := range []tea.KeyMsg{letterKey('h'), letterKey('s'), {Type: tea.KeySpace}, {Type: tea.KeyRunes, Runes: []rune{'A'}, Shift: true}} {
		updated, _ = m.Update(key)
		m = updated.(model)
	}
	if m.query != "hs A" || m.helpVisible || m.statsOpen {
		t.Fatalf("letters triggered actions during search: query=%q help=%v stats=%v", m.query, m.helpVisible, m.statsOpen)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(model)
	if m.aliasMode != aliasModeCommand || m.query != "hs A" {
		t.Fatalf("Esc did not return to command mode with the query: %#v", m)
	}
	updated, _ = m.Update(letterKey('s'))
	if !updated.(model).statsOpen {
		t.Fatal("s did not open Stats after leaving search")
	}
}

func TestStatsLettersTakePriorityOverGlobalActions(t *testing.T) {
	m := model{services: applicationServices(), statsOpen: true, width: 90, height: 24, aliasMode: aliasModeCommand, shortcutProfile: shortcutLinux}
	updated, _ := m.Update(letterKey('a'))
	m = updated.(model)
	if !m.statsOpen || m.adding || m.statsViewIndex != 1 {
		t.Fatalf("a did not select the Stats aliases view: %#v", m)
	}
	updated, _ = m.Update(letterKey('n'))
	m = updated.(model)
	if m.statsViewIndex != 2 {
		t.Fatalf("n did not move to the next Stats view: %d", m.statsViewIndex)
	}
}

func TestCustomSingleLetterActionWorksInCommandMode(t *testing.T) {
	config := defaultConfig()
	config.Shortcuts = map[string]string{"help": "b"}
	if err := validateAppConfig(config); err != nil {
		t.Fatal(err)
	}
	m := model{services: applicationServices(), width: 90, height: 24, aliasMode: aliasModeCommand, shortcutProfile: resolvedShortcutProfile(config)}
	updated, _ := m.Update(letterKey('b'))
	if !updated.(model).helpVisible {
		t.Fatal("custom one-letter key did not open Help")
	}
}

func TestSyncActionTakesPriorityOverSameLetterPageAction(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	for _, overrides := range []map[string]string{{"delete": "b", "help": "d"}, {"sync.diff": "h"}} {
		config := defaultConfig()
		config.Shortcuts = overrides
		if err := validateAppConfig(config); err != nil {
			t.Fatal(err)
		}
		key := 'd'
		if overrides["sync.diff"] != "" {
			key = 'h'
		}
		m := model{services: applicationServices(), trackedOnly: true, width: 90, height: 24, aliasMode: aliasModeCommand, shortcutProfile: resolvedShortcutProfile(config)}
		updated, _ := m.Update(letterKey(key))
		result := updated.(model)
		if result.helpVisible || !result.trackedOnly || result.status == "" {
			t.Fatalf("sync diff key was intercepted by Help: overrides=%#v page=%v status=%q", overrides, result.currentPage(), result.status)
		}
	}
}

func TestSearchAndFormsKeepTextKeysWhenBoundGlobally(t *testing.T) {
	config := defaultConfig()
	config.Shortcuts = map[string]string{"help": "Backspace", "quit": "q"}
	if err := validateAppConfig(config); err != nil {
		t.Fatal(err)
	}
	profile := resolvedShortcutProfile(config)
	m := model{services: applicationServices(), aliasMode: aliasModeSearch, query: "abc", shortcutProfile: profile}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	result := updated.(model)
	if result.query != "ab" || result.helpVisible {
		t.Fatalf("Backspace did not edit search: %#v", result)
	}
	form := model{services: applicationServices(), adding: true, shortcutProfile: profile}
	updated, _ = form.Update(letterKey('q'))
	result = updated.(model)
	if !result.adding || result.form[0] != "q" {
		t.Fatalf("q canceled the alias form: %#v", result)
	}
	config.Shortcuts["quit"] = "Alt+Q"
	form.shortcutProfile = resolvedShortcutProfile(config)
	updated, _ = form.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}, Alt: true})
	if updated.(model).adding {
		t.Fatal("modified custom Quit key did not cancel the alias form")
	}
}

func TestHelpFilterAcceptsPrintableBoundToPages(t *testing.T) {
	config := defaultConfig()
	config.Shortcuts = map[string]string{"stats": "1"}
	if err := validateAppConfig(config); err != nil {
		t.Fatal(err)
	}
	m := model{services: applicationServices(), helpVisible: true, shortcutProfile: resolvedShortcutProfile(config)}
	updated, _ := m.Update(letterKey('1'))
	result := updated.(model)
	if !result.helpVisible || result.helpQuery != "1" {
		t.Fatalf("printable page key escaped the help filter: %#v", result)
	}
}

func TestSearchLeavesHealthFilter(t *testing.T) {
	m := model{services: applicationServices(), healthOnly: true, aliasMode: aliasModeCommand, shortcutProfile: shortcutLinux}
	updated, _ := m.Update(letterKey('/'))
	result := updated.(model)
	if result.healthOnly || result.aliasMode != aliasModeSearch {
		t.Fatalf("/ did not open the full alias search from Health: %#v", result)
	}
}

func TestFooterTextAcceptsSettingsLetter(t *testing.T) {
	m := model{services: applicationServices(), settingsOpen: true, shortcutProfile: shortcutLinux, settingsForm: appearanceForm(defaultAppearanceConfig(), defaultFooterConfig())}
	updated, _ := m.Update(letterKey('o'))
	result := updated.(model)
	if !result.settingsOpen || result.settingsForm[settingsMessage] == m.settingsForm[settingsMessage] {
		t.Fatalf("o closed Footer Settings instead of editing the message: %#v", result)
	}
}
