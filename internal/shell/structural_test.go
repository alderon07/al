//go:build !windows

package shell_test

import (
	"alias-lens/internal/catalog"
	"alias-lens/internal/shell"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStructuralBoundariesAndPrivateJSON(t *testing.T) {
	source := []byte("# Synthetic\nalias demo='printf synthetic'\n")
	results := shell.ImportShadowSource("bash", source)
	if len(results) != 1 || results[0].Entry == nil || results[0].StartByte != 0 || results[0].EndByte != len(source) {
		t.Fatalf("parse: %#v", results)
	}
	shell.ValidateShadowCandidates(results)
	shell.CompareShadowRoundTrip("bash", &results[0])
	if results[0].Status != "equivalent" {
		t.Fatalf("roundtrip: %#v", results)
	}
	value, err := json.Marshal(results[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"Origin", "Entry", "Rendered", "origin", "entry", "rendered", "synthetic"} {
		if bytes.Contains(value, []byte(private)) {
			t.Fatalf("private JSON: %s", value)
		}
	}
	called := false
	resolver := func(string) (string, error) { called = true; return "", nil }
	if err := shell.ValidateSyntax(context.Background(), "../bash", source, resolver); err == nil || called {
		t.Fatal("unsupported shell reached resolver")
	}
	if _, err := shell.TrustedShadowShell("../bash"); err == nil {
		t.Fatal("trusted traversal accepted")
	}
	if shell.ValidateCatalogNativeDeclaration("fish", source) == nil || shell.ValidateCatalogDeclaration("fish", catalog.Entry{Name: "demo"}, source) == nil {
		t.Fatal("unsupported declarations accepted")
	}
	if _, _, err := shell.CatalogStartupPlacement(nil, "fish", "/synthetic", "/synthetic/.aliases", nil); err == nil {
		t.Fatal("unsupported startup accepted")
	}
	if shell.CatalogLoaderBlock("bash';printf injected;#", "", "", "", false) != "" {
		t.Fatal("unsafe loader generated")
	}
	if len(shell.ImportShadowSource("bash", bytes.Repeat([]byte("x"), 1<<20+1))) != 1 || shell.ImportShadowSource("fish", source)[0].Entry != nil {
		t.Fatal("source bounds")
	}
}

func TestCatalogZDOTDIRRulesAndReadOnlyParsing(t *testing.T) {
	home := t.TempDir()
	directory := t.TempDir()
	t.Setenv("ZDOTDIR", "")
	envPath := filepath.Join(home, ".zshenv")
	contents := []byte("export ZDOTDIR=" + shell.Quote(directory) + "\n")
	if err := os.WriteFile(envPath, contents, 0600); err != nil {
		t.Fatal(err)
	}
	path, err := shell.CatalogZshStartupPath(home)
	if err != nil || path != filepath.Join(directory, ".zshrc") {
		t.Fatalf("static ZDOTDIR: %s %v", path, err)
	}
	actual, _ := os.ReadFile(envPath)
	if !bytes.Equal(actual, contents) {
		t.Fatal("parser wrote .zshenv")
	}
	if err := os.WriteFile(envPath, []byte("export ZDOTDIR=$(printf synthetic)\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := shell.CatalogZshStartupPath(home); err == nil {
		t.Fatal("dynamic ZDOTDIR accepted")
	}
	t.Setenv("ZDOTDIR", "relative")
	if _, err := shell.CatalogZshStartupPath(home); err == nil {
		t.Fatal("relative catalog ZDOTDIR accepted")
	}
	if path, err := shell.ZshStartupPath(home); err != nil || !filepath.IsAbs(path) || !strings.HasSuffix(path, "relative/.zshrc") {
		t.Fatalf("legacy rule changed: %s %v", path, err)
	}
}

func TestInvalidNativeSourceCannotHideControlMasks(t *testing.T) {
	for _, prefix := range [][]byte{[]byte("# \xff\n"), []byte("# \x00\n")} {
		source := append(prefix, []byte("alias harmless='printf harmless' builtin='printf unsafe'\n")...)
		if err := shell.ValidateCatalogNativeControls("bash", source); err == nil {
			t.Fatal("invalid source concealed control assignment")
		}
		results := shell.ImportShadowSource("bash", source)
		if len(results) != 1 || results[0].Entry != nil || results[0].EndByte != len(source) {
			t.Fatalf("refusal range: %#v", results)
		}
	}
}
