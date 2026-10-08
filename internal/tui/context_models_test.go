package tui

import (
	"github.com/alderon07/al/internal/app"
	tea "github.com/alderon07/al/internal/tea"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDefaultPickerCanReachPastFirstTwelveAliases(t *testing.T) {
	aliases := make([]aliasEntry, 20)
	for index := range aliases {
		name := "a" + strconv.Itoa(index)
		aliases[index] = aliasEntry{Name: name, Command: "echo " + name, Description: name}
	}
	ranked := suggestedAliases(applicationServices(),

		aliases)
	if len(ranked) != len(aliases) {
		t.Fatalf("default picker has %d of %d aliases", len(ranked), len(aliases))
	}
	view := (model{services: applicationServices(), aliases: aliases, cursor: 19, width: 100, height: 30}).View()
	if !strings.Contains(view, "of 20") || !strings.Contains(view, ranked[19].Name) {
		t.Fatalf("picker did not expose the last alias and range:\n%s", view)
	}
}

func TestTUIContextShortcutTogglesSelectedAlias(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	project := t.TempDir()
	alias := aliasEntry{Name: "deploy", Command: "go run ./deploy", Type: "alias"}
	ranking := contextRankingFixture(t, project, project)
	m := model{services: applicationServices(), aliases: []aliasEntry{alias}, context: ranking, width: 90, height: 24, shortcutProfile: shortcutLinux}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	marked := updated.(model)
	if marked.context.Match(alias) != 1 || !strings.Contains(marked.View(), "LOCAL") || strings.Contains(marked.View(), "HERE") {
		t.Fatalf("TUI did not show context mark: %s", marked.View())
	}
	updated, _ = marked.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	if updated.(model).context.Match(alias) != 0 {
		t.Fatal("TUI shortcut did not remove context mark")
	}
}

func TestTUIContextShortcutPrefersActiveDirectoryMark(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	project := t.TempDir()
	folder := filepath.Join(project, "src")
	if err := os.Mkdir(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	alias := aliasEntry{Name: "deploy", Command: "go run ./deploy", Type: "alias"}
	seedContextFixture(t, alias, app.ContextRepository, project)
	seedContextFixture(t, alias, app.ContextDirectory, folder)
	ranking := contextRankingFixture(t, folder, project)
	m := model{services: applicationServices(), aliases: []aliasEntry{alias}, context: ranking, width: 90, height: 24, shortcutProfile: shortcutLinux}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	withoutFolder := updated.(model)
	if withoutFolder.context.HasBinding(alias, app.ContextDirectory, folder) || !withoutFolder.context.HasBinding(alias, app.ContextRepository, project) {
		t.Fatal("TUI shortcut did not remove the active folder mark while preserving the project mark")
	}
	updated, _ = withoutFolder.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	withoutProject := updated.(model)
	if withoutProject.context.Match(alias) != 0 {
		t.Fatal("TUI shortcut did not remove the remaining project mark")
	}
}

func TestTUIContextShortcutRejectsDuplicateNames(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	alias := aliasEntry{Name: "same", Command: "echo one", Type: "alias"}
	m := model{services: applicationServices(), aliases: []aliasEntry{alias, {Name: "same", Command: "echo two", Type: "alias"}},
		context: contextRankingFixture(t, t.TempDir(), ""),
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	if !strings.Contains(updated.(model).status, "Duplicate alias name") {
		t.Fatalf("duplicate name was marked: %q", updated.(model).status)
	}
	path, _ := contextPathFixture()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("duplicate mark wrote context data: %v", err)
	}
}
