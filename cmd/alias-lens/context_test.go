package main

import (
	"bytes"
	"encoding/json"

	"os"
	"path/filepath"

	"strings"
	"testing"
)

func TestContextCommandAndSearchUseCurrentProject(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv("ALIAS_LENS_SHELL", "bash")
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), []byte("alias depot='echo depot'\nalias deploy='go run ./deploy'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(filepath.Join(project, "src"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(project, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(project, "src"))
	if err := runContextCommand([]string{"add", "deploy"}); err != nil {
		t.Fatal(err)
	}
	marked := captureSearchOutput(t, []string{"dep"})
	global := captureSearchOutput(t, []string{"--global", "dep"})
	if !strings.HasPrefix(marked, "deploy\t") || !strings.HasPrefix(global, "depot\t") {
		t.Fatalf("context search order = %q; global order = %q", marked, global)
	}
	var jsonResults []Alias
	jsonOutput := captureSearchOutput(t, []string{"--json", "dep"})
	if err := json.Unmarshal([]byte(jsonOutput), &jsonResults); err != nil || len(jsonResults) != 2 || jsonResults[0].Name != "deploy" {
		t.Fatalf("context JSON results = %#v, %v", jsonResults, err)
	}
	if strings.Contains(jsonOutput, `"context"`) || strings.Contains(jsonOutput, `"repository"`) || strings.Contains(jsonOutput, `"directory"`) {
		t.Fatalf("search JSON exposed local context: %s", jsonOutput)
	}
	if err := runContextCommand([]string{"remove", "deploy"}); err != nil {
		t.Fatal(err)
	}
	if got := captureSearchOutput(t, []string{"dep"}); got != global {
		t.Fatalf("search order after removing mark = %q, want %q", got, global)
	}
}

func captureSearchOutput(t *testing.T, arguments []string) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdout
	os.Stdout = writer
	defer func() {
		os.Stdout = previous
		reader.Close()
	}()
	searchErr := runSearchCommand(arguments)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if searchErr != nil {
		t.Fatal(searchErr)
	}
	contents := new(bytes.Buffer)
	if _, err := contents.ReadFrom(reader); err != nil {
		t.Fatal(err)
	}
	return contents.String()
}
