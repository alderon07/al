package main

import (
	"os"
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
	if !strings.Contains(zshIntegration, `bindkey '^G' _alias_lens_insert`) {
		t.Fatal("Zsh integration does not install the ZLE key binding")
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
