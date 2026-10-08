package tui

import (
	"github.com/alderon07/al/internal/providers"
	"testing"
)

func TestFilterRemoteRepos(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	repositories := []providers.RemoteRepo{
		{Provider: "github", FullName: "naqi/al", Description: "Alias manager"},
		{Provider: "gitlab", FullName: "naqi/dotfiles", Description: "Shell configuration"},
	}

	if got := filterRemoteRepos(repositories, "dot"); len(got) != 1 || got[0].FullName != "naqi/dotfiles" {
		t.Fatalf("name filter returned %#v", got)
	}
	if got := filterRemoteRepos(repositories, "manager"); len(got) != 1 || got[0].FullName != "naqi/al" {
		t.Fatalf("description filter returned %#v", got)
	}
	if got := filterRemoteRepos(repositories, "gitlab"); len(got) != 1 || got[0].FullName != "naqi/dotfiles" {
		t.Fatalf("provider filter returned %#v", got)
	}
	if got := filterRemoteRepos(repositories, ""); len(got) != len(repositories) {
		t.Fatalf("empty filter returned %d repositories", len(got))
	}
}
