package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAliasExportFormatsPreserveData(t *testing.T) {
	export := aliasExport{
		Kind:       "aliases",
		ExportedAt: time.Unix(1789448000, 0).UTC(),
		Aliases:    []Alias{{Name: "say", Command: "printf 'hello: world'", Description: "Say hello", Tags: []string{"demo", "daily"}}},
	}
	jsonContents, err := encodeAliasExport(export, "json")
	if err != nil {
		t.Fatal(err)
	}
	var decoded aliasExport
	if err := json.Unmarshal(jsonContents, &decoded); err != nil || decoded.Aliases[0].Command != export.Aliases[0].Command {
		t.Fatalf("JSON did not round trip: %s, %v", jsonContents, err)
	}
	yamlContents, err := encodeAliasExport(export, "yaml")
	if err != nil || !strings.Contains(string(yamlContents), `command: "printf 'hello: world'"`) || !strings.Contains(string(yamlContents), `tags: ["demo","daily"]`) {
		t.Fatalf("unexpected YAML:\n%s%v", yamlContents, err)
	}
}

func TestCSVExportNeutralizesSpreadsheetFormulas(t *testing.T) {
	contents, err := encodeAliasExport(aliasExport{Aliases: []Alias{{Name: "risky", Command: "=cmd|' /C calc'!A0"}}}, "csv")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), "'=cmd") {
		t.Fatalf("CSV formula was not neutralized:\n%s", contents)
	}
}

func TestPrivateExportReplacesFileAtomically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "aliases.json")
	if err := writePrivateExport(path, []byte("first\n")); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateExport(path, []byte("second\n")); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	info, statErr := os.Stat(path)
	if err != nil || statErr != nil || string(contents) != "second\n" || info.Mode().Perm() != 0o600 {
		t.Fatalf("unexpected private export: %q mode=%v read=%v stat=%v", contents, info.Mode().Perm(), err, statErr)
	}
}
