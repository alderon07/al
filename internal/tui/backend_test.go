package tui

import (
	"alias-lens/internal/app"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModelKeepsInjectedApplicationHomeAcrossUpdates(t *testing.T) {
	homes := []string{privateTestHome(t), privateTestHome(t)}
	models := make([]model, 2)
	for index, home := range homes {
		if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), []byte("alias sample='printf synthetic'\n"), 0600); err != nil {
			t.Fatal(err)
		}
		services := app.NewServices(app.Dependencies{HomeDir: func() (string, error) { return home, nil }, Environment: func(key string) string {
			switch key {
			case "HOME":
				return home
			case "ALIAS_LENS_SHELL":
				return "bash"
			case "SHELL":
				return "/bin/bash"
			case "PATH":
				return "/usr/bin:/bin"
			}
			return ""
		}})
		aliases, err := services.Entries()
		if err != nil {
			t.Fatal(err)
		}
		models[index] = model{services: services, aliases: aliases, width: 90, height: 24, aliasMode: aliasModeCommand, shortcutProfile: shortcutLinux}
	}
	t.Setenv("HOME", privateTestHome(t))
	updated, _ := models[0].Update(letterKey('f'))
	if updated.(model).service() != models[0].service() {
		t.Fatal("update replaced the application service")
	}
	first, err := os.ReadFile(filepath.Join(homes[0], ".bash_aliases"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(homes[1], ".bash_aliases"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(first), "favorite=true") || strings.Contains(string(second), "favorite=true") {
		t.Fatalf("favorite did not stay within its injected home: first=%q second=%q", first, second)
	}
	if _, err := os.Stat(filepath.Join(homes[1], ".bash_aliases.alias-lens.bak")); !os.IsNotExist(err) {
		t.Fatalf("untouched model received a backup: %v", err)
	}
}
