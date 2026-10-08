package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileCommandAppliesThroughTransaction(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	if err := saveConfig(defaultConfig()); err != nil {
		t.Fatal(err)
	}
	if err := runConfigProfileCommand([]string{"add", "work"}); err != nil {
		t.Fatal(err)
	}
	observed, err := observeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(observed.Config.Profiles) != 1 || observed.Config.Profiles[0] != "work" {
		t.Fatalf("profiles = %#v", observed.Config.Profiles)
	}
	directory := filepath.Join(home, ".local", "state", "alias-lens", "workflows")
	entries, e := os.ReadDir(directory)
	if e != nil {
		t.Fatal(e)
	}
	backups := 0
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".workflow") {
			t.Fatalf("committed journal remains: %s", entry.Name())
		}
		if strings.HasSuffix(entry.Name(), ".backup") {
			backups++
		}
	}
	if backups != 1 {
		t.Fatalf("private backup count=%d", backups)
	}
}
