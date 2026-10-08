package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositorySyncPreservesUnrelatedStagedFiles(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
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
	if info, err := os.Stat(filepath.Join(repository, "shell", ".bash_aliases")); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("repository alias copy mode = %v, %v; want 0600", info, err)
	}
	if err := os.Chmod(filepath.Join(repository, "shell", ".bash_aliases"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := syncRepositoryFiles(AppConfig{Repository: repository, AliasFile: "shell/.bash_aliases"}, source, false); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(repository, "shell", ".bash_aliases")); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("matching repository alias copy mode = %v, %v; want 0600", info, err)
	}
	if err := os.Chmod(filepath.Join(repository, "shell", ".bash_aliases"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("alias gs='git status --short'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := syncRepositoryFiles(AppConfig{Repository: repository, AliasFile: "shell/.bash_aliases"}, source, false); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(repository, "shell", ".bash_aliases")); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("replaced repository alias copy mode = %v, %v; want 0600", info, err)
	}
}

func TestPlainSyncRefusesRecordedConflictWithoutChangingFilesOrGit(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	repository := filepath.Join(home, "dotfiles")
	if err := os.Mkdir(repository, 0o700); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "init")
	runGit(t, repository, "config", "user.email", "alias-lens@example.test")
	runGit(t, repository, "config", "user.name", "Alias Lens Test")
	local := []byte("alias keep='printf local'\n")
	remote := []byte("alias frog='printf remote'\nalias keep='printf base'\n")
	aliasPath := filepath.Join(home, ".bash_aliases")
	repositoryAliasPath := filepath.Join(repository, ".bash_aliases")
	if err := os.WriteFile(aliasPath, local, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(repositoryAliasPath, remote, 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "add", ".bash_aliases")
	runGit(t, repository, "commit", "-m", "Add remote aliases")
	config := defaultConfig()
	config.Repository = repository
	if err := saveConfig(config); err != nil {
		t.Fatal(err)
	}
	if err := writeSyncStatus("conflict", "both local and remote aliases changed; run al diff", contentHash(local), contentHash(remote)); err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(runGit(t, repository, "rev-parse", "HEAD"))

	_, err := syncRepository(false)
	if err == nil {
		t.Fatal("plain sync accepted an unresolved conflict")
	}
	for _, expected := range []string{"repository was not changed", "al diff", "al sync --pull", "al sync --push"} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("sync error does not contain %q: %v", expected, err)
		}
	}
	if after, readErr := os.ReadFile(aliasPath); readErr != nil || string(after) != string(local) {
		t.Fatalf("live aliases = %q, %v", after, readErr)
	}
	if after, readErr := os.ReadFile(repositoryAliasPath); readErr != nil || string(after) != string(remote) {
		t.Fatalf("repository aliases = %q, %v", after, readErr)
	}
	if afterHead := strings.TrimSpace(runGit(t, repository, "rev-parse", "HEAD")); afterHead != head {
		t.Fatalf("HEAD changed from %s to %s", head, afterHead)
	}
	if status := strings.TrimSpace(runGit(t, repository, "status", "--porcelain")); status != "" {
		t.Fatalf("repository changed: %q", status)
	}
}

func TestExplicitPushCanResolveRecordedConflict(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	remoteRepository := filepath.Join(home, "remote.git")
	repository := filepath.Join(home, "dotfiles")
	runGit(t, home, "init", "--bare", remoteRepository)
	runGit(t, home, "clone", remoteRepository, repository)
	runGit(t, repository, "config", "user.email", "alias-lens@example.test")
	runGit(t, repository, "config", "user.name", "Alias Lens Test")
	remote := []byte("alias frog='printf remote'\n")
	repositoryAliasPath := filepath.Join(repository, ".bash_aliases")
	if err := os.WriteFile(repositoryAliasPath, remote, 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "add", ".bash_aliases")
	runGit(t, repository, "commit", "-m", "Add remote aliases")
	runGit(t, repository, "push", "--set-upstream", "origin", "HEAD")
	local := []byte("alias keep='printf local'\n")
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), local, 0o600); err != nil {
		t.Fatal(err)
	}
	config := defaultConfig()
	config.Repository = repository
	if err := saveConfig(config); err != nil {
		t.Fatal(err)
	}
	if err := writeSyncStatus("conflict", "both local and remote aliases changed; run al diff", contentHash(local), contentHash(remote)); err != nil {
		t.Fatal(err)
	}

	message, err := syncRepository(true)
	if err != nil {
		t.Fatal(err)
	}
	if message != "Aliases committed and pushed" {
		t.Fatalf("sync message = %q", message)
	}
	if after, readErr := os.ReadFile(repositoryAliasPath); readErr != nil || string(after) != string(local) {
		t.Fatalf("repository aliases = %q, %v", after, readErr)
	}
	if pushed := runGit(t, remoteRepository, "show", "HEAD:.bash_aliases"); pushed != string(local) {
		t.Fatalf("pushed aliases = %q", pushed)
	}
}

func TestPushFailureLeavesCompleteRepositoryCopy(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
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
	home := privateTestHome(t)
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
