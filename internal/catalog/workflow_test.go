package catalog

import (
	"encoding/json"
	"strings"
	"testing"
)

func catalogV2() Catalog {
	value := validCatalog()
	value.SchemaVersion = SchemaVersion
	value.Entries[0].When = &Conditions{ProfilesAny: []string{"work", "laptop"}, ProfilesNone: []string{"personal"}, Shells: []string{"zsh", "bash"}}
	return value
}

func TestConditionsAndVersionBoundary(t *testing.T) {
	value := catalogV2()
	encoded, diagnostics := Encode(value)
	if len(diagnostics) > 0 {
		t.Fatalf("Encode diagnostics: %#v", diagnostics)
	}
	if !strings.Contains(string(encoded), `"when": {`) {
		t.Fatalf("encoded catalog has no conditions: %s", encoded)
	}
	if decoded, diagnostics := Decode(encoded); len(diagnostics) > 0 || decoded.SchemaVersion != 2 {
		t.Fatalf("Decode = %#v, %#v", decoded, diagnostics)
	}

	value.SchemaVersion = 1
	if diagnostics := Validate(value); len(diagnostics) == 0 || diagnostics[0].Code != "invalid_schema_version" {
		t.Fatalf("version 1 was accepted: %#v", diagnostics)
	}
	input := `{"schema_version":2,"entries":[{"id":"87f4d803c44a4d8792c4824f8e0bc3f1","name":"gs","kind":"command","when":{"profiles_any":[]},"portable":{"program":"git","args":[],"pass_arguments":true}}]}`
	if _, diagnostics := Decode([]byte(input)); len(diagnostics) == 0 {
		t.Fatal("Decode accepted a present empty condition")
	}
}

func TestResolveConditionMatrix(t *testing.T) {
	value := catalogV2()
	resolved, diagnostics := Resolve(value, ResolveContext{Shell: "bash", Platform: "linux", Profiles: []string{"work"}})
	if len(diagnostics) > 0 || len(resolved) != 1 || !resolved[0].Available {
		t.Fatalf("available Resolve = %#v, %#v", resolved, diagnostics)
	}
	resolved, diagnostics = Resolve(value, ResolveContext{Shell: "bash", Platform: "linux", Profiles: []string{"personal"}})
	if len(diagnostics) > 0 || resolved[0].Available || len(resolved[0].UnavailableReasons) != 2 {
		t.Fatalf("unavailable Resolve = %#v, %#v", resolved, diagnostics)
	}
	first, _ := json.Marshal(resolved)
	second, _ := json.Marshal(resolved)
	if string(first) != string(second) {
		t.Fatal("Resolve is not deterministic")
	}
}

func TestSemanticDiffRedactsValuesAndUsesStableIDs(t *testing.T) {
	before := catalogV2()
	after := Normalize(before)
	after.Entries[0].Name = "status"
	secret := "printf super-secret-value"
	after.Entries[0].Native = map[string]NativeImplementation{"bash": {AliasValue: &secret}}
	report := SemanticDiff(before, after, "repository", "")
	if report.Summary.Changed != 1 || len(report.Changes) != 2 {
		t.Fatalf("SemanticDiff = %#v", report)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "super-secret-value") {
		t.Fatalf("semantic report leaked implementation text: %s", encoded)
	}
	if report.Changes[0].EntryID != before.Entries[0].ID {
		t.Fatalf("stable ID missing: %#v", report.Changes[0])
	}
}

func TestThreeWayMergeNonOverlappingFields(t *testing.T) {
	base := catalogV2()
	local := Normalize(base)
	remote := Normalize(base)
	local.Entries[0].Description = "Show a short status"
	remote.Entries[0].Category = "source-control"
	result := ThreeWayMerge(base, local, remote)
	if result.Catalog == nil || len(result.Conflicts) > 0 {
		t.Fatalf("ThreeWayMerge = %#v", result)
	}
	entry := result.Catalog.Entries[0]
	if entry.Description != local.Entries[0].Description || entry.Category != remote.Entries[0].Category {
		t.Fatalf("merge lost changes: %#v", entry)
	}
}

func TestThreeWayMergeConflictsOnSameFieldAndNameCollision(t *testing.T) {
	base := catalogV2()
	local := Normalize(base)
	remote := Normalize(base)
	local.Entries[0].Description = "local"
	remote.Entries[0].Description = "remote"
	result := ThreeWayMerge(base, local, remote)
	if result.Catalog != nil || len(result.Conflicts) != 1 || result.Conflicts[0].Path != "description" {
		t.Fatalf("field conflict = %#v", result)
	}

	local = Normalize(base)
	remote = Normalize(base)
	local.Entries = append(local.Entries, Entry{ID: "11111111111111111111111111111111", Name: "same", Kind: "command", Portable: &Portable{Program: "git", Args: []string{}, PassArguments: false}})
	remote.Entries = append(remote.Entries, Entry{ID: "22222222222222222222222222222222", Name: "same", Kind: "command", Portable: &Portable{Program: "git", Args: []string{}, PassArguments: false}})
	result = ThreeWayMerge(base, local, remote)
	if result.Catalog != nil || len(result.Conflicts) == 0 || result.Conflicts[0].Kind != "name" {
		t.Fatalf("name conflict = %#v", result)
	}
}
