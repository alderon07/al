//go:build !windows

package tui

import pty "github.com/creack/pty/v2"

import (
	"io"

	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	tea "alias-lens/internal/tea"
)

func TestWideAliasNavigationPTYHelper(t *testing.T) {
	if os.Getenv("ALIAS_LENS_WIDE_REDRAW_HELPER") != "1" {
		return
	}
	theme := builtInTheme("phosphor")
	applyTheme(theme)
	aliases := []aliasEntry{
		{Name: "one", Category: "tools", Command: "printf detail_one", Description: "one"},
		{Name: "two", Category: "tools", Command: "printf detail_two", Description: "The second synthetic alias has a longer description that wraps across multiple terminal cells and lines."},
		{Name: "three", Category: "git", Command: "printf detail_three", Description: "three"},
		{Name: "four", Category: "files", Command: "printf detail_four", Description: "The fourth synthetic alias has a longer description that wraps across multiple terminal cells and lines."},
		{Name: "five", Category: "tools", Command: "printf detail_five", Description: "five"},
	}
	_, err := tea.NewProgram(model{services: applicationServices(), aliases: aliases, width: 128, height: 33, theme: theme, aliasMode: aliasModeCommand, shortcutProfile: shortcutLinux}, tea.WithAltScreen()).Run()
	if err != nil {
		t.Fatal(err)
	}
}

func TestWideAliasNavigationPTY(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestWideAliasNavigationPTYHelper$")
	command.Env = append(os.Environ(), "ALIAS_LENS_WIDE_REDRAW_HELPER=1", "HOME="+t.TempDir(), "TERM=xterm-256color")
	terminal, err := pty.StartWithSize(command, &pty.Winsize{Rows: 33, Cols: 128})
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
	waitForPTYText(t, output, 0, "detail_one")
	scrollRegion := regexp.MustCompile("\x1b\\[[0-9]+;[0-9]+r|\x1bM")
	previous := "detail_one"
	for _, detail := range []string{"detail_two", "detail_five", "detail_four"} {
		offset := output.length()
		if _, err := terminal.Write([]byte("\x1b[B")); err != nil {
			t.Fatal(err)
		}
		redraw := waitForPTYText(t, output, offset, detail)
		if scrollRegion.MatchString(redraw) {
			t.Fatalf("navigation used a scroll region: %q", redraw)
		}
		clear := strings.LastIndex(redraw, "\x1b[2J")
		if clear < 0 {
			t.Fatalf("navigation did not clear the old frame: %q", redraw)
		}
		frame := redraw[clear:]
		if strings.Contains(frame, previous) || strings.Count(frame, "Selected alias") != 1 || strings.Count(frame, "Showing 1-5 of 5") != 1 {
			t.Fatalf("navigation left old details or duplicated the browser: %q", frame)
		}
		previous = detail
	}
	if _, err := terminal.Write([]byte("/")); err != nil {
		t.Fatal(err)
	}
	waitForPTYText(t, output, 0, "SEARCH")
	offset := output.length()
	if _, err := terminal.Write([]byte("three")); err != nil {
		t.Fatal(err)
	}
	redraw := waitForPTYText(t, output, offset, "detail_three")
	if scrollRegion.MatchString(redraw) || !strings.Contains(redraw, "\x1b[2J") {
		t.Fatalf("search did not repaint the wide browser: %q", redraw)
	}
}
