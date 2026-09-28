package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFavoriteTogglePreservesMetadataAndSelection(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	path := filepath.Join(home, ".bash_aliases")
	original := "alias aa='echo alpha'\n# al: tags=work platforms=linux category=tools\n# Show beta\nalias bb='echo beta'\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	aliases, err := loadAliases()
	if err != nil {
		t.Fatal(err)
	}
	m := model{aliases: aliases, cursor: 1, width: 90, height: 24, aliasMode: aliasModeCommand, shortcutProfile: shortcutLinux}
	for _, want := range []bool{true, false} {
		updated, _ := m.Update(letterKey('f'))
		m = updated.(model)
		if selected := m.currentAliases()[m.cursor]; selected.Name != "bb" || selected.Favorite != want {
			t.Fatalf("selected alias after toggle = %#v, want bb favorite=%t", selected, want)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(contents), "tags=work") || !strings.Contains(string(contents), "platforms=linux") || !strings.Contains(string(contents), "category=tools") || !strings.Contains(string(contents), "# Show beta") || !strings.Contains(string(contents), "alias aa='echo alpha'") {
			t.Fatalf("favorite toggle lost alias data: %q", contents)
		}
		if strings.Contains(string(contents), "favorite=true") != want {
			t.Fatalf("favorite metadata after toggle = %q, want favorite=%t", contents, want)
		}
	}
	backup, err := os.ReadFile(path + ".alias-lens.bak")
	if err != nil || !strings.Contains(string(backup), "favorite=true") {
		t.Fatalf("backup before unmarking favorite = %q, %v", backup, err)
	}
	m.status = ""
	if !strings.Contains(m.View(), "f favorite") {
		t.Fatal("TUI did not show the favorite control")
	}
	updated, _ := m.Update(letterKey('/'))
	m = updated.(model)
	updated, _ = m.Update(letterKey('f'))
	m = updated.(model)
	if m.query != "f" || m.aliases[1].Favorite {
		t.Fatalf("f in search mode changed favorite instead of searching: query=%q aliases=%#v", m.query, m.aliases)
	}
}
