package tui

import filepath "path/filepath"
import os "os"
import testing "testing"

func setupRepositoryDiffTest(t *testing.T, local, remote []byte) {
	t.Helper()
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv("ALIAS_LENS_SHELL", "bash")
	repository := filepath.Join(home, "dotfiles")
	if err := os.MkdirAll(repository, 0o700); err != nil {
		t.Fatal(err)
	}
	config := defaultConfig()
	config.Repository = repository
	if err := saveConfig(config); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, config.AliasFile), local, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, config.AliasFile), remote, 0o600); err != nil {
		t.Fatal(err)
	}
}
