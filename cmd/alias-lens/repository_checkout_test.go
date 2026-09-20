package main

import (
	"path/filepath"
	"testing"
)

func TestManagedRepositoryDestinationKeepsRemoteHierarchy(t *testing.T) {
	home := t.TempDir()
	destination, err := managedRepositoryDestination(home, RemoteRepo{Provider: "gitlab", FullName: "team/tools/dotfiles"})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".local", "share", "alias-lens", "repos", "gitlab", "team", "tools", "dotfiles")
	if destination != want {
		t.Fatalf("destination = %q, want %q", destination, want)
	}

	for _, fullName := range []string{"dotfiles", "team//dotfiles", "team/../dotfiles", "/team/dotfiles", `team\dotfiles`} {
		if _, err := managedRepositoryDestination(home, RemoteRepo{Provider: "github", FullName: fullName}); err == nil {
			t.Errorf("unsafe repository name %q was accepted", fullName)
		}
	}
}
