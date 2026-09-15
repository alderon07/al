package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSetupDetectsShellFromEnvironment(t *testing.T) {
	t.Setenv(activeShellEnvironment, "")
	t.Setenv("SHELL", "/usr/local/bin/zsh")
	adapter, err := requestedShellAdapter("")
	if err != nil {
		t.Fatal(err)
	}
	if adapter.Name() != "zsh" {
		t.Fatalf("detected %q, want zsh", adapter.Name())
	}
}

func TestExplicitSetupShellOverridesEnvironment(t *testing.T) {
	t.Setenv(activeShellEnvironment, "zsh")
	t.Setenv("SHELL", "/bin/zsh")
	adapter, err := requestedShellAdapter("bash")
	if err != nil {
		t.Fatal(err)
	}
	if adapter.Name() != "bash" {
		t.Fatalf("selected %q, want bash", adapter.Name())
	}
}

func TestSetupRejectsUnsupportedDetectedShell(t *testing.T) {
	t.Setenv(activeShellEnvironment, "")
	t.Setenv("SHELL", "/usr/bin/fish")
	if _, err := requestedShellAdapter(""); err == nil || !strings.Contains(err.Error(), "unsupported shell") {
		t.Fatalf("unsupported detected shell returned %v", err)
	}
}

func TestZshStartupSetupIsIdempotent(t *testing.T) {
	t.Setenv("ZDOTDIR", "")
	home := t.TempDir()
	adapter := zshShellAdapter{}
	if err := adapter.ConfigureStartup(home, "darwin"); err != nil {
		t.Fatal(err)
	}
	if err := adapter.ConfigureStartup(home, "linux"); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(home, ".zshrc"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(contents), ".zsh_aliases") != 2 {
		t.Fatalf("Zsh loader was duplicated or incomplete:\n%s", contents)
	}
}

func TestZshSetupRespectsZdotdir(t *testing.T) {
	home := t.TempDir()
	zdotdir := filepath.Join(home, "zsh")
	if err := os.MkdirAll(zdotdir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZDOTDIR", zdotdir)
	if err := (zshShellAdapter{}).ConfigureStartup(home, "darwin"); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(zdotdir, ".zshrc"))
	if err != nil || !strings.Contains(string(contents), ".zsh_aliases") {
		t.Fatalf("ZDOTDIR startup file was not configured: %s, %v", contents, err)
	}
	if _, err := os.Stat(filepath.Join(home, ".zshrc")); !os.IsNotExist(err) {
		t.Fatal("setup modified the home .zshrc despite ZDOTDIR")
	}
}

func TestZshExtendedHistoryIsNormalized(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".zsh_history")
	contents := ": 1789059600:4;git status --short --branch\n: 1789059610:0;git status --short --branch\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	counts, err := historyCountsFromShell(path, "zsh")
	if err != nil {
		t.Fatal(err)
	}
	if counts["git status --short --branch"] != 2 {
		t.Fatalf("unexpected Zsh history counts: %#v", counts)
	}
}

func TestZshIntegrationExecutesAliasName(t *testing.T) {
	if !strings.Contains(zshIntegration, `ALIAS_LENS_SHELL=zsh ALIAS_LENS_HISTORY_FILE=`) {
		t.Fatal("Zsh integration does not pin commands to the Zsh adapter")
	}
	if !strings.Contains(zshIntegration, `builtin eval "$_alias_lens_name"`) {
		t.Fatal("Zsh integration does not execute the selected alias name")
	}
	if !strings.Contains(zshIntegration, `alias-lens shell-entry "$_alias_lens_name"`) {
		t.Fatal("Zsh integration does not load a newly added alias before executing it")
	}
	if strings.Contains(zshIntegration, "entry-summary") || strings.Contains(zshIntegration, "Alias Lens ran") {
		t.Fatal("Zsh integration still prints a post-execution receipt")
	}
	if !strings.Contains(zshIntegration, `_alias_lens_flush_history 2>/dev/null || true`) || !strings.Contains(zshIntegration, `setopt localoptions extendedhistory`) || !strings.Contains(zshIntegration, `print -s -- "$_alias_lens_name"`) || !strings.Contains(zshIntegration, `fc -AI "${HISTFILE:-$HOME/.zsh_history}"`) {
		t.Fatal("Zsh integration does not record picker runs in native history")
	}
	if !strings.Contains(zshIntegration, `bindkey '^G' _alias_lens_launch`) {
		t.Fatal("Zsh integration does not install the ZLE key binding")
	}
	if !strings.Contains(zshIntegration, `ALIAS_LENS_NOBIND`) || !strings.Contains(zshIntegration, `[[ -n "$BUFFER" ]]`) {
		t.Fatal("Zsh binding cannot be disabled or preserve Ctrl+G on a non-empty prompt")
	}
}

func TestShellEntryDefinitionReloadsAliasesAndFunctions(t *testing.T) {
	definition, err := shellEntryDefinition(Alias{Name: "cl", Command: "printf '%s\\n' cleared", Type: "alias"})
	if err != nil {
		t.Fatal(err)
	}
	name, command, ok := parseAliasDefinition(definition)
	if !ok || name != "cl" || command != "printf '%s\\n' cleared" {
		t.Fatalf("alias definition did not round trip: %q", definition)
	}

	definition, err = shellEntryDefinition(Alias{Name: "mkcd", Command: "mkdir -p \"$1\"; cd \"$1\"", Type: "function"})
	if err != nil || !strings.Contains(definition, "mkcd() {") || !strings.Contains(definition, "mkdir -p") {
		t.Fatalf("function definition was not reconstructed: %q, %v", definition, err)
	}
}

func TestNewlyWrittenAliasCanBeLoadedIntoCurrentShell(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	path := filepath.Join(home, ".bash_aliases")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := addAliasToFile(path, "cl", "printf clear", "Clear the screen"); err != nil {
		t.Fatal(err)
	}
	definition, err := loadShellEntry("cl")
	if err != nil {
		t.Fatal(err)
	}
	name, command, ok := parseAliasDefinition(definition)
	if !ok || name != "cl" || command != "printf clear" {
		t.Fatalf("saved alias could not be loaded: %q", definition)
	}
}

func TestBashIntegrationLoadsNewAliasBeforeRunningIt(t *testing.T) {
	directory := t.TempDir()
	shim := filepath.Join(directory, "alias-lens")
	contents := `#!/bin/sh
if [ "${1-}" = "shell-entry" ]; then
  printf "alias cl='printf newly-loaded'\n"
elif [ "${1-}" = "watch" ]; then
  :
else
  printf 'cl\n'
fi
`
	if err := os.WriteFile(shim, []byte(contents), 0o700); err != nil {
		t.Fatal(err)
	}
	script := "shopt -s expand_aliases\n" + bashIntegration + "\nal\n"
	command := exec.Command("bash", "--noprofile", "--norc", "-c", script)
	command.Env = append(os.Environ(), "PATH="+directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Bash integration failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "newly-loaded") {
		t.Fatalf("new alias did not run in the existing shell:\n%s", output)
	}
	if strings.Contains(string(output), "Alias Lens ran") {
		t.Fatalf("integration printed an unwanted execution receipt:\n%s", output)
	}
}

func TestAliasFormAcceptsSpacesInCommandsAndDescriptions(t *testing.T) {
	m := model{adding: true, field: 1}
	m.form[1] = "git"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	command := updated.(model)
	if command.form[1] != "git " {
		t.Fatalf("command field lost its space: %q", command.form[1])
	}

	command.field = 2
	command.form[2] = "Show"
	updated, _ = command.Update(tea.KeyMsg{Type: tea.KeySpace})
	description := updated.(model)
	if description.form[2] != "Show " {
		t.Fatalf("description field lost its space: %q", description.form[2])
	}
}

func TestAliasFormRejectsSpacesInAliasName(t *testing.T) {
	m := model{adding: true, field: 0}
	m.form[0] = "git"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	result := updated.(model)
	if result.form[0] != "git" || result.status != "Alias names cannot contain spaces" {
		t.Fatalf("alias name accepted a space or lacked guidance: value=%q status=%q", result.form[0], result.status)
	}
}

func TestZshRevisionsUseAliasFilename(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".zsh_aliases")
	if err := saveRevision(path, []byte("alias gs='git status'\n")); err != nil {
		t.Fatal(err)
	}
	revisions, err := listRevisions(path)
	if err != nil || len(revisions) != 1 {
		t.Fatalf("expected one Zsh revision: %#v, %v", revisions, err)
	}
	if !strings.HasSuffix(revisions[0].Path, ".zsh_aliases") {
		t.Fatalf("unexpected revision path %q", revisions[0].Path)
	}
}
