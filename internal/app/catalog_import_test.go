//go:build !windows

package app

import (
	"context"
	"os"
	"path/filepath"

	neutralcatalog "github.com/alderon07/al/internal/catalog"
	"testing"
)

func TestCatalogImportAddsOnlySelectedShellToExistingEntry(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), []byte("alias gs='git status --short'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configDirectory := filepath.Join(home, ".config", "alias-lens")
	if err := os.MkdirAll(configDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	zshValue := "git status"
	existing := neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{{
		ID: "00000000000000000000000000000001", Name: "gs", Kind: "command", Description: "Keep this description", Tags: []string{"keep"},
		Native: map[string]neutralcatalog.NativeImplementation{"zsh": {AliasValue: &zshValue}},
	}}}
	encoded, diagnostics := neutralcatalog.Encode(existing)
	if len(diagnostics) != 0 {
		t.Fatalf("encode fixture: %+v", diagnostics)
	}
	if err := os.WriteFile(filepath.Join(configDirectory, "catalog.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	originalValidator := shadowSyntaxValidator
	shadowSyntaxValidator = func(context.Context, string, []byte) error { return nil }
	t.Cleanup(func() { shadowSyntaxValidator = originalValidator })

	plan, err := DefaultServices().buildCatalogImportPlan("bash")
	if err != nil || plan.Summary.Blocked || len(plan.Actions) != 1 {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
	updated, problems := neutralcatalog.Decode(plan.Actions[0].Target.PlannedBytes)
	if len(problems) != 0 || len(updated.Entries) != 1 {
		t.Fatalf("updated catalog=%+v problems=%+v", updated, problems)
	}
	entry := updated.Entries[0]
	if entry.ID != existing.Entries[0].ID || entry.Description != "Keep this description" || len(entry.Tags) != 1 || entry.Tags[0] != "keep" {
		t.Fatalf("import overwrote catalog identity or metadata: %+v", entry)
	}
	if _, ok := entry.Native["bash"]; !ok {
		t.Fatal("Bash implementation was not added")
	}
	if got := entry.Native["zsh"].AliasValue; got == nil || *got != zshValue {
		t.Fatalf("Zsh implementation was changed: %+v", entry.Native["zsh"])
	}
}

func TestCatalogImportKeepsUnsafeRangeNativeAndImportsSafeEntries(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	source := "alias first='git status'\nthis is not an alias definition\nalias second='git log'\n"
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	originalValidator := shadowSyntaxValidator
	shadowSyntaxValidator = func(context.Context, string, []byte) error { return nil }
	t.Cleanup(func() { shadowSyntaxValidator = originalValidator })

	plan, err := DefaultServices().buildCatalogImportPlan("bash")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Summary.Blocked || len(plan.Actions) != 1 || len(plan.Diagnostics) == 0 {
		t.Fatalf("mixed import plan = %#v", plan)
	}
	value, diagnostics := neutralcatalog.Decode(plan.Actions[0].Target.PlannedBytes)
	if len(diagnostics) > 0 || len(value.Entries) != 2 || value.Entries[0].Name != "first" || value.Entries[1].Name != "second" {
		t.Fatalf("catalog = %#v, diagnostics=%#v", value, diagnostics)
	}
}
