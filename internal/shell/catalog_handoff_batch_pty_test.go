//go:build !windows

package shell_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alderon07/al/internal/catalog"
	"github.com/alderon07/al/internal/catalogstore"
	"github.com/alderon07/al/internal/shell"
)

func TestZshRuntimeHandoffBatchBoundariesPTY(t *testing.T) {
	executable, err := exec.LookPath("zsh")
	if err != nil {
		if os.Getenv("AL_REQUIRE_PTY_SHELLS") == "1" {
			t.Fatal(err)
		}
		t.Skip("Zsh runtime unavailable")
	}
	for _, attribute := range []string{"ordinary", "readonly-functions", "readonly-aliases"} {
		t.Run(attribute, func(t *testing.T) {
			home := t.TempDir()
			run := readonlyHandoffPTY(t, executable, home, "-f", "-i")
			var original, reference strings.Builder
			entries := make([]catalogstore.GenerationEntry, 205)
			for index := range entries {
				name := fmt.Sprintf("synthetic_%03d", index)
				fmt.Fprintf(&original, "function %s { builtin printf old; }; builtin alias %s='printf masked'\n", name, name)
				body := `builtin printf '%s\n' "$1" "quoted ' body"; : > "$HOME/entry-executed"; return 23`
				value := catalog.Entry{ID: fmt.Sprintf("%032x", index+1), Name: name, Kind: "function", Native: map[string]catalog.NativeImplementation{"zsh": {FunctionBody: &body}}}
				if index%3 == 0 {
					body = "printf '%s\\n' 'alias quoted body'"
					value.Kind = "command"
					value.Native = map[string]catalog.NativeImplementation{"zsh": {AliasValue: &body}}
				}
				declaration, err := catalogstore.Declaration(value, "zsh", "")
				if err != nil {
					t.Fatal(err)
				}
				entries[index] = catalogstore.GenerationEntry{Entry: value, Declaration: declaration}
				if value.Kind == "function" {
					fmt.Fprintf(&reference, "builtin unalias -- %s\n", shell.QuoteShadow(name))
				}
				reference.WriteString(declaration)
			}
			write := func(name, contents string) string {
				t.Helper()
				path := filepath.Join(home, name)
				if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
					t.Fatal(err)
				}
				return shell.QuoteShadow(path)
			}
			handoff, err := shell.RuntimeHandoff("zsh", entries)
			if err != nil {
				t.Fatal(err)
			}
			run("builtin source " + write("original.zsh", original.String()))
			run("builtin setopt noclobber nonomatch; IFS=:")
			snapshot := "builtin functions ${(ko)functions[(I)synthetic_*]}; builtin alias -L ${(ko)aliases[(I)synthetic_*]}; builtin printf '\\nIFS=<%s>\\n' \"$IFS\"; builtin setopt"
			if attribute == "readonly-functions" {
				run("builtin typeset -r functions")
			}
			if attribute == "readonly-aliases" {
				run("builtin typeset -r aliases")
			}
			before, after := filepath.Join(home, "before"), filepath.Join(home, "after")
			run("{ " + snapshot + "; } > " + shell.QuoteShadow(before))
			output := run("builtin source " + write("handoff.zsh", handoff) + "; builtin printf 'HANDOFF_STATUS=%s\\n' \"$?\"")
			status := "HANDOFF_STATUS=1"
			if attribute == "ordinary" {
				status = "HANDOFF_STATUS=0"
			}
			if !strings.Contains(output, status) {
				t.Fatal(output)
			}
			if _, err := os.Stat(filepath.Join(home, "entry-executed")); !os.IsNotExist(err) {
				t.Fatal("handoff executed a function body", err)
			}
			run("{ " + snapshot + "; } > " + shell.QuoteShadow(after))
			current, err := os.ReadFile(after)
			if err != nil {
				t.Fatal(err)
			}
			previous, err := os.ReadFile(before)
			if err != nil {
				t.Fatal(err)
			}
			_, beforeOptions, beforeFound := strings.Cut(string(previous), "\nIFS=")
			_, afterOptions, afterFound := strings.Cut(string(current), "\nIFS=")
			if !beforeFound || !afterFound || beforeOptions != afterOptions {
				t.Fatal("handoff changed IFS or shell options")
			}
			if attribute != "ordinary" {
				if !bytes.Equal(previous, current) {
					t.Fatal("readonly preflight changed definitions or shell options")
				}
			} else {
				run("builtin source " + write("reference.zsh", reference.String()))
				expected := filepath.Join(home, "expected")
				run("{ " + snapshot + "; } > " + shell.QuoteShadow(expected))
				direct, err := os.ReadFile(expected)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(direct, current) {
					t.Fatal("batched handoff differs from fresh declarations")
				}
				output = run("synthetic_100 'two words'; builtin printf 'ENTRY_STATUS=%s\\n' \"$?\"")
				if !strings.Contains(output, "two words") || !strings.Contains(output, "ENTRY_STATUS=23") {
					t.Fatal(output)
				}
			}
		})
	}
}
