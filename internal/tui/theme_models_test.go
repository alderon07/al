package tui

import (
	"strings"
	"testing"
)

func TestEveryThemeRendersTheAliasBrowser(t *testing.T) {
	for _, theme := range availableThemes() {
		applyTheme(theme)
		view := (model{services: applicationServices(),
			aliases: []aliasEntry{{Name: "gs", Command: "git status", Description: "Show repository status"}},
			width:   120,
			height:  24,
			theme:   theme,
		}).View()
		if !strings.Contains(view, "ALIAS LENS") || !strings.Contains(view, "gs") {
			t.Errorf("theme %s did not render a complete browser", theme.Preset)
		}
	}
}
