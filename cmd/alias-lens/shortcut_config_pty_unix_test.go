//go:build !windows

package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "alias-lens/cmd/alias-lens/internal/tea"
	"github.com/creack/pty/v2"
)

func TestConfiguredTUIShortcutInPTY(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestCustomTUIShortcutHelper$")
	command.Env = append(os.Environ(), "ALIAS_LENS_CUSTOM_SHORTCUT_HELPER=1", "TERM=xterm-256color", "NO_COLOR=1")
	terminal, err := pty.StartWithSize(command, &pty.Winsize{Rows: 24, Cols: 90})
	if err != nil {
		t.Fatal(err)
	}
	output := &synchronizedBuffer{}
	go func() { _, _ = io.Copy(output, terminal) }()
	t.Cleanup(func() {
		_, _ = terminal.Write([]byte{3})
		_ = terminal.Close()
		if command.Process != nil {
			_ = command.Process.Kill()
		}
		_ = command.Wait()
	})
	waitForPTYText(t, output, 0, "sample")
	offset := output.length()
	if _, err := terminal.Write([]byte{0x19}); err != nil {
		t.Fatal(err)
	}
	waitForPTYText(t, output, offset, "Keyboard guide")
}

func TestCustomTUIShortcutHelper(t *testing.T) {
	if os.Getenv("ALIAS_LENS_CUSTOM_SHORTCUT_HELPER") != "1" {
		return
	}
	config := defaultConfig()
	config.ShortcutProfile = "linux"
	config.Shortcuts = map[string]string{"help": "Ctrl+Y"}
	applyTheme(builtInTheme("phosphor"))
	_, err := tea.NewProgram(model{aliases: []Alias{{Name: "sample", Command: "printf sample"}}, width: 90, height: 24, theme: builtInTheme("phosphor"), shortcutProfile: resolvedShortcutProfile(config)}, tea.WithAltScreen()).Run()
	if err != nil {
		t.Fatal(err)
	}
}

func TestConfiguredLauncherInShellPTY(t *testing.T) {
	config := defaultConfig()
	config.Shortcuts = map[string]string{"launcher": "Ctrl+K"}
	for _, test := range []struct {
		adapter ShellAdapter
		exe     string
		args    []string
	}{
		{bashShellAdapter{}, "bash", []string{"--noprofile", "--norc", "-i"}},
		{zshShellAdapter{}, "zsh", []string{"-f"}},
	} {
		t.Run(test.adapter.Name(), func(t *testing.T) {
			executable, err := exec.LookPath(test.exe)
			if err != nil {
				t.Skipf("%s is unavailable", test.exe)
			}
			if test.exe == "bash" && bashMajorVersion(t, executable) < 4 {
				t.Skip("Bash 3.2 does not install a prompt binding")
			}
			home := t.TempDir()
			bin := filepath.Join(home, "bin")
			if err := os.Mkdir(bin, 0o700); err != nil {
				t.Fatal(err)
			}
			shim := "#!/bin/sh\n[ \"${1-}\" = watch ] && exit 0\nprintf 'CUSTOM_LAUNCH\\n' >&2\nexit 1\n"
			if err := os.WriteFile(filepath.Join(bin, "alias-lens"), []byte(shim), 0o700); err != nil {
				t.Fatal(err)
			}
			integration, err := shellIntegrationForConfig(test.adapter, config)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(home, "integration"), []byte(integration), 0o600); err != nil {
				t.Fatal(err)
			}
			environment := []string{"HOME=" + home, "PATH=" + bin + ":/usr/bin:/bin", "TERM=xterm-256color", "HISTFILE=" + filepath.Join(home, ".history")}
			if test.exe == "bash" {
				environment = append(environment, "PS1="+ptyPrompt)
			} else {
				environment = append(environment, "PROMPT="+ptyPrompt, "ZDOTDIR="+home)
			}
			session := startShellPTY(t, executable, test.args, environment)
			session.run(`source "$HOME/integration"`)
			offset := session.mark()
			session.write("\x0b")
			session.waitFor(offset, "CUSTOM_LAUNCH")
			defaultIntegration, err := shellIntegrationForConfig(test.adapter, defaultConfig())
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(home, "integration"), []byte(defaultIntegration), 0o600); err != nil {
				t.Fatal(err)
			}
			session.run(`source "$HOME/integration"`)
			offset = session.mark()
			session.write("\x0b")
			if output := session.run(":"); strings.Contains(output, "CUSTOM_LAUNCH") || strings.Contains(session.output.stringFrom(offset), "CUSTOM_LAUNCH") {
				t.Fatal("old launcher remained active after reloading integration")
			}
			offset = session.mark()
			session.write("\x07")
			session.waitFor(offset, "CUSTOM_LAUNCH")
		})
	}
}
