package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestThemePickerPreviewsAndEscRestores(t *testing.T) {
	original := builtInTheme("tokyo-night")
	applyTheme(original)
	defer applyTheme(defaultTheme())

	updated, _ := (model{theme: original, width: 100, height: 24}).Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	picker := updated.(model)
	if !picker.themePicker || picker.theme.Preset != "tokyo-night" {
		t.Fatalf("theme picker did not open on the current theme: %+v", picker)
	}
	if view := picker.View(); !strings.Contains(view, "Choose a theme") || !strings.Contains(view, "preview") {
		t.Fatalf("theme picker instructions are missing:\n%s", view)
	}

	updated, _ = picker.Update(tea.KeyMsg{Type: tea.KeyDown})
	preview := updated.(model)
	if preview.theme.Preset != "vercel" {
		t.Fatalf("moving down previewed %q, want vercel", preview.theme.Preset)
	}

	updated, _ = preview.Update(tea.KeyMsg{Type: tea.KeyEsc})
	restored := updated.(model)
	if restored.themePicker || restored.theme.Preset != "tokyo-night" {
		t.Fatalf("escape did not restore the original theme: %+v", restored)
	}
}

func TestThemePickerEnterSavesPreview(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	applyTheme(builtInTheme("tokyo-night"))
	defer applyTheme(defaultTheme())

	updated, _ := (model{theme: builtInTheme("tokyo-night"), width: 100, height: 24}).Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	updated, _ = updated.(model).Update(tea.KeyMsg{Type: tea.KeyDown})
	preview := updated.(model)
	if _, err := os.Stat(filepath.Join(home, ".config", "alias-lens", "theme.json")); !os.IsNotExist(err) {
		t.Fatal("preview saved a theme before confirmation")
	}

	updated, _ = preview.Update(tea.KeyMsg{Type: tea.KeyEnter})
	saved := updated.(model)
	if saved.themePicker || saved.theme.Preset != "vercel" {
		t.Fatalf("enter did not accept the previewed theme: %+v", saved)
	}
	contents, err := os.ReadFile(filepath.Join(home, ".config", "alias-lens", "theme.json"))
	if err != nil || !strings.Contains(string(contents), `"preset": "vercel"`) {
		t.Fatalf("saved theme is wrong: %s, %v", contents, err)
	}
}
