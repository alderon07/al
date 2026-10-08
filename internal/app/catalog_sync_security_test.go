package app

import (
	"alias-lens/internal/catalog"
	"alias-lens/internal/catalogstore"
	"alias-lens/internal/managedgit"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatalogPushIgnoresReplacementTrees(t *testing.T) {
	repository, value := setupCatalogSyncFixture(t)
	service := DefaultServices()
	if err := os.WriteFile(filepath.Join(repository, "unenrolled"), []byte("original synthetic content"), 0600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "add", "unenrolled")
	runGit(t, repository, "commit", "-qm", "original tree")
	base := strings.TrimSpace(runGit(t, repository, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(repository, "unenrolled"), []byte("replacement synthetic content"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "synthetic-extra"), []byte("synthetic private content"), 0600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "add", "unenrolled", "synthetic-extra")
	tree := strings.TrimSpace(runGit(t, repository, "write-tree"))
	replacement := strings.TrimSpace(runGit(t, repository, "commit-tree", tree, "-m", "replacement"))
	malicious := strings.TrimSpace(runGit(t, repository, "commit-tree", tree, "-p", base, "-m", "malicious intent"))
	runGit(t, repository, "replace", base, replacement)
	beforeIndex := runGit(t, repository, "ls-files", "--stage")
	if err := os.WriteFile(filepath.Join(repository, "unenrolled"), []byte("dirty synthetic content"), 0600); err != nil {
		t.Fatal(err)
	}
	record, _, err := service.readCatalogSyncRecord()
	if err != nil {
		t.Fatal(err)
	}
	record.HEAD = base
	record.RemoteURL = "https://example.invalid/synthetic/repo.git"
	record.RemoteRef = "refs/heads/catalog"
	data, problems := catalog.Encode(value)
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	runner, cleanup, err := managedgit.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	record.PushIntent = &catalogstore.CatalogPushIntent{ExpectedRemote: base, DestinationRef: record.RemoteRef, SourceCommit: malicious, CatalogSHA256: hashBytes(data)}
	if err := service.verifyCatalogPushIntent(context.Background(), runner, record); err == nil || !strings.Contains(err.Error(), "unenrolled path") {
		t.Fatalf("replacement concealed malicious intent: %v", err)
	}
	value.Entries[0].Description = "legitimate catalog edit"
	data, problems = catalog.Encode(value)
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	commit, _, err := createCatalogPathCommit(context.Background(), runner, CatalogPushPreview{Record: record, HEAD: base, Local: data, Date: "2026-10-03T12:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if runGit(t, repository, "--no-replace-objects", "show", commit+":unenrolled") != "original synthetic content" {
		t.Fatal("catalog commit copied replacement content")
	}
	paths := strings.Fields(runGit(t, repository, "--no-replace-objects", "ls-tree", "--name-only", commit))
	if len(paths) != 2 || paths[0] != "catalog.json" || paths[1] != "unenrolled" {
		t.Fatalf("catalog commit copied extra paths: %v", paths)
	}
	record.PushIntent.SourceCommit = commit
	record.PushIntent.CatalogSHA256 = hashBytes(data)
	if err := service.verifyCatalogPushIntent(context.Background(), runner, record); err != nil {
		t.Fatal("legitimate intent rejected", err)
	}
	if strings.TrimSpace(runGit(t, repository, "rev-parse", "refs/replace/"+base)) != replacement || beforeIndex != runGit(t, repository, "ls-files", "--stage") {
		t.Fatal("replacement ref or staged files changed")
	}
	dirty, err := os.ReadFile(filepath.Join(repository, "unenrolled"))
	if err != nil || string(dirty) != "dirty synthetic content" {
		t.Fatal("dirty file changed", err)
	}
}
