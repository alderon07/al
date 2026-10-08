//go:build !windows

package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/alderon07/al/internal/app"
	"github.com/alderon07/al/internal/catalog"
	shellapi "github.com/alderon07/al/internal/shell"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty/v2"
)

type startupBenchmarkFixture struct {
	home, shell, executable, path, kind string
	count, bytes                        int
}

func BenchmarkShellStartup(b *testing.B) {
	b.StopTimer()
	root := b.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		b.Fatal(err)
	}
	binary := filepath.Join(root, "alias-lens")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	build := exec.CommandContext(ctx, "go", "build", "-buildvcs=false", "-o", binary, ".")
	output, err := build.CombinedOutput()
	cancel()
	if err != nil {
		b.Fatalf("build benchmark executable: %v %s", err, output)
	}
	for _, shell := range []string{"bash", "zsh"} {
		executable, err := exec.LookPath(shell)
		if err != nil {
			if os.Getenv("AL_REQUIRE_PTY_SHELLS") == "1" {
				b.Fatal(err)
			}
			b.Logf("%s startup cases unavailable", shell)
			continue
		}
		b.Run(shell, func(b *testing.B) {
			for _, kind := range []string{"empty", "native", "catalog"} {
				sizes := []int{100, 1000, 10000}
				if kind == "empty" {
					sizes = []int{0}
				}
				for _, size := range sizes {
					b.Run(fmt.Sprintf("%s/%d", kind, size), func(b *testing.B) {
						b.StopTimer()
						fixture := prepareStartupBenchmark(b, binary, shell, executable, kind, size)
						for _, login := range []bool{false, true} {
							route := "nonlogin"
							if login {
								route = "login"
							}
							b.Run(route, func(b *testing.B) {
								b.StopTimer()
								if err := runStartupBenchmark(fixture, login, true); err != nil {
									b.Fatal(err)
								}
								b.ReportAllocs()
								b.ResetTimer()
								b.StartTimer()
								for i := 0; i < b.N; i++ {
									if err := runStartupBenchmark(fixture, login, false); err != nil {
										b.Fatal(err)
									}
								}
								b.StopTimer()
								b.ReportMetric(float64(size), "entries")
								b.ReportMetric(float64(fixture.bytes), "input-bytes")
								if _, err := os.Stat(filepath.Join(fixture.home, "benchmark-entry-executed")); !os.IsNotExist(err) {
									b.Fatal("startup executed an entry", err)
								}
							})
						}
					})
				}
			}
		})
	}
}

func prepareStartupBenchmark(b *testing.B, binary, shell, executable, kind string, count int) startupBenchmarkFixture {
	b.Helper()
	home := b.TempDir()
	if err := os.Chmod(home, 0700); err != nil {
		b.Fatal(err)
	}
	path := filepath.Dir(binary) + string(os.PathListSeparator) + os.Getenv("PATH")
	fixture := startupBenchmarkFixture{home: home, shell: shell, executable: executable, path: path, kind: kind, count: count}
	startup := ".bashrc"
	if shell == "zsh" {
		startup = ".zshrc"
	}
	if err := os.WriteFile(filepath.Join(home, startup), []byte(initPTYStartup(shell, "")), 0600); err != nil {
		b.Fatal(err)
	}
	if shell == "bash" {
		if err := os.WriteFile(filepath.Join(home, ".bash_profile"), []byte("source \"$HOME/.bashrc\"\n"), 0600); err != nil {
			b.Fatal(err)
		}
	}
	environment := func(key string) string {
		switch key {
		case "HOME":
			return home
		case "PATH":
			return path
		case "ALIAS_LENS_SHELL":
			return shell
		case "SHELL":
			return executable
		case "GIT_CONFIG_GLOBAL":
			return os.DevNull
		case "GIT_CONFIG_NOSYSTEM":
			return "1"
		}
		return ""
	}
	services := app.NewServices(app.Dependencies{HomeDir: func() (string, error) { return home, nil }, WorkingDir: func() (string, error) { return home, nil }, Environment: environment, Executable: func() (string, error) { return binary, nil }, FindTrustedShell: func(name string) (string, error) {
		if name != shell {
			return "", fmt.Errorf("unexpected validator %s", name)
		}
		return executable, nil
	}, StartWatcher: func() error { return nil }})
	if kind != "empty" {
		config := services.Settings.Defaults()
		config.Shell = shell
		config.AliasFile = "." + shell + "_aliases"
		config.AutoSync.Enabled = false
		if err := services.Settings.Save(config); err != nil {
			b.Fatal(err)
		}
		if kind == "native" {
			var contents strings.Builder
			for i := 0; i < count; i++ {
				fmt.Fprintf(&contents, "alias bench_%05d='printf synthetic > \"$HOME/benchmark-entry-executed\"'\n", i)
			}
			fixture.bytes = contents.Len()
			if err := os.WriteFile(filepath.Join(home, config.AliasFile), []byte(contents.String()), 0600); err != nil {
				b.Fatal(err)
			}
			if _, err := services.Setup(app.SetupRequest{Shell: shell}); err != nil {
				b.Fatal(err)
			}
		} else {
			repo := filepath.Join(home, "repository")
			if err := os.Mkdir(repo, 0700); err != nil {
				b.Fatal(err)
			}
			value := catalog.Catalog{SchemaVersion: 2, Entries: make([]catalog.Entry, count)}
			for i := range value.Entries {
				value.Entries[i] = catalog.Entry{ID: fmt.Sprintf("%032x", i+1), Name: fmt.Sprintf("bench_%05d", i), Kind: "command", Portable: &catalog.Portable{Program: "touch", Args: []string{"benchmark-entry-executed"}, PassArguments: true}}
			}
			contents, diagnostics := catalog.Encode(value)
			if len(diagnostics) != 0 {
				b.Fatal(diagnostics)
			}
			fixture.bytes = len(contents)
			if err := os.WriteFile(filepath.Join(repo, "catalog.json"), contents, 0600); err != nil {
				b.Fatal(err)
			}
			for _, args := range [][]string{{"init", "-q"}, {"add", "--", "catalog.json"}, {"-c", "user.name=Synthetic", "-c", "user.email=synthetic@example.invalid", "commit", "-qm", "synthetic"}} {
				startupBenchmarkGit(b, home, repo, args)
			}
			options := app.CatalogInitOptions{Source: repo, Shell: shell, CatalogPath: "catalog.json"}
			decisions := app.CatalogLifecycleDecisions{ConfirmedAt: services.LifecycleTimestamp()}
			plan, err := services.BuildLocalCatalogInitPlan(options, decisions)
			if err != nil {
				b.Fatal(err)
			}
			if plan.Summary.Blocked {
				b.Fatal(plan.Diagnostics)
			}
			if err := services.ApplyLocalCatalogInit(plan, options, decisions); err != nil {
				b.Fatal(err)
			}
		}
	}
	if shell == "zsh" {
		if err := os.WriteFile(filepath.Join(home, ".zshenv"), []byte("skip_global_compinit=1\n"), 0600); err != nil {
			b.Fatal(err)
		}
	}
	return fixture
}

func startupBenchmarkGit(b *testing.B, home, repo string, args []string) {
	b.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "git", append([]string{"-C", repo}, args...)...)
	command.Env = []string{"HOME=" + home, "PATH=" + os.Getenv("PATH"), "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1"}
	if output, err := command.CombinedOutput(); err != nil {
		b.Fatalf("synthetic repository preparation: %v %s", err, output)
	}
}

type startupBenchmarkOutput struct {
	mu    sync.Mutex
	text  strings.Builder
	ready chan struct{}
	once  sync.Once
}

func (output *startupBenchmarkOutput) Write(contents []byte) (int, error) {
	output.mu.Lock()
	defer output.mu.Unlock()
	if output.text.Len() < 65536 {
		remaining := 65536 - output.text.Len()
		output.text.Write(contents[:min(len(contents), remaining)])
	}
	if strings.Contains(output.text.String(), ptyPrompt) {
		output.once.Do(func() { close(output.ready) })
	}
	return len(contents), nil
}
func (output *startupBenchmarkOutput) String() string {
	output.mu.Lock()
	defer output.mu.Unlock()
	return output.text.String()
}

func runStartupBenchmark(fixture startupBenchmarkFixture, login, fullMembership bool) error {
	args := []string{"-i"}
	if login {
		args = []string{"-l", "-i"}
	}
	if fixture.shell == "bash" && !login {
		args = []string{"--noprofile", "-i"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, fixture.executable, args...)
	command.Dir = fixture.home
	command.Cancel = func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	command.Env = []string{"HOME=" + fixture.home, "PATH=" + fixture.path, "TERM=xterm-256color", "HISTFILE=/dev/null", "PS1=" + ptyPrompt, "ALIAS_LENS_SHELL=" + fixture.shell, "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1"}
	if fpath, ok := os.LookupEnv("FPATH"); ok {
		command.Env = append(command.Env, "FPATH="+fpath)
	}
	terminal, err := pty.StartWithSize(command, &pty.Winsize{Rows: 24, Cols: 100})
	if err != nil {
		return err
	}
	output := &startupBenchmarkOutput{ready: make(chan struct{})}
	drained := make(chan struct{})
	go func() { _, _ = io.Copy(output, terminal); close(drained) }()
	defer func() { _ = terminal.Close(); <-drained }()
	select {
	case <-output.ready:
	case <-ctx.Done():
		_ = command.Cancel()
		_ = command.Wait()
		return fmt.Errorf("startup timeout: %s", output.String())
	}
	check := "! type al >/dev/null 2>&1 && ! type bench_00000 >/dev/null 2>&1"
	if fixture.kind != "empty" {
		last := fmt.Sprintf("bench_%05d", fixture.count-1)
		if fixture.shell == "bash" {
			category := "alias"
			if fixture.kind == "catalog" {
				category = "function"
			}
			check = "declare -F al >/dev/null && [[ $(type -t " + last + ") == " + category + " ]]"
			if fullMembership {
				check += " && { count=0; while IFS= read -r name; do case $name in bench_*) count=$((count+1));; esac; done < <(compgen -A " + category + "); [ \"$count\" = " + fmt.Sprint(fixture.count) + " ]; }"
				lookup := "builtin alias \"$name\" >/dev/null 2>&1"
				if fixture.kind == "catalog" {
					lookup = "builtin declare -F \"$name\" >/dev/null && ! builtin alias \"$name\" >/dev/null 2>&1"
				}
				check += " && { expected=0; while [ \"$expected\" -lt " + fmt.Sprint(fixture.count) + " ]; do builtin printf -v name 'bench_%05d' \"$expected\"; " + lookup + " || break; expected=$((expected+1)); done; [ \"$expected\" = " + fmt.Sprint(fixture.count) + " ]; }"
			}
		} else {
			table := "aliases"
			if fixture.kind == "catalog" {
				table = "functions"
			}
			check = "(( $+functions[al] == 1 && $+" + table + "[" + last + "] == 1 ))"
			if fullMembership {
				check += " && { count=0; for name in ${(k)" + table + "}; do [[ $name == bench_* ]] && (( count++ )); done; [[ $count == " + fmt.Sprint(fixture.count) + " ]]; }"
				lookup := "(( $+" + table + "[$name] == 1 ))"
				if fixture.kind == "catalog" {
					lookup += " && (( $+aliases[$name] == 0 ))"
				}
				check += " && { expected=0; while (( expected < " + fmt.Sprint(fixture.count) + " )); do builtin printf -v name 'bench_%05d' \"$expected\"; " + lookup + " || break; (( expected++ )); done; (( expected == " + fmt.Sprint(fixture.count) + " )); }"
			}
		}
	}
	if _, err := io.WriteString(terminal, "if "+check+"; then printf '\\nPB_CHECK_OK\\n'; else printf '\\nPB_CHECK_FAILED\\n'; fi; exit\n"); err != nil {
		_ = command.Cancel()
		_ = command.Wait()
		return err
	}
	err = command.Wait()
	if ctx.Err() != nil {
		return fmt.Errorf("startup exit timeout: %s", output.String())
	}
	select {
	case <-drained:
	case <-ctx.Done():
		return fmt.Errorf("startup output drain timeout")
	}
	text := output.String()
	if err != nil {
		return fmt.Errorf("shell exit: %w %s", err, text)
	}
	if !strings.Contains(text, "\r\nPB_CHECK_OK\r\n") || strings.Contains(text, "\r\nPB_CHECK_FAILED\r\n") {
		return fmt.Errorf("startup membership check failed: %s", text)
	}
	if _, err := os.Stat(filepath.Join(fixture.home, "benchmark-entry-executed")); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("startup executed an entry")
	}
	return nil
}

func TestStartupBenchmarkRejectsMissingDeclarationsPTY(t *testing.T) {
	executable, err := exec.LookPath("bash")
	if err != nil {
		if os.Getenv("AL_REQUIRE_PTY_SHELLS") == "1" {
			t.Fatal(err)
		}
		t.Skip("Bash runtime unavailable")
	}
	for _, scenario := range []struct {
		name, declarations string
		count              int
	}{
		{"integration", "alias bench_00000='touch benchmark-entry-executed'\n", 1},
		{"last-entry", "al() { :; }\nalias bench_00000='touch benchmark-entry-executed'\nalias bench_00002='touch benchmark-entry-executed'\n", 2},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			home := t.TempDir()
			if err := os.Chmod(home, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte(initPTYStartup("bash", "")+scenario.declarations), 0600); err != nil {
				t.Fatal(err)
			}
			fixture := startupBenchmarkFixture{home: home, shell: "bash", executable: executable, path: os.Getenv("PATH"), kind: "native", count: scenario.count}
			if err := runStartupBenchmark(fixture, false, true); err == nil || !strings.Contains(err.Error(), "membership check failed") {
				t.Fatalf("invalid fixture accepted: %v", err)
			}
			if _, err := os.Stat(filepath.Join(home, "benchmark-entry-executed")); !os.IsNotExist(err) {
				t.Fatal("membership validation executed an entry", err)
			}
		})
	}
}

func TestStartupBenchmarkCompleteMembershipPTY(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		executable, err := exec.LookPath(shell)
		if err != nil {
			if os.Getenv("AL_REQUIRE_PTY_SHELLS") == "1" {
				t.Fatal(err)
			}
			t.Skip(shell + " runtime unavailable")
		}
		for _, kind := range []string{"native", "catalog"} {
			for _, scenario := range []string{"complete", "first", "middle", "last", "wrong-kind"} {
				t.Run(shell+"/"+kind+"/"+scenario, func(t *testing.T) {
					home := t.TempDir()
					if err := os.Chmod(home, 0700); err != nil {
						t.Fatal(err)
					}
					if shell == "zsh" {
						directory := seedInsecureZshCompletion(t, home)
						environment := "skip_global_compinit=1\nfpath=(" + shellapi.Quote(directory) + " $fpath)\n" + initPTYGlobalCompinit + "\n"
						if err := os.WriteFile(filepath.Join(home, ".zshenv"), []byte(environment), 0600); err != nil {
							t.Fatal(err)
						}
					}
					var declarations strings.Builder
					declarations.WriteString(initPTYStartup(shell, "") + "function al { :; }\n")
					for index := 0; index < 3; index++ {
						name := fmt.Sprintf("bench_%05d", index)
						if (scenario == "first" && index == 0) || (scenario == "middle" && index == 1) || (scenario == "last" && index == 2) {
							name = "bench_99999"
						}
						function := kind == "catalog"
						if scenario == "wrong-kind" && index == 1 {
							function = !function
						}
						if function {
							fmt.Fprintf(&declarations, "function %s { touch benchmark-entry-executed; }\n", name)
						} else {
							fmt.Fprintf(&declarations, "alias %s='touch benchmark-entry-executed'\n", name)
						}
					}
					if scenario == "wrong-kind" {
						if kind == "catalog" {
							declarations.WriteString("function bench_99999 { touch benchmark-entry-executed; }\n")
						} else {
							declarations.WriteString("alias bench_99999='touch benchmark-entry-executed'\n")
						}
					}
					startup := ".bashrc"
					if shell == "zsh" {
						startup = ".zshrc"
					}
					if err := os.WriteFile(filepath.Join(home, startup), []byte(declarations.String()), 0600); err != nil {
						t.Fatal(err)
					}
					fixture := startupBenchmarkFixture{home: home, shell: shell, executable: executable, path: os.Getenv("PATH"), kind: kind, count: 3}
					err := runStartupBenchmark(fixture, false, true)
					if scenario == "complete" {
						if err != nil {
							t.Fatal(err)
						}
					} else if err == nil || !strings.Contains(err.Error(), "membership check failed") {
						t.Fatalf("incomplete fixture accepted: %v", err)
					}
					if _, err := os.Stat(filepath.Join(home, "benchmark-entry-executed")); !os.IsNotExist(err) {
						t.Fatal("membership verification executed an entry", err)
					}
					if _, err := os.Stat(filepath.Join(home, "unsafe-completion-executed")); !os.IsNotExist(err) {
						t.Fatal("membership fixture executed an insecure completion", err)
					}
				})
			}
		}
	}
}
