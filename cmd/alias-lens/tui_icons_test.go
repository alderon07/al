package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestPixelIconsHaveOneFixedWidthRow(t *testing.T) {
	icons := []pixelIcon{
		iconAlias,
		iconBrand,
		iconCommand,
		iconEdit,
		iconFavorite,
		iconFunction,
		iconHealth,
		iconHeart,
		iconHelp,
		iconHistory,
		iconRepository,
		iconSearch,
		iconSpark,
		iconStats,
		iconSync,
		iconTheme,
	}
	for index, icon := range icons {
		rendered := renderPixelIcon(icon)
		if strings.ContainsRune(rendered, '\n') {
			t.Errorf("icon %d contains a newline: %q", index, rendered)
		}
		if width := lipgloss.Width(rendered); width != 4 {
			t.Errorf("icon %d width = %d, want 4: %q", index, width, rendered)
		}
	}
}

func TestInteractiveViewsPairPixelIconsWithText(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	applyTheme(builtInTheme("phosphor"))
	t.Cleanup(func() { applyTheme(defaultTheme()) })

	tests := []struct {
		name  string
		model model
		icon  pixelIcon
		label string
	}{
		{name: "aliases", model: model{width: 90, height: 24}, icon: iconAlias, label: "Set up your first shortcut"},
		{name: "help", model: model{width: 90, height: 24, helpVisible: true}, icon: iconHelp, label: "Keyboard guide"},
		{name: "themes", model: model{width: 90, height: 24, themePicker: true}, icon: iconTheme, label: "Choose a theme"},
		{name: "revisions", model: model{width: 90, height: 24, revisionOpen: true}, icon: iconHistory, label: "Restore an earlier alias file"},
		{name: "sync", model: model{width: 90, height: 24, trackedOnly: true}, icon: iconSync, label: "SYNC STATUS"},
		{name: "health", model: model{width: 90, height: 24, healthOnly: true}, icon: iconHealth, label: "ALIAS HEALTH"},
		{name: "add", model: model{width: 90, height: 24, adding: true}, icon: iconAlias, label: "ADD AN ALIAS"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			view := test.model.View()
			if !strings.Contains(view, renderPixelIcon(test.icon)) || !strings.Contains(view, test.label) {
				t.Fatalf("view lacks icon %q beside label %q:\n%s", renderPixelIcon(test.icon), test.label, view)
			}
			assertMakerCredit(t, view)
		})
	}

	statsView := (statsModel{width: 90, height: 24, theme: builtInTheme("phosphor")}).View()
	if !strings.Contains(statsView, renderPixelIcon(iconStats)) || !strings.Contains(statsView, "Alias rhythm") {
		t.Fatalf("stats view lacks its pixel icon and label:\n%s", statsView)
	}
	assertMakerCredit(t, statsView)

	repositoryView := (repoPickerModel{width: 90, height: 24}).View()
	if !strings.Contains(repositoryView, renderPixelIcon(iconRepository)) || !strings.Contains(repositoryView, "Choose a remote repository") {
		t.Fatalf("repository picker lacks its pixel icon and label:\n%s", repositoryView)
	}
	assertMakerCredit(t, repositoryView)
}
