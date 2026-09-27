package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositoryDiffClearsStaleConflictForExactMatch(t *testing.T) {
	local := []byte("# Status\nalias gs='git status'\n")
	setupRepositoryDiffTest(t, local, local)
	if err := writeSyncStatus("conflict", "both local and remote aliases changed; run al diff", "old-local", "old-remote"); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := showRepositoryDiffTo(&output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "files match") {
		t.Fatalf("diff output = %q", output.String())
	}
	state, err := loadSyncState()
	if err != nil {
		t.Fatal(err)
	}
	wantHash := contentHash(local)
	if state.Status != "synced" || state.LocalHash != wantHash || state.RemoteHash != wantHash || state.Message != "files match" {
		t.Fatalf("sync state was not refreshed: %#v", state)
	}
}

func TestRepositoryDiffReportsWholeFileDifferences(t *testing.T) {
	local := []byte("# Local description\nalias gs='git status'\n")
	remote := []byte("# Tracked description\nalias gs='git status'\n")
	setupRepositoryDiffTest(t, local, remote)
	if err := writeSyncStatus("conflict", "both local and remote aliases changed; run al diff", contentHash(local), contentHash(remote)); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := showRepositoryDiffTo(&output); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"FILES DIFFER", "comments, metadata, ordering, whitespace, or unparsed syntax differ", "Neither file was changed", "al sync --push"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("diff output does not contain %q: %s", expected, output.String())
		}
	}
	state, err := loadSyncState()
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != "conflict" {
		t.Fatalf("non-identical files cleared conflict state: %#v", state)
	}
}

func TestRepositoryDiffExplainsResolutionByDifferenceType(t *testing.T) {
	tests := []struct {
		name       string
		local      string
		remote     string
		expected   []string
		unexpected []string
	}{
		{
			name:     "repository only",
			local:    "alias keep='true'\n",
			remote:   "alias keep='true'\nalias frog='printf remote'\n",
			expected: []string{"Summary: 0 local-only, 1 repository-only, 0 changed", "REMOTE ONLY frog", "1. Run al sync --pull", "2. Run al diff again", "remaining REMOTE ONLY functions", "3. Run al sync --push"},
		},
		{
			name:       "local only",
			local:      "alias keep='true'\nalias local='printf local'\n",
			remote:     "alias keep='true'\n",
			expected:   []string{"Summary: 1 local-only, 0 repository-only, 0 changed", "LOCAL ONLY  local", "al sync --push", "publish the local-only aliases"},
			unexpected: []string{"al sync --pull"},
		},
		{
			name:     "additions on both sides",
			local:    "alias local='printf local'\n",
			remote:   "alias frog='printf remote'\n",
			expected: []string{"Summary: 1 local-only, 1 repository-only, 0 changed", "To keep aliases from both files", "1. Run al sync --pull", "2. Run al diff again", "remaining REMOTE ONLY functions", "3. Run al sync --push"},
		},
		{
			name:     "changed command",
			local:    "alias keep='printf local'\n",
			remote:   "alias keep='printf remote'\nalias frog='printf remote'\n",
			expected: []string{"Summary: 0 local-only, 1 repository-only, 1 changed", "CHANGED    keep", "cannot choose between commands", "Copy any REMOTE ONLY aliases", "al diff again", "al sync --push"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupRepositoryDiffTest(t, []byte(test.local), []byte(test.remote))
			var output bytes.Buffer
			if err := showRepositoryDiffTo(&output); err != nil {
				t.Fatal(err)
			}
			result := output.String()
			for _, expected := range append([]string{"Alias files differ", "Neither file was changed", "active:", "repository:"}, test.expected...) {
				if !strings.Contains(result, expected) {
					t.Fatalf("diff output does not contain %q: %s", expected, result)
				}
			}
			for _, unexpected := range test.unexpected {
				if strings.Contains(result, unexpected) {
					t.Fatalf("diff output unexpectedly contains %q: %s", unexpected, result)
				}
			}
		})
	}
}

func setupRepositoryDiffTest(t *testing.T, local, remote []byte) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
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
