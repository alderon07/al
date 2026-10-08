package app

import (
	"alias-lens/internal/providers"
	"path/filepath"
	"testing"
)

func TestManagedRepositoryDestinationKeepsRemoteHierarchy(t *testing.T) {
	home := privateTestHome(t)
	destination, err := managedRepositoryDestination(home, providers.RemoteRepo{Provider: "gitlab", FullName: "team/tools/dotfiles"})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".local", "share", "alias-lens", "repos", "gitlab", "team", "tools", "dotfiles")
	if destination != want {
		t.Fatalf("destination = %q, want %q", destination, want)
	}

	for _, fullName := range []string{"dotfiles", "team//dotfiles", "team/../dotfiles", "/team/dotfiles", `team\dotfiles`} {
		if _, err := managedRepositoryDestination(home, providers.RemoteRepo{Provider: "github", FullName: fullName}); err == nil {
			t.Errorf("unsafe repository name %q was accepted", fullName)
		}
	}
}
