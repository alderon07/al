//go:build !windows

package tui

import pty "github.com/creack/pty/v2"

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"

	tea "alias-lens/internal/tea"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestCustomTUIShortcutHelper(t *testing.T) {
	if os.Getenv("ALIAS_LENS_CUSTOM_SHORTCUT_HELPER") != "1" {
		return
	}
	config := defaultConfig()
	config.ShortcutProfile = "linux"
	config.Shortcuts = map[string]string{"help": "Ctrl+Y"}
	applyTheme(builtInTheme("phosphor"))
	_, err := tea.NewProgram(model{services: applicationServices(), aliases: []aliasEntry{{Name: "sample", Command: "printf sample"}}, width: 90, height: 24, theme: builtInTheme("phosphor"), shortcutProfile: resolvedShortcutProfile(config)}, tea.WithAltScreen()).Run()
	if err != nil {
		t.Fatal(err)
	}
}

func assertPTYSearchQueryBeforeEmptyState(t *testing.T, redraw, query, emptyState string) {
	t.Helper()
	plain := ansi.Strip(redraw)
	queryIndex := strings.Index(plain, query)
	emptyIndex := strings.Index(plain, emptyState)
	if queryIndex < 0 || emptyIndex < 0 || queryIndex >= emptyIndex {
		t.Fatalf("search field did not show %q before empty state: %q", query, plain)
	}
}

func TestShortcutEditorPTYHelper(t *testing.T) {
	if os.Getenv("ALIAS_LENS_SHORTCUT_EDITOR_HELPER") != "1" {
		return
	}
	config := defaultConfig()
	config.ShortcutProfile = "linux"
	applyTheme(builtInTheme("phosphor"))
	_, err := tea.NewProgram(model{services: applicationServices(), aliases: []aliasEntry{{Name: "sample", Command: "printf sample"}}, width: 90, height: 24, theme: builtInTheme("phosphor"), shortcutProfile: resolvedShortcutProfile(config), shortcutLauncher: launcherLabel(config), aliasMode: aliasModeCommand}, tea.WithAltScreen()).Run()
	if err != nil {
		t.Fatal(err)
	}
}

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

func TestShortcutEditorInPTY(t *testing.T) {
	home := privateTestHome(t)
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
	waitForPTYText(t, output, offset, "enter change")
	for _, size := range []struct{ columns, rows uint16 }{{48, 18}, {120, 36}} {
		offset = output.length()
		if err := pty.Setsize(terminal, &pty.Winsize{Rows: size.rows, Cols: size.columns}); err != nil {
			t.Fatal(err)
		}
		redraw := waitForPTYText(t, output, offset, "enter change")
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
	redraw := waitForPTYText(t, output, offset, `No alias matched "zz"`)
	assertPTYSearchQueryBeforeEmptyState(t, redraw, "SEARCH zz", `No alias matched "zz"`)
	offset = output.length()
	_, _ = terminal.Write([]byte{27})
	waitForPTYText(t, output, offset, "command mode")
	offset = output.length()
	_, _ = terminal.Write([]byte("s"))
	waitForPTYText(t, output, offset, "Alias rhythm")
}

func TestKeyboardGuideSearchQueryInPTY(t *testing.T) {
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
	for _, character := range "dffd" {
		offset = output.length()
		_, _ = terminal.Write([]byte(string(character)))
		waitForPTYText(t, output, offset, string(character)+"█")
	}
	for _, size := range []struct{ columns, rows uint16 }{{48, 18}, {120, 36}} {
		offset = output.length()
		if err := pty.Setsize(terminal, &pty.Winsize{Rows: size.rows, Cols: size.columns}); err != nil {
			t.Fatal(err)
		}
		redraw := waitForPTYText(t, output, offset, `No shortcut matched "dffd"`)
		assertPTYSearchQueryBeforeEmptyState(t, redraw, "FILTER dffd", `No shortcut matched "dffd"`)
	}
}
