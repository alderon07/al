package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/alderon07/al/internal/tea"
)

func TestDeleteShortcutProtectsSearchText(t *testing.T) {
	aliases := []aliasEntry{{Name: "gs", Command: "git status"}}

	updated, _ := (model{services: applicationServices(), aliases: aliases, shortcutProfile: shortcutLinux}).Update(tea.KeyMsg{Type: tea.KeyDelete})
	if got := updated.(model).deleteName; got != "gs" {
		t.Fatalf("Delete on an empty search selected %q for deletion, want gs", got)
	}

	updated, _ = (model{services: applicationServices(), aliases: aliases, query: "g", shortcutProfile: shortcutLinux}).Update(tea.KeyMsg{Type: tea.KeyDelete})
	result := updated.(model)
	if result.query != "" || result.deleteName != "" {
		t.Fatalf("Delete with search text produced query %q and deletion %q", result.query, result.deleteName)
	}

	updated, _ = (model{services: applicationServices(), aliases: aliases, shortcutProfile: shortcutMacOS}).Update(tea.KeyMsg{Type: tea.KeyBackspace, Super: true})
	if got := updated.(model).deleteName; got != "gs" {
		t.Fatalf("Cmd+Backspace selected %q for deletion, want gs", got)
	}
}

func TestShortcutGuideUsesFriendlyMacLabels(t *testing.T) {
	rows := shortcutGuide(shortcutMacOS, false)
	var text strings.Builder
	for _, row := range rows {
		text.WriteString(row[0])
		text.WriteString(" ")
		text.WriteString(row[1])
		text.WriteString("\n")
	}
	guide := text.String()
	for _, want := range []string{"a / Cmd+N / Ctrl+N", "Add an alias", "y / Cmd+6 / F6", "saved versions"} {
		if !strings.Contains(guide, want) {
			t.Fatalf("macOS shortcut guide is missing %q:\n%s", want, guide)
		}
	}
	view := (model{services: applicationServices(), width: 120, height: 36, helpVisible: true, shortcutProfile: shortcutMacOS}).View()
	for _, want := range []string{"a / Cmd+N / Ctrl+N", "d / Cmd+Backspace / Delete", "y / Cmd+6 / F6", "o / Cmd+, / F3", "h / Cmd+? / F1 / ? / Esc"} {
		if !strings.Contains(view, want) {
			t.Fatalf("rendered macOS keyboard guide is missing %q:\n%s", want, view)
		}
	}
}

func TestWideNavigationUsesReadableModifierNames(t *testing.T) {
	hint := pageNavigationHint(120, shortcutLinux)
	for _, want := range []string{"h help", "o footer", "v versions"} {
		if !strings.Contains(hint, want) {
			t.Fatalf("wide navigation hint is missing %q: %s", want, hint)
		}
	}
}

func TestPreferredSettingsShortcutTogglesSettingsPage(t *testing.T) {
	tests := []struct {
		profile shortcutProfile
		key     tea.KeyMsg
	}{
		{profile: shortcutLinux, key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}, Ctrl: true}},
		{profile: shortcutWindows, key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}, Ctrl: true}},
		{profile: shortcutMacOS, key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}, Super: true}},
	}
	for _, test := range tests {
		m := model{services: applicationServices(), settingsOpen: true, shortcutProfile: test.profile}
		updated, _ := m.Update(test.key)
		if updated.(model).settingsOpen {
			t.Errorf("%s settings shortcut did not return to aliases", test.profile)
		}
	}
}

func TestPreferredControlPunctuationOpensPages(t *testing.T) {
	tests := []struct {
		name string
		key  tea.KeyMsg
		page tuiPage
	}{
		{name: "help", key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}, Ctrl: true, Shift: true}, page: pageHelp},
		{name: "settings", key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}, Ctrl: true}, page: pageSettings},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page, ok := pageForShortcut(test.key, shortcutLinux)
			if !ok || page != test.page {
				t.Fatalf("pageForShortcut(%s) = %d, %t, want %d, true", test.key.String(), page, ok, test.page)
			}
		})
	}
}

func TestLegacyControlQuestionEncodingOpensHelp(t *testing.T) {
	key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'_'}, Ctrl: true}
	page, ok := pageForShortcut(key, shortcutLinux)
	if !ok || page != pageHelp {
		t.Fatalf("pageForShortcut(%s) = %d, %t, want %d, true", key.String(), page, ok, pageHelp)
	}
}

func TestUnassignedCommandKeysDoNotBecomeText(t *testing.T) {
	key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}, Super: true}
	browser, _ := (model{services: applicationServices(), query: "git", shortcutProfile: shortcutMacOS}).Update(key)
	if got := browser.(model).query; got != "git" {
		t.Fatalf("Command+C changed search text to %q", got)
	}
	form, _ := (model{services: applicationServices(), adding: true, field: 1, form: [5]string{"gs", "git"}, shortcutProfile: shortcutMacOS}).Update(key)
	if got := form.(model).form[1]; got != "git" {
		t.Fatalf("Command+C changed form text to %q", got)
	}
}

func TestPastedAndModifiedKeysDoNotTriggerActions(t *testing.T) {
	pastedQuestion := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}, Paste: true}
	updated, _ := (model{services: applicationServices(), shortcutProfile: shortcutMacOS}).Update(pastedQuestion)
	result := updated.(model)
	if result.helpVisible || result.query != "?" {
		t.Fatalf("pasted question mark triggered help or was lost: %#v", result)
	}
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{'n'}, Alt: true},
		{Type: tea.KeyRunes, Runes: []rune{'n'}, Ctrl: true},
		{Type: tea.KeyRunes, Runes: []rune{'n'}, Meta: true},
	} {
		updated, _ := (model{services: applicationServices(), query: "git", shortcutProfile: shortcutMacOS}).Update(key)
		if got := updated.(model).query; got != "git" {
			t.Fatalf("modified key changed search text to %q: %#v", got, key)
		}
		if matchesShortcut(key, shortcutMacOS, shortcutAdd) {
			t.Fatalf("modified key triggered add: %#v", key)
		}
	}
	form, _ := (model{services: applicationServices(), adding: true, field: 1, form: [5]string{"gs", "git"}, shortcutProfile: shortcutMacOS}).Update(tea.KeyMsg{Type: tea.KeySpace, Super: true})
	if got := form.(model).form[1]; got != "git" {
		t.Fatalf("modified Space changed form text to %q", got)
	}
}

func TestInvalidSettingsStopPickerBeforeAliasAccess(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".config", "alias-lens", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":2,"shell":"zsh","alias_file":".zsh_aliases","shortcut_profile":"amiga","footer":{"message":"x","icon":"none"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runAliasPicker(applicationServices(),

		"", false, false); err == nil || !strings.Contains(err.Error(), "shortcut style") {
		t.Fatalf("picker settings error = %v", err)
	}
	for _, name := range []string{".bash_aliases", ".zsh_aliases"} {
		if _, err := os.Stat(filepath.Join(home, name)); !os.IsNotExist(err) {
			t.Fatalf("picker touched %s before rejecting settings: %v", name, err)
		}
	}
}

func TestMacStatsDoesNotAcceptUndisclosedControlShortcut(t *testing.T) {
	m := model{services: applicationServices(), statsOpen: true, shortcutProfile: shortcutMacOS}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if !updated.(model).statsOpen {
		t.Fatal("Ctrl+S closed macOS stats even though it is not in the macOS profile")
	}
}
