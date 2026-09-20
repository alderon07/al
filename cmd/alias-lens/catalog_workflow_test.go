package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	neutralcatalog "alias-lens/internal/catalog"
)

func TestCatalogDiffIsSemanticRedactedAndReadOnly(t *testing.T) {
	home := t.TempDir()
	repository := filepath.Join(home, "repo")
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(repository, "alias-lens"), 0o700); err != nil {
		t.Fatal(err)
	}
	config := defaultConfig()
	config.Repository = repository
	if err := saveConfig(config); err != nil {
		t.Fatal(err)
	}
	oldSecret := "printf old-private-canary"
	newSecret := "printf new-private-canary"
	before := neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{{
		ID: "11111111111111111111111111111111", Name: "private", Kind: "command",
		Native: map[string]neutralcatalog.NativeImplementation{"bash": {AliasValue: &oldSecret}},
	}}}
	after := before
	after.Entries = append([]neutralcatalog.Entry(nil), before.Entries...)
	after.Entries[0].Native = map[string]neutralcatalog.NativeImplementation{"bash": {AliasValue: &newSecret}}
	writeCatalogFixture(t, filepath.Join(repository, "alias-lens", "catalog.json"), before)
	localPath := filepath.Join(home, ".config", "alias-lens", "catalog.json")
	writeCatalogFixture(t, localPath, after)
	localBefore, _ := os.ReadFile(localPath)
	remoteBefore, _ := os.ReadFile(filepath.Join(repository, "alias-lens", "catalog.json"))
	var output bytes.Buffer
	previous := catalogWorkflowStdout
	catalogWorkflowStdout = &output
	t.Cleanup(func() { catalogWorkflowStdout = previous })
	code, err := runCatalogDiff([]string{"--json"})
	if err != nil || code != 1 {
		t.Fatalf("runCatalogDiff = %d, %v", code, err)
	}
	if strings.Contains(output.String(), "private-canary") || !strings.Contains(output.String(), `"path": "native.bash"`) {
		t.Fatalf("redacted diff output = %s", output.String())
	}
	localAfter, _ := os.ReadFile(localPath)
	remoteAfter, _ := os.ReadFile(filepath.Join(repository, "alias-lens", "catalog.json"))
	if !bytes.Equal(localBefore, localAfter) || !bytes.Equal(remoteBefore, remoteAfter) {
		t.Fatal("catalog diff changed a catalog")
	}
}

func TestCatalogDiffPlainUsesFriendlyFields(t *testing.T) {
	before := neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{}}
	after := neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{}}
	report := neutralcatalog.SemanticDiffReport{
		SchemaVersion: neutralcatalog.SemanticDiffSchemaVersion,
		Source:        "repository",
		Changes:       []neutralcatalog.SemanticChange{{Scope: "catalog", Kind: "changed", Path: "schema_version"}},
		Summary:       neutralcatalog.SemanticSummary{Changed: 1},
	}
	output := string(renderCatalogDiffPlain(report, before, after))
	if !strings.Contains(output, "data format") || strings.Contains(output, "schema_version") {
		t.Fatalf("plain output = %q", output)
	}
}

func TestInstalledCatalogSnapshotMustMatchRecordedFingerprint(t *testing.T) {
	home := t.TempDir()
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
	if _, err := readInstalledCatalogSnapshot("bash"); err == nil || !strings.Contains(err.Error(), "saved fingerprint") {
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
