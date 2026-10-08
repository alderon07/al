package app

import (
	"bytes"
	"github.com/alderon07/al/internal/catalog"
	"github.com/alderon07/al/internal/catalogstore"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupCatalogSyncFixture(t *testing.T) (string, catalog.Catalog) {
	t.Helper()
	home := t.TempDir()
	os.Chmod(home, 0700)
	t.Setenv("HOME", home)
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	repo := filepath.Join(home, "repo")
	os.MkdirAll(repo, 0700)
	runGit(t, repo, "init", "-q")
	runGit(t, repo, "config", "user.name", "Synthetic")
	runGit(t, repo, "config", "user.email", "synthetic@example.invalid")
	value := catalog.Catalog{SchemaVersion: 2, Entries: []catalog.Entry{{ID: "11111111111111111111111111111111", Name: "demo", Kind: "command", Portable: &catalog.Portable{Program: "printf", Args: []string{"synthetic"}, PassArguments: true}}}}
	writeCatalogFixture(t, filepath.Join(repo, "catalog.json"), value)
	runGit(t, repo, "add", "--", "catalog.json")
	runGit(t, repo, "commit", "-qm", "synthetic")
	bytes, _ := catalog.Encode(value)
	record := catalogstore.CatalogSyncRecord{Version: 1, Repository: repo, CatalogPath: "catalog.json", BaseBytes: string(bytes), BaseSHA256: hashBytes(bytes), SourceSHA256: hashBytes(bytes), HEAD: strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD")), Blob: strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD:catalog.json"))}
	encoded, err := catalogstore.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Dir(DefaultServices().catalogSyncPath()), 0700)
	os.WriteFile(DefaultServices().catalogSyncPath(), encoded, 0600)
	writeCatalogFixture(t, DefaultServices().localCatalogPath(), value)
	return repo, value
}
func TestCatalogSemanticPullDoesNotActivate(t *testing.T) {
	repo, value := setupCatalogSyncFixture(t)
	local := value
	local.Entries = append([]catalog.Entry{}, value.Entries...)
	local.Entries[0].Description = "local description"
	remote := value
	remote.Entries = append([]catalog.Entry{}, value.Entries...)
	remote.Entries[0].Category = "remote category"
	writeCatalogFixture(t, DefaultServices().localCatalogPath(), local)
	writeCatalogFixture(t, filepath.Join(repo, "catalog.json"), remote)
	beforeIndex := runGit(t, repo, "ls-files", "--stage")
	preview, err := DefaultServices().buildCatalogSyncPullPlan()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range preview.Actions {
		if a.TargetRole != "catalog" && a.TargetRole != "catalog_sync" && a.TargetRole != "catalog_revision" {
			t.Fatalf("unexpected mutation %s", a.TargetRole)
		}
	}
	if err := DefaultServices().applyMutationPlan(preview, DefaultServices().buildCatalogSyncPullPlan); err != nil {
		t.Fatal(err)
	}
	combined, err := readCatalogFile(DefaultServices().localCatalogPath())
	if err != nil || combined.Entries[0].Description != "local description" || combined.Entries[0].Category != "remote category" {
		t.Fatal("semantic merge", err)
	}
	if beforeIndex != runGit(t, repo, "ls-files", "--stage") {
		t.Fatal("index changed")
	}
	if _, err := os.Stat(filepath.Join(DefaultServices().homeDirectory(), ".bash_aliases")); !os.IsNotExist(err) {
		t.Fatal("native fallback changed")
	}
}
func TestCatalogPullConflictAndStalePreview(t *testing.T) {
	repo, value := setupCatalogSyncFixture(t)
	value.Entries[0].Description = "remote"
	writeCatalogFixture(t, filepath.Join(repo, "catalog.json"), value)
	value.Entries[0].Description = "local"
	writeCatalogFixture(t, DefaultServices().localCatalogPath(), value)
	before, _ := os.ReadFile(DefaultServices().localCatalogPath())
	preview, err := DefaultServices().buildCatalogSyncPullPlan()
	if err != nil || preview.Operation != "catalog.sync.conflict" {
		t.Fatal("missing conflict", err)
	}
	if err := DefaultServices().applyMutationPlan(preview, DefaultServices().buildCatalogSyncPullPlan); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(DefaultServices().localCatalogPath())
	if !bytes.Equal(before, after) {
		t.Fatal("conflict overwrote catalog")
	}
	info, err := os.Stat(preview.Actions[0].Target.Path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("conflict not private", err)
	}
	value.Entries[0].Description = "changed"
	writeCatalogFixture(t, DefaultServices().localCatalogPath(), value)
	if DefaultServices().applyMutationPlan(preview, DefaultServices().buildCatalogSyncPullPlan) == nil {
		t.Fatal("stale preview accepted")
	}
}

func TestCatalogAutoInspectIsReadOnly(t *testing.T) {
	repo, value := setupCatalogSyncFixture(t)
	before, _ := os.ReadFile(DefaultServices().localCatalogPath())
	state, _ := os.ReadFile(DefaultServices().catalogSyncPath())
	value.Entries[0].Description = "remote"
	writeCatalogFixture(t, filepath.Join(repo, "catalog.json"), value)
	if DefaultServices().inspectCatalogSync() == nil {
		t.Fatal("remote change undetected")
	}
	after, _ := os.ReadFile(DefaultServices().localCatalogPath())
	next, _ := os.ReadFile(DefaultServices().catalogSyncPath())
	if !bytes.Equal(before, after) || !bytes.Equal(state, next) {
		t.Fatal("auto inspect mutated state")
	}
}
