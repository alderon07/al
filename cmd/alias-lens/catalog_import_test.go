//go:build !windows

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	neutralcatalog "alias-lens/internal/catalog"
)

func TestCatalogPreviewIsReadOnlyAndUsesFriendlyNextStep(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), []byte("alias gs='git status'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	originalValidator := shadowSyntaxValidator
	shadowSyntaxValidator = func(context.Context, string, []byte) error { return nil }
	t.Cleanup(func() { shadowSyntaxValidator = originalValidator })
	originalOutput := catalogWorkflowStdout
	var output bytes.Buffer
	catalogWorkflowStdout = &output
	t.Cleanup(func() { catalogWorkflowStdout = originalOutput })

	code, err := runCatalogPreviewCommand([]string{"--from", "bash"})
	if err != nil || code != 0 {
		t.Fatalf("preview code=%d err=%v output=%s", code, err, output.String())
	}
	if _, err := os.Stat(localCatalogPath()); !os.IsNotExist(err) {
		t.Fatalf("preview created a catalog: %v", err)
	}
	if !strings.Contains(output.String(), "Nothing was changed") || !strings.Contains(output.String(), "al catalog import --from bash") {
		t.Fatalf("preview did not explain the result and next step: %s", output.String())
	}
}

func TestCatalogImportCreatesInactiveCatalogAndKeepsNativeFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sourcePath := filepath.Join(home, ".bash_aliases")
	source := []byte("# Show status\nalias gs='git status'\n")
	if err := os.WriteFile(sourcePath, source, 0o600); err != nil {
		t.Fatal(err)
	}
	originalValidator := shadowSyntaxValidator
	shadowSyntaxValidator = func(context.Context, string, []byte) error { return nil }
	t.Cleanup(func() { shadowSyntaxValidator = originalValidator })
	originalOutput := catalogWorkflowStdout
	var output bytes.Buffer
	catalogWorkflowStdout = &output
	t.Cleanup(func() { catalogWorkflowStdout = originalOutput })

	code, err := runCatalogImportCommand([]string{"--from", "bash"})
	if err != nil || code != 0 {
		t.Fatalf("import code=%d err=%v output=%s", code, err, output.String())
	}
	unchanged, err := os.ReadFile(sourcePath)
	if err != nil || !bytes.Equal(unchanged, source) {
		t.Fatalf("native alias file changed: %q, %v", unchanged, err)
	}
	contents, err := os.ReadFile(localCatalogPath())
	if err != nil {
		t.Fatal(err)
	}
	catalog, diagnostics := neutralcatalog.Decode(contents)
	if len(diagnostics) != 0 || catalog.SchemaVersion != 2 || len(catalog.Entries) != 1 || catalog.Entries[0].Name != "gs" {
		t.Fatalf("unexpected catalog: %+v diagnostics=%+v", catalog, diagnostics)
	}
	if !strings.Contains(output.String(), "ready but inactive") || !strings.Contains(output.String(), "current aliases were left unchanged") {
		t.Fatalf("import output was not clear: %s", output.String())
	}
}

func TestCatalogImportAddsOnlySelectedShellToExistingEntry(t *testing.T) {
	home := t.TempDir()
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

	plan, err := buildCatalogImportPlan("bash")
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
	home := t.TempDir()
	t.Setenv("HOME", home)
	source := "alias first='git status'\nthis is not an alias definition\nalias second='git log'\n"
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	originalValidator := shadowSyntaxValidator
	shadowSyntaxValidator = func(context.Context, string, []byte) error { return nil }
	t.Cleanup(func() { shadowSyntaxValidator = originalValidator })

	plan, err := buildCatalogImportPlan("bash")
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

func TestCatalogImportWithOnlyUnsupportedContentWritesNothing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sourcePath := filepath.Join(home, ".bash_aliases")
	source := []byte("this is not an alias definition\n")
	if err := os.WriteFile(sourcePath, source, 0o600); err != nil {
		t.Fatal(err)
	}
	originalValidator := shadowSyntaxValidator
	shadowSyntaxValidator = func(context.Context, string, []byte) error { return nil }
	t.Cleanup(func() { shadowSyntaxValidator = originalValidator })
	originalOutput := catalogWorkflowStdout
	var output bytes.Buffer
	catalogWorkflowStdout = &output
	t.Cleanup(func() { catalogWorkflowStdout = originalOutput })

	code, err := runCatalogImportCommand([]string{"--from", "bash"})
	if err != nil || code != 0 {
		t.Fatalf("import code=%d err=%v output=%s", code, err, output.String())
	}
	if _, err := os.Stat(localCatalogPath()); !os.IsNotExist(err) {
		t.Fatalf("unsupported-only import created a catalog: %v", err)
	}
	unchanged, err := os.ReadFile(sourcePath)
	if err != nil || !bytes.Equal(unchanged, source) {
		t.Fatalf("unsupported-only import changed the source: %q, %v", unchanged, err)
	}
	if !strings.Contains(output.String(), "No entries could be copied safely") || !strings.Contains(output.String(), "left unchanged") {
		t.Fatalf("import output did not explain the safe no-op: %s", output.String())
	}
}
