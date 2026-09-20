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

func TestInteractiveViewsUsePlainTextLabels(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	applyTheme(builtInTheme("phosphor"))
	t.Cleanup(func() { applyTheme(defaultTheme()) })

	tests := []struct {
		name  string
		model model
		label string
	}{
		{name: "aliases", model: model{width: 90, height: 24}, label: "Set up your first shortcut"},
		{name: "help", model: model{width: 90, height: 24, helpVisible: true}, label: "Keyboard guide"},
		{name: "themes", model: model{width: 90, height: 24, themePicker: true}, label: "Choose a theme"},
		{name: "revisions", model: model{width: 90, height: 24, revisionOpen: true}, label: "Restore an earlier alias file"},
		{name: "sync", model: model{width: 90, height: 24, trackedOnly: true}, label: "SYNC STATUS"},
		{name: "health", model: model{width: 90, height: 24, healthOnly: true}, label: "ALIAS HEALTH"},
		{name: "add", model: model{width: 90, height: 24, adding: true}, label: "ADD AN ALIAS"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			view := test.model.View()
			if !strings.Contains(view, test.label) {
				t.Fatalf("view lacks label %q:\n%s", test.label, view)
			}
			for _, icon := range []pixelIcon{iconAlias, iconHelp, iconTheme, iconHistory, iconSync, iconHealth} {
				if strings.Contains(view, renderPixelIcon(icon)) {
					t.Fatalf("view contains built-in bitmap art %q:\n%s", renderPixelIcon(icon), view)
				}
			}
			assertMakerCredit(t, view)
		})
	}

	statsView := (statsModel{width: 90, height: 24, theme: builtInTheme("phosphor")}).View()
	if !strings.Contains(statsView, "Alias rhythm") || strings.Contains(statsView, renderPixelIcon(iconStats)) {
		t.Fatalf("stats view does not use a plain label:\n%s", statsView)
	}
	assertMakerCredit(t, statsView)

	repositoryView := (repoPickerModel{width: 90, height: 24}).View()
	if !strings.Contains(repositoryView, "Choose a remote repository") || strings.Contains(repositoryView, renderPixelIcon(iconRepository)) {
		t.Fatalf("repository picker does not use a plain label:\n%s", repositoryView)
	}
	assertMakerCredit(t, repositoryView)
}

func TestBrandArtStylesRenderFullCompactTextAndNone(t *testing.T) {
	t.Cleanup(func() { applyAppearanceConfig(defaultAppearanceConfig()) })
	tests := []struct {
		style       string
		want        string
		doesNotWant string
	}{
		{style: "full", want: "▐▛ ◉ ▜▌"},
		{style: "compact", want: "▗◉▖ ALIAS LENS", doesNotWant: aliasLensFullMark},
		{style: "text", want: "ALIAS LENS", doesNotWant: aliasLensFullMark},
		{style: "none", doesNotWant: "ALIAS LENS"},
	}
	for _, test := range tests {
		t.Run(test.style, func(t *testing.T) {
			applyAppearanceConfig(AppearanceConfig{Brand: "MY ALIASES", ArtStyle: test.style, MarkerStyle: "symbols"})
			view := (model{width: 100, height: 30}).View()
			if test.want != "" && !strings.Contains(view, test.want) {
				t.Fatalf("%s style is missing %q:\n%s", test.style, test.want, view)
			}
			if test.doesNotWant != "" && strings.Contains(view, test.doesNotWant) {
				t.Fatalf("%s style unexpectedly contains %q:\n%s", test.style, test.doesNotWant, view)
			}
		})
	}
}

func TestMarkerStylesOfferSymbolsASCIIAndNone(t *testing.T) {
	t.Cleanup(func() { applyAppearanceConfig(defaultAppearanceConfig()) })
	tests := []struct {
		style string
		want  string
	}{
		{style: "symbols", want: "⌕"},
		{style: "ascii", want: "/"},
		{style: "none", want: ""},
	}
	for _, test := range tests {
		t.Run(test.style, func(t *testing.T) {
			appearance := defaultAppearanceConfig()
			appearance.MarkerStyle = test.style
			applyAppearanceConfig(appearance)
			if got := interfaceMarker(iconSearch); got != test.want {
				t.Fatalf("search marker = %q, want %q", got, test.want)
			}
		})
	}
}
