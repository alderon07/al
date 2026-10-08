package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	neutralcatalog "github.com/alderon07/al/internal/catalog"
	workflowplan "github.com/alderon07/al/internal/plan"
)

func TestProfilePlanDoesNotWriteAndNamesAffectedEntries(t *testing.T) {
	home := privateTestHome(t)
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
	value, err := DefaultServices().buildProfilePlan("add", "work")
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

func TestApplyPrivatePlanRejectsChangedPreview(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	if err := DefaultServices().saveConfig(DefaultServices().defaultConfig()); err != nil {
		t.Fatal(err)
	}
	preview, err := DefaultServices().buildProfilePlan("add", "work")
	if err != nil {
		t.Fatal(err)
	}
	config, err := DefaultServices().loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	config.Footer.Message = "Changed elsewhere"
	if err := DefaultServices().saveConfig(config); err != nil {
		t.Fatal(err)
	}
	path, _ := DefaultServices().configPath()
	err = DefaultServices().applyPrivatePlan(filepath.Dir(path), preview, func() (workflowplan.OperationPlan, error) {
		return DefaultServices().buildProfilePlan("add", "work")
	})
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("changed after the preview")) {
		t.Fatalf("apply error = %v", err)
	}
	observed, _ := DefaultServices().observeConfig()
	if len(observed.Config.Profiles) != 0 || observed.Config.Footer.Message != "Changed elsewhere" {
		t.Fatalf("stale apply changed settings: %#v", observed.Config)
	}
}

func TestApplyPrivatePlanRejectsSameBytesAtNewIdentity(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	if err := DefaultServices().saveConfig(DefaultServices().defaultConfig()); err != nil {
		t.Fatal(err)
	}
	preview, err := DefaultServices().buildProfilePlan("add", "work")
	if err != nil {
		t.Fatal(err)
	}
	path, _ := DefaultServices().configPath()
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
	err = DefaultServices().applyPrivatePlan(filepath.Dir(path), preview, func() (workflowplan.OperationPlan, error) {
		return DefaultServices().buildProfilePlan("add", "work")
	})
	if err == nil || !strings.Contains(err.Error(), "changed after the preview") {
		t.Fatalf("apply error = %v", err)
	}
}
