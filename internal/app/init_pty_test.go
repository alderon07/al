//go:build !windows

package app

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"alias-lens/internal/catalog"

	"github.com/creack/pty/v2"
)

func TestCompiledInitPTYCancellationAndPortableApply(t *testing.T) {
	if _, err := trustedShadowShell("bash"); err != nil {
		if os.Getenv("AL_REQUIRE_PTY_SHELLS") == "1" {
			t.Fatal(err)
		}
		t.Skip("normal compiled bootstrap requires root-owned system Bash")
	}
	binary := filepath.Join(t.TempDir(), "alias-lens")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/alias-lens")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, output)
	}
	shell, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	capturedPATH := os.Getenv("PATH")
	for _, scenario := range []struct {
		name                  string
		apply, native, cancel bool
	}{{"cancel", false, true, true}, {"portable-apply", true, false, false}, {"native-review-apply", false, true, false}} {
		apply, name := scenario.apply, scenario.name
		t.Run(name, func(t *testing.T) {
			repository, value := setupCatalogSyncFixture(t)
			os.Remove(DefaultServices().localCatalogPath())
			os.Remove(DefaultServices().catalogSyncPath())
			if scenario.native {
				native := "printf synthetic"
				value.Entries[0].Portable = nil
				value.Entries[0].Native = map[string]catalog.NativeImplementation{"bash": {AliasValue: &native}}
				writeCatalogFixture(t, filepath.Join(repository, "catalog.json"), value)
				os.WriteFile(filepath.Join(DefaultServices().homeDirectory(), ".bash_aliases"), []byte("alias demo='printf synthetic'\n"), 0600)
			}
			fallbackBefore, _ := os.ReadFile(filepath.Join(DefaultServices().homeDirectory(), ".bash_aliases"))
			os.WriteFile(filepath.Join(DefaultServices().homeDirectory(), ".bashrc"), []byte("PS1="+quoteShadow(ptyPrompt)+"\n"), 0600)
			args := []string{"init", repository, "--shell", "bash", "--catalog-path", "catalog.json"}
			if apply {
				args = append(args, "--apply")
			}
			command := exec.Command(binary, args...)
			command.Env = []string{"HOME=" + DefaultServices().homeDirectory(), "XDG_CONFIG_HOME=" + filepath.Join(DefaultServices().homeDirectory(), ".config"), "PATH=" + capturedPATH, "TERM=xterm-256color", "ALIAS_LENS_SHELL=bash", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull}
			terminal, err := pty.StartWithSize(command, &pty.Winsize{Rows: 24, Cols: 100})
			if err != nil {
				t.Fatal(err)
			}
			defer terminal.Close()
			output := &synchronizedBuffer{}
			go func() { io.Copy(output, terminal) }()
			session := &shellPTY{t: t, file: terminal, cmd: command, output: output}
			if !apply {
				session.waitFor(0, "Approve this exact native implementation?")
				session.write("y\n")
				session.waitFor(0, "Enroll this exact native fallback?")
				session.write("y\n")
				session.waitFor(0, "Apply this catalog enrollment and installation?")
				if scenario.cancel {
					session.write("n\n")
				} else {
					session.write("y\n")
				}
			}
			if err := command.Wait(); err != nil {
				t.Fatalf("init: %v %s", err, output.stringFrom(0))
			}
			if scenario.cancel {
				for _, path := range []string{DefaultServices().localCatalogPath(), DefaultServices().catalogSyncPath(), DefaultServices().catalogApprovalsPath(), DefaultServices().catalogAdoptionsPath(), DefaultServices().catalogInstalledPath()} {
					if _, err := os.Stat(path); !os.IsNotExist(err) {
						t.Fatal("cancelled init persisted state")
					}
				}
				after, _ := os.ReadFile(filepath.Join(DefaultServices().homeDirectory(), ".bash_aliases"))
				if string(after) != string(fallbackBefore) {
					t.Fatal("cancel changed fallback")
				}
				return
			}
			fresh := startShellPTY(t, shell, []string{"--noprofile", "-i"}, []string{"HOME=" + DefaultServices().homeDirectory(), "PATH=" + capturedPATH, "TERM=xterm-256color", "HISTFILE=/dev/null", "PS1=" + ptyPrompt, "ALIAS_LENS_SHELL=bash"})
			if result := fresh.run("demo; printf '\\n'"); !strings.Contains(result, "synthetic") {
				t.Fatal("installed command unavailable", result)
			}
		})
	}
}
