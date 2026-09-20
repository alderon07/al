package catalog

import (
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

func text(value string) *string { return &value }

func validCatalog() Catalog {
	return Catalog{SchemaVersion: 1, Entries: []Entry{{
		ID: "87f4d803c44a4d8792c4824f8e0bc3f1", Name: "gs", Kind: "command",
		Tags: []string{"git", "daily", "git"}, Platforms: []string{"wsl", "linux"}, Favorite: true,
		Portable: &Portable{Program: "git", Args: []string{"status", "--short"}, PassArguments: true},
	}}}
}

func TestCanonicalCatalogGolden(t *testing.T) {
	encoded, diagnostics := Encode(validCatalog())
	if len(diagnostics) != 0 {
		t.Fatalf("Encode diagnostics: %#v", diagnostics)
	}
	want, err := os.ReadFile("testdata/catalog-v1/canonical.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, want) {
		t.Fatalf("canonical output mismatch\nwant:\n%s\ngot:\n%s", want, encoded)
	}
	decoded, diagnostics := Decode(encoded)
	if len(diagnostics) != 0 || len(decoded.Entries) != 1 {
		t.Fatalf("Decode = %#v, %#v", decoded, diagnostics)
	}
}

func TestDecodeRejectsUnsafeStructure(t *testing.T) {
	tests := []string{
		`{"schema_version":1,"schema_version":1,"entries":[]}`,
		`{"schema_version":1,"entries":null}`,
		`{"schema_version":1,"entries":[],"extra":true}`,
		`{"schema_version":1,"entries":[{"id":"87f4d803c44a4d8792c4824f8e0bc3f1","name":"x","kind":"command","portable":{"program":"git","args":[]}}]}`,
		`{"schema_version":1,"entries":[]} {}`,
	}
	for _, input := range tests {
		if _, diagnostics := Decode([]byte(input)); len(diagnostics) == 0 {
			t.Errorf("Decode accepted %s", input)
		}
	}
}

func TestCatalogImportAllowlist(t *testing.T) {
	allowed := map[string]bool{"bytes": true, "crypto/sha256": true, "encoding/hex": true, "encoding/json": true, "errors": true, "fmt": true, "io": true, "regexp": true, "sort": true, "strings": true, "unicode/utf8": true}
	packages, err := parser.ParseDir(token.NewFileSet(), ".", func(info os.FileInfo) bool { return !strings.HasSuffix(info.Name(), "_test.go") }, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range packages {
		for _, file := range pkg.Files {
			for _, spec := range file.Imports {
				path, err := strconv.Unquote(spec.Path.Value)
				if err != nil {
					t.Fatal(err)
				}
				if !allowed[path] {
					t.Errorf("production import %q is outside the phase 2 allowlist", path)
				}
			}
		}
	}
}

func TestCatalogLimitBoundaries(t *testing.T) {
	if _, diagnostics := Decode(bytes.Repeat([]byte{' '}, MaxDocumentBytes+1)); len(diagnostics) == 0 || diagnostics[0].Code != "document_too_large" {
		t.Fatalf("oversized input diagnostics: %#v", diagnostics)
	}
	value := validCatalog()
	value.Entries[0].Portable.Args = []string{strings.Repeat("x", 65537)}
	if diagnostics := Validate(value); len(diagnostics) == 0 {
		t.Fatal("oversized argument passed")
	}
}

func TestNativeImplementationShapeMatrix(t *testing.T) {
	aliasValue, body := "git status", "echo ok"
	tests := []struct {
		name  string
		entry Entry
		valid bool
	}{
		{"command", Entry{ID: "00000000000000000000000000000000", Name: "x", Kind: "command", Native: map[string]NativeImplementation{"bash": {AliasValue: &aliasValue}}}, true},
		{"function", Entry{ID: "00000000000000000000000000000000", Name: "x", Kind: "function", Native: map[string]NativeImplementation{"zsh": {FunctionBody: &body}}}, true},
		{"mismatch", Entry{ID: "00000000000000000000000000000000", Name: "x", Kind: "command", Native: map[string]NativeImplementation{"bash": {FunctionBody: &body}}}, false},
	}
	for _, test := range tests {
		diagnostics := Validate(Catalog{SchemaVersion: 1, Entries: []Entry{test.entry}})
		if (len(diagnostics) == 0) != test.valid {
			t.Errorf("%s diagnostics: %#v", test.name, diagnostics)
		}
	}
}

func TestCanonicalCatalogPermutations(t *testing.T) {
	first := validCatalog()
	second := validCatalog()
	second.Entries[0].Tags = []string{"git", "daily"}
	second.Entries[0].Platforms = []string{"linux", "wsl"}
	a, _ := Encode(first)
	b, _ := Encode(second)
	if !bytes.Equal(a, b) {
		t.Fatalf("permutations differ\n%s\n%s", a, b)
	}
}

func TestValidateCrossFieldAndPortableRules(t *testing.T) {
	value := validCatalog()
	value.Entries = append(value.Entries,
		Entry{ID: "f98010ca0aa84af69fd4df32ec91726b", Name: "fn.bad", Kind: "function", Portable: &Portable{Program: "cd", Args: []string{}, PassArguments: false}},
		Entry{ID: "00000000000000000000000000000000", Name: "gs", Kind: "command", Native: map[string]NativeImplementation{"fish": {AliasValue: text("x")}}},
	)
	diagnostics := Validate(value)
	codes := map[string]bool{}
	for _, item := range diagnostics {
		codes[item.Code] = true
	}
	for _, code := range []string{"invalid_name", "portable_function", "invalid_program", "duplicate_name", "invalid_native_shell"} {
		if !codes[code] {
			t.Errorf("missing diagnostic %s in %#v", code, diagnostics)
		}
	}
}

func TestNormalizeIsIdempotentAndIndependent(t *testing.T) {
	first := Normalize(validCatalog())
	second := Normalize(first)
	one, _ := Encode(first)
	two, _ := Encode(second)
	if !bytes.Equal(one, two) {
		t.Fatalf("normalization is not idempotent")
	}
	first.Entries[0].Tags[0] = "changed"
	if validCatalog().Entries[0].Tags[0] == "changed" {
		t.Fatal("Normalize retained input slice")
	}
}

func TestDiagnosticsAreBounded(t *testing.T) {
	value := Catalog{SchemaVersion: 0, Entries: make([]Entry, 200)}
	diagnostics := Validate(value)
	if len(diagnostics) != MaxDiagnostics+1 {
		t.Fatalf("got %d diagnostics", len(diagnostics))
	}
	last := diagnostics[len(diagnostics)-1]
	if last.Code != "diagnostics_truncated" || last.Omitted == 0 {
		t.Fatalf("bad truncation record: %#v", last)
	}
}

func TestCompareUsesStableIDs(t *testing.T) {
	left := validCatalog()
	right := Normalize(left)
	right.Entries[0].Name = "status"
	changes, diagnostics := Compare(left, right)
	if len(diagnostics) != 0 || len(changes) != 1 || strings.Join(changes[0].Fields, ",") != "name" {
		t.Fatalf("Compare = %#v, %#v", changes, diagnostics)
	}
}

func FuzzDecode(f *testing.F) {
	f.Add([]byte(`{"schema_version":1,"entries":[]}`))
	f.Fuzz(func(t *testing.T, data []byte) { Decode(data) })
}
