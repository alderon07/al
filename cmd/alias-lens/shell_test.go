package main

import (
	"fmt"
	"os"
	"path/filepath"

	shellapi "github.com/alderon07/al/internal/shell"

	"strings"
	"testing"

	"github.com/alderon07/al/internal/app"
)

func TestSetupRemoveKeepsAliasesAndUnrelatedBashSettings(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv("ALIAS_LENS_SHELL", "bash")
	aliasPath := filepath.Join(home, ".bash_aliases")
	aliases := "alias al='echo personal'\nalias ll='ls -al'\n\n# Alias Lens Bash integration\neval \"$(command alias-lens shell-init bash)\"\n"
	if err := os.WriteFile(aliasPath, []byte(aliases), 0o600); err != nil {
		t.Fatal(err)
	}
	bashrcPath := filepath.Join(home, ".bashrc")
	userBashrc := "export EDITOR=vim\n. \"$HOME/.bash_aliases\"\n"
	bashrc := userBashrc + shellapi.StartupPathBlock(home, filepath.Join(home, ".local", "bin")) + shellapi.BashAliasLoader
	if err := os.WriteFile(bashrcPath, []byte(bashrc), 0o640); err != nil {
		t.Fatal(err)
	}
	profilePath := filepath.Join(home, ".profile")
	userProfile := "export LANG=C\n"
	if err := os.WriteFile(profilePath, []byte(userProfile+shellapi.BashLoginLoader), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runSetupCommand([]string{"--remove", "bash"}); err != nil {
		t.Fatal(err)
	}
	if err := runSetupCommand([]string{"--remove", "bash"}); err != nil {
		t.Fatal(err)
	}
	updatedAliases, err := os.ReadFile(aliasPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updatedAliases), "alias al='echo personal'") || !strings.Contains(string(updatedAliases), "alias ll='ls -al'") {
		t.Fatalf("setup removal changed user aliases:\n%s", updatedAliases)
	}
	if strings.Contains(string(updatedAliases), "shell-init") || strings.Contains(string(updatedAliases), "Alias Lens Bash integration") {
		t.Fatalf("setup removal left generated alias integration:\n%s", updatedAliases)
	}
	updatedBashrc, err := os.ReadFile(bashrcPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(updatedBashrc) != userBashrc {
		t.Fatalf("setup removal changed unrelated Bash settings:\n%s", updatedBashrc)
	}
	info, err := os.Stat(bashrcPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("setup removal changed Bash file mode: %v", info.Mode().Perm())
	}
	updatedProfile, err := os.ReadFile(profilePath)
	if err != nil || string(updatedProfile) != userProfile {
		t.Fatalf("setup removal changed unrelated Bash login settings: %q, %v", updatedProfile, err)
	}
}

func TestSetupRemoveRespectsZdotdir(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	home := privateTestHome(t)
	zdotdir := filepath.Join(home, "zsh")
	t.Setenv("HOME", home)
	t.Setenv("ZDOTDIR", zdotdir)
	t.Setenv("ALIAS_LENS_SHELL", "zsh")
	if err := os.MkdirAll(zdotdir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".zsh_aliases"), []byte("# Alias Lens Zsh integration\neval \"$(command alias-lens shell-init zsh)\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	zshrcPath := filepath.Join(zdotdir, ".zshrc")
	if err := os.WriteFile(zshrcPath, []byte("setopt autocd\n"+shellapi.ZshAliasLoader), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runSetupCommand([]string{"zsh", "--remove"}); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(zshrcPath)
	if err != nil || string(contents) != "setopt autocd\n" {
		t.Fatalf("ZDOTDIR removal result = %q, %v", contents, err)
	}
	if _, err := os.Stat(filepath.Join(home, ".zshrc")); !os.IsNotExist(err) {
		t.Fatalf("setup removal changed the wrong Zsh file: %v", err)
	}
}

func TestSetupRepairNormalizesDuplicateGeneratedBlocks(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv("ALIAS_LENS_SHELL", "bash")
	aliasPath := filepath.Join(home, ".bash_aliases")
	integration := "# Alias Lens Bash integration\neval \"$(command alias-lens shell-init bash)\"\n"
	if err := os.WriteFile(aliasPath, []byte("alias ll='ls -al'\n"+integration+integration), 0o600); err != nil {
		t.Fatal(err)
	}
	bashrcPath := filepath.Join(home, ".bashrc")
	if err := os.WriteFile(bashrcPath, []byte(shellapi.BashAliasLoader+shellapi.BashAliasLoader), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runSetupCommand([]string{"--repair", "bash"}); err != nil {
		t.Fatal(err)
	}
	updatedAliases, err := os.ReadFile(aliasPath)
	if err != nil || strings.Count(string(updatedAliases), "shell-init bash") != 1 {
		t.Fatalf("repair did not normalize alias integration: %s, %v", updatedAliases, err)
	}
	updatedBashrc, err := os.ReadFile(bashrcPath)
	if err != nil || strings.Count(string(updatedBashrc), ".bash_aliases") != 2 {
		t.Fatalf("repair did not normalize startup loader: %s, %v", updatedBashrc, err)
	}
}

func assertShellIntegrationGolden(t *testing.T, adapter app.ShellAdapter, filename string) {
	t.Helper()
	want, err := os.ReadFile(filepath.Join("testdata", "phase3", filename))
	if err != nil {
		t.Fatal(err)
	}
	if got := adapter.Integration(); got != string(want) {
		t.Fatalf("%s integration changed from the approved phase 3 baseline", adapter.DisplayName())
	}
}

func assertShellAdapterContract(t *testing.T, adapter app.ShellAdapter, historyCommand, promptAction string) {
	t.Helper()
	if adapter.ExecutionSpec().HistoryCommand == "" || !strings.Contains(adapter.ExecutionSpec().HistoryCommand, historyCommand) {
		t.Fatalf("%s execution history contract = %#v", adapter.Name(), adapter.ExecutionSpec())
	}
	if adapter.PromptSpec().SelectionPrefix != "$ " || adapter.PromptSpec().NonEmptyPromptAction != promptAction {
		t.Fatalf("%s prompt contract = %#v", adapter.Name(), adapter.PromptSpec())
	}
	if binding := adapter.BindingSpec(); binding.Key != "Ctrl+G" || binding.DisableEnvironment != "ALIAS_LENS_NOBIND" {
		t.Fatalf("%s binding contract = %#v", adapter.Name(), binding)
	}
}

func testTreeManifest(t *testing.T, root string) string {
	t.Helper()
	var entries []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		value := ""
		if info.Mode()&os.ModeSymlink != 0 {
			value, err = os.Readlink(path)
		} else if info.Mode().IsRegular() {
			contents, readErr := os.ReadFile(path)
			err = readErr
			value = string(contents)
		}
		if err != nil {
			return err
		}
		entries = append(entries, fmt.Sprintf("%s|%s|%04o|%s", relative, info.Mode().Type(), info.Mode().Perm(), value))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(entries, "\n")
}
