package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositorySyncPreservesUnrelatedStagedFiles(t *testing.T) {
	directory := t.TempDir()
	repository := filepath.Join(directory, "dotfiles")
	if err := os.Mkdir(repository, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "init")
	runGit(t, repository, "config", "user.email", "alias-lens@example.test")
	runGit(t, repository, "config", "user.name", "Alias Lens Test")
	unrelated := filepath.Join(repository, "editor.conf")
	if err := os.WriteFile(unrelated, []byte("staged but not committed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "add", "editor.conf")
	source := filepath.Join(directory, ".bash_aliases")
	aliases := []byte("alias gs='git status'\n")
	if err := os.WriteFile(source, aliases, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := syncRepositoryFiles(AppConfig{Repository: repository, AliasFile: "shell/.bash_aliases"}, source, false); err != nil {
		t.Fatal(err)
	}
	if staged := strings.TrimSpace(runGit(t, repository, "diff", "--cached", "--name-only")); staged != "editor.conf" {
		t.Fatalf("unrelated staged paths = %q", staged)
	}
	if contents, err := os.ReadFile(unrelated); err != nil || string(contents) != "staged but not committed\n" {
		t.Fatalf("unrelated file = %q, %v", contents, err)
	}
	if target, err := os.ReadFile(filepath.Join(repository, "shell", ".bash_aliases")); err != nil || string(target) != string(aliases) {
		t.Fatalf("repository alias copy = %q, %v", target, err)
	}
}

func TestPushFailureLeavesCompleteRepositoryCopy(t *testing.T) {
	directory := t.TempDir()
	repository := filepath.Join(directory, "dotfiles")
	if err := os.Mkdir(repository, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "init")
	runGit(t, repository, "config", "user.email", "alias-lens@example.test")
	runGit(t, repository, "config", "user.name", "Alias Lens Test")
	source := filepath.Join(directory, ".bash_aliases")
	aliases := []byte("alias gs='git status'\n")
	if err := os.WriteFile(source, aliases, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := syncRepositoryFiles(AppConfig{Repository: repository, AliasFile: ".bash_aliases"}, source, true)
	if err == nil || !strings.Contains(err.Error(), "retry with al sync --push") {
		t.Fatalf("push failure = %v", err)
	}
	if target, readErr := os.ReadFile(filepath.Join(repository, ".bash_aliases")); readErr != nil || string(target) != string(aliases) {
		t.Fatalf("repository alias copy = %q, %v", target, readErr)
	}
	if live, readErr := os.ReadFile(source); readErr != nil || string(live) != string(aliases) {
		t.Fatalf("live alias file = %q, %v", live, readErr)
	}
}

func TestPullFailureLeavesLiveAliasesUnchanged(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	repository := filepath.Join(home, "dotfiles")
	if err := os.Mkdir(repository, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "init")
	runGit(t, repository, "remote", "add", "origin", filepath.Join(home, "missing-remote"))
	aliasPath := filepath.Join(home, ".bash_aliases")
	original := []byte("alias keep='true'\n")
	if err := os.WriteFile(aliasPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	config := defaultConfig()
	config.Repository = repository
	if err := saveConfig(config); err != nil {
		t.Fatal(err)
	}

	_, err := pullRepository()
	if err == nil || !strings.Contains(err.Error(), "retry with al sync --pull") {
		t.Fatalf("pull failure = %v", err)
	}
	if after, readErr := os.ReadFile(aliasPath); readErr != nil || string(after) != string(original) {
		t.Fatalf("live aliases = %q, %v", after, readErr)
	}
}
