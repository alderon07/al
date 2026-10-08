//go:build !windows

package main

import (
	"bytes"
	"context"
	"crypto/sha256"

	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"runtime"

	"strings"

	neutralcatalog "github.com/alderon07/al/internal/catalog"
	shellapi "github.com/alderon07/al/internal/shell"
	"testing"

	"github.com/alderon07/al/internal/app"
)

func TestShadowRenderDeterministic(t *testing.T) {
	bashValue := "echo bash"
	zshValue := "echo zsh"
	entry := neutralcatalog.Entry{
		ID:   strings.Repeat("a", 32),
		Name: "x",
		Kind: "command",
		Native: map[string]neutralcatalog.NativeImplementation{
			"zsh":  {AliasValue: &zshValue},
			"bash": {AliasValue: &bashValue},
		},
	}
	want := renderShadowEntry(entry, strings.Repeat("b", 64))
	for index := 0; index < 100; index++ {
		if got := renderShadowEntry(entry, strings.Repeat("b", 64)); !bytes.Equal(got, want) {
			t.Fatal("renderer depends on map iteration order")
		}
	}
}

func shadowTreeManifest(t *testing.T, root string) map[string]string {
	t.Helper()
	manifest := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			manifest[relative] = fmt.Sprintf("directory:%o", info.Mode().Perm())
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(contents)
		manifest[relative] = fmt.Sprintf("file:%o:%x", info.Mode().Perm(), sum)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

func repeatedShadowLines(format string, count int) string {
	var output strings.Builder
	for index := 0; index < count; index++ {
		fmt.Fprintf(&output, format, index)
	}
	return output.String()
}

func TestShadowReportErrorMatrix(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), []byte("alias x='true'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	originalValidator := applicationDependencies.ValidateSyntax
	originalStdout := shadowStdout
	t.Cleanup(func() {
		applicationDependencies.ValidateSyntax = originalValidator
		shadowStdout = originalStdout
	})

	applicationDependencies.ValidateSyntax = func(context.Context, string, []byte) error { panic("private canary") }
	code, err := runCatalogCommand([]string{"shadow", "--shell", "bash", "--json"})
	if code != 2 || err == nil || strings.Contains(err.Error(), "canary") {
		t.Fatalf("panic result = %d, %v", code, err)
	}

	applicationDependencies.ValidateSyntax = func(context.Context, string, []byte) error { return nil }
	shadowStdout = shortShadowWriter{}
	code, err = runCatalogCommand([]string{"shadow", "--shell", "bash", "--json"})
	if code != 2 || !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write result = %d, %v", code, err)
	}
}

type shortShadowWriter struct{}

func (shortShadowWriter) Write(contents []byte) (int, error) {
	if len(contents) == 0 {
		return 0, nil
	}
	return len(contents) - 1, nil
}

func TestShadowPlainReportGolden(t *testing.T) {
	report := shadowGoldenReport()
	output, err := renderShadowReport(report, false)
	if err != nil {
		t.Fatal(err)
	}
	want := "Alias Lens shadow inspection · bash\n\n" +
		"invalid     inv\n" +
		"blocked     block\n" +
		"duplicate   dup\n" +
		"different   diff\n" +
		"unsupported unsup\n" +
		"equivalent  ok\n\n" +
		"1 equivalent · 1 unsupported · 1 different · 1 duplicate · 1 invalid · 1 blocked\n"
	if string(output) != want {
		t.Fatalf("plain report:\n%s", output)
	}
}

func TestShadowJSONReportGolden(t *testing.T) {
	report := shadowGoldenReport()
	output, err := renderShadowReport(report, true)
	if err != nil {
		t.Fatal(err)
	}
	var decoded app.ShadowReport
	if err := json.Unmarshal(output, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SchemaVersion != 1 || decoded.Shell != "bash" || len(decoded.Results) != 6 || decoded.Summary.Equivalent != 1 || decoded.Summary.Blocked != 1 {
		t.Fatalf("JSON report = %#v", decoded)
	}
	for _, forbidden := range []string{"origin", "rendered", "entry", "/tmp/"} {
		if strings.Contains(string(output), forbidden) {
			t.Fatalf("JSON contains %q", forbidden)
		}
	}
}

func shadowGoldenReport() app.ShadowReport {
	report := app.ShadowReport{SchemaVersion: 1, Shell: "bash", Diagnostics: []shellapi.Diagnostic{}, Results: []shellapi.Result{
		{Unit: 5, Name: "ok", Kind: "command", Status: "equivalent", StartByte: 50, EndByte: 51, StartLine: 6, EndLine: 6, Diagnostics: []shellapi.Diagnostic{}},
		{Unit: 4, Name: "unsup", Status: "unsupported", StartByte: 40, EndByte: 41, StartLine: 5, EndLine: 5, Diagnostics: []shellapi.Diagnostic{}},
		{Unit: 3, Name: "diff", Kind: "command", Status: "different", StartByte: 30, EndByte: 31, StartLine: 4, EndLine: 4, DifferentFields: []string{"description"}, Diagnostics: []shellapi.Diagnostic{}},
		{Unit: 2, Name: "dup", Kind: "command", Status: "duplicate", StartByte: 20, EndByte: 21, StartLine: 3, EndLine: 3, Diagnostics: []shellapi.Diagnostic{}},
		{Unit: 1, Name: "block", Kind: "command", Status: "blocked", StartByte: 10, EndByte: 11, StartLine: 2, EndLine: 2, Diagnostics: []shellapi.Diagnostic{}},
		{Unit: 0, Name: "inv", Kind: "command", Status: "invalid", StartByte: 0, EndByte: 1, StartLine: 1, EndLine: 1, Diagnostics: []shellapi.Diagnostic{}},
	}}
	for i, j := 0, len(report.Results)-1; i < j; i, j = i+1, j-1 {
		report.Results[i], report.Results[j] = report.Results[j], report.Results[i]
	}
	report.Summary.Equivalent = 1
	report.Summary.Unsupported = 1
	report.Summary.Different = 1
	report.Summary.Duplicate = 1
	report.Summary.Invalid = 1
	report.Summary.Blocked = 1
	return report
}

func TestShadowLinuxMatrix(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux matrix")
	}
	testShadowEnvironmentMatrix(t)
}

func TestShadowMacOSMatrix(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS matrix")
	}
	testShadowEnvironmentMatrix(t)
}

func testShadowEnvironmentMatrix(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		shell := shell
		t.Run(shell, func(t *testing.T) {
			validator, err := exec.LookPath(shell)
			if err != nil {
				validator, err = exec.LookPath("bash")
				if err != nil {
					t.Skipf("%s is not installed", shell)
				}
			}
			originalValidator := applicationDependencies.FindTrustedShell
			applicationDependencies.FindTrustedShell = func(string) (string, error) { return validator, nil }
			t.Cleanup(func() { applicationDependencies.FindTrustedShell = originalValidator })
			home := t.TempDir()
			t.Setenv("HOME", home)
			source := []byte("# List files\n# al: tags=files platforms=linux,wsl favorite=true category=files\nalias ll='ls -la'\n\ncproj() {\n cd \"$HOME/code\"\n}\n")
			filename := ".bash_aliases"
			if shell == "zsh" {
				filename = ".zsh_aliases"
			}
			if err := os.WriteFile(filepath.Join(home, filename), source, 0o600); err != nil {
				t.Fatal(err)
			}
			report, err := applicationServices().InspectCatalogShadow(shell)
			if err != nil {
				t.Fatal(err)
			}
			plain, err := renderShadowReport(report, false)
			if err != nil {
				t.Fatal(err)
			}
			jsonReport, err := renderShadowReport(report, true)
			if err != nil {
				t.Fatal(err)
			}
			assertShadowHash(t, filepath.Join("testdata", "phase4", shell+".sha256"), "plain", plain)
			assertShadowHash(t, filepath.Join("testdata", "phase4", shell+".sha256"), "json", jsonReport)
		})
	}
}

func assertShadowHash(t *testing.T, path, label string, contents []byte) {
	t.Helper()
	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(contents)
	want := label + " " + fmt.Sprintf("%x", sum)
	if !strings.Contains(string(expected), want) {
		t.Fatalf("%s hash is %s", label, want)
	}
}
