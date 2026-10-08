package main

import (
	"alias-lens/internal/catalog"
	"alias-lens/internal/catalogstore"
	"bytes"
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
	os.MkdirAll(filepath.Dir(catalogSyncPath()), 0700)
	os.WriteFile(catalogSyncPath(), encoded, 0600)
	writeCatalogFixture(t, localCatalogPath(), value)
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
	writeCatalogFixture(t, localCatalogPath(), local)
	writeCatalogFixture(t, filepath.Join(repo, "catalog.json"), remote)
	beforeIndex := runGit(t, repo, "ls-files", "--stage")
	preview, err := buildCatalogSyncPullPlan()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range preview.Actions {
		if a.TargetRole != "catalog" && a.TargetRole != "catalog_sync" && a.TargetRole != "catalog_revision" {
			t.Fatalf("unexpected mutation %s", a.TargetRole)
		}
	}
	if err := applyMutationPlan(preview, buildCatalogSyncPullPlan); err != nil {
		t.Fatal(err)
	}
	combined, err := readCatalogFile(localCatalogPath())
	if err != nil || combined.Entries[0].Description != "local description" || combined.Entries[0].Category != "remote category" {
		t.Fatal("semantic merge", err)
	}
	if beforeIndex != runGit(t, repo, "ls-files", "--stage") {
		t.Fatal("index changed")
	}
	if _, err := os.Stat(filepath.Join(homeDirectory(), ".bash_aliases")); !os.IsNotExist(err) {
		t.Fatal("native fallback changed")
	}
}
func TestCatalogPullConflictAndStalePreview(t *testing.T) {
	repo, value := setupCatalogSyncFixture(t)
	value.Entries[0].Description = "remote"
	writeCatalogFixture(t, filepath.Join(repo, "catalog.json"), value)
	value.Entries[0].Description = "local"
	writeCatalogFixture(t, localCatalogPath(), value)
	before, _ := os.ReadFile(localCatalogPath())
	preview, err := buildCatalogSyncPullPlan()
	if err != nil || preview.Operation != "catalog.sync.conflict" {
		t.Fatal("missing conflict", err)
	}
	if err := applyMutationPlan(preview, buildCatalogSyncPullPlan); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(localCatalogPath())
	if !bytes.Equal(before, after) {
		t.Fatal("conflict overwrote catalog")
	}
	info, err := os.Stat(preview.Actions[0].Target.Path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("conflict not private", err)
	}
	value.Entries[0].Description = "changed"
	writeCatalogFixture(t, localCatalogPath(), value)
	if applyMutationPlan(preview, buildCatalogSyncPullPlan) == nil {
		t.Fatal("stale preview accepted")
	}
}

func TestCatalogAutoInspectIsReadOnly(t *testing.T) {
	repo, value := setupCatalogSyncFixture(t)
	before, _ := os.ReadFile(localCatalogPath())
	state, _ := os.ReadFile(catalogSyncPath())
	value.Entries[0].Description = "remote"
	writeCatalogFixture(t, filepath.Join(repo, "catalog.json"), value)
	if inspectCatalogSync() == nil {
		t.Fatal("remote change undetected")
	}
	after, _ := os.ReadFile(localCatalogPath())
	next, _ := os.ReadFile(catalogSyncPath())
	if !bytes.Equal(before, after) || !bytes.Equal(state, next) {
		t.Fatal("auto inspect mutated state")
	}
}
