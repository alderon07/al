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

func TestShortcutEditorInPTY(t *testing.T) {
	home := t.TempDir()
	command := exec.Command(os.Args[0], "-test.run=^TestShortcutEditorPTYHelper$")
	command.Env = append(os.Environ(), "ALIAS_LENS_SHORTCUT_EDITOR_HELPER=1", "HOME="+home, "TERM=xterm-256color", "NO_COLOR=1")
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
	_, _ = terminal.Write([]byte("\x1bOP"))
	waitForPTYText(t, output, offset, "Keyboard guide")
	offset = output.length()
	_, _ = terminal.Write([]byte{9})
	waitForPTYText(t, output, offset, "Configure shortcuts")
	for _, size := range []struct{ columns, rows uint16 }{{48, 18}, {120, 36}} {
		offset = output.length()
		if err := pty.Setsize(terminal, &pty.Winsize{Rows: size.rows, Cols: size.columns}); err != nil {
			t.Fatal(err)
		}
		redraw := waitForPTYText(t, output, offset, "Configure shortcuts")
		if !strings.Contains(redraw, "launcher") || !strings.Contains(redraw, "enter change") {
			t.Fatalf("%dx%d editor lost controls: %q", size.columns, size.rows, redraw)
		}
	}
	offset = output.length()
	_, _ = terminal.Write([]byte{13})
	waitForPTYText(t, output, offset, "Press the new key")
	offset = output.length()
	_, _ = terminal.Write([]byte{11})
	waitForPTYText(t, output, offset, "Enter to save ctrl+k")
	offset = output.length()
	_, _ = terminal.Write([]byte{13})
	waitForPTYText(t, output, offset, "reload your shell integration")
	configBytes, err := os.ReadFile(filepath.Join(home, ".config", "alias-lens", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(configBytes), `"launcher": "ctrl+k"`) {
		t.Fatalf("launcher was not saved: %q", configBytes)
	}
}

func TestSingleLetterModesInPTY(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestShortcutEditorPTYHelper$")
	command.Env = append(os.Environ(), "ALIAS_LENS_SHORTCUT_EDITOR_HELPER=1", "HOME="+t.TempDir(), "TERM=xterm-256color", "NO_COLOR=1")
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
	waitForPTYText(t, output, 0, "command mode")
	offset := output.length()
	_, _ = terminal.Write([]byte("h"))
	waitForPTYText(t, output, offset, "Keyboard guide")
	offset = output.length()
	_, _ = terminal.Write([]byte{27})
	waitForPTYText(t, output, offset, "command mode")
	offset = output.length()
	if err := pty.Setsize(terminal, &pty.Winsize{Rows: 18, Cols: 48}); err != nil {
		t.Fatal(err)
	}
	waitForPTYText(t, output, offset, "/ search")
	offset = output.length()
	_, _ = terminal.Write([]byte("/"))
	waitForPTYText(t, output, offset, "type to search")
	offset = output.length()
	_, _ = terminal.Write([]byte("zz"))
	search := waitForPTYText(t, output, offset, "zz")
	if strings.Contains(search, "Keyboard guide") {
		t.Fatal("h opened Help while typing an alias search")
	}
	offset = output.length()
	if err := pty.Setsize(terminal, &pty.Winsize{Rows: 36, Cols: 120}); err != nil {
		t.Fatal(err)
	}
	waitForPTYText(t, output, offset, "zz")
	offset = output.length()
	_, _ = terminal.Write([]byte{27})
	waitForPTYText(t, output, offset, "command mode")
	offset = output.length()
	_, _ = terminal.Write([]byte("s"))
	waitForPTYText(t, output, offset, "Alias rhythm")
}

func TestShortcutEditorPTYHelper(t *testing.T) {
	if os.Getenv("ALIAS_LENS_SHORTCUT_EDITOR_HELPER") != "1" {
		return
	}
	config := defaultConfig()
	config.ShortcutProfile = "linux"
	applyTheme(builtInTheme("phosphor"))
	_, err := tea.NewProgram(model{aliases: []Alias{{Name: "sample", Command: "printf sample"}}, width: 90, height: 24, theme: builtInTheme("phosphor"), shortcutProfile: resolvedShortcutProfile(config), shortcutLauncher: launcherLabel(config), aliasMode: aliasModeCommand}, tea.WithAltScreen()).Run()
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
