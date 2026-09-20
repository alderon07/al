//go:build !windows

package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty/v2"

	tea "alias-lens/cmd/alias-lens/internal/tea"
)

func TestTUIFooterSurvivesPTYResize(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestTUIResizeHelper$")
	command.Env = append(os.Environ(), "ALIAS_LENS_TUI_RESIZE_HELPER=1", "NO_COLOR=1", "TERM=xterm-256color")
	terminal, err := pty.StartWithSize(command, &pty.Winsize{Rows: 36, Cols: 120})
	if err != nil {
		t.Fatal(err)
	}
	output := &synchronizedBuffer{}
	go func() {
		_, _ = io.Copy(output, terminal)
	}()
	t.Cleanup(func() {
		_, _ = terminal.Write([]byte{3})
		_ = terminal.Close()
		if command.Process != nil {
			_ = command.Process.Kill()
		}
		_ = command.Wait()
	})

	initial := waitForPTYText(t, output, 0, resizeFooterMessage(120, 36, pageAliases))
	assertPTYContentWidth(t, initial, '─', 108, "aliases 120x36")
	for _, size := range []struct{ columns, rows uint16 }{{80, 24}, {100, 30}, {48, 18}, {120, 36}} {
		offset := output.length()
		if err := pty.Setsize(terminal, &pty.Winsize{Rows: size.rows, Cols: size.columns}); err != nil {
			t.Fatalf("resize PTY to %dx%d: %v", size.columns, size.rows, err)
		}
		redraw := waitForPTYText(t, output, offset, resizeFooterMessage(int(size.columns), int(size.rows), pageAliases))
		if !strings.Contains(redraw, "esc quit") {
			t.Fatalf("%dx%d PTY redraw lost control footer:\n%q", size.columns, size.rows, redraw)
		}
		assertPTYContentWidth(t, redraw, '─', newMainTUIFrame(int(size.columns), int(size.rows)).contentWidth, "aliases resize")
	}

	statsOffset := output.length()
	if _, err := terminal.Write([]byte("\x1bOQ")); err != nil {
		t.Fatal(err)
	}
	statsRedraw := waitForPTYText(t, output, statsOffset, resizeFooterMessage(120, 36, pageStats))
	assertPTYContentWidth(t, statsRedraw, '·', 108, "stats 120x36")
	for _, size := range []struct{ columns, rows uint16 }{{80, 24}, {120, 36}} {
		offset := output.length()
		if err := pty.Setsize(terminal, &pty.Winsize{Rows: size.rows, Cols: size.columns}); err != nil {
			t.Fatalf("resize stats PTY to %dx%d: %v", size.columns, size.rows, err)
		}
		redraw := waitForPTYText(t, output, offset, resizeFooterMessage(int(size.columns), int(size.rows), pageStats))
		if !strings.Contains(redraw, "r refresh") {
			t.Fatalf("%dx%d stats PTY redraw lost control footer:\n%q", size.columns, size.rows, redraw)
		}
		assertPTYContentWidth(t, redraw, '·', newMainTUIFrame(int(size.columns), int(size.rows)).contentWidth, "stats resize")
	}

	helpOffset := output.length()
	if _, err := terminal.Write([]byte("\x1bOP")); err != nil {
		t.Fatal(err)
	}
	helpRedraw := waitForPTYText(t, output, helpOffset, resizeFooterMessage(120, 36, pageHelp))
	if !strings.Contains(helpRedraw, "Keyboard guide") {
		t.Fatalf("help PTY redraw did not open keyboard guide:\n%q", helpRedraw)
	}
	assertPTYContentWidth(t, helpRedraw, '─', 108, "help 120x36")
	for _, size := range []struct{ columns, rows uint16 }{{80, 24}, {48, 18}, {120, 36}} {
		offset := output.length()
		if err := pty.Setsize(terminal, &pty.Winsize{Rows: size.rows, Cols: size.columns}); err != nil {
			t.Fatalf("resize help PTY to %dx%d: %v", size.columns, size.rows, err)
		}
		redraw := waitForPTYText(t, output, offset, resizeFooterMessage(int(size.columns), int(size.rows), pageHelp))
		if !strings.Contains(redraw, "esc") || !strings.Contains(redraw, "close") {
			t.Fatalf("%dx%d help PTY redraw lost control footer:\n%q", size.columns, size.rows, redraw)
		}
		assertPTYContentWidth(t, redraw, '─', newMainTUIFrame(int(size.columns), int(size.rows)).contentWidth, "help resize")
	}
}

func TestControlPunctuationPTY(t *testing.T) {
	tests := []struct {
		name     string
		sequence string
		page     tuiPage
		expected string
	}{
		{name: "kitty ctrl comma", sequence: "\x1b[44;5u", page: pageSettings, expected: "Compose your footer"},
		{name: "xterm ctrl comma", sequence: "\x1b[27;5;44~", page: pageSettings, expected: "Compose your footer"},
		{name: "kitty ctrl question", sequence: "\x1b[47:63;6u", page: pageHelp},
		{name: "xterm ctrl question", sequence: "\x1b[27;6;63~", page: pageHelp},
		{name: "legacy ctrl question", sequence: "\x1f", page: pageHelp},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := exec.Command(os.Args[0], "-test.run=^TestTUIResizeHelper$")
			command.Env = append(os.Environ(), "ALIAS_LENS_TUI_RESIZE_HELPER=1", "NO_COLOR=1", "TERM=xterm-256color")
			terminal, err := pty.StartWithSize(command, &pty.Winsize{Rows: 30, Cols: 100})
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
			waitForPTYText(t, output, 0, resizeFooterMessage(100, 30, pageAliases))
			offset := output.length()
			if _, err := terminal.Write([]byte(test.sequence)); err != nil {
				t.Fatal(err)
			}
			expected := test.expected
			if expected == "" {
				expected = resizeFooterMessage(100, 30, test.page)
			}
			waitForPTYText(t, output, offset, expected)
		})
	}
}

func TestManagedRepositoryPathPTY(t *testing.T) {
	home := t.TempDir()
	command := exec.Command(os.Args[0], "-test.run=^TestManagedRepositoryPathHelper$")
	command.Env = append(os.Environ(), "ALIAS_LENS_SYNC_PATH_HELPER=1", "HOME="+home, "NO_COLOR=1", "TERM=xterm-256color")
	terminal, err := pty.StartWithSize(command, &pty.Winsize{Rows: 24, Cols: 80})
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

	narrow := waitForPTYText(t, output, 0, resizeFooterMessage(80, 24, pageSync))
	if !strings.Contains(narrow, "alderon07") || !strings.Contains(narrow, "dotfiles") || strings.Contains(narrow, "alderon07--dotfiles") {
		t.Fatalf("narrow sync view has the wrong repository path:\n%q", narrow)
	}
	offset := output.length()
	if err := pty.Setsize(terminal, &pty.Winsize{Rows: 36, Cols: 140}); err != nil {
		t.Fatal(err)
	}
	wide := waitForPTYText(t, output, offset, resizeFooterMessage(140, 36, pageSync))
	if !strings.Contains(wide, "github/alderon07/dotfiles") || strings.Contains(wide, "alderon07--dotfiles") {
		t.Fatalf("wide sync view has the wrong repository path:\n%q", wide)
	}
}

func TestManagedRepositoryPathHelper(t *testing.T) {
	if os.Getenv("ALIAS_LENS_SYNC_PATH_HELPER") != "1" {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	applyTheme(builtInTheme("phosphor"))
	probe := resizeProbeModel{model: model{
		width:           80,
		height:          24,
		theme:           builtInTheme("phosphor"),
		shortcutProfile: shortcutLinux,
		trackedOnly:     true,
		trackedRepo:     filepath.Join(home, ".local", "share", "alias-lens", "repos", "github", "alderon07", "dotfiles"),
		autoSyncEnabled: true,
		syncInterval:    15,
		primarySync:     trackedFileItem{State: SyncState{Status: "synced", Message: "files match"}},
	}}
	_, err = tea.NewProgram(probe, tea.WithAltScreen()).Run()
	if err != nil {
		t.Fatal(err)
	}
}

func TestTUIResizeHelper(t *testing.T) {
	if os.Getenv("ALIAS_LENS_TUI_RESIZE_HELPER") != "1" {
		return
	}
	applyTheme(builtInTheme("phosphor"))
	applyFooterConfig(FooterConfig{Message: resizeFooterMessage(120, 36, pageAliases), Icon: "none", Alignment: "center", Tone: "quiet", Rule: resizeProbeRule(pageAliases)})
	aliases := []Alias{
		{Name: "a", Command: "printf a", Description: "first"},
		{Name: "b", Command: "printf b", Description: "second"},
		{Name: "c", Command: "printf c", Description: "third"},
		{Name: "d", Command: "printf d", Description: "fourth"},
		{Name: "e", Command: "printf e", Description: "fifth"},
		{Name: "f", Command: "printf f", Description: "sixth"},
		{Name: "g", Command: "printf g", Description: "seventh"},
		{Name: "h", Command: "printf h", Description: "eighth"},
	}
	probe := resizeProbeModel{model: model{
		aliases:         aliases,
		width:           120,
		height:          36,
		theme:           builtInTheme("phosphor"),
		executeMode:     true,
		shortcutProfile: shortcutLinux,
	}}
	_, err := tea.NewProgram(probe, tea.WithAltScreen()).Run()
	if err != nil {
		t.Fatal(err)
	}
}

type resizeProbeModel struct {
	model model
}

func (m resizeProbeModel) Init() tea.Cmd { return m.model.Init() }

func (m resizeProbeModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	updated, command := m.model.Update(message)
	m.model = updated.(model)
	page := m.model.currentPage()
	applyFooterConfig(FooterConfig{Message: resizeFooterMessage(m.model.width, m.model.height, page), Icon: "none", Alignment: "center", Tone: "quiet", Rule: resizeProbeRule(page)})
	return m, command
}

func (m resizeProbeModel) View() string { return m.model.View() }

func resizeFooterMessage(width, height int, page tuiPage) string {
	pageName := "aliases"
	switch page {
	case pageStats:
		pageName = "stats"
	case pageHelp:
		pageName = "help"
	case pageSettings:
		pageName = "settings"
	case pageSync:
		pageName = "sync"
	}
	return "Made by Naqi " + strconv.Itoa(width) + "x" + strconv.Itoa(height) + " " + pageName
}

func resizeProbeRule(page tuiPage) string {
	if page == pageStats {
		return "dots"
	}
	return "thin"
}

func assertPTYContentWidth(t *testing.T, redraw string, rule rune, expected int, label string) {
	t.Helper()
	if longest := longestRuneRun(redraw, rule); longest != expected {
		t.Fatalf("%s divider width = %d, want %d:\n%q", label, longest, expected, redraw)
	}
}

func longestRuneRun(value string, target rune) int {
	longest, current := 0, 0
	for _, character := range value {
		if character == target {
			current++
			longest = max(longest, current)
		} else {
			current = 0
		}
	}
	return longest
}

func waitForPTYText(t *testing.T, output *synchronizedBuffer, offset int, expected string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		contents := output.stringFrom(offset)
		if strings.Contains(contents, expected) {
			return contents
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("PTY redraw did not contain %q:\n%q", expected, output.stringFrom(offset))
	return ""
}
