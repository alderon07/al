package main

import (
	"context"
	"io"
	"net/http"
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

	if err := connectGitLab(context.Background(), "gitlab.com"); err != nil {
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

func TestBitbucketConnectKeepsPromptedTokenOutOfConfig(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	t.Setenv("BITBUCKET_API_TOKEN", "")
	originalReader := readProviderSecret
	readProviderSecret = func(prompt string) (string, error) {
		if prompt != "Bitbucket API token: " {
			t.Fatalf("prompt = %q", prompt)
		}
		return "temporary-session-value", nil
	}
	defer func() { readProviderSecret = originalReader }()

	originalTransport := catalogProviderTransport
	t.Cleanup(func() { catalogProviderTransport = originalTransport })
	catalogProviderTransport = catalogProviderRoundTrip(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "api.bitbucket.org" || request.Header.Get("Authorization") != "Bearer temporary-session-value" {
			t.Fatal("temporary token was not retained by connection runtime")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"values":[{"permission":"write","repository":{"full_name":"synthetic/repo"}}]}`))}, nil
	})
	config := defaultConfig()
	settings := config.Providers["bitbucket"]
	settings.Workspaces = []string{"synthetic"}
	config.Providers["bitbucket"] = settings
	provider, _, err := connectRepoProvider(context.Background(), config, "bitbucket")
	if err != nil {
		t.Fatal(err)
	}
	if provider == nil || provider.ID() != "bitbucket" {
		t.Fatalf("provider = %#v", provider)
	}
	if repos, err := provider.List(context.Background()); err != nil || len(repos) != 1 {
		t.Fatal("temporary token discovery failed", err)
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
