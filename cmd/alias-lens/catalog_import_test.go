//go:build !windows

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	neutralcatalog "github.com/alderon07/al/internal/catalog"
)

func TestCatalogPreviewIsReadOnlyAndUsesFriendlyNextStep(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), []byte("alias gs='git status'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	originalValidator := applicationDependencies.ValidateSyntax
	applicationDependencies.ValidateSyntax = func(context.Context, string, []byte) error { return nil }
	t.Cleanup(func() { applicationDependencies.ValidateSyntax = originalValidator })
	originalOutput := catalogWorkflowStdout
	var output bytes.Buffer
	catalogWorkflowStdout = &output
	t.Cleanup(func() { catalogWorkflowStdout = originalOutput })

	code, err := runCatalogPreviewCommand([]string{"--from", "bash"})
	if err != nil || code != 0 {
		t.Fatalf("preview code=%d err=%v output=%s", code, err, output.String())
	}
	if _, err := os.Stat(catalogPathFixture()); !os.IsNotExist(err) {
		t.Fatalf("preview created a catalog: %v", err)
	}
	if !strings.Contains(output.String(), "Nothing was changed") || !strings.Contains(output.String(), "al catalog import --from bash") {
		t.Fatalf("preview did not explain the result and next step: %s", output.String())
	}
}

func TestCatalogImportCreatesInactiveCatalogAndKeepsNativeFile(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	sourcePath := filepath.Join(home, ".bash_aliases")
	source := []byte("# Show status\nalias gs='git status'\n")
	if err := os.WriteFile(sourcePath, source, 0o600); err != nil {
		t.Fatal(err)
	}
	originalValidator := applicationDependencies.ValidateSyntax
	applicationDependencies.ValidateSyntax = func(context.Context, string, []byte) error { return nil }
	t.Cleanup(func() { applicationDependencies.ValidateSyntax = originalValidator })
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
	contents, err := os.ReadFile(catalogPathFixture())
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

func TestCatalogImportWithOnlyUnsupportedContentWritesNothing(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	sourcePath := filepath.Join(home, ".bash_aliases")
	source := []byte("this is not an alias definition\n")
	if err := os.WriteFile(sourcePath, source, 0o600); err != nil {
		t.Fatal(err)
	}
	originalValidator := applicationDependencies.ValidateSyntax
	applicationDependencies.ValidateSyntax = func(context.Context, string, []byte) error { return nil }
	t.Cleanup(func() { applicationDependencies.ValidateSyntax = originalValidator })
	originalOutput := catalogWorkflowStdout
	var output bytes.Buffer
	catalogWorkflowStdout = &output
	t.Cleanup(func() { catalogWorkflowStdout = originalOutput })

	code, err := runCatalogImportCommand([]string{"--from", "bash"})
	if err != nil || code != 0 {
		t.Fatalf("import code=%d err=%v output=%s", code, err, output.String())
	}
	if _, err := os.Stat(catalogPathFixture()); !os.IsNotExist(err) {
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
