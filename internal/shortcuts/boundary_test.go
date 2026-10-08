package shortcuts_test

import (
	"github.com/alderon07/al/internal/shortcuts"
	tea "github.com/alderon07/al/internal/tea"
	"testing"
)

func TestOverrideValidationAndScopedTranslation(t *testing.T) {
	for _, binding := range []string{"F1", "s", "Ctrl+I", "Alt+"} {
		if err := shortcuts.ValidateOverrides("linux", map[string]string{"add": binding}); err == nil {
			t.Errorf("accepted add=%q", binding)
		}
	}
	overrides := map[string]string{"add": "Alt+N", "up": "Alt+K", "stats.next-view": "Alt+V", "diff.next-change": "Alt+V"}
	if err := shortcuts.ValidateOverrides("linux", overrides); err != nil {
		t.Fatal(err)
	}
	profile := shortcuts.ResolveProfile("linux", overrides)
	key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}, Alt: true}
	if !shortcuts.Matches(key, profile, shortcuts.Add) || shortcuts.Matches(tea.KeyMsg{Type: tea.KeyCtrlN}, profile, shortcuts.Add) || shortcuts.Label(profile, shortcuts.Add) != "Alt+N" {
		t.Fatal("override did not replace original binding")
	}
	if got := shortcuts.Translate(tea.KeyMsg{Type: tea.KeyUp}, profile, shortcuts.MoveUp); got.Type != tea.KeyNull {
		t.Fatal("old global navigation remained active")
	}
	if got := shortcuts.Translate(tea.KeyMsg{Type: tea.KeyUp}, profile, shortcuts.StatsPreviousRow); got.Type != tea.KeyUp {
		t.Fatal("global override suppressed scoped navigation")
	}
	overrides["add"] = "Alt+X"
	if shortcuts.Label(profile, shortcuts.Add) != "Alt+N" {
		t.Fatal("profile retained mutable override map")
	}
	key.Paste = true
	if shortcuts.Matches(key, profile, shortcuts.Add) {
		t.Fatal("paste triggered action")
	}
}

func TestDescriptorsAndLauncher(t *testing.T) {
	descriptors := shortcuts.Descriptors()
	if len(descriptors) == 0 {
		t.Fatal("missing descriptors")
	}
	first := descriptors[0]
	descriptors[0].Name = "changed"
	if shortcuts.Descriptors()[0] != first {
		t.Fatal("mutable descriptors exposed registry")
	}
	for _, label := range []string{"Alt+K", "Ctrl+C", "Ctrl+I", "Ctrl+J", "Ctrl+M", "Ctrl+X", "Ctrl+Z"} {
		if _, err := shortcuts.ParseLauncherKey(label); err == nil {
			t.Fatalf("accepted launcher %s", label)
		}
	}
	if key, err := shortcuts.ParseLauncherKey(" Ctrl+k "); err != nil || key != "K" {
		t.Fatalf("launcher %s %v", key, err)
	}
	if shortcuts.LauncherLabel(nil) != "Ctrl+G" || shortcuts.LauncherLabel(map[string]string{"launcher": "Ctrl+K"}) != "Ctrl+K" {
		t.Fatal("launcher labels changed")
	}
}
