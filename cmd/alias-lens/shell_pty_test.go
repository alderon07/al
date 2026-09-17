//go:build !windows

package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty/v2"
)

const ptyPrompt = "ALIAS_LENS_TEST> "

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

func TestBashPTYExecution(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is not installed")
	}
	runShellPTYExecutionChecks(t, "bash", bash, []string{"--noprofile", "--norc", "-i"}, bashIntegration)
}

func TestZshPTYExecution(t *testing.T) {
	zsh := findZsh()
	if zsh == "" {
		t.Skip("zsh is not installed")
	}
	runShellPTYExecutionChecks(t, "zsh", zsh, []string{"-f"}, zshIntegration)
}

func findZsh() string {
	if path, err := exec.LookPath("zsh"); err == nil {
		return path
	}
	for _, path := range []string{"/home/linuxbrew/.linuxbrew/bin/zsh", "/usr/local/bin/zsh", "/usr/bin/zsh"} {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return path
		}
	}
	return ""
}

func runShellPTYExecutionChecks(t *testing.T, shellName, executable string, arguments []string, integration string) {
	t.Helper()
	home := t.TempDir()
	binDirectory := filepath.Join(home, "bin")
	if err := os.MkdirAll(binDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(binDirectory, "alias-lens")
	shimContents := `#!/bin/sh
case "${1-}" in
  shell-entry)
    case "${2-}" in
      bad) printf "alias bad='false'\n" ;;
      jump) printf "alias jump='cd %s'\n" "$ALIAS_LENS_TEST_DIRECTORY" ;;
      ok) printf "alias ok='printf PTY_OK\\n'\n" ;;
      *) exit 1 ;;
    esac
    ;;
  record-use)
    printf '%s\n' "${2-}" >> "$HOME/usage"
    ;;
  watch)
    ;;
  *)
    cat "$HOME/selection"
    ;;
esac
`
	if err := os.WriteFile(shim, []byte(shimContents), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "integration"), []byte(integration), 0o600); err != nil {
		t.Fatal(err)
	}
	selectionPath := filepath.Join(home, "selection")
	if err := os.WriteFile(selectionPath, []byte("bad\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	environment := []string{
		"HOME=" + home,
		"PATH=" + binDirectory + ":/usr/bin:/bin",
		"TERM=xterm-256color",
		"ALIAS_LENS_SHELL=" + shellName,
		"ALIAS_LENS_TEST_DIRECTORY=" + home,
		"HISTFILE=" + filepath.Join(home, "."+shellName+"_history"),
	}
	if shellName == "bash" {
		environment = append(environment, "PS1="+ptyPrompt)
	} else {
		environment = append(environment, "PROMPT="+ptyPrompt, "ZDOTDIR="+home)
	}
	session := startShellPTY(t, executable, arguments, environment)
	session.run(`source "$HOME/integration"`)

	selectionOutput := session.pressCtrlG()
	if count := strings.Count(selectionOutput, "bad"); count != 1 {
		t.Fatalf("%s selection appeared %d times, want once:\n%s", shellName, count, selectionOutput)
	}
	if output := session.run(`printf 'STATUS=%s\n' "$?"`); !strings.Contains(output, "STATUS=1") {
		t.Fatalf("%s did not preserve the selected command status:\n%s", shellName, output)
	}

	if err := os.WriteFile(selectionPath, []byte("jump\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	session.pressCtrlG()
	if output := session.run("pwd"); !strings.Contains(output, home) {
		t.Fatalf("%s did not change the current directory:\n%s", shellName, output)
	}

	historyCommand := "history 20"
	if shellName == "zsh" {
		historyCommand = "fc -l -20"
	}
	history := strings.ReplaceAll(session.run(historyCommand), "\r", "")
	badHistoryLine := regexp.MustCompile(`(?m)^\s*\d+\s+bad$`)
	if count := len(badHistoryLine.FindAllString(history, -1)); count != 1 {
		t.Fatalf("%s history contains bad %d times, want once:\n%s", shellName, count, history)
	}

	cancelled := filepath.Join(home, "cancelled")
	session.write(`touch "$HOME/cancelled"`)
	session.pressCtrlG()
	session.run(":")
	if _, err := os.Stat(cancelled); !os.IsNotExist(err) {
		t.Fatalf("%s executed text from a nonempty prompt: %v", shellName, err)
	}

	stale := filepath.Join(home, "stale-ran")
	session.run(`alias stale='touch "$HOME/stale-ran"'`)
	if err := os.WriteFile(selectionPath, []byte("stale\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	session.pressCtrlGAndPause()
	session.run(":")
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("%s executed a stale definition: %v", shellName, err)
	}

	usage, err := os.ReadFile(filepath.Join(home, "usage"))
	if err != nil {
		t.Fatal(err)
	}
	if string(usage) != "bad\njump\n" {
		t.Fatalf("%s usage events = %q, want bad and jump once", shellName, usage)
	}
}
