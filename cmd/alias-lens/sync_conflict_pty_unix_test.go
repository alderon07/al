//go:build !windows

package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/creack/pty/v2"
)

func TestPlainSyncConflictMessageInNarrowPTY(t *testing.T) {
	runSyncConflictPTY(t, "sync", 1, []string{"sync conflict is unresolved", "repository was not changed", "al diff", "al sync --pull", "al sync --push"})
}

func TestRepositoryDiffGuidanceInNarrowPTY(t *testing.T) {
	runSyncConflictPTY(t, "diff", 0, []string{"Alias files differ", "1 local-only, 1 repository-only", "al diff --tui", "To keep aliases from both files", "al sync --pull", "al diff again", "al sync --push"})
}

func runSyncConflictPTY(t *testing.T, subcommand string, expectedExit int, expected []string) {
	t.Helper()
	if helper := os.Getenv("ALIAS_LENS_SYNC_CONFLICT_PTY_HELPER"); helper != "" {
		os.Args = []string{"alias-lens", helper}
		if code := runMain(); code != expectedExit {
			t.Fatalf("%s exit code = %d, want %d", helper, code, expectedExit)
		}
		return
	}

	home := setupPlainSyncConflictPTY(t)
	command := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$")
	command.Env = append(os.Environ(), "HOME="+home, "ALIAS_LENS_SHELL"+"=bash", "ALIAS_LENS_SYNC_CONFLICT_PTY_HELPER="+subcommand, "NO_COLOR=1", "TERM=xterm-256color")
	terminal, err := pty.StartWithSize(command, &pty.Winsize{Rows: 12, Cols: 44})
	if err != nil {
		t.Fatal(err)
	}
	output, readErr := io.ReadAll(terminal)
	if waitErr := command.Wait(); waitErr != nil {
		t.Fatalf("PTY helper failed: %v; output=%q", waitErr, output)
	}
	if readErr != nil && !errors.Is(readErr, syscall.EIO) {
		t.Fatal(readErr)
	}
	rendered := string(output)
	for _, text := range expected {
		if !strings.Contains(rendered, text) {
			t.Fatalf("PTY output does not contain %q: %q", text, rendered)
		}
	}
}

func setupPlainSyncConflictPTY(t *testing.T) string {
	t.Helper()
	home := privateTestHome(t)
	repository := filepath.Join(home, "dotfiles")
	if err := os.Mkdir(repository, 0o700); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "init")
	runGit(t, repository, "config", "user.email", "alias-lens@example.test")
	runGit(t, repository, "config", "user.name", "Alias Lens Test")
	local := []byte("alias keep='printf local'\n")
	remote := []byte("alias frog='printf remote'\n")
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), local, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, ".bash_aliases"), remote, 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "add", ".bash_aliases")
	runGit(t, repository, "commit", "-m", "Add remote aliases")
	t.Setenv("HOME", home)
	t.Setenv("ALIAS_LENS_SHELL", "bash")
	config := defaultConfig()
	config.Repository = repository
	if err := saveConfig(config); err != nil {
		t.Fatal(err)
	}
	if err := seedSyncStateFixture("conflict", "both local and remote aliases changed; run al diff", applicationServices().ContentHash(local), applicationServices().ContentHash(remote)); err != nil {
		t.Fatal(err)
	}
	return home
}
