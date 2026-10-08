package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPullReportsRemainingDifferencesWithoutNewAliases(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	bare := filepath.Join(home, "remote.git")
	repository := filepath.Join(home, "dotfiles")
	runGit(t, home, "init", "--bare", bare)
	runGit(t, home, "clone", bare, repository)
	runGit(t, repository, "config", "user.email", "alias-lens@example.test")
	runGit(t, repository, "config", "user.name", "Alias Lens Test")
	remote := []byte("# Repository note\nalias keep='true'\n")
	if err := os.WriteFile(filepath.Join(repository, ".bash_aliases"), remote, 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "add", ".bash_aliases")
	runGit(t, repository, "commit", "-m", "Add aliases")
	runGit(t, repository, "push", "--set-upstream", "origin", "HEAD")
	local := []byte("alias keep='true'\nalias mine='printf local'\n")
	aliasPath := filepath.Join(home, ".bash_aliases")
	if err := os.WriteFile(aliasPath, local, 0o600); err != nil {
		t.Fatal(err)
	}
	config := defaultConfig()
	config.Repository = repository
	if err := saveConfig(config); err != nil {
		t.Fatal(err)
	}

	message, err := pullRepository()
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"imported 0 aliases", "still differ", "al diff"} {
		if !strings.Contains(message, expected) {
			t.Fatalf("pull result does not contain %q: %q", expected, message)
		}
	}
	if after, readErr := os.ReadFile(aliasPath); readErr != nil || string(after) != string(local) {
		t.Fatalf("active aliases = %q, %v", after, readErr)
	}
}

func TestConflictErrorsIdentifyPrivateCopies(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	t.Setenv(activeShellEnvironment, "bash")
	local := []byte("alias keep='printf local'\n")
	remote := []byte("alias frog='printf remote'\n")
	err := saveSyncConflict(local, remote, "both copies changed")
	if err == nil {
		t.Fatal("expected an alias conflict")
	}
	directory, pathErr := syncDataPath("conflicts")
	if pathErr != nil {
		t.Fatal(pathErr)
	}
	for name, contents := range map[string][]byte{"local.bash_aliases": local, "remote.bash_aliases": remote} {
		path := filepath.Join(directory, name)
		if !strings.Contains(err.Error(), path) {
			t.Fatalf("conflict error omits %s: %v", path, err)
		}
		stored, readErr := os.ReadFile(path)
		if readErr != nil || string(stored) != string(contents) {
			t.Fatalf("conflict copy %s = %q, %v", path, stored, readErr)
		}
		info, statErr := os.Stat(path)
		if statErr != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("conflict copy permissions = %v, %v", info, statErr)
		}
	}
	state, stateErr := loadSyncState()
	if stateErr != nil || !strings.Contains(state.Message, filepath.Join(directory, "remote.bash_aliases")) {
		t.Fatalf("sync status omits recovery copy: %#v, %v", state, stateErr)
	}

	tracked := TrackedFileConfig{Source: filepath.Join(t.TempDir(), "settings.toml"), RepositoryPath: "settings.toml"}
	trackedPath, pathErr := trackedStatePath(tracked)
	if pathErr != nil {
		t.Fatal(pathErr)
	}
	err = saveTrackedConflict(tracked, []byte("local\n"), []byte("remote\n"), trackedPath)
	if err == nil {
		t.Fatal("expected a tracked file conflict")
	}
	id := contentHash([]byte(tracked.Source + "\x00" + tracked.RepositoryPath))[:16]
	for _, name := range []string{"local", "remote"} {
		path := filepath.Join(directory, id, name)
		if !strings.Contains(err.Error(), path) {
			t.Fatalf("tracked conflict error omits %s: %v", path, err)
		}
	}
	trackedState, stateErr := loadSyncStateAt(trackedPath)
	if stateErr != nil || !strings.Contains(trackedState.Message, filepath.Join(directory, id, "remote")) {
		t.Fatalf("tracked sync status omits recovery copy: %#v, %v", trackedState, stateErr)
	}
}

func TestAutoSyncStatusReportsInvalidState(t *testing.T) {
	t.Run("primary", func(t *testing.T) {
		t.Setenv("HOME", privateTestHome(t))
		path, err := syncDataPath("sync-state.json")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		err = runAutoSyncCommand([]string{"status"})
		if err == nil || !strings.Contains(err.Error(), "read automatic sync status") || !strings.Contains(err.Error(), path) {
			t.Fatalf("invalid primary status error = %v", err)
		}
	})

	t.Run("tracked", func(t *testing.T) {
		home := privateTestHome(t)
		t.Setenv("HOME", home)
		tracked := TrackedFileConfig{Source: filepath.Join(home, "settings.toml"), RepositoryPath: "settings.toml"}
		config := defaultConfig()
		config.TrackedFiles = []TrackedFileConfig{tracked}
		if err := saveConfig(config); err != nil {
			t.Fatal(err)
		}
		path, err := trackedStatePath(tracked)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		err = runAutoSyncCommand([]string{"status"})
		if err == nil || !strings.Contains(err.Error(), "read tracked sync status") || !strings.Contains(err.Error(), path) {
			t.Fatalf("invalid tracked status error = %v", err)
		}
	})
}
