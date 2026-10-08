//go:build !windows

package shell_test

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty/v2"

	"github.com/alderon07/al/internal/catalog"
	"github.com/alderon07/al/internal/catalogstore"
	"github.com/alderon07/al/internal/shell"
)

func TestBashRuntimeHandoffReadonlyPreflightPTY(t *testing.T) {
	executable, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"function", "alias", "portable"} {
		for _, attribute := range []string{"ordinary", "readonly", "exported-readonly"} {
			t.Run(kind+"/"+attribute, func(t *testing.T) {
				home := t.TempDir()
				run := readonlyHandoffPTY(t, executable, home)
				run("function first { builtin printf first-original; }; function collision { builtin printf collision-original; }; builtin export -f first; builtin alias first='printf first-mask'; builtin alias collision='printf collision-mask'")
				run("function COLLISION { builtin printf BODY_EXECUTED; builtin cat <<'BODY_END'\ndeclare -fr collision\nBODY_END\n}; builtin readonly -f COLLISION")
				if attribute != "ordinary" {
					run("builtin readonly -f collision")
				}
				if attribute == "exported-readonly" {
					run("builtin export -f collision")
				}
				run("builtin shopt -s extdebug nocasematch; builtin set -o noclobber; IFS=:")
				before := filepath.Join(home, "before")
				after := filepath.Join(home, "after")
				snapshot := "builtin printf 'IFS=<%s>\\n' \"$IFS\"; builtin declare -f first collision; builtin declare -F; builtin alias first collision; builtin printf 'OPTIONS_BEGIN\\n'; builtin set +o; builtin shopt -p"
				run("{ " + snapshot + "; } > " + shell.Quote(before))
				body := "builtin printf first-replacement"
				first := catalog.Entry{ID: strings.Repeat("1", 32), Name: "first", Kind: "function", Native: map[string]catalog.NativeImplementation{"bash": {FunctionBody: &body}}}
				collisionBody := "builtin printf collision-replacement"
				second := catalog.Entry{ID: strings.Repeat("2", 32), Name: "collision", Kind: "function", Native: map[string]catalog.NativeImplementation{"bash": {FunctionBody: &collisionBody}}}
				resolved := ""
				if kind == "alias" {
					second.Kind = "command"
					second.Native = map[string]catalog.NativeImplementation{"bash": {AliasValue: &collisionBody}}
				}
				if kind == "portable" {
					second.Kind = "command"
					second.Native = nil
					second.Portable = &catalog.Portable{Program: "printf", Args: []string{"collision-replacement"}}
					resolved = "/usr/bin/printf"
				}
				entries := make([]catalogstore.GenerationEntry, 0, 2)
				for i, value := range []catalog.Entry{first, second} {
					target := ""
					if i == 1 {
						target = resolved
					}
					declaration, err := catalogstore.Declaration(value, "bash", target)
					if err != nil {
						t.Fatal(err)
					}
					entries = append(entries, catalogstore.GenerationEntry{Entry: value, Declaration: declaration})
				}
				handoff, err := shell.RuntimeHandoff("bash", entries)
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(home, "handoff.bash")
				if err := os.WriteFile(path, []byte(handoff), 0600); err != nil {
					t.Fatal(err)
				}
				output := run("builtin source " + shell.Quote(path) + "; builtin printf 'HANDOFF_STATUS=%s\\n' \"$?\"")
				if strings.Contains(output, "syntax error") || strings.Contains(output, "replacement") || strings.Contains(output, "BODY_EXECUTED") {
					t.Fatal(output)
				}
				expected := "HANDOFF_STATUS=1"
				if attribute == "ordinary" {
					expected = "HANDOFF_STATUS=0"
				}
				if !strings.Contains(output, expected) {
					t.Fatal(output)
				}
				run("{ " + snapshot + "; } > " + shell.Quote(after))
				original, err := os.ReadFile(before)
				if err != nil {
					t.Fatal(err)
				}
				current, err := os.ReadFile(after)
				if err != nil {
					t.Fatal(err)
				}
				beforeIFS, _, _ := strings.Cut(string(original), "\n")
				afterIFS, _, _ := strings.Cut(string(current), "\n")
				_, beforeOptions, beforeFound := strings.Cut(string(original), "OPTIONS_BEGIN\n")
				_, afterOptions, afterFound := strings.Cut(string(current), "OPTIONS_BEGIN\n")
				if beforeIFS != afterIFS || !beforeFound || !afterFound || beforeOptions != afterOptions {
					t.Fatalf("handoff changed IFS or shell options:\nbefore=%s\nafter=%s", original, current)
				}
				if attribute != "ordinary" {
					if !bytes.Equal(original, current) {
						t.Fatalf("refused handoff changed definitions, aliases, attributes or options:\nbefore=%s\nafter=%s", original, current)
					}
					output = run(`first; collision; \first; \collision; builtin printf '\n'`)
					for _, want := range []string{"first-mask", "collision-mask", "first-original", "collision-original"} {
						if !strings.Contains(output, want) {
							t.Fatal(output)
						}
					}
				} else {
					output = run(`first; collision; builtin printf '\n'`)
					if !strings.Contains(output, "first-replacement") || !strings.Contains(output, "collision-replacement") {
						t.Fatal(output)
					}
					if !strings.Contains(string(current), "declare -fx first") || strings.Contains(string(current), "declare -fr first") || strings.Contains(string(current), "declare -fr collision") {
						t.Fatalf("ordinary function attributes changed: %s", current)
					}
				}
			})
		}
	}
}

func readonlyHandoffPTY(t *testing.T, executable, home string, arguments ...string) func(string) string {
	t.Helper()
	if len(arguments) == 0 {
		arguments = []string{"--noprofile", "--norc", "-i"}
	}
	command := exec.Command(executable, arguments...)
	command.Env = []string{"HOME=" + home, "PATH=/usr/bin:/bin", "TERM=xterm", "PS1=READONLY_READY> "}
	terminal, err := pty.Start(command)
	if err != nil {
		t.Fatal(err)
	}
	output := &handoffOutput{}
	go func() { _, _ = io.Copy(output, terminal) }()
	t.Cleanup(func() { _ = terminal.Close(); _ = command.Process.Kill(); _ = command.Wait() })
	wait := func(offset int) string {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			text := output.snapshot()
			if offset < len(text) && strings.Contains(text[offset:], "READONLY_READY> ") {
				return text[offset:]
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("PTY prompt missing: %s", output.snapshot())
		return ""
	}
	wait(0)
	run := func(text string) string {
		t.Helper()
		offset := len(output.snapshot())
		if _, err := io.WriteString(terminal, text+"\n"); err != nil {
			t.Fatal(err)
		}
		return wait(offset)
	}
	run("stty -echo")
	return run
}
