package app

import (
	"os"
	"path/filepath"

	neutralcatalog "github.com/alderon07/al/internal/catalog"
	"strings"
	"testing"
)

func TestInstalledCatalogSnapshotMustMatchRecordedFingerprint(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	stateDirectory := filepath.Join(home, ".local", "state", "alias-lens")
	snapshotDirectory := filepath.Join(stateDirectory, "catalog-snapshots")
	if err := os.MkdirAll(snapshotDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	original := neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{}}
	originalBytes, _ := neutralcatalog.Encode(original)
	hash := hashBytes(originalBytes)
	changed := neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{{ID: "11111111111111111111111111111111", Name: "gs", Kind: "command", Portable: &neutralcatalog.Portable{Program: "git", Args: []string{"status"}, PassArguments: true}}}}
	changedBytes, _ := neutralcatalog.Encode(changed)
	state := []byte(`{"schema_version":1,"installed_shells":{"bash":{"source_catalog_sha256":"` + hash + `"}}}`)
	if err := os.WriteFile(filepath.Join(stateDirectory, "catalog-state.json"), state, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapshotDirectory, hash+".json"), changedBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := DefaultServices().readInstalledCatalogSnapshot("bash"); err == nil || !strings.Contains(err.Error(), "saved fingerprint") {
		t.Fatalf("snapshot error = %v", err)
	}
}

func writeCatalogFixture(t *testing.T, path string, value neutralcatalog.Catalog) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	contents, diagnostics := neutralcatalog.Encode(value)
	if len(diagnostics) > 0 {
		t.Fatalf("encode diagnostics: %#v", diagnostics)
	}
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
}
