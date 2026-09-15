package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitLabConnectSignsInAndVerifiesAuthentication(t *testing.T) {
	directory := t.TempDir()
	logPath := filepath.Join(directory, "commands.log")
	markerPath := filepath.Join(directory, "authenticated")
	glabPath := filepath.Join(directory, "glab")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$GLAB_TEST_LOG"
if [ "$1 $2" = "auth status" ]; then
  test -f "$GLAB_TEST_MARKER"
  exit $?
fi
if [ "$1 $2" = "auth login" ]; then
  : > "$GLAB_TEST_MARKER"
  exit 0
fi
exit 2
`
	if err := os.WriteFile(glabPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory)
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("GLAB_TEST_LOG", logPath)
	t.Setenv("GLAB_TEST_MARKER", markerPath)

	provider := &gitlabProvider{host: "gitlab.com", protocol: "auto"}
	if err := provider.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	commands := strings.Split(strings.TrimSpace(string(contents)), "\n")
	want := []string{
		"auth status --hostname gitlab.com",
		"auth login --hostname gitlab.com --web --git-protocol https",
		"auth status --hostname gitlab.com",
	}
	if len(commands) != len(want) {
		t.Fatalf("commands = %#v, want %#v", commands, want)
	}
	for index := range want {
		if commands[index] != want[index] {
			t.Fatalf("command %d = %q, want %q", index, commands[index], want[index])
		}
	}
}

func TestGitLabListUsesAuthenticatedCLIWithoutEnvironmentToken(t *testing.T) {
	directory := t.TempDir()
	glabPath := filepath.Join(directory, "glab")
	script := `#!/bin/sh
if [ "$1" = "api" ]; then
  printf '%s\n' '[{"path_with_namespace":"team/dotfiles","visibility":"private","description":"Shell files","ssh_url_to_repo":"git@gitlab.com:team/dotfiles.git","http_url_to_repo":"https://gitlab.com/team/dotfiles.git"}]'
  exit 0
fi
exit 2
`
	if err := os.WriteFile(glabPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory)
	t.Setenv("GITLAB_TOKEN", "")

	repositories, err := (&gitlabProvider{host: "gitlab.com"}).List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(repositories) != 1 || repositories[0].FullName != "team/dotfiles" || !repositories[0].Private {
		t.Fatalf("repositories = %#v", repositories)
	}
}

func TestBitbucketConnectKeepsPromptedTokenOutOfConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("BITBUCKET_API_TOKEN", "")
	originalReader := readProviderSecret
	readProviderSecret = func(prompt string) (string, error) {
		if prompt != "Bitbucket API token: " {
			t.Fatalf("prompt = %q", prompt)
		}
		return "temporary-session-value", nil
	}
	defer func() { readProviderSecret = originalReader }()

	config := defaultConfig()
	provider, _, err := connectRepoProvider(context.Background(), config, "bitbucket")
	if err != nil {
		t.Fatal(err)
	}
	bitbucket, ok := provider.(*bitbucketProvider)
	if !ok || bitbucket.token != "temporary-session-value" {
		t.Fatalf("provider = %#v", provider)
	}
	path, err := configPath()
	if err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "temporary-session-value") {
		t.Fatal("temporary Bitbucket token was written to config")
	}
}

func TestBitbucketListDiscoversWorkspaces(t *testing.T) {
	t.Setenv("BITBUCKET_API_TOKEN", "")
	originalGetJSON := providerGetJSON
	providerGetJSON = func(_ context.Context, endpoint, _, credential string, target any) error {
		if credential != "Bearer temporary-session-value" {
			t.Fatalf("credential header = %q", credential)
		}
		payload := `{"values":[{"workspace":{"slug":"team"}}]}`
		if strings.Contains(endpoint, "/permissions/repositories") {
			payload = `{"values":[{"permission":"write","repository":{"full_name":"team/dotfiles","is_private":true,"description":"Shell files"}}]}`
		}
		return json.Unmarshal([]byte(payload), target)
	}
	defer func() { providerGetJSON = originalGetJSON }()

	provider := &bitbucketProvider{token: "temporary-session-value"}
	repositories, err := provider.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(repositories) != 1 || repositories[0].FullName != "team/dotfiles" || !repositories[0].Private {
		t.Fatalf("repositories = %#v", repositories)
	}
}

func TestBitbucketHTTPSCloneKeepsTokenOutOfArgumentsAndRemote(t *testing.T) {
	directory := t.TempDir()
	logPath := filepath.Join(directory, "git.log")
	gitPath := filepath.Join(directory, "git")
	script := `#!/bin/sh
printf 'args=%s\n' "$*" > "$BITBUCKET_CLONE_LOG"
printf 'askpass=%s\n' "$GIT_ASKPASS" >> "$BITBUCKET_CLONE_LOG"
printf 'answer=' >> "$BITBUCKET_CLONE_LOG"
"$GIT_ASKPASS" 'Password for Bitbucket' >> "$BITBUCKET_CLONE_LOG"
exit 0
`
	if err := os.WriteFile(gitPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory)
	t.Setenv("BITBUCKET_CLONE_LOG", logPath)

	repository := RemoteRepo{ProviderTag: "Bitbucket", FullName: "team/dotfiles"}
	cloneURL := "https://x-bitbucket-api-token-auth@bitbucket.org/team/dotfiles.git"
	if err := gitCloneWithBitbucketToken(context.Background(), cloneURL, repository, filepath.Join(directory, "checkout"), "temporary-session-value"); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	log := string(contents)
	if strings.Contains(strings.Split(log, "\n")[0], "temporary-session-value") {
		t.Fatalf("clone arguments contained token: %s", log)
	}
	if !strings.Contains(log, "answer=temporary-session-value") {
		t.Fatalf("askpass did not receive the in-memory token: %s", log)
	}
	var askPassPath string
	for _, line := range strings.Split(log, "\n") {
		if strings.HasPrefix(line, "askpass=") {
			askPassPath = strings.TrimPrefix(line, "askpass=")
		}
	}
	if askPassPath == "" {
		t.Fatal("git did not receive an askpass helper")
	}
	if _, err := os.Stat(askPassPath); !os.IsNotExist(err) {
		t.Fatalf("temporary askpass helper still exists: %v", err)
	}
}
