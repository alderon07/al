//go:build !windows

package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "alias-lens/cmd/alias-lens/internal/tea"
	"github.com/creack/pty/v2"
)

func TestTerminalDiffPTYAtNarrowAndWideWidths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	repository := filepath.Join(home, "dotfiles")
	if err := os.MkdirAll(repository, 0o700); err != nil {
		t.Fatal(err)
	}
	config := defaultConfig()
	config.Repository = repository
	if err := saveConfig(config); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, config.AliasFile), []byte("# current\nalias gs='git status -sb'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, config.AliasFile), []byte("# tracked\nalias gs='git status'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestTerminalDiffPTYHelper$")
	command.Env = append(os.Environ(), "ALIAS_LENS_DIFF_PTY_HELPER=1", "HOME="+home, activeShellEnvironment+"=bash", "NO_COLOR=1", "TERM=xterm-256color")
	terminal, err := pty.StartWithSize(command, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		t.Fatal(err)
	}
	output := &synchronizedBuffer{}
	go func() { _, _ = io.Copy(output, terminal) }()
	t.Cleanup(func() {
		_, _ = terminal.Write([]byte{3})
		_ = terminal.Close()
		if command.Process != nil {
			_ = command.Process.Kill()
		}
		_ = command.Wait()
	})

	narrow := waitForPTYText(t, output, 0, "Repository comparison")
	for _, expected := range []string{"# tracked", "# current", "OLD   NEW   CHANGE"} {
		if !strings.Contains(narrow, expected) {
			t.Fatalf("narrow PTY diff missing %q:\n%q", expected, narrow)
		}
	}
	compactOffset := output.length()
	if err := pty.Setsize(terminal, &pty.Winsize{Rows: 18, Cols: 48}); err != nil {
		t.Fatal(err)
	}
	compact := waitForPTYText(t, output, compactOffset, "Repository comparison")
	if !strings.Contains(compact, "OLD   NEW   CHANGE") || !strings.Contains(compact, "esc back") {
		t.Fatalf("compact PTY diff lost its contents or controls:\n%q", compact)
	}
	offset := output.length()
	if err := pty.Setsize(terminal, &pty.Winsize{Rows: 36, Cols: 120}); err != nil {
		t.Fatal(err)
	}
	wide := waitForPTYText(t, output, offset, "Tracked copy")
	if !strings.Contains(wide, "Current file") || !strings.Contains(wide, "│") {
		t.Fatalf("wide PTY diff did not show split columns:\n%q", wide)
	}
	if _, err := terminal.Write([]byte("q")); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(3 * time.Second)
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("diff viewer exited with error: %v", err)
		}
	case <-deadline:
		t.Fatal("q did not close the native diff viewer")
	}
}

func TestTerminalDiffPTYHelper(t *testing.T) {
	if os.Getenv("ALIAS_LENS_DIFF_PTY_HELPER") != "1" {
		return
	}
	if err := runTUIWithDiff(true); err != nil {
		t.Fatal(err)
	}
}

func TestRevisionPreviewPTYBeforeRestore(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	path := filepath.Join(home, ".bash_aliases")
	current := []byte("alias gs='git status -sb'\n")
	previous := []byte("alias gs='git status'\n")
	if err := os.WriteFile(path, current, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := saveRevision(path, previous); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestRevisionPreviewPTYHelper$")
	command.Env = append(os.Environ(), "ALIAS_LENS_REVISION_PTY_HELPER=1", "HOME="+home, activeShellEnvironment+"=bash", "NO_COLOR=1", "TERM=xterm-256color")
	terminal, err := pty.StartWithSize(command, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		t.Fatal(err)
	}
	output := &synchronizedBuffer{}
	go func() { _, _ = io.Copy(output, terminal) }()
	t.Cleanup(func() {
		_, _ = terminal.Write([]byte{3})
		_ = terminal.Close()
		if command.Process != nil {
			_ = command.Process.Kill()
		}
		_ = command.Wait()
	})
	waitForPTYText(t, output, 0, "Restore an earlier alias file")
	if _, err := terminal.Write([]byte("\r")); err != nil {
		t.Fatal(err)
	}
	waitForPTYText(t, output, 0, "Preview restore")
	if contents, err := os.ReadFile(path); err != nil || string(contents) != string(current) {
		t.Fatalf("PTY preview changed aliases: %q, %v", contents, err)
	}
	backOffset := output.length()
	if _, err := terminal.Write([]byte("\x1b")); err != nil {
		t.Fatal(err)
	}
	waitForPTYText(t, output, backOffset, "Restore an earlier alias file")
	if _, err := terminal.Write([]byte("\r")); err != nil {
		t.Fatal(err)
	}
	waitForPTYText(t, output, backOffset, "Preview restore")
	if _, err := terminal.Write([]byte("r")); err != nil {
		t.Fatal(err)
	}
	waitForPTYText(t, output, 0, "Restore this revision?")
	if _, err := terminal.Write([]byte("y")); err != nil {
		t.Fatal(err)
	}
	waitForPTYText(t, output, 0, "previous version saved")
	if contents, err := os.ReadFile(path); err != nil || string(contents) != string(previous) {
		t.Fatalf("PTY restore did not apply revision: %q, %v", contents, err)
	}
}

func TestRevisionPreviewPTYHelper(t *testing.T) {
	if os.Getenv("ALIAS_LENS_REVISION_PTY_HELPER") != "1" {
		return
	}
	applyTheme(builtInTheme("phosphor"))
	m := model{width: 80, height: 24, theme: builtInTheme("phosphor"), shortcutProfile: shortcutLinux}
	m.openRevisionDrawer()
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		t.Fatal(err)
	}
}
