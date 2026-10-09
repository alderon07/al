//go:build !windows

package app

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alderon07/al/internal/catalog"
	"github.com/creack/pty/v2"
)

func TestCompiledCatalogRetainedNativePTY(t *testing.T) {
	if _, err := trustedShadowShell("bash"); err != nil {
		if os.Getenv("AL_REQUIRE_PTY_SHELLS") == "1" {
			t.Fatal(err)
		}
		t.Skip("requires trusted system Bash")
	}
	binary := filepath.Join(t.TempDir(), "alias-lens")
	if output, err := exec.Command("go", "build", "-o", binary, "../../cmd/alias-lens").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, output)
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Dir(binary) + string(os.PathListSeparator) + os.Getenv("PATH")
	for _, width := range []uint16{52, 140} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			repository, value := setupCatalogSyncFixture(t)
			svc := DefaultServices()
			os.Remove(svc.localCatalogPath())
			os.Remove(svc.catalogSyncPath())
			nativeValue := "printf 'installed-synthetic:%s\\n' --demo demo /demo"
			otherValue := "printf 'other-synthetic:%s\\n' --other other /other"
			selfValue := "self_command --self_command"
			value.Entries[0].Portable = nil
			value.Entries[0].Native = map[string]catalog.NativeImplementation{"bash": {AliasValue: &nativeValue}}
			value.Entries = append(value.Entries, catalog.Entry{ID: strings.Repeat("2", 32), Name: "other", Kind: "command", Native: map[string]catalog.NativeImplementation{"bash": {AliasValue: &otherValue}}}, catalog.Entry{ID: strings.Repeat("3", 32), Name: "self_command", Kind: "command", Native: map[string]catalog.NativeImplementation{"bash": {AliasValue: &selfValue}}})
			if err := os.WriteFile(filepath.Join(filepath.Dir(binary), "self_command"), []byte("#!/bin/sh\nprintf 'self-command-synthetic:%s\\n' \"$*\"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			writeCatalogFixture(t, filepath.Join(repository, "catalog.json"), value)
			sentinel := filepath.Join(svc.homeDirectory(), "inspection-sentinel")
			helpers := []byte("retained_helper() {\n local value=\"$(printf '%s' \"${HOME##*/}\")\"\n printf 'retained-synthetic:%s\\n' \\\n  \"$value\"\n}\nretained_sentinel() { printf '%s' \"$(printf unexpected > " + quoteShadow(sentinel) + ")\"; }\n")
			route := []byte("eval \"$(command alias-lens shell-init bash)\"\n")
			alias := []byte("alias demo=" + quoteShadow(nativeValue) + "\nalias other=" + quoteShadow(otherValue) + "\nalias self_command=" + quoteShadow(selfValue) + "\n")
			trailing := []byte("alias trailing='printf trailing-synthetic'\n")
			original := bytes.Join([][]byte{alias, helpers, route, trailing}, nil)
			aliasPath := filepath.Join(svc.homeDirectory(), ".bash_aliases")
			if err := os.WriteFile(aliasPath, original, 0600); err != nil {
				t.Fatal(err)
			}
			startupBefore := []byte("case $- in\n    *i*) ;;\n      *) return;;\nesac\nalias demo=\"printf earlier-default\"\nalias unrelated='printf demo --demo return'\nPS1=" + quoteShadow(ptyPrompt) + "\nif [ -f \"$HOME/.bash_aliases\" ]; then\n . \"$HOME/.bash_aliases\"\nfi\ndeferred() { return; }\n")
			if err := os.WriteFile(filepath.Join(svc.homeDirectory(), ".bashrc"), startupBefore, 0600); err != nil {
				t.Fatal(err)
			}
			command := exec.Command(binary, "init", repository, "--shell", "bash", "--catalog-path", "catalog.json")
			command.Env = []string{"HOME=" + svc.homeDirectory(), "XDG_CONFIG_HOME=" + filepath.Join(svc.homeDirectory(), ".config"), "PATH=" + path, "TERM=xterm-256color", "ALIAS_LENS_SHELL=bash", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull}
			terminal, err := pty.StartWithSize(command, &pty.Winsize{Rows: 24, Cols: width})
			if err != nil {
				t.Fatal(err)
			}
			defer terminal.Close()
			output := &synchronizedBuffer{}
			go func() { io.Copy(output, terminal) }()
			session := &shellPTY{t: t, file: terminal, cmd: command, output: output}
			session.waitFor(0, "Approve this exact batch?")
			if !strings.Contains(output.stringFrom(0), "3 native approvals and 3 fallback enrollments") {
				t.Fatal("compiled batch omitted complete alias review counts")
			}
			session.write("y\n")
			session.waitFor(0, "Apply this catalog enrollment and installation?")
			if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
				t.Fatal("inspection executed substitution")
			}
			session.write("y\n")
			if err := command.Wait(); err != nil {
				t.Fatalf("init: %v %s", err, output.stringFrom(0))
			}
			after, err := os.ReadFile(aliasPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(after, bytes.Join([][]byte{alias, helpers, trailing}, nil)) {
				t.Fatal("retained native bytes changed")
			}
			fresh := startShellPTY(t, bash, []string{"--noprofile", "-i"}, []string{"HOME=" + svc.homeDirectory(), "PATH=" + path, "TERM=xterm-256color", "HISTFILE=/dev/null", "PS1=" + ptyPrompt, "ALIAS_LENS_SHELL=bash"})
			for _, check := range []struct{ command, want string }{{"demo; printf '\\n'", "installed-synthetic"}, {"other; printf '\\n'", "other-synthetic:"}, {"self_command", "self-command-synthetic:--self_command"}, {"retained_helper", "retained-synthetic:"}, {"trailing; printf '\\n'", "trailing-synthetic"}} {
				if result := fresh.run(check.command); !strings.Contains(result, check.want) {
					t.Fatalf("new shell missing %s: %s", check.want, result)
				}
			}
			if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
				t.Fatal("installation executed sentinel")
			}
			if err := os.Rename(repository, repository+"-offline"); err != nil {
				t.Fatal(err)
			}
			rollback := exec.Command(binary, "catalog", "rollback", "--shell", "bash", "--apply")
			rollback.Env = command.Env
			if result, err := rollback.CombinedOutput(); err != nil {
				t.Fatalf("offline rollback: %v %s", err, result)
			}
			restored, err := os.ReadFile(aliasPath)
			if err != nil || !bytes.Equal(restored, original) {
				t.Fatal("offline rollback did not restore exact native baseline")
			}
			startup, err := os.ReadFile(filepath.Join(svc.homeDirectory(), ".bashrc"))
			if err != nil || !bytes.Equal(startup, startupBefore) {
				t.Fatal("offline rollback did not restore exact startup baseline")
			}
			if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
				t.Fatal("rollback executed sentinel")
			}
		})
	}
}
