package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMissingDefaultsSkipExistingNamesAndCommands(t *testing.T) {
	contents := []byte("alias gs='git status'\nalias stage='git add'\n")
	missing := missingDefaultAliases(contents)
	for _, candidate := range missing {
		if candidate.Name == "gs" {
			t.Fatal("default replaced an existing alias name")
		}
		if candidate.Name == "ga" {
			t.Fatal("default duplicated an existing command")
		}
	}
}

func TestDefaultAliasOfferExplainsAndDefaultsToNo(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".bash_aliases")
	original := []byte("alias mine='echo mine'\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := offerDefaultAliases(path, strings.NewReader("\n"), &output); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"git status --short --branch", "Nothing above runs during setup", "[y/N]", "Skipped optional developer aliases"} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("offer did not explain %q:\n%s", expected, output.String())
		}
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(updated, original) {
		t.Fatalf("declining changed the alias file:\n%s", updated)
	}
}

func TestAcceptingDefaultsAddsOnlyMissingAliases(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".bash_aliases")
	if err := os.WriteFile(path, []byte("alias gs='custom status'\nalias stage='git add'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := offerDefaultAliases(path, strings.NewReader("yes\n"), &output); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	aliases := aliasDefinitionMap(contents)
	if aliases["gs"] != "custom status" {
		t.Fatal("accepting defaults replaced an existing alias")
	}
	if _, exists := aliases["ga"]; exists {
		t.Fatal("accepting defaults duplicated git add")
	}
	for _, name := range []string{"gc", "gd", "gl", "ll"} {
		if _, exists := aliases[name]; !exists {
			t.Errorf("missing accepted default %s", name)
		}
	}
}
