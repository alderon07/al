package main

import (
	"os/exec"

	"testing"
)

func TestBuiltInThemesCanCycle(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	absolutely := nextTheme("phosphor")
	ayu := nextTheme(absolutely.Preset)
	catppuccin := nextTheme(ayu.Preset)
	if absolutely.Preset != "absolutely" || ayu.Preset != "ayu" || catppuccin.Preset != "catppuccin" {
		t.Fatalf("unexpected theme cycle: %q, %q, then %q", absolutely.Preset, ayu.Preset, catppuccin.Preset)
	}
	darcula := builtInTheme("darcula")
	if darcula.Background != "#2B2B2B" || darcula.Text != "#A9B7C6" || darcula.Selected != "#214283" {
		t.Fatalf("Darcula preset drifted from JetBrains values: %+v", darcula)
	}
}

func TestOfficialThemePaletteValues(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	dracula := builtInTheme("dracula")
	if dracula.Background != "#282A36" || dracula.Text != "#F8F8F2" || dracula.Selected != "#44475A" || dracula.Accent != "#FF79C6" {
		t.Fatalf("Dracula preset drifted from its official palette: %+v", dracula)
	}
	mocha := builtInTheme("catppuccin")
	if mocha.Background != "#1E1E2E" || mocha.Text != "#CDD6F4" || mocha.Panel != "#181825" || mocha.Selected != "#313244" || mocha.Border != "#585B70" {
		t.Fatalf("Catppuccin Mocha preset drifted from its official palette: %+v", mocha)
	}
}

func runGit(t *testing.T, directory string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
	return string(output)
}
