package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	neutralcatalog "alias-lens/internal/catalog"
	workflowstate "alias-lens/internal/state"
)

func TestStatusIsObservational(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configDirectory := filepath.Join(home, ".config", "alias-lens")
	if err := os.MkdirAll(configDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	config := []byte(`{"version":1,"repository":"","alias_file":".bash_aliases","shell":"bash","providers":{},"auto_sync":{"enabled":false,"interval_seconds":15},"tracked_files":[]}`)
	configPath := filepath.Join(configDirectory, "config.json")
	if err := os.WriteFile(configPath, config, 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	report := inspectWorkflowStatus()
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("status changed config.json")
	}
	if _, err := os.Stat(configPath + ".alias-lens.bak"); !os.IsNotExist(err) {
		t.Fatalf("status created a backup: %v", err)
	}
	if report.Config.State != workflowstate.ConfigInvalid {
		t.Fatalf("config state = %s", report.Config.State)
	}
	if _, err := os.Stat(filepath.Join(configDirectory, "catalog.json")); !os.IsNotExist(err) {
		t.Fatalf("status created a catalog: %v", err)
	}
}

func TestStatusReportsCatalogWithoutLeakingCommands(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	directory := filepath.Join(home, ".config", "alias-lens")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	secret := "printf private-command-canary"
	value := neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{{
		ID: "11111111111111111111111111111111", Name: "private", Kind: "command",
		Native: map[string]neutralcatalog.NativeImplementation{"bash": {AliasValue: &secret}},
	}}}
	encoded, diagnostics := neutralcatalog.Encode(value)
	if len(diagnostics) > 0 {
		t.Fatalf("encode diagnostics: %#v", diagnostics)
	}
	if err := os.WriteFile(filepath.Join(directory, "catalog.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	report := inspectWorkflowStatus()
	output, err := workflowstate.EncodeJSON(report)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(output, []byte("private-command-canary")) {
		t.Fatalf("status leaked command: %s", output)
	}
	sum := sha256.Sum256(encoded)
	if report.Catalog.CatalogSHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("catalog hash = %q", report.Catalog.CatalogSHA256)
	}
}

func TestStatusReportsCorruptStateAsUnreadable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configDirectory := filepath.Join(home, ".config", "alias-lens")
	stateDirectory := filepath.Join(home, ".local", "state", "alias-lens")
	if err := os.MkdirAll(configDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(stateDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	value := neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{}}
	encoded, _ := neutralcatalog.Encode(value)
	if err := os.WriteFile(filepath.Join(configDirectory, "catalog.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDirectory, "catalog-state.json"), []byte(`{"schema_version":99}`), 0o600); err != nil {
		t.Fatal(err)
	}
	report := inspectWorkflowStatus()
	if len(report.Shells) != 1 || report.Shells[0].State != workflowstate.ShellUnreadable {
		t.Fatalf("shell status = %#v", report.Shells)
	}
}

func TestStatusReadsEnrolledCatalogInsideRepository(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repository := filepath.Join(home, "dotfiles")
	if err := os.MkdirAll(filepath.Join(repository, "custom"), 0o700); err != nil {
		t.Fatal(err)
	}
	config := defaultConfig()
	config.Repository = repository
	if err := saveConfig(config); err != nil {
		t.Fatal(err)
	}
	value := neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{}}
	encoded, diagnostics := neutralcatalog.Encode(value)
	if len(diagnostics) > 0 {
		t.Fatalf("encode catalog: %#v", diagnostics)
	}
	if err := os.WriteFile(localCatalogPath(), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "custom", "catalog.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	stateDirectory := filepath.Join(home, ".local", "state", "alias-lens")
	if err := os.MkdirAll(stateDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	hash := hashBytes(encoded)
	state := []byte(`{"schema_version":1,"installed_shells":{"bash":{}},"catalog_sync":{"repository_path":"custom/catalog.json","base_catalog_sha256":"` + hash + `","local_catalog_sha256":"` + hash + `","remote_catalog_sha256":"` + hash + `"}}`)
	if err := os.WriteFile(filepath.Join(stateDirectory, "catalog-state.json"), state, 0o600); err != nil {
		t.Fatal(err)
	}
	report := inspectWorkflowStatus()
	if report.Sync.State != workflowstate.SyncClean {
		t.Fatalf("sync status = %#v", report.Sync)
	}
}
