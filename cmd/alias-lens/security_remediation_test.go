package main

import (
	"os"
	"path/filepath"

	"testing"
)

func setupPushRepository(t *testing.T) (string, string) {
	t.Helper()
	directory := privateTestHome(t)
	bare := filepath.Join(directory, "remote.git")
	repository := filepath.Join(directory, "repository")
	runGit(t, directory, "init", "--bare", bare)
	if err := os.Mkdir(repository, 0o700); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "init")
	runGit(t, repository, "branch", "-M", "main")
	runGit(t, repository, "config", "user.email", "alias-lens@example.test")
	runGit(t, repository, "config", "user.name", "Alias Lens Test")
	if err := os.WriteFile(filepath.Join(repository, ".bash_aliases"), []byte("alias safe='true'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "add", ".bash_aliases")
	runGit(t, repository, "commit", "-m", "baseline")
	runGit(t, repository, "remote", "add", "origin", bare)
	runGit(t, repository, "push", "-u", "origin", "main")
	return repository, bare
}
