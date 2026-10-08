package main

import (
	"alias-lens/internal/catalog"
	"alias-lens/internal/catalogstore"

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
	record := catalogstore.CatalogSyncRecord{Version: 1, Repository: repo, CatalogPath: "catalog.json", BaseBytes: string(bytes), BaseSHA256: applicationServices().HashBytes(bytes), SourceSHA256: applicationServices().HashBytes(bytes), HEAD: strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD")), Blob: strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD:catalog.json"))}
	encoded, err := catalogstore.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Dir(filepath.Join(filepath.Dir(catalogPathFixture()), "catalog-sync.json")), 0700)
	os.WriteFile(filepath.Join(filepath.Dir(catalogPathFixture()), "catalog-sync.json"), encoded, 0600)
	writeCatalogFixture(t, catalogPathFixture(), value)
	return repo, value
}
