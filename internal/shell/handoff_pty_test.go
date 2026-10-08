//go:build !windows

package shell_test

import (
	"bytes"
	"github.com/alderon07/al/internal/entry"
	"github.com/alderon07/al/internal/shell"
	"github.com/creack/pty/v2"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

type handoffOutput struct {
	sync.Mutex
	buffer bytes.Buffer
}

func (output *handoffOutput) Write(data []byte) (int, error) {
	output.Lock()
	defer output.Unlock()
	return output.buffer.Write(data)
}
func (output *handoffOutput) snapshot() string {
	output.Lock()
	defer output.Unlock()
	return output.buffer.String()
}

func TestShellEntryHandoffPTYArgumentsStatusMaskingAndSignal(t *testing.T) {
	for _, name := range []string{"bash", "zsh"} {
		t.Run(name, func(t *testing.T) {
			executable, err := exec.LookPath(name)
			if err != nil {
				t.Skip(name + " unavailable")
			}
			args := []string{"--noprofile", "--norc", "-i"}
			if name == "zsh" {
				args = []string{"-f", "-i"}
			}
			command := exec.Command(executable, args...)
			home := t.TempDir()
			command.Env = []string{"HOME=" + home, "ZDOTDIR=" + home, "PATH=/usr/bin:/bin", "TERM=xterm", "PS1=HANDOFF_READY> ", "PROMPT=HANDOFF_READY> "}
			terminal, err := pty.Start(command)
			if err != nil {
				t.Fatal(err)
			}
			output := &handoffOutput{}
			go func() { _, _ = io.Copy(output, terminal) }()
			t.Cleanup(func() {
				_, _ = terminal.Write([]byte("exit\n"))
				_ = terminal.Close()
				_ = command.Process.Kill()
				_ = command.Wait()
			})
			wait := func(offset int, want string) string {
				t.Helper()
				deadline := time.Now().Add(5 * time.Second)
				for time.Now().Before(deadline) {
					text := output.snapshot()
					if offset < len(text) && strings.Contains(text[offset:], want) {
						return text[offset:]
					}
					time.Sleep(10 * time.Millisecond)
				}
				t.Fatalf("missing %q: %s", want, output.snapshot())
				return ""
			}
			wait(0, "HANDOFF_READY> ")
			run := func(text string) string {
				t.Helper()
				offset := len(output.snapshot())
				if _, err := io.WriteString(terminal, text+"\n"); err != nil {
					t.Fatal(err)
				}
				return wait(offset, "HANDOFF_READY> ")
			}
			run("stty -echo")
			run("alias demo='printf STALE_ALIAS'")
			adapter, err := shell.New(name)
			if err != nil {
				t.Fatal(err)
			}
			declaration, err := shell.ShellEntryHandoff(adapter, entry.Alias{Name: "demo", Type: "function", Command: `printf 'ARG=<%s>\n' "$1"; return 23`})
			if err != nil {
				t.Fatal(err)
			}
			run(declaration)
			text := run("demo 'two words'; builtin printf 'RESULT=%s\\n' \"$?\"")
			if !strings.Contains(text, "ARG=<two words>") || !strings.Contains(text, "RESULT=23") || strings.Contains(text, "STALE_ALIAS") {
				t.Fatalf("handoff: %s", text)
			}
			signalDefinition, err := shell.ShellEntryHandoff(adapter, entry.Alias{Name: "demo", Type: "function", Command: `builtin printf 'FOREGROUND_STARTED\n'; sleep 30`})
			if err != nil {
				t.Fatal(err)
			}
			run(signalDefinition)
			offset := len(output.snapshot())
			if _, err := io.WriteString(terminal, "demo\n"); err != nil {
				t.Fatal(err)
			}
			wait(offset, "FOREGROUND_STARTED")
			time.Sleep(100 * time.Millisecond)
			if _, err := terminal.Write([]byte{3}); err != nil {
				t.Fatal(err)
			}
			wait(offset, "HANDOFF_READY> ")
			text = run("builtin printf 'INTERRUPT_RESULT=%s\\n' \"$?\"")
			if !strings.Contains(text, "INTERRUPT_RESULT=130") || strings.Contains(text, "STALE_ALIAS") {
				t.Fatalf("signal status: %s", text)
			}
		})
	}
}

func TestNativeDeclarationBoundaryPTY(t *testing.T) {
	for _, name := range []string{"bash", "zsh"} {
		t.Run(name, func(t *testing.T) {
			executable, err := exec.LookPath(name)
			if err != nil {
				if os.Getenv("AL_REQUIRE_PTY_SHELLS") == "1" {
					t.Fatal(name + " unavailable")
				}
				t.Skip(name + " unavailable")
			}
			for _, body := range []string{
				"\nbuiltin printf '%s' \\'\nbuiltin printf ':BODY\\n'\n",
				"\nbuiltin printf '%s' \"$HOME\" >/dev/null\nbuiltin printf ':BODY\\n'\n",
				"\nbuiltin printf '%s' $((8 << 2)) >/dev/null\nbuiltin printf ':BODY\\n'\n",
				"\ncat <<\\EOF\n} DATA {\nEOF\nbuiltin printf ':BODY\\n'\n",
				"\ncat <<'EO'F\n} DATA {\nEOF\nbuiltin printf ':BODY\\n'\n",
				"\ncat <<'EOF'\nEO\\\nF\n} DATA {\nEOF\nbuiltin printf ':BODY\\n'\n",
				"\nbuiltin printf '%s' '<<< literal' >/dev/null\nbuiltin printf ':BODY\\n'\n",
				"\ncat <<-EOF\n\t} DATA {\n\tEOF\nbuiltin printf ':BODY\\n'\n",
			} {
				declaration := "synthetic() {" + body + "}\n"
				if err := shell.ValidateCatalogNativeDeclaration(name, []byte(declaration)); err != nil {
					t.Fatal(err)
				}
				program := declaration + "builtin printf ':DEFINED\\n'\nsynthetic\nbuiltin printf ':DONE\\n'\n"
				args := []string{"--noprofile", "--norc", "-c", program}
				if name == "zsh" {
					args = []string{"-f", "-c", program}
				}
				home := t.TempDir()
				command := exec.Command(executable, args...)
				command.Env = []string{"HOME=" + home, "ZDOTDIR=" + home, "PATH=/usr/bin:/bin", "BASH_ENV=/dev/null", "ENV=/dev/null", "TERM=xterm", "FPATH=" + os.Getenv("FPATH")}
				terminal, err := pty.Start(command)
				if err != nil {
					t.Fatal(err)
				}
				output := &handoffOutput{}
				copied := make(chan struct{})
				go func() { _, _ = io.Copy(output, terminal); close(copied) }()
				waited := make(chan error, 1)
				go func() { waited <- command.Wait() }()
				select {
				case err := <-waited:
					if err != nil {
						t.Fatal(err, output.snapshot())
					}
				case <-time.After(5 * time.Second):
					_ = command.Process.Kill()
					_ = terminal.Close()
					<-waited
					t.Fatal("PTY declaration timed out")
				}
				select {
				case <-copied:
				case <-time.After(time.Second):
					_ = terminal.Close()
					<-copied
				}
				_ = terminal.Close()
				text := strings.ReplaceAll(output.snapshot(), "\r\n", "\n")
				if !strings.HasPrefix(text, ":DEFINED\n") || !strings.Contains(text, ":BODY\n") || !strings.HasSuffix(text, ":DONE\n") {
					t.Fatalf("definition executed early or invocation failed: %q", text)
				}
			}
		})
	}
}
