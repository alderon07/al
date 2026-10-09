//go:build !windows

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/alderon07/al/internal/app"
	neutralcatalog "github.com/alderon07/al/internal/catalog"
	shellapi "github.com/alderon07/al/internal/shell"
)

func TestCatalogPreviewDetailsReadOnlyAndPartialImport(t *testing.T) {
	for _, name := range []string{"bash", "zsh"} {
		t.Run(name, func(t *testing.T) {
			home := privateTestHome(t)
			t.Setenv("HOME", home)
			source := []byte("alias safe='echo synthetic'\nalias unsupported=\"echo $HOME\"\nunsupported_fn() {\n echo $(touch \"$HOME/executed\")\n}\nalias grouped='echo synthetic'\n")
			path := filepath.Join(home, "."+name+"_aliases")
			if err := os.WriteFile(path, source, 0o600); err != nil {
				t.Fatal(err)
			}
			previousValidator := applicationDependencies.ValidateSyntax
			applicationDependencies.ValidateSyntax = func(context.Context, string, []byte) error { return nil }
			t.Cleanup(func() { applicationDependencies.ValidateSyntax = previousValidator })
			previousOutput := catalogWorkflowStdout
			var output bytes.Buffer
			catalogWorkflowStdout = &output
			t.Cleanup(func() { catalogWorkflowStdout = previousOutput })
			before := shadowTreeManifest(t, home)
			for _, args := range [][]string{{"--from", name}, {"--from", name, "--json"}} {
				output.Reset()
				code, err := runCatalogPreviewCommand(args)
				if err != nil || code != 1 {
					t.Fatalf("code=%d error=%v", code, err)
				}
				for _, expected := range []string{"alias_quoting", "function_command_substitution", "expansion timing", "Scanner stopped at line 4", "remaining lines are grouped", "Next:"} {
					if !strings.Contains(output.String(), expected) {
						t.Fatalf("missing %q in %s", expected, output.String())
					}
				}
				if strings.Contains(output.String(), "touch") || strings.Contains(output.String(), "$HOME") || strings.Contains(output.String(), "echo synthetic") {
					t.Fatal("preview disclosed source commands")
				}
				if len(args) == 3 {
					var report app.ShadowReport
					if err := json.Unmarshal(output.Bytes(), &report); err != nil {
						t.Fatal(err)
					}
					if report.SchemaVersion != 1 || report.Summary.Equivalent != 1 || report.Summary.Unsupported != 2 {
						t.Fatalf("unexpected preview report %+v", report)
					}
				}
			}
			if !reflect.DeepEqual(before, shadowTreeManifest(t, home)) {
				t.Fatal("preview modified private files")
			}
			output.Reset()
			code, err := runCatalogImportCommand([]string{"--from", name})
			if err != nil || code != 0 {
				t.Fatalf("partial import code=%d error=%v", code, err)
			}
			encoded, err := os.ReadFile(catalogPathFixture())
			if err != nil {
				t.Fatal(err)
			}
			catalog, diagnostics := neutralcatalog.Decode(encoded)
			if len(diagnostics) != 0 || len(catalog.Entries) != 1 || catalog.Entries[0].Name != "safe" {
				t.Fatalf("import copied unproven definitions: %+v", catalog)
			}
			unchanged, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(unchanged, source) {
				t.Fatal("partial import modified native definitions")
			}
		})
	}
}

func TestCatalogPreviewDiagnosticBudgetAndPrivacy(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	source := strings.Repeat("alias a=\"SYNTHETIC_PRIVATE_MARKER\"\n", 105)
	source += "# token: ghp_" + strings.Repeat("S", 36) + "\n"
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := applicationServices().InspectCatalogPreview("bash")
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, result := range report.Results {
		total += len(result.Diagnostics)
	}
	if total != 100 || len(report.Diagnostics) != 1 || report.Diagnostics[0].Code != "diagnostics_truncated" {
		t.Fatalf("diagnostic budget changed: %d %+v", total, report.Diagnostics)
	}
	for _, jsonOutput := range []bool{false, true} {
		output, err := renderCatalogPreviewReport(report, jsonOutput)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(output, []byte("diagnostics_truncated")) {
			t.Fatal("truncation not displayed")
		}
		if !jsonOutput && !bytes.Contains(output, []byte("Details unavailable or omitted")) {
			t.Fatal("omitted range lacks fallback")
		}
		if bytes.Contains(output, []byte("SYNTHETIC_PRIVATE_MARKER")) || bytes.Contains(output, []byte("ghp_")) {
			t.Fatal("preview disclosed private source")
		}
	}
}

type previewBrokenWriter struct{ err error }

func (writer previewBrokenWriter) Write(contents []byte) (int, error) {
	return len(contents) - 1, writer.err
}

func TestCatalogPreviewDetectsOutputFailures(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), []byte("export EXAMPLE=synthetic\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	previous := catalogWorkflowStdout
	t.Cleanup(func() { catalogWorkflowStdout = previous })
	for _, failure := range []error{nil, io.ErrClosedPipe} {
		catalogWorkflowStdout = previewBrokenWriter{failure}
		code, err := runCatalogPreviewCommand([]string{"--from", "bash"})
		expected := failure
		if expected == nil {
			expected = io.ErrShortWrite
		}
		if code != 1 || !errors.Is(err, expected) {
			t.Fatalf("code=%d error=%v expected=%v", code, err, expected)
		}
	}
}

func TestCatalogPreviewDisplaysGlobalDiagnostics(t *testing.T) {
	report := app.ShadowReport{Shell: "bash", Diagnostics: []shellapi.Diagnostic{{Code: "inspection_failure", Message: "Inspection could not complete. Next: run al catalog preview again."}}}
	output, err := renderCatalogPreviewReport(report, false)
	if err != nil || !bytes.Contains(output, []byte("report [inspection_failure]")) {
		t.Fatalf("missing report-level diagnostic %s %v", output, err)
	}
}

func TestCatalogPreviewPreservesAcceptedNamesAndCoordinates(t *testing.T) {
	report := app.ShadowReport{SchemaVersion: 1, Shell: "bash", Diagnostics: []shellapi.Diagnostic{}, Results: []shellapi.Result{
		{Name: "ll", Status: "equivalent", StartLine: 1, EndLine: 1, Diagnostics: []shellapi.Diagnostic{}},
		{Name: "dup", Status: "duplicate", StartLine: 2, EndLine: 2, Diagnostics: []shellapi.Diagnostic{}},
		{Name: "SYNTHETIC_PRIVATE_MARKER", Status: "unsupported", StartLine: 3, EndLine: 3, Diagnostics: []shellapi.Diagnostic{}},
		{Name: "escaped\x1b[31m\nname", Status: "equivalent", StartLine: 4, EndLine: 4, Diagnostics: []shellapi.Diagnostic{}},
	}}
	output, err := renderCatalogPreviewReport(report, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"ll (lines 1-1)", "dup (lines 2-2)", "unsupported lines 3-3", `escaped\x1b[31m\x0aname (lines 4-4)`} {
		if !bytes.Contains(output, []byte(expected)) {
			t.Fatalf("missing %q in %s", expected, output)
		}
	}
	if bytes.Contains(output, []byte("SYNTHETIC_PRIVATE_MARKER")) || bytes.Contains(output, []byte{0x1b}) {
		t.Fatal("plain preview revealed unsupported text or terminal controls")
	}
	previewJSON, err := renderCatalogPreviewReport(report, true)
	if err != nil {
		t.Fatal(err)
	}
	shadowJSON, err := renderShadowReport(report, true)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(previewJSON, shadowJSON) {
		t.Fatal("plain labels changed preview JSON contract")
	}
}

func TestCatalogPreviewHidesSecretNamesBeforeDiagnosticTruncation(t *testing.T) {
	for _, name := range []string{"bash", "zsh"} {
		t.Run(name, func(t *testing.T) {
			home := privateTestHome(t)
			t.Setenv("HOME", home)
			token := "ghp_" + strings.Repeat("S", 36)
			source := strings.Repeat("alias "+token+"='echo synthetic'\n", 105)
			source += "# SYNTHETIC_PRIVATE_DESCRIPTION " + token + "\nalias attached='echo synthetic'\n"
			source += "body() { echo '" + token + "'; }\n"
			if err := os.WriteFile(filepath.Join(home, "."+name+"_aliases"), []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			report, err := applicationServices().InspectCatalogPreview(name)
			if err != nil {
				t.Fatal(err)
			}
			if report.Summary.Blocked != 107 {
				t.Fatalf("expected all secret ranges blocked: %+v", report.Summary)
			}
			omitted := 0
			for _, result := range report.Results {
				if result.Status != "blocked" || result.Name != "" {
					t.Fatalf("secret range retained its name: status=%s", result.Status)
				}
				if len(result.Diagnostics) == 0 {
					omitted++
				}
			}
			if omitted == 0 {
				t.Fatal("test did not exercise names hidden before diagnostic truncation")
			}
			for _, jsonOutput := range []bool{false, true} {
				output, err := renderCatalogPreviewReport(report, jsonOutput)
				if err != nil {
					t.Fatal(err)
				}
				for _, private := range []string{token, "SYNTHETIC_PRIVATE_DESCRIPTION", "echo synthetic"} {
					if bytes.Contains(output, []byte(private)) {
						t.Fatal("preview disclosed secret name or private source")
					}
				}
				if jsonOutput && bytes.Contains(output, []byte(`"name":`)) {
					t.Fatal("preview JSON retained a secret-blocked name")
				}
			}
		})
	}
}
