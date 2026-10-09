//go:build !windows

package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alderon07/al/internal/catalog"
	"github.com/alderon07/al/internal/catalogstore"
)

func TestCatalogPendingReviewSkipsExactRecordsAndRejectsDrift(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	service := NewServices(Dependencies{ValidateSyntax: func(context.Context, string, []byte) error { return nil }})
	value := "echo synthetic"
	entry := catalog.Entry{ID: strings.Repeat("1", 32), Name: "fixture", Kind: "command", Native: map[string]catalog.NativeImplementation{"bash": {AliasValue: &value}}}
	source := catalog.Catalog{SchemaVersion: 2, Entries: []catalog.Entry{entry}}
	native := []byte("alias fixture='echo synthetic'\n")
	nativePath := filepath.Join(home, ".bash_aliases")
	if err := os.WriteFile(nativePath, native, 0600); err != nil {
		t.Fatal(err)
	}
	writeCatalogFixture(t, service.localCatalogPath(), source)
	review, err := service.PrepareCatalogReview(source, "bash")
	if err != nil {
		t.Fatal(err)
	}
	items, err := service.CatalogPendingReview(review, source, true)
	if err != nil || len(items) != 2 {
		t.Fatalf("pending review: %d %v", len(items), err)
	}
	decisions := StageCatalogReviewItems(review.Decisions, items)
	ledger := catalogstore.AdoptionsFile{Version: 1, Records: decisions.Adoptions}
	ledger.Records[0].OriginalSHA256 = strings.Repeat("9", 64)
	ledger.Records[0].OriginalPath = filepath.Join(home, "original-synthetic")
	approvals := catalogstore.ApprovalFile{Version: 2, Records: decisions.Approvals}
	for path, record := range map[string]any{service.catalogApprovalsPath(): approvals, service.catalogAdoptionsPath(): ledger} {
		encoded, err := catalogstore.Encode(record)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, encoded, 0600); err != nil {
			t.Fatal(err)
		}
	}
	review, err = service.PrepareCatalogReview(source, "bash")
	if err != nil {
		t.Fatal(err)
	}
	items, err = service.CatalogPendingReview(review, source, true)
	if err != nil || len(items) != 0 {
		t.Fatal("matching installed records were reviewed again")
	}
	changedCatalog := source
	changedCatalog.Entries = append([]catalog.Entry(nil), source.Entries...)
	changedCatalog.Entries[0].Description = "synthetic changed after review"
	writeCatalogFixture(t, service.localCatalogPath(), changedCatalog)
	if _, err := service.BuildCatalogDecisionPlan(decisions); err == nil {
		t.Fatal("catalog drift was accepted after captured review")
	}
	writeCatalogFixture(t, service.localCatalogPath(), source)
	changed := append([]byte("# unrelated synthetic\n"), native...)
	if err := os.WriteFile(nativePath, changed, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.BuildCatalogDecisionPlan(decisions); err == nil {
		t.Fatal("native drift was accepted after captured review")
	}
	review, err = service.PrepareCatalogReview(source, "bash")
	if err != nil {
		t.Fatal(err)
	}
	items, err = service.CatalogPendingReview(review, source, true)
	if err != nil || len(items) != 1 || items[0].Adoption == nil {
		t.Fatal("changed source ownership was skipped")
	}
	for _, mutate := range []func(*catalogstore.Adoption){
		func(record *catalogstore.Adoption) { record.Name = "different" },
		func(record *catalogstore.Adoption) { record.Kind = "function" },
		func(record *catalogstore.Adoption) { record.Path += "-other" },
		func(record *catalogstore.Adoption) { record.FileSHA256 = strings.Repeat("7", 64) },
		func(record *catalogstore.Adoption) { record.Start++ },
		func(record *catalogstore.Adoption) { record.End++ },
		func(record *catalogstore.Adoption) { record.Definition = "different" },
		func(record *catalogstore.Adoption) { record.DefinitionSHA256 = strings.Repeat("8", 64) },
	} {
		original := items[0].Adoption.Adoption
		changed := original
		mutate(&changed)
		if catalogOwnershipMatches(changed, original) {
			t.Fatal("nonmatching ownership proof was skipped")
		}

	}
}

func TestCatalogStatusKeepsEntriesAndConflictsWithUnsafeNative(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	service := NewServices(Dependencies{ValidateSyntax: func(context.Context, string, []byte) error { return nil }})
	unsafeBody := "echo $(printf synthetic)\n"
	safeValue := "echo synthetic"
	unsafe := catalog.Entry{ID: strings.Repeat("1", 32), Name: "unsafe_fixture", Kind: "function", Native: map[string]catalog.NativeImplementation{"bash": {FunctionBody: &unsafeBody}}}
	safe := catalog.Entry{ID: strings.Repeat("2", 32), Name: "safe_fixture", Kind: "command", Native: map[string]catalog.NativeImplementation{"bash": {AliasValue: &safeValue}}}
	value := catalog.Catalog{SchemaVersion: 2, Entries: []catalog.Entry{unsafe, safe}}
	writeCatalogFixture(t, service.localCatalogPath(), value)
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), []byte("alias ordinary='echo synthetic'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("3", 64)
	conflictRoot := filepath.Join(filepath.Dir(service.localCatalogPath()), "catalog-conflicts", id)
	if err := os.MkdirAll(conflictRoot, 0700); err != nil {
		t.Fatal(err)
	}
	metadata := struct {
		Version   int                     `json:"version"`
		Conflicts []catalog.MergeConflict `json:"conflicts"`
	}{Version: 1, Conflicts: []catalog.MergeConflict{{EntryID: safe.ID, Name: safe.Name, Path: "description", Kind: "changed", Message: "synthetic conflict"}}}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(conflictRoot, "conflicts.json"), encoded, 0600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.LoadCatalogReview("bash")
	if err != nil || snapshot.Warning == "" {
		t.Fatalf("unsafe declaration aborted status instead of warning: %v", err)
	}
	if len(snapshot.Entries) != 1 || snapshot.Entries[0].Name != "ordinary" {
		t.Fatal("unsafe declaration lost native status entries")
	}
	if len(snapshot.Conflicts) != 1 || snapshot.Conflicts[0].ID != id || len(snapshot.Conflicts[0].Fields) != 1 {
		t.Fatal("unsafe declaration hid saved semantic conflicts")
	}
	if len(snapshot.Items) != 1 || snapshot.Items[0].Native == nil || snapshot.Items[0].Native.Entry.ID != safe.ID {
		t.Fatal("tolerant status did not skip only unsafe review items")
	}
	decisions := StageCatalogReviewItems(snapshot.Decisions, snapshot.Items)
	if len(decisions.Approvals) != 1 || decisions.Approvals[0].Key.EntryID != safe.ID {
		t.Fatal("unsafe native record was staged")
	}
	review, err := service.PrepareCatalogReview(value, "bash")
	if err != nil {
		t.Fatal(err)
	}
	items, err := service.CatalogPendingReview(review, value, true)
	if err == nil || len(items) != 0 {
		t.Fatal("strict CLI review did not fail closed on unsafe declaration")
	}
}
