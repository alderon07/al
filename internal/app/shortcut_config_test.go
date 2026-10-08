package app

import (
	"testing"
)

func TestShortcutOverridesRejectConflictsAndInvalidKeys(t *testing.T) {
	for _, binding := range []string{"F1", "s", "Ctrl+I", "Alt+"} {
		config := DefaultServices().defaultConfig()
		config.Shortcuts = map[string]string{"add": binding}
		if err := DefaultServices().validateAppConfig(config); err == nil {
			t.Errorf("accepted add=%q", binding)
		}
	}
}

func TestShortcutConflictDetectsShiftTolerantDefault(t *testing.T) {
	config := DefaultServices().defaultConfig()
	config.Shortcuts = map[string]string{"confirm": "Shift+?"}
	if err := DefaultServices().validateAppConfig(config); err != nil {
		t.Fatalf("confirmation key conflicts with a key used outside confirmations: %v", err)
	}
}

func TestDisjointViewBindingsCanShareKey(t *testing.T) {
	config := DefaultServices().defaultConfig()
	config.Shortcuts = map[string]string{"stats.next-view": "Alt+V", "diff.next-change": "Alt+V"}
	if err := DefaultServices().validateAppConfig(config); err != nil {
		t.Fatal(err)
	}
}
