//go:build !windows

package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/alderon07/al/internal/catalog"
	"strings"
	"testing"

	shellapi "github.com/alderon07/al/internal/shell"
	"github.com/creack/pty/v2"
)

func TestMain(m *testing.M) {
	if os.Getenv("AL_INIT_PTY_HELPER") == "1" {
		applicationDependencies.FindTrustedShell = func(shell string) (string, error) { return exec.LookPath(shell) }
		os.Exit(runMain())
	}
	os.Exit(m.Run())
}

func runHelperInitGuidedPTY(t *testing.T, binary, shellName string) {
	runInitGuidedPTY(t, binary, shellName, false)
}

func runInitGuidedPTY(t *testing.T, binary, shellName string, insecureCompletions bool) {
	t.Helper()
	adapter, err := applicationServices().ShellAdapter(shellName)
	if err != nil {
		t.Fatal(err)
	}
	startup := ".bashrc"
	if shellName == "zsh" {
		startup = ".zshrc"
	}
	shell, err := exec.LookPath(shellName)
	if err != nil {
		if os.Getenv("AL_REQUIRE_PTY_SHELLS") == "1" {
			t.Fatal(err)
		}
		t.Skip("shell runtime unavailable")
	}
	capturedPATH := os.Getenv("PATH")
	for _, scenario := range []struct {
		name                  string
		apply, native, cancel bool
	}{{"cancel", false, true, true}, {"portable-apply", true, false, false}, {"native-review-apply", false, true, false}} {
		apply, name := scenario.apply, scenario.name
		t.Run(name, func(t *testing.T) {
			repository, value := setupCatalogSyncFixture(t)
			os.Remove(catalogPathFixture())
			os.Remove(filepath.Join(filepath.Dir(catalogPathFixture()), "catalog-sync.json"))
			if scenario.native {
				native := "printf synthetic"
				value.Entries[0].Portable = nil
				value.Entries[0].Native = map[string]catalog.NativeImplementation{shellName: {AliasValue: &native}}
				writeCatalogFixture(t, filepath.Join(repository, "catalog.json"), value)
				os.WriteFile(filepath.Join(os.Getenv("HOME"), adapter.AliasFilename()), []byte("alias demo='printf synthetic'\n"), 0600)
			}
			fallbackBefore, _ := os.ReadFile(filepath.Join(os.Getenv("HOME"), adapter.AliasFilename()))
			if shellName == "zsh" {
				completionPath := filepath.Join(os.Getenv("HOME"), ".config", "alias-lens", "completion.zsh")
				if err := os.WriteFile(completionPath, []byte(renderZshCompletion()), 0600); err != nil {
					t.Fatal(err)
				}
			}
			insecureDirectory := ""
			if insecureCompletions {
				insecureDirectory = seedInsecureZshCompletion(t, os.Getenv("HOME"))
			}
			if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), startup), []byte(initPTYStartup(shellName, insecureDirectory)), 0600); err != nil {
				t.Fatal(err)
			}
			args := []string{"init", repository, "--shell", shellName, "--catalog-path", "catalog.json"}
			if apply {
				args = append(args, "--apply")
			}
			command := exec.Command(binary, args...)
			command.Env = []string{"AL_INIT_PTY_HELPER=1", "HOME=" + os.Getenv("HOME"), "XDG_CONFIG_HOME=" + filepath.Join(os.Getenv("HOME"), ".config"), "PATH=" + capturedPATH, "TERM=xterm-256color", "ALIAS_LENS_SHELL=" + shellName, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull}
			terminal, err := pty.StartWithSize(command, &pty.Winsize{Rows: 24, Cols: 100})
			if err != nil {
				t.Fatal(err)
			}
			defer terminal.Close()
			output := &synchronizedBuffer{}
			go func() { io.Copy(output, terminal) }()
			session := &shellPTY{t: t, file: terminal, cmd: command, output: output}
			if !apply {
				session.waitFor(0, "Approve this exact batch?")
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
				for _, path := range []string{catalogPathFixture(), filepath.Join(filepath.Dir(catalogPathFixture()), "catalog-sync.json"), filepath.Join(os.Getenv("HOME"), ".local", "state", "alias-lens", "native-approvals.json"), filepath.Join(os.Getenv("HOME"), ".local", "state", "alias-lens", "native-adoptions.json"), filepath.Join(os.Getenv("HOME"), ".local", "state", "alias-lens", "catalog-installed.json")} {
					if _, err := os.Stat(path); !os.IsNotExist(err) {
						t.Fatal("cancelled init persisted state")
					}
				}
				after, _ := os.ReadFile(filepath.Join(os.Getenv("HOME"), adapter.AliasFilename()))
				if string(after) != string(fallbackBefore) {
					t.Fatal("cancel changed fallback")
				}
				return
			}
			if shellName == "zsh" {
				zshEnvironment := "skip_global_compinit=1\n"
				if insecureCompletions {
					zshEnvironment += "fpath=(" + shellapi.Quote(insecureDirectory) + " $fpath)\n" + initPTYGlobalCompinit + "\n"
				}
				if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), ".zshenv"), []byte(zshEnvironment), 0600); err != nil {
					t.Fatal(err)
				}
			}
			shellArgs := []string{"-i"}
			if shellName == "bash" {
				shellArgs = []string{"--noprofile", "-i"}
			}
			freshEnvironment := []string{"AL_INIT_PTY_HELPER=1", "HOME=" + os.Getenv("HOME"), "PATH=" + capturedPATH, "TERM=xterm-256color", "HISTFILE=/dev/null", "PS1=" + ptyPrompt, "ALIAS_LENS_SHELL=" + shellName}
			if fpath, ok := os.LookupEnv("FPATH"); ok {
				freshEnvironment = append(freshEnvironment, "FPATH="+fpath)
			}
			fresh := startShellPTY(t, shell, shellArgs, freshEnvironment)
			if shellName == "zsh" {
				result := fresh.run(`print -r -- "AL_COMPLETION:${_comps[al]-}:${_comps[alias-lens]-}:$+functions[compdef]"`)
				if !strings.Contains(result, "AL_COMPLETION:_alias_lens_complete:_alias_lens_complete:1") {
					t.Fatal("real completion registration unavailable", result)
				}
				if insecureCompletions {
					result := fresh.run(`print -r -- "AL_UNSAFE:$+functions[_al_unsafe_fixture]:${_comps[al_unsafe_fixture]-}:${fpath[(Ie)$HOME/insecure-completions]}"`)
					if !strings.Contains(result, "AL_UNSAFE:0::0") {
						t.Fatal("insecure completion was accepted", result)
					}
					if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), "unsafe-completion-executed")); !os.IsNotExist(err) {
						t.Fatal("insecure completion executed", err)
					}
				}
			}
			if result := fresh.run("demo; printf '\\n'"); !strings.Contains(result, "synthetic") {
				t.Fatal("installed command unavailable", result)
			}
		})
	}
}

func TestHelperInitGuidedBashAndZshPTY(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, shellName := range []string{"bash", "zsh"} {
		t.Run(shellName, func(t *testing.T) { runHelperInitGuidedPTY(t, binary, shellName) })
	}
}

func TestCompiledZshInitGuidedPTY(t *testing.T) {
	if _, err := shellapi.TrustedShadowShell("zsh"); err != nil {
		if os.Getenv("AL_REQUIRE_PTY_SHELLS") == "1" {
			t.Fatal(err)
		}
		t.Skip("normal compiled bootstrap requires root-owned system Zsh")
	}
	binary := filepath.Join(t.TempDir(), "alias-lens")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, output)
	}
	runHelperInitGuidedPTY(t, binary, "zsh")
}

func initPTYStartup(shellName, insecureDirectory string) string {
	startup := "PS1=" + shellapi.Quote(ptyPrompt) + "\n"
	if shellName == "zsh" {
		if insecureDirectory != "" {
			startup += "fpath=(" + shellapi.Quote(insecureDirectory) + " $fpath)\n"
		}
		startup += "autoload -Uz compinit\ncompinit -i\n"
	}
	return startup
}

func seedInsecureZshCompletion(t *testing.T, home string) string {
	t.Helper()
	directory := filepath.Join(home, "insecure-completions")
	if err := os.Mkdir(directory, 0777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(directory, 0777); err != nil {
		t.Fatal(err)
	}
	contents := "#compdef al_unsafe_fixture\nprint unsafe > \"$HOME/unsafe-completion-executed\"\n"
	if err := os.WriteFile(filepath.Join(directory, "_al_unsafe_fixture"), []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	return directory
}

func TestHelperInitGuidedZshInsecureCompletionsPTY(t *testing.T) {
	executable, err := exec.LookPath("zsh")
	if err != nil {
		if os.Getenv("AL_REQUIRE_PTY_SHELLS") == "1" {
			t.Fatal(err)
		}
		t.Skip("shell runtime unavailable")
	}
	home := privateTestHome(t)
	directory := seedInsecureZshCompletion(t, home)
	probeEnvironment := []string{"HOME=" + home, "PATH=" + os.Getenv("PATH"), "TERM=xterm-256color", "PS1=" + ptyPrompt, "HISTFILE=/dev/null"}
	if fpath, ok := os.LookupEnv("FPATH"); ok {
		probeEnvironment = append(probeEnvironment, "FPATH="+fpath)
	}
	probe := startShellPTY(t, executable, []string{"-f", "-i"}, probeEnvironment)
	offset := probe.mark()
	probe.write("fpath=(" + shellapi.Quote(directory) + " $fpath); " + initPTYGlobalCompinit + "\n")
	probe.waitFor(offset, "Ignore insecure directories and continue")
	probe.write("n\n")
	probe.waitFor(offset, ptyPrompt)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	runInitGuidedPTY(t, binary, "zsh", true)
}

const initPTYGlobalCompinit = `if [[ -z "$skip_global_compinit" ]]; then autoload -Uz compinit; compinit; fi`
