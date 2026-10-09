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

func TestCompiledCatalogExplicitStartupPTY(t *testing.T) {
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
			repository, catalogValue := setupCatalogSyncFixture(t)
			nativeValue := `\printf native-fallback`
			catalogValue.Entries[0].Portable = nil
			catalogValue.Entries[0].Native = map[string]catalog.NativeImplementation{"bash": {AliasValue: &nativeValue}}
			catalogValue.Entries = append(catalogValue.Entries, catalog.Entry{ID: strings.Repeat("2", 32), Name: "portable_demo", Kind: "command", Portable: &catalog.Portable{Program: "printf", Args: []string{"synthetic"}}})
			writeCatalogFixture(t, filepath.Join(repository, "catalog.json"), catalogValue)
			svc := DefaultServices()
			os.Remove(svc.localCatalogPath())
			os.Remove(svc.catalogSyncPath())
			native := []byte("alias demo='\\printf native-fallback'\nretained_helper() { printf retained-helper; }\n")
			nativePath := filepath.Join(svc.homeDirectory(), ".bash_aliases")
			if err := os.WriteFile(nativePath, native, 0600); err != nil {
				t.Fatal(err)
			}
			sentinel := filepath.Join(svc.homeDirectory(), "alias-sentinel")
			opaque := []byte("alias demo='printf opaque-default'\nalias retained_helper=" + quoteShadow("builtin() { printf unsafe > "+quoteShadow(sentinel)+"; }") + "\nalias builtin='printf BUILTIN_SENTINEL'\nalias printf='printf PRINTF_SENTINEL'\nalias if='printf IF_SENTINEL'\n")
			extra := filepath.Join(svc.homeDirectory(), "extra-startup")
			if err := os.WriteFile(extra, opaque, 0600); err != nil {
				t.Fatal(err)
			}
			guard := []byte("if [ -f \"$HOME/.bash_aliases\" ]; then\n . \"$HOME/.bash_aliases\"\nfi\n")
			startup := bytes.Join([][]byte{[]byte("case $- in\n    *i*) ;;\n      *) return;;\nesac\nPS1=" + quoteShadow(ptyPrompt) + "\n"), guard, []byte("export SYNTHETIC_DIR=\"$HOME\"\nexport HOME_TOOLS=/synthetic\nprintf '%s' '{'\nexport PATH=\"$HOME/bin:$PATH\" # harmless HOME read\n[ -s \"$SYNTHETIC_DIR/extra-startup\" ] && \\. \"$SYNTHETIC_DIR/extra-startup\" # reviewed opaque load\n[ -f \"$SYNTHETIC_DIR/extra-startup\" ] && \\. \"$SYNTHETIC_DIR/extra-startup\"\ndeferred() { printf synthetic; alias demo='printf deferred'; }\ndeferred_markers() {\n" + catalogLoaderStart + "\n" + catalogLoaderEnd + "\n# >>> Alias Lens explicitly enrolled native source >>>\n# <<< Alias Lens explicitly enrolled native source <<<\nreturn;\n}\nPS2='" + catalogLoaderStart + "\n" + catalogLoaderEnd + "\n# >>> Alias Lens explicitly enrolled native source >>>\n# <<< Alias Lens explicitly enrolled native source <<<\n'\n")}, nil)
			startupPath := filepath.Join(svc.homeDirectory(), ".bashrc")
			if err := os.WriteFile(startupPath, startup, 0600); err != nil {
				t.Fatal(err)
			}
			command := exec.Command(binary, "init", repository, "--shell", "bash", "--catalog-path", "catalog.json", "--startup-path", startupPath)
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
			session.write("y\n")
			session.waitFor(0, "Apply this catalog enrollment and installation?")
			if !strings.Contains(output.stringFrom(0), "explicitly reorder") {
				t.Fatal("plan did not disclose explicit reorder")
			}
			session.write("y\n")
			if err := command.Wait(); err != nil {
				t.Fatalf("init: %v %s", err, output.stringFrom(0))
			}
			fresh := startShellPTY(t, bash, []string{"--noprofile", "-i"}, []string{"HOME=" + svc.homeDirectory(), "PATH=" + path, "TERM=xterm-256color", "HISTFILE=/dev/null", "PS1=" + ptyPrompt, "ALIAS_LENS_SHELL=bash"})
			if result := fresh.run("demo; \\builtin printf '\\n'"); !strings.Contains(result, "native-fallback") || strings.Contains(result, "SENTINEL") {
				t.Fatal("opaque alias changed approved native dispatch", result)
			}
			if result := fresh.run("portable_demo; \\builtin printf '\\n'"); !strings.Contains(result, "synthetic") || strings.Contains(result, "SENTINEL") {
				t.Fatal("opaque alias changed portable overlay", result)
			}
			if result := fresh.run("\\retained_helper; \\builtin printf '\\n'"); !strings.Contains(result, "retained-helper") || strings.Contains(result, "SENTINEL") {
				t.Fatal("retained definition changed", result)
			}
			if result := fresh.run("\\builtin declare -F builtin; \\builtin printf 'protected-status:%s\\n' $?"); !strings.Contains(result, "protected-status:1") {
				t.Fatal("opaque alias created protected builtin function", result)
			}
			if result := fresh.run("\\builtin shopt -q expand_aliases; \\builtin printf 'alias-option:%s\\n' $?; \\builtin alias builtin printf if retained_helper"); !strings.Contains(result, "alias-option:0") || !strings.Contains(result, "PRINTF_SENTINEL") {
				t.Fatal("alias option/table not restored", result)
			}
			if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
				t.Fatal("unknown alias executed")
			}
			reenable := exec.Command(binary, "catalog", "enable", "--shell", "bash", "--apply")
			reenable.Env = command.Env
			if result, err := reenable.CombinedOutput(); err != nil {
				t.Fatalf("verified reenrollment: %v %s", err, result)
			}
			updated, err := os.ReadFile(startupPath)
			if err != nil || bytes.Count(updated, guard) != 1 {
				t.Fatal("reenable lost exact moved source route")
			}
			offValid := startShellPTY(t, bash, []string{"--noprofile", "--norc", "-i"}, fresh.cmd.Env)
			offValid.run("\\builtin shopt -u expand_aliases")
			if result := offValid.run("\\builtin source " + quoteShadow(startupPath) + "; portable_demo; \\builtin shopt -q expand_aliases; \\builtin printf 'valid-off-option:%s\\n' $?"); !strings.Contains(result, "valid-off-option:1") || !strings.Contains(result, "synthetic") {
				t.Fatal("valid helper changed initially-off aliases", result)
			}
			pointer := filepath.Join(svc.catalogGeneratedRoot("bash"), "active")
			if err := os.WriteFile(pointer, []byte("missing\n"), 0600); err != nil {
				t.Fatal(err)
			}
			fallback := startShellPTY(t, bash, []string{"--noprofile", "-i"}, fresh.cmd.Env)
			if result := fallback.run("demo; \\builtin printf '\\n'; \\builtin shopt -q expand_aliases; \\builtin printf 'fallback-option:%s\\n' $?"); !strings.Contains(result, "native-fallback") || !strings.Contains(result, "fallback-option:0") {
				t.Fatal("native fallback/option failed", result)
			}
			off := startShellPTY(t, bash, []string{"--noprofile", "--norc", "-i"}, fresh.cmd.Env)
			off.run("\\builtin shopt -u expand_aliases")
			if result := off.run("\\builtin source " + quoteShadow(startupPath) + "; \\builtin shopt -q expand_aliases; \\builtin printf 'off-option:%s\\n' $?"); !strings.Contains(result, "off-option:1") {
				t.Fatal("initially disabled alias option changed", result)
			}
			if err := os.Rename(repository, repository+"-offline"); err != nil {
				t.Fatal(err)
			}
			rollback := exec.Command(binary, "catalog", "rollback", "--shell", "bash", "--apply")
			rollback.Env = command.Env
			if result, err := rollback.CombinedOutput(); err != nil {
				t.Fatalf("offline rollback: %v %s", err, result)
			}
			for file, expected := range map[string][]byte{nativePath: native, startupPath: startup, extra: opaque} {
				actual, err := os.ReadFile(file)
				if err != nil || !bytes.Equal(actual, expected) {
					t.Fatal("offline rollback changed exact baseline")
				}
			}
		})
	}
}
