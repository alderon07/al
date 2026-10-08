//go:build !windows

package main

import (
	"alias-lens/internal/catalogstore"
	"alias-lens/internal/transaction"
	"encoding/json"
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
	prior := watchProcessLaunch
	watchProcessLaunch = func(*mutationSession) error { return nil }
	t.Cleanup(func() { watchProcessLaunch = prior })
	native := filepath.Join(homeDirectory(), ".bash_aliases")
	if e := os.WriteFile(native, []byte("alias localonly='printf synthetic'\n"), 0o600); e != nil {
		t.Fatal(e)
	}
	writeAutosyncInstalledFixture(t, "bash")
	before, _ := os.ReadFile(native)
	catalogBefore, _ := os.ReadFile(localCatalogPath())
	if e := runAutoSyncCommand([]string{"enable"}); e != nil {
		t.Fatal(e)
	}
	actual, e := loadConfig()
	if e != nil || !actual.AutoSync.Enabled || actual.Repository != "" {
		t.Fatal("catalog-only configuration rejected", e)
	}
	if e := runWatch(false); e != nil {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(native)
	catalogAfter, _ := os.ReadFile(localCatalogPath())
	if string(after) != string(before) || string(catalogAfter) != string(catalogBefore) {
		t.Fatal("watch activated catalog or changed fallback")
	}
	state, e := loadSyncState()
	if e != nil || state.Status != "synced" {
		t.Fatal(state, e)
	}
	if _, e := os.Stat(filepath.Join(homeDirectory(), ".local", "state", "alias-lens", "sync.lock")); !os.IsNotExist(e) {
		t.Fatal("legacy age lock created", e)
	}
}
func TestAutosyncMixedCatalogAndLegacyUnits(t *testing.T) {
	catalogRepo, _ := setupCatalogSyncFixture(t)
	home := homeDirectory()
	t.Setenv(activeShellEnvironment, "zsh")
	zshPath := filepath.Join(home, ".zsh_aliases")
	zshBefore := []byte("alias zshonly='printf zshsynthetic'\n")
	if e := os.WriteFile(zshPath, zshBefore, 0o600); e != nil {
		t.Fatal(e)
	}
	config := defaultConfig()
	config.Shell = "bash"
	config.Repository = catalogRepo
	config.AutoSync.Enabled = true
	aliasPath := filepath.Join(home, ".bash_aliases")
	aliases := []byte("alias fixture='printf synthetic'\n")
	if e := os.WriteFile(aliasPath, aliases, 0o600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(catalogRepo, ".bash_aliases"), aliases, 0o600); e != nil {
		t.Fatal(e)
	}
	tracked := filepath.Join(home, "notes.txt")
	if e := os.WriteFile(tracked, []byte("synthetic notes\n"), 0o600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(catalogRepo, "notes.txt"), []byte("synthetic notes\n"), 0o600); e != nil {
		t.Fatal(e)
	}
	config.TrackedFiles = []TrackedFileConfig{{Source: tracked, RepositoryPath: "notes.txt"}}
	runGit(t, catalogRepo, "add", "--", ".bash_aliases", "notes.txt")
	runGit(t, catalogRepo, "commit", "-qm", "synthetic native")
	upstream := filepath.Join(home, "upstream.git")
	runGit(t, home, "init", "--bare", "-q", upstream)
	runGit(t, catalogRepo, "remote", "add", "origin", upstream)
	runGit(t, catalogRepo, "push", "-qu", "origin", "HEAD")
	record, _, e := readCatalogSyncRecord()
	if e != nil {
		t.Fatal(e)
	}
	record.HEAD = stringTrimGit(t, catalogRepo, "rev-parse", "HEAD")
	encoded, e := catalogstore.Encode(record)
	if e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(catalogSyncPath(), encoded, 0o600); e != nil {
		t.Fatal(e)
	}
	if e := saveConfig(config); e != nil {
		t.Fatal(e)
	}
	writeAutosyncInstalledFixture(t, "zsh")
	catalogUnit, nativeUnit, e := autosyncUnits(config)
	if e != nil || !catalogUnit || !nativeUnit {
		t.Fatal("mixed units not selected", e)
	}
	if e := runWatch(false); e != nil {
		t.Fatal(e)
	}
	zshAfter, e := os.ReadFile(zshPath)
	if e != nil || string(zshAfter) != string(zshBefore) {
		t.Fatal("configured Bash cycle changed Zsh fallback", e)
	}
	state, e := loadSyncState()
	if e != nil || state.Status != "synced" {
		t.Fatal("native unit did not run", state, e)
	}
	path, e := trackedStatePath(config.TrackedFiles[0])
	if e != nil {
		t.Fatal(e)
	}
	trackedState, e := loadSyncStateAt(path)
	if e != nil || trackedState.Status != "synced" {
		t.Fatal("tracked unit did not run", trackedState, e)
	}
}
func stringTrimGit(t *testing.T, repo string, args ...string) string {
	return strings.TrimSpace(runGit(t, repo, args...))
}

func writeAutosyncInstalledFixture(t *testing.T, shell string) {
	t.Helper()
	root := filepath.Join(homeDirectory(), ".local", "state", "alias-lens")
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
func TestAutosyncRefusesTrackedCatalogFallbackBeforeAnyUnitWrites(t *testing.T) {
	repo, _ := setupCatalogSyncFixture(t)
	home := homeDirectory()
	config := defaultConfig()
	config.AutoSync.Enabled = true
	config.Repository = repo
	native := filepath.Join(home, ".bash_aliases")
	if e := os.WriteFile(native, []byte("alias fixture='printf synthetic'\n"), 0o600); e != nil {
		t.Fatal(e)
	}
	config.TrackedFiles = []TrackedFileConfig{{Source: native, RepositoryPath: "fallback.txt"}}
	if e := saveConfig(config); e != nil {
		t.Fatal(e)
	}
	writeAutosyncInstalledFixture(t, "bash")
	before := mutationInventory(t, home)
	if e := runWatch(false); e == nil || !strings.Contains(e.Error(), "remove it from config.json") {
		t.Fatal("stale fallback registry accepted", e)
	}
	if after := mutationInventory(t, home); after != before {
		t.Fatal("stale fallback policy changed files")
	}
}
func TestCatalogOnlyWatchProcessRefusesHeldMutationLock(t *testing.T) {
	setupCatalogSyncFixture(t)
	config := defaultConfig()
	config.AutoSync.Enabled = true
	if e := saveConfig(config); e != nil {
		t.Fatal(e)
	}
	root := filepath.Join(homeDirectory(), ".local", "state", "alias-lens")
	lock, e := transaction.AcquireLock(root, filepath.Join(root, "mutation.lock"))
	if e != nil {
		t.Fatal(e)
	}
	defer lock.Close()
	before := mutationInventory(t, homeDirectory())
	args, _ := json.Marshal([]string{"watch"})
	command := exec.Command(os.Args[0], "-test.run=^TestMutationCommandProcess$")
	command.Env = append(os.Environ(), "AL_MUTATION_PROCESS=1", "AL_MUTATION_ARGS="+string(args))
	out, e := command.CombinedOutput()
	if e == nil || !strings.Contains(string(out), "lock") {
		t.Fatal("catalog-only watch ignored mutation lock", e, string(out))
	}
	if after := mutationInventory(t, homeDirectory()); after != before {
		t.Fatal("catalog watch changed files while locked")
	}
}

func TestNewAliasRefusesMissingUserParents(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	path := filepath.Join(home, "missing", ".bash_aliases")
	if e := writeNewAliasFile(path, []byte("alias fixture='printf synthetic'\n")); e == nil {
		t.Fatal("missing user parent created implicitly")
	}
	if _, e := os.Stat(filepath.Dir(path)); !os.IsNotExist(e) {
		t.Fatal("user parent changed", e)
	}
}
func TestTrackedReplacementPreservesLeafModeAndPrivateBackup(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	target := filepath.Join(home, "notes")
	leaf := filepath.Join(home, "enrolled")
	if e := os.WriteFile(target, []byte("before"), 0o640); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink("notes", leaf); e != nil {
		t.Fatal(e)
	}
	if e := replaceTrackedFile(leaf, []byte("after")); e != nil {
		t.Fatal(e)
	}
	link, e := os.Readlink(leaf)
	if e != nil || link != "notes" {
		t.Fatal("enrolled leaf replaced", e)
	}
	info, e := os.Stat(target)
	if e != nil || info.Mode().Perm() != 0o640 {
		t.Fatal("enrolled mode changed", e)
	}
	data, e := os.ReadFile(leaf + ".alias-lens.bak")
	if e != nil || string(data) != "before" {
		t.Fatal("private backup absent", e)
	}
	info, e = os.Stat(leaf + ".alias-lens.bak")
	if e != nil || info.Mode().Perm() != 0o600 {
		t.Fatal("backup metadata changed", e)
	}
}
