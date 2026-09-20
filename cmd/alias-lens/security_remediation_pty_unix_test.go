//go:build !windows

package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"

	"github.com/creack/pty/v2"
)

func TestRepositoryControlledTextIsEscapedInPTY(t *testing.T) {
	if os.Getenv("ALIAS_LENS_SECURITY_PTY_HELPER") == "1" {
		printPlainAliasList(os.Stdout, []Alias{{
			Name:    "unsafe\x1b]52;c;payload\a",
			Command: "printf '\x1b[2J'\u202e",
		}}, "")
		return
	}

	command := exec.Command(os.Args[0], "-test.run=^TestRepositoryControlledTextIsEscapedInPTY$")
	command.Env = append(os.Environ(), "ALIAS_LENS_SECURITY_PTY_HELPER=1", "NO_COLOR=1", "TERM=xterm-256color")
	terminal, err := pty.StartWithSize(command, &pty.Winsize{Rows: 18, Cols: 48})
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
	for _, raw := range []string{"\x1b]52;c;payload\a", "\x1b[2J", "\u202e"} {
		if strings.Contains(rendered, raw) {
			t.Fatalf("PTY emitted raw repository-controlled sequence %q: %q", raw, rendered)
		}
	}
	for _, escaped := range []string{`\x1b]52;c;payload\x07`, `\x1b[2J`, `\u202e`} {
		if !strings.Contains(rendered, escaped) {
			t.Fatalf("PTY output is missing visible escape %q: %q", escaped, rendered)
		}
	}
}
