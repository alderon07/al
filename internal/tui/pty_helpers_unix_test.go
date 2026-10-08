//go:build !windows

package tui

import pty "github.com/creack/pty/v2"

import (
	"bytes"

	"io"
	"os"
	"os/exec"

	"strings"
	"sync"
	"testing"
	"time"
)

type synchronizedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (buffer *synchronizedBuffer) Write(contents []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.buffer.Write(contents)
}

func (buffer *synchronizedBuffer) length() int {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.buffer.Len()
}

func (buffer *synchronizedBuffer) stringFrom(offset int) string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	contents := buffer.buffer.Bytes()
	if offset > len(contents) {
		offset = len(contents)
	}
	return string(append([]byte(nil), contents[offset:]...))
}

type shellPTY struct {
	t      *testing.T
	file   *os.File
	cmd    *exec.Cmd
	output *synchronizedBuffer
}

func startShellPTY(t *testing.T, executable string, arguments, environment []string) *shellPTY {
	t.Helper()
	command := exec.Command(executable, arguments...)
	command.Env = environment
	terminal, err := pty.StartWithSize(command, &pty.Winsize{Rows: 24, Cols: 100})
	if err != nil {
		t.Fatal(err)
	}
	output := &synchronizedBuffer{}
	go func() {
		_, _ = io.Copy(output, terminal)
	}()
	session := &shellPTY{t: t, file: terminal, cmd: command, output: output}
	t.Cleanup(func() {
		_, _ = terminal.Write([]byte("exit\n"))
		_ = terminal.Close()
		if command.Process != nil {
			_ = command.Process.Kill()
		}
		_ = command.Wait()
	})
	session.waitFor(0, ptyPrompt)
	return session
}

func (session *shellPTY) mark() int {
	return session.output.length()
}

func (session *shellPTY) write(contents string) {
	session.t.Helper()
	if _, err := session.file.Write([]byte(contents)); err != nil {
		session.t.Fatal(err)
	}
}

func (session *shellPTY) waitFor(offset int, expected string) string {
	session.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		output := session.output.stringFrom(offset)
		if strings.Contains(output, expected) {
			return output
		}
		time.Sleep(10 * time.Millisecond)
	}
	session.t.Fatalf("PTY output did not contain %q:\n%s", expected, session.output.stringFrom(offset))
	return ""
}

func (session *shellPTY) run(command string) string {
	session.t.Helper()
	offset := session.mark()
	session.write(command + "\n")
	return session.waitFor(offset, ptyPrompt)
}

func (session *shellPTY) pressCtrlG() string {
	session.t.Helper()
	offset := session.mark()
	session.write("\x07")
	return session.waitFor(offset, ptyPrompt)
}

func (session *shellPTY) pressCtrlGAndPause() {
	session.t.Helper()
	session.write("\x07")
	time.Sleep(100 * time.Millisecond)
}

const ptyPrompt = "ALIAS_LENS_TEST> "
