package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	neutralcatalog "alias-lens/internal/catalog"
	workflowplan "alias-lens/internal/plan"
)

func TestProfilePlanDoesNotWriteAndNamesAffectedEntries(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	directory := filepath.Join(home, ".config", "alias-lens")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	configBytes := []byte(`{"version":2,"repository":"","alias_file":".bash_aliases","shell":"bash","providers":{},"auto_sync":{"enabled":false,"interval_seconds":15}}`)
	configFile := filepath.Join(directory, "config.json")
	if err := os.WriteFile(configFile, configBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	catalog := neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{{
		ID: "11111111111111111111111111111111", Name: "work", Kind: "command",
		When:     &neutralcatalog.Conditions{ProfilesAny: []string{"work"}},
		Portable: &neutralcatalog.Portable{Program: "git", Args: []string{"status"}, PassArguments: true},
	}}}
	encoded, diagnostics := neutralcatalog.Encode(catalog)
	if len(diagnostics) > 0 {
		t.Fatalf("encode diagnostics: %#v", diagnostics)
	}
	if err := os.WriteFile(filepath.Join(directory, "catalog.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(configFile)
	value, err := buildProfilePlan("add", "work")
	if err != nil {
		t.Fatal(err)
	}
	if len(value.Actions) != 1 || value.Actions[0].Kind != workflowplan.ActionReplace || value.Actions[0].Risk != workflowplan.RiskReview {
		t.Fatalf("plan = %#v", value)
	}
	if !bytes.Contains([]byte(value.Actions[0].Reason), []byte("work (bash and zsh)")) {
		t.Fatalf("reason = %q", value.Actions[0].Reason)
	}
	after, _ := os.ReadFile(configFile)
	if !bytes.Equal(before, after) {
		t.Fatal("planning changed the configuration")
	}
}

func TestCatalogMigrationPlanPreservesEntries(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	directory := filepath.Join(home, ".config", "alias-lens")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	value := neutralcatalog.Catalog{SchemaVersion: 1, Entries: []neutralcatalog.Entry{{
		ID: "11111111111111111111111111111111", Name: "gs", Kind: "command",
		Portable: &neutralcatalog.Portable{Program: "git", Args: []string{"status"}, PassArguments: true},
	}}}
	encoded, _ := neutralcatalog.Encode(value)
	path := filepath.Join(directory, "catalog.json")
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	preview, err := buildCatalogMigrationPlan()
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Actions) != 2 || preview.Actions[0].TargetRole != "catalog_revision" || preview.Actions[0].Kind != workflowplan.ActionCreate || !bytes.Equal(preview.Actions[0].Target.PlannedBytes, encoded) || !preview.Actions[1].Backup || !preview.Actions[1].Reversible {
		t.Fatalf("plan = %#v", preview)
	}
	migrated, diagnostics := neutralcatalog.Decode(preview.Actions[1].Target.PlannedBytes)
	if len(diagnostics) > 0 || migrated.SchemaVersion != 2 || len(migrated.Entries) != 1 || migrated.Entries[0].ID != value.Entries[0].ID {
		t.Fatalf("migrated = %#v, %#v", migrated, diagnostics)
	}
	current, _ := os.ReadFile(path)
	if !bytes.Equal(current, encoded) {
		t.Fatal("planning changed the catalog")
	}
}

func TestCatalogMigrationWritesExactDatedRevision(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	directory := filepath.Join(home, ".config", "alias-lens")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	value := neutralcatalog.Catalog{SchemaVersion: 1, Entries: []neutralcatalog.Entry{{
		ID: "11111111111111111111111111111111", Name: "gs", Kind: "command",
		Portable: &neutralcatalog.Portable{Program: "git", Args: []string{"status"}, PassArguments: true},
	}}}
	original, _ := neutralcatalog.Encode(value)
	path := filepath.Join(directory, "catalog.json")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	originalOutput := catalogWorkflowStdout
	catalogWorkflowStdout = &bytes.Buffer{}
	t.Cleanup(func() { catalogWorkflowStdout = originalOutput })
	if code, err := runCatalogMigrationCommand([]string{"--to", "2"}); err != nil || code != 0 {
		t.Fatalf("migrate code=%d err=%v", code, err)
	}
	revisions, err := filepath.Glob(filepath.Join(directory, "catalog.revision-*.json"))
	if err != nil || len(revisions) != 1 {
		t.Fatalf("revisions = %#v, %v", revisions, err)
	}
	revision, err := os.ReadFile(revisions[0])
	if err != nil || !bytes.Equal(revision, original) {
		t.Fatalf("revision differs: %q, %v", revision, err)
	}
}

func TestProfileCommandAppliesThroughTransaction(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := saveConfig(defaultConfig()); err != nil {
		t.Fatal(err)
	}
	if err := runConfigProfileCommand([]string{"add", "work"}); err != nil {
		t.Fatal(err)
	}
	observed, err := observeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(observed.Config.Profiles) != 1 || observed.Config.Profiles[0] != "work" {
		t.Fatalf("profiles = %#v", observed.Config.Profiles)
	}
	configDirectory := filepath.Join(home, ".config", "alias-lens")
	entries, err := os.ReadDir(filepath.Join(configDirectory, "transactions"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("committed journal was not cleaned up: %#v", entries)
	}
	backups, err := os.ReadDir(filepath.Join(configDirectory, "backups"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("private backup count = %d, err %v", len(backups), err)
	}
}

func TestApplyPrivatePlanRejectsChangedPreview(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := saveConfig(defaultConfig()); err != nil {
		t.Fatal(err)
	}
	preview, err := buildProfilePlan("add", "work")
	if err != nil {
		t.Fatal(err)
	}
	config, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	config.Footer.Message = "Changed elsewhere"
	if err := saveConfig(config); err != nil {
		t.Fatal(err)
	}
	path, _ := configPath()
	err = applyPrivatePlan(filepath.Dir(path), preview, func() (workflowplan.OperationPlan, error) {
		return buildProfilePlan("add", "work")
	})
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("changed after the preview")) {
		t.Fatalf("apply error = %v", err)
	}
	observed, _ := observeConfig()
	if len(observed.Config.Profiles) != 0 || observed.Config.Footer.Message != "Changed elsewhere" {
		t.Fatalf("stale apply changed settings: %#v", observed.Config)
	}
}

func TestApplyPrivatePlanRejectsSameBytesAtNewIdentity(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := saveConfig(defaultConfig()); err != nil {
		t.Fatal(err)
	}
	preview, err := buildProfilePlan("add", "work")
	if err != nil {
		t.Fatal(err)
	}
	path, _ := configPath()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(filepath.Dir(path), "replacement.tmp")
	if err := os.WriteFile(replacement, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	err = applyPrivatePlan(filepath.Dir(path), preview, func() (workflowplan.OperationPlan, error) {
		return buildProfilePlan("add", "work")
	})
	if err == nil || !strings.Contains(err.Error(), "changed after the preview") {
		t.Fatalf("apply error = %v", err)
	}
}

func TestProfileChangeMigratesWithExactBackup(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	directory := filepath.Join(home, ".config", "alias-lens")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := []byte(`{"version":1,"repository":"","alias_file":".bash_aliases","shell":"bash","providers":{},"auto_sync":{"enabled":false,"interval_seconds":15}}`)
	path := filepath.Join(directory, "config.json")
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runConfigProfileCommand([]string{"add", "work"}); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(path + ".alias-lens.bak")
	if err != nil || !bytes.Equal(backup, legacy) {
		t.Fatalf("migration backup = %q, %v", backup, err)
	}
}

func TestConfigMigrateKeepsExactFixedBackup(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	directory := filepath.Join(home, ".config", "alias-lens")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := []byte("{\n  \"version\": 1,\n  \"repository\": \"\",\n  \"alias_file\": \".bash_aliases\",\n  \"shell\": \"bash\",\n  \"providers\": {},\n  \"auto_sync\": {\"enabled\": false, \"interval_seconds\": 15}\n}\n")
	path := filepath.Join(directory, "config.json")
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runConfigMigrationCommand(); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(path + ".alias-lens.bak")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(backup, legacy) {
		t.Fatalf("backup differs\nwant: %q\ngot:  %q", legacy, backup)
	}
	observed, err := observeConfig()
	if err != nil || observed.MigrationRequired || observed.Config.Version != 2 {
		t.Fatalf("observed = %#v, %v", observed, err)
	}
}
