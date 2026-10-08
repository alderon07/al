package app

import (
	"os"
	"path/filepath"

	"testing"
)

func TestLegacyThemeAliasLoadsCanonicalPreset(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	directory := filepath.Join(home, ".config", "alias-lens")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "theme.json"), []byte("{\"preset\":\"catppuccin-mocha\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	theme, err := DefaultServices().loadTheme()
	if err != nil {
		t.Fatal(err)
	}
	if theme.Preset != "catppuccin" || theme.Name != "Catppuccin" {
		t.Fatalf("legacy theme alias loaded as %+v", theme)
	}
}
