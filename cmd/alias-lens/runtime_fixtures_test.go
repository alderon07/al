package main

import (
	"alias-lens/internal/app"
	"alias-lens/internal/catalog"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testShellAdapter(name string) app.ShellAdapter {
	adapter, err := applicationServices().ShellAdapter(name)
	if err != nil {
		panic(err)
	}
	return adapter
}

func seedRevisionFixture(path string, contents []byte) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	directory := filepath.Join(home, ".local", "share", "alias-lens", "revisions")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	name := time.Now().UTC().Format("20060102T150405.000000000Z") + filepath.Base(path)
	return os.WriteFile(filepath.Join(directory, name), contents, 0o600)
}

func seedTourFixture() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	directory := filepath.Join(home, ".local", "state", "alias-lens")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, "tour-state"), []byte("pending\n"), 0o600)
}

func contextRankingFixture(t *testing.T, directory, repository string) app.ContextRanking {
	t.Helper()
	if repository != "" {
		if _, err := os.Stat(filepath.Join(repository, ".git")); os.IsNotExist(err) {
			runGit(t, repository, "init", "-q")
		}
	}
	original := applicationDependencies.WorkingDir
	applicationDependencies.WorkingDir = func() (string, error) { return directory, nil }
	t.Cleanup(func() { applicationDependencies.WorkingDir = original })
	t.Setenv("ALIAS_LENS_SHELL", "bash")
	ranking, err := applicationServices().CurrentContextRanking()
	if err != nil {
		t.Fatal(err)
	}
	return ranking
}

func seedContextFixture(t *testing.T, alias Alias, kind, path string) {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(home, ".config", "alias-lens", "contexts.json")
	var saved struct {
		Version  int                 `json:"version"`
		Bindings []map[string]string `json:"bindings"`
	}
	if contents, err := os.ReadFile(destination); err == nil {
		if err := json.Unmarshal(contents, &saved); err != nil {
			t.Fatal(err)
		}
	}
	digest := sha256.Sum256([]byte(alias.Command))
	kindName := alias.Type
	if kindName == "" {
		kindName = "alias"
	}
	saved.Version = 1
	saved.Bindings = append(saved.Bindings, map[string]string{"shell": "bash", "name": alias.Name, "type": kindName, "command_sha256": fmt.Sprintf("%x", digest), "kind": kind, "path": path})
	contents, err := json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, contents, 0o600); err != nil {
		t.Fatal(err)
	}
}

func catalogPathFixture() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "alias-lens", "catalog.json")
}

func contextPathFixture() (string, error) {
	return filepath.Join(os.Getenv("HOME"), ".config", "alias-lens", "contexts.json"), nil
}

func aliasPathFixture(adapter app.ShellAdapter) (string, error) {
	return filepath.Join(os.Getenv("HOME"), adapter.AliasFilename()), nil
}

func readCatalogFixture(path string) (catalog.Catalog, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return catalog.Catalog{}, err
	}
	value, problems := catalog.Decode(contents)
	if len(problems) != 0 {
		return catalog.Catalog{}, fmt.Errorf("invalid synthetic catalog: %v", problems)
	}
	return value, nil
}

func seedSyncStateFixture(status, message, localHash, remoteHash string) error {
	state := app.SyncState{Status: status, Message: message, LocalHash: localHash, RemoteHash: remoteHash, UpdatedAt: time.Now()}
	contents, err := json.Marshal(state)
	if err != nil {
		return err
	}
	root := filepath.Join(os.Getenv("HOME"), ".local", "state", "alias-lens")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "sync-state.json"), contents, 0o600)
}

func repositoryPathsFixture() (AppConfig, string, string, error) {
	config, err := loadConfig()
	if err != nil {
		return AppConfig{}, "", "", err
	}
	return config, filepath.Join(os.Getenv("HOME"), config.AliasFile), filepath.Join(config.Repository, config.AliasFile), nil
}

func primarySyncStateFixture() (app.SyncState, error) {
	snapshot, err := applicationServices().SyncStatus()
	return snapshot.Primary.State, err
}

func syncStatePathFixture(name string) (string, error) {
	return filepath.Join(os.Getenv("HOME"), ".local", "state", "alias-lens", name), nil
}

func trackedStatePathFixture(tracked TrackedFileConfig) (string, error) {
	digest := sha256.Sum256([]byte(tracked.Source + "\x00" + tracked.RepositoryPath))
	return syncStatePathFixture("file-" + fmt.Sprintf("%x", digest)[:16] + ".json")
}
