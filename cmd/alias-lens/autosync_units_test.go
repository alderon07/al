//go:build !windows

package main

import (
	"github.com/alderon07/al/internal/catalogstore"

	"encoding/json"
	"github.com/alderon07/al/internal/transaction"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatalogOnlyAutosyncEnableAndWatchOnce(t *testing.T) {
	setupCatalogSyncFixture(t)
	config := defaultConfig()
	if e := saveConfig(config); e != nil {
		t.Fatal(e)
	}
	prior := applicationDependencies.StartWatcher
	applicationDependencies.StartWatcher = func() error { return nil }
	t.Cleanup(func() { applicationDependencies.StartWatcher = prior })
	native := filepath.Join(os.Getenv("HOME"), ".bash_aliases")
	if e := os.WriteFile(native, []byte("alias localonly='printf synthetic'\n"), 0o600); e != nil {
		t.Fatal(e)
	}
	writeAutosyncInstalledFixture(t, "bash")
	before, _ := os.ReadFile(native)
	catalogBefore, _ := os.ReadFile(catalogPathFixture())
	if e := runAutoSyncCommand([]string{"enable"}); e != nil {
		t.Fatal(e)
	}
	actual, e := loadConfig()
	if e != nil || !actual.AutoSync.Enabled || actual.Repository != "" {
		t.Fatal("catalog-only configuration rejected", e)
	}
	if e := applicationServices().RunWatch(false); e != nil {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(native)
	catalogAfter, _ := os.ReadFile(catalogPathFixture())
	if string(after) != string(before) || string(catalogAfter) != string(catalogBefore) {
		t.Fatal("watch activated catalog or changed fallback")
	}
	state, e := primarySyncStateFixture()
	if e != nil || state.Status != "synced" {
		t.Fatal(state, e)
	}
	if _, e := os.Stat(filepath.Join(os.Getenv("HOME"), ".local", "state", "alias-lens", "sync.lock")); !os.IsNotExist(e) {
		t.Fatal("legacy age lock created", e)
	}
}

func stringTrimGit(t *testing.T, repo string, args ...string) string {
	return strings.TrimSpace(runGit(t, repo, args...))
}

func writeAutosyncInstalledFixture(t *testing.T, shell string) {
	t.Helper()
	root := filepath.Join(os.Getenv("HOME"), ".local", "state", "alias-lens")
	if e := os.MkdirAll(root, 0o700); e != nil {
		t.Fatal(e)
	}
	file := catalogstore.InstalledFile{Version: 1, Records: []catalogstore.InstalledState{{Shell: shell, Renderer: shell + "/v2", GenerationID: strings.Repeat("a", 64)}}}
	data, e := catalogstore.Encode(file)
	if e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(root, "catalog-installed.json"), data, 0o600); e != nil {
		t.Fatal(e)
	}
}

func TestCatalogOnlyWatchProcessRefusesHeldMutationLock(t *testing.T) {
	setupCatalogSyncFixture(t)
	config := defaultConfig()
	config.AutoSync.Enabled = true
	if e := saveConfig(config); e != nil {
		t.Fatal(e)
	}
	root := filepath.Join(os.Getenv("HOME"), ".local", "state", "alias-lens")
	lock, e := transaction.AcquireLock(root, filepath.Join(root, "mutation.lock"))
	if e != nil {
		t.Fatal(e)
	}
	defer lock.Close()
	before := mutationInventory(t, os.Getenv("HOME"))
	args, _ := json.Marshal([]string{"watch"})
	command := exec.Command(os.Args[0], "-test.run=^TestMutationCommandProcess$")
	command.Env = append(os.Environ(), "AL_MUTATION_PROCESS=1", "AL_MUTATION_ARGS="+string(args))
	out, e := command.CombinedOutput()
	if e == nil || !strings.Contains(string(out), "lock") {
		t.Fatal("catalog-only watch ignored mutation lock", e, string(out))
	}
	if after := mutationInventory(t, os.Getenv("HOME")); after != before {
		t.Fatal("catalog watch changed files while locked")
	}
}
