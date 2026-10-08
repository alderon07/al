//go:build !windows

package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "alias-lens/internal/tea"
	"github.com/creack/pty/v2"
)

func TestFavoriteToggleInPTY(t *testing.T) {
	home := privateTestHome(t)
	path := filepath.Join(home, ".bash_aliases")
	if err := os.WriteFile(path, []byte("alias sample='echo sample'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestFavoriteTogglePTYHelper$")
	command.Env = append(os.Environ(), "ALIAS_LENS_FAVORITE_PTY_HELPER=1", "HOME="+home, activeShellEnvironment+"=bash", "TERM=xterm-256color", "NO_COLOR=1")
	terminal, err := pty.StartWithSize(command, &pty.Winsize{Rows: 24, Cols: 90})
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
	waitForPTYText(t, output, 0, "sample")
	for _, size := range []struct{ rows, cols uint16 }{{18, 48}, {36, 120}} {
		offset := output.length()
		if err := pty.Setsize(terminal, &pty.Winsize{Rows: size.rows, Cols: size.cols}); err != nil {
			t.Fatal(err)
		}
		waitForPTYText(t, output, offset, "sample")
	}
	offset := output.length()
	if _, err := terminal.Write([]byte("f")); err != nil {
		t.Fatal(err)
	}
	waitForPTYText(t, output, offset, "Marked sample as a favorite")
	contents, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(contents), "favorite=true") {
		t.Fatalf("PTY favorite mark was not saved: %q, %v", contents, err)
	}
	offset = output.length()
	if _, err := terminal.Write([]byte("f")); err != nil {
		t.Fatal(err)
	}
	waitForPTYText(t, output, offset, "Removed sample from favorites")
	contents, err = os.ReadFile(path)
	if err != nil || strings.Contains(string(contents), "favorite=true") {
		t.Fatalf("PTY favorite removal was not saved: %q, %v", contents, err)
	}
}

func TestFavoriteTogglePTYHelper(t *testing.T) {
	if os.Getenv("ALIAS_LENS_FAVORITE_PTY_HELPER") != "1" {
		return
	}
	aliases, err := loadAliases()
	if err != nil {
		t.Fatal(err)
	}
	theme := builtInTheme("phosphor")
	applyTheme(theme)
	_, err = tea.NewProgram(model{aliases: aliases, width: 90, height: 24, theme: theme, aliasMode: aliasModeCommand, shortcutProfile: shortcutLinux}, tea.WithAltScreen()).Run()
	if err != nil {
		t.Fatal(err)
	}
}
