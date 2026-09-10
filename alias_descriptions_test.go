package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddAliasDescriptionsPreservesExistingCommentsAndMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".bash_aliases")
	original := "# Show status my way\nalias gs='git status -sb'\n\n# al: tags=git\nalias gc='git commit'\nalias ll='ls -alF'\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	added, err := addAliasDescriptionsToFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if added != 2 {
		t.Fatalf("added %d descriptions, want 2", added)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	for _, expected := range []string{
		"# Show status my way\nalias gs='git status -sb'",
		"# Create a commit from staged changes\n# al: tags=git\nalias gc='git commit'",
		"# List files and directories\nalias ll='ls -alF'",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("updated file is missing %q:\n%s", expected, text)
		}
	}
}

func TestAddAliasDescriptionsImprovesOldGeneratedComments(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".bash_aliases")
	original := "# Runs git add\nalias ga='git add'\n\n# My status format\nalias gs='git status -sb'\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := addAliasDescriptionsToFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if changed != 1 {
		t.Fatalf("changed %d descriptions, want 1", changed)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	if !strings.Contains(text, "# Stage files for the next commit\nalias ga=") {
		t.Fatalf("generated description was not improved:\n%s", text)
	}
	if !strings.Contains(text, "# My status format\nalias gs=") {
		t.Fatalf("custom description was changed:\n%s", text)
	}
}

func TestAddAliasDescriptionsDoesNothingWhenAllExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".bash_aliases")
	original := []byte("# Show status\nalias gs='git status -sb'\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	added, err := addAliasDescriptionsToFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if added != 0 {
		t.Fatalf("added %d descriptions, want 0", added)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != string(original) {
		t.Fatal("no-op description pass changed the file")
	}
}
