//go:build !windows

package app

import (
	"bytes"
	"github.com/alderon07/al/internal/catalogstore"
	workflowplan "github.com/alderon07/al/internal/plan"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLocalInitPreviewPinsDirtyBytesWithoutWriting(t *testing.T) {
	pinInitTestValidator(t)
	repo, value := setupCatalogSyncFixture(t)
	os.Remove(DefaultServices().localCatalogPath())
	os.Remove(DefaultServices().catalogSyncPath())
	value.Entries[0].Description = "dirty working tree"
	writeCatalogFixture(t, filepath.Join(repo, "catalog.json"), value)
	source, _ := os.ReadFile(filepath.Join(repo, "catalog.json"))
	beforeIndex := runGit(t, repo, "ls-files", "--stage")
	beforeHEAD := runGit(t, repo, "rev-parse", "HEAD")
	options := CatalogInitOptions{Source: repo, Shell: "bash", CatalogPath: "catalog.json"}
	record, observed, raw, err := DefaultServices().observeLocalCatalogInit(options)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(source, raw) || record.SourceSHA256 != hashBytes(source) || record.BaseBytes == string(source) || observed.Entries[0].Description != "dirty working tree" {
		t.Fatal("dirty bytes not pinned")
	}
	committed := runGit(t, repo, "rev-parse", "HEAD:catalog.json")
	if record.Blob != managedGitText([]byte(committed)) {
		t.Fatal("dirty bytes misattributed to blob")
	}
	plan, err := DefaultServices().buildLocalCatalogInitPlan(options, CatalogLifecycleDecisions{ConfirmedAt: "2026-10-03T12:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Summary.Blocked {
		t.Fatal(plan.Diagnostics)
	}
	if beforeHEAD != runGit(t, repo, "rev-parse", "HEAD") || beforeIndex != runGit(t, repo, "ls-files", "--stage") {
		t.Fatal("preview changed repository")
	}
	for _, path := range []string{DefaultServices().localCatalogPath(), DefaultServices().catalogSyncPath(), filepath.Join(DefaultServices().homeDirectory(), ".bash_aliases")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("preview wrote managed state")
		}
	}
}
func TestLocalInitRejectsChangedSourceAtApply(t *testing.T) {
	pinInitTestValidator(t)
	repo, value := setupCatalogSyncFixture(t)
	os.Remove(DefaultServices().localCatalogPath())
	os.Remove(DefaultServices().catalogSyncPath())
	options := CatalogInitOptions{Source: repo, Shell: "bash", CatalogPath: "catalog.json"}
	decisions := CatalogLifecycleDecisions{ConfirmedAt: "2026-10-03T12:00:00Z"}
	preview, err := DefaultServices().buildLocalCatalogInitPlan(options, decisions)
	if err != nil {
		t.Fatal(err)
	}
	value.Entries[0].Description = "changed after preview"
	writeCatalogFixture(t, filepath.Join(repo, "catalog.json"), value)
	if DefaultServices().applyMutationPlan(preview, func() (workflowplan.OperationPlan, error) {
		return DefaultServices().buildLocalCatalogInitPlan(options, decisions)
	}) == nil {
		t.Fatal("stale init applied")
	}
	if _, err := os.Stat(DefaultServices().catalogSyncPath()); !os.IsNotExist(err) {
		t.Fatal("stale enrollment persisted")
	}
}
func TestLocalInitPreservesLegacyRepositoryConfiguration(t *testing.T) {
	pinInitTestValidator(t)
	repo, _ := setupCatalogSyncFixture(t)
	os.Remove(DefaultServices().localCatalogPath())
	os.Remove(DefaultServices().catalogSyncPath())
	legacy := filepath.Join(DefaultServices().homeDirectory(), "legacy")
	os.MkdirAll(legacy, 0700)
	config := DefaultServices().defaultConfig()
	config.Repository = legacy
	config.AliasFile = ".bash_aliases"
	if err := DefaultServices().saveConfig(config); err != nil {
		t.Fatal(err)
	}
	options := CatalogInitOptions{Source: repo, Shell: "bash", CatalogPath: "catalog.json"}
	preview, err := DefaultServices().buildLocalCatalogInitPlan(options, CatalogLifecycleDecisions{ConfirmedAt: "2026-10-03T12:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range preview.Actions {
		if action.TargetRole == "config" {
			t.Fatal("init overwrote legacy config")
		}
	}
	if err := DefaultServices().applyMutationPlan(preview, func() (workflowplan.OperationPlan, error) {
		return DefaultServices().buildLocalCatalogInitPlan(options, CatalogLifecycleDecisions{ConfirmedAt: "2026-10-03T12:00:00Z"})
	}); err != nil {
		t.Fatal(err)
	}
	record, _, err := DefaultServices().readCatalogSyncRecord()
	if err != nil || record.Repository != repo {
		t.Fatal("enrollment missing", err)
	}
	updated, err := DefaultServices().loadConfig()
	if err != nil || updated.Repository != legacy {
		t.Fatal("legacy config replaced", err)
	}
	if err := catalogstore.Validate(record); err != nil {
		t.Fatal(err)
	}
}

func pinInitTestValidator(t *testing.T) {
	t.Helper()
	original := shadowValidatorPath
	shadowValidatorPath = func(shell string) (string, error) { return exec.LookPath(shell) }
	t.Cleanup(func() { shadowValidatorPath = original })
}
