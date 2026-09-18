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
	"reflect"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unicode/utf8"

	neutralcatalog "alias-lens/internal/catalog"
)

func TestShadowOriginKnownAnswer(t *testing.T) {
	// The approved vector uses a source whose SHA-256 is all zeroes. Exercise the
	// framing helper separately because no source bytes have that known digest.
	if got := shadowOriginFromDigest("bash", make([]byte, 32), 0, 10); got != "0e4470091b064fefd0a1574d8aa97801eab2016fe14c3fc7fe88ba518b360bfb" {
		t.Fatalf("origin vector = %s", got)
	}
}

func TestShadowGenerationHashVectors(t *testing.T) {
	approval := sha256.Sum256(nil)
	got := shadowGenerationHash("bash/v1", "linux", approval[:], []byte("# body\n"))
	if got != "06fc5ce2f0aaa98290cf5ceecb582c406e0b4249606891779927bcf8f86bc205" {
		t.Fatalf("generation vector = %s", got)
	}
}

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

func TestShadowImportSupportedDefinitions(t *testing.T) {
	source := []byte("# Show files\n# al: tags=files platforms=linux favorite=true category=files\nalias ll='ls -la'\n\ncproj() {\n cd \"$HOME/code\"\n}\n")
	results := importShadowSource("bash", source)
	if len(results) != 2 {
		t.Fatalf("results = %#v", results)
	}
	if results[0].Name != "ll" || results[0].Kind != "command" || results[0].entry.Description != "Show files" {
		t.Fatalf("alias result = %#v", results[0])
	}
	if results[1].Name != "cproj" || results[1].Kind != "function" {
		t.Fatalf("function result = %#v", results[1])
	}
	body := *results[1].entry.Native["bash"].FunctionBody
	if body != "\n cd \"$HOME/code\"\n" {
		t.Fatalf("function body changed: %q", body)
	}
}

func TestShadowCommentOwnershipAndHorizontalWhitespace(t *testing.T) {
	source := []byte("## section\n#ordinary-without-required-space\n\t# Describes x\nalias\tx='echo x' # trailing\n")
	results := importShadowSource("bash", source)
	if len(results) != 1 || results[0].entry == nil {
		t.Fatalf("results = %#v", results)
	}
	if results[0].entry.Description != "Describes x" || results[0].StartLine != 3 {
		t.Fatalf("comment ownership = %#v", results[0])
	}
	for _, invalid := range []string{"alias x='echo x'#not-comment\n", "alias x='echo x'\\\nalias y='true'\n"} {
		results = importShadowSource("bash", []byte(invalid))
		if len(results) != 1 || results[0].Status != "unsupported" || results[0].EndByte != len(invalid) {
			t.Errorf("invalid trailing syntax = %#v", results)
		}
	}
}

func TestShadowRejectsUnsafeAliasSyntax(t *testing.T) {
	for _, source := range []string{"alias x=\"$(touch /tmp/no)\"\n", "alias x='one' y='two'\n", "alias x=$HOME\n"} {
		results := importShadowSource("bash", []byte(source))
		if len(results) != 1 || results[0].Status != "unsupported" {
			t.Errorf("source %q = %#v", source, results)
		}
	}
}

func TestShadowAmbiguousRangeConsumesEOF(t *testing.T) {
	for _, source := range []string{
		"broken() {\n echo nope\nalias recovered='false'\n",
		"alias broken='unterminated\nalias recovered='false'\n",
	} {
		results := importShadowSource("bash", []byte(source))
		if len(results) != 1 || results[0].Status != "unsupported" || results[0].EndByte != len(source) {
			t.Fatalf("ambiguous source produced %#v", results)
		}
		if results[0].Name == "recovered" {
			t.Fatal("recovered a definition from an ambiguous range")
		}
	}
}

func TestShadowHeredocRanges(t *testing.T) {
	source := []byte("show() {\n cat <<'EOF'\n } inside data\nEOF\n echo done\n}\nalias next='true'\n")
	results := importShadowSource("bash", source)
	if len(results) != 2 || results[0].Name != "show" || results[1].Name != "next" {
		t.Fatalf("closed heredoc ranges = %#v", results)
	}
	body := *results[0].entry.Native["bash"].FunctionBody
	if body != "\n cat <<'EOF'\n } inside data\nEOF\n echo done\n" {
		t.Fatalf("heredoc body changed: %q", body)
	}

	unclosed := []byte("show() {\n cat <<EOF\n } inside data\nalias hidden='true'\n")
	results = importShadowSource("bash", unclosed)
	if len(results) != 1 || results[0].Status != "unsupported" || results[0].EndByte != len(unclosed) {
		t.Fatalf("unclosed heredoc = %#v", results)
	}
}

func TestShadowArithmeticShiftIsNotHeredoc(t *testing.T) {
	source := []byte("shifted() {\n echo $((8 << 2))\n}\nalias next='true'\n")
	results := importShadowSource("bash", source)
	if len(results) != 2 || results[0].Name != "shifted" || results[1].Name != "next" {
		t.Fatalf("arithmetic shift ranges = %#v", results)
	}
}

func TestShadowUnsupportedHeredocRange(t *testing.T) {
	closed := []byte("cat <<EOF\nnot an alias }\nEOF\nalias next='true'\n")
	results := importShadowSource("bash", closed)
	if len(results) != 2 || results[0].EndLine != 3 || results[1].Name != "next" {
		t.Fatalf("closed unsupported heredoc = %#v", results)
	}
	unclosed := []byte("cat <<EOF\nnot an alias\nalias hidden='true'\n")
	results = importShadowSource("bash", unclosed)
	if len(results) != 1 || results[0].EndByte != len(unclosed) || results[0].Diagnostics[0].Code != "ambiguous_syntax" {
		t.Fatalf("unclosed unsupported heredoc = %#v", results)
	}
}

func TestShadowMetadataGrammar(t *testing.T) {
	valid := "# al: tags=git,files platforms=linux,wsl favorite=true category=tools\nalias x='echo x'\n"
	results := importShadowSource("bash", []byte(valid))
	if len(results) != 1 || results[0].entry == nil || results[0].entry.Category != "tools" {
		t.Fatalf("valid metadata = %#v", results)
	}
	invalid := []string{
		"# al: category=tools tags=git",
		"# al: tags=Git",
		"# al: tags=git,git",
		"# al: platforms=wsl,linux",
		"# al: favorite=false",
		"# al: tags=git  category=tools",
	}
	for _, metadata := range invalid {
		results = importShadowSource("bash", []byte(metadata+"\nalias x='echo x'\n"))
		if len(results) != 1 || results[0].Status != "unsupported" || results[0].entry != nil {
			t.Errorf("metadata %q = %#v", metadata, results)
		}
	}
}

func TestShadowDuplicateSetIsExcluded(t *testing.T) {
	results := importShadowSource("bash", []byte("alias x='one'\nalias x='two'\n"))
	markShadowDuplicates(results)
	for _, result := range results {
		if result.Status != "duplicate" || result.entry != nil {
			t.Fatalf("duplicate result = %#v", result)
		}
	}
}

func TestShadowSecretsAreBlockedWithoutValue(t *testing.T) {
	secret := "ghp_abcdefghijklmnopqrstuvwxyz123456"
	source := []byte("alias token='echo " + secret + "'\n")
	results := importShadowSource("bash", source)
	results = blockShadowSecrets(results, findSecretFindings(source), source)
	if results[0].Status != "blocked" || results[0].entry != nil {
		t.Fatalf("result = %#v", results[0])
	}
	report := shadowReport{SchemaVersion: 1, Shell: "bash", Diagnostics: []shadowDiagnostic{}, Results: results}
	finalizeShadowReport(&report)
	output, err := renderShadowReport(report, true)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(output, []byte(secret)) {
		t.Fatal("JSON report leaked secret")
	}
	if bytes.Contains(output, []byte("origin")) {
		t.Fatal("JSON report exposed internal origin")
	}
}

func TestShadowRawSecretScanMatrix(t *testing.T) {
	secret := "ghp_abcdefghijklmnopqrstuvwxyz123456"
	for _, source := range []string{
		"# " + secret + "\n",
		"# " + secret + "\nalias x='echo safe'\n",
		"broken() { # " + secret + "\n",
	} {
		contents := []byte(source)
		results := blockShadowSecrets(importShadowSource("bash", contents), findSecretFindings(contents), contents)
		found := false
		for _, result := range results {
			found = found || result.Status == "blocked"
		}
		if !found {
			t.Errorf("secret was not blocked for fixture %d", len(source))
		}
		report := shadowReport{SchemaVersion: 1, Shell: "bash", Diagnostics: []shadowDiagnostic{}, Results: results}
		finalizeShadowReport(&report)
		output, err := renderShadowReport(report, true)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(output, []byte(secret)) {
			t.Fatal("report leaked raw secret")
		}
	}
}

func TestShadowUnitsFollowSourceRanges(t *testing.T) {
	secret := "ghp_abcdefghijklmnopqrstuvwxyz123456"
	source := []byte("# " + secret + "\n\nalias x='true'\n")
	results := blockShadowSecrets(importShadowSource("bash", source), findSecretFindings(source), source)
	assignShadowUnits(results)
	if len(results) != 2 {
		t.Fatalf("results = %#v", results)
	}
	for _, result := range results {
		if result.Status == "blocked" && result.Unit != 0 {
			t.Fatalf("blocked source unit = %d", result.Unit)
		}
		if result.Name == "x" && result.Unit != 1 {
			t.Fatalf("alias source unit = %d", result.Unit)
		}
	}
}

func TestShadowInspectionIsReadOnly(t *testing.T) {
	originalValidator := shadowValidatorPath
	shadowValidatorPath = func(shell string) (string, error) { return exec.LookPath(shell) }
	t.Cleanup(func() { shadowValidatorPath = originalValidator })
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	aliasPath := filepath.Join(home, ".bash_aliases")
	contents := []byte("alias ok='echo OK'\n")
	if err := os.WriteFile(aliasPath, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := inspectCatalogShadow("bash")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Results) != 1 || report.Results[0].Status != "equivalent" {
		t.Fatalf("report = %#v", report)
	}
	after, err := os.ReadFile(aliasPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(contents, after) {
		t.Fatal("alias file changed")
	}
	if _, err := os.Stat(filepath.Join(home, ".config")); !os.IsNotExist(err) {
		t.Fatalf("shadow created config state: %v", err)
	}
}

func TestShadowReadOnlyManifest(t *testing.T) {
	originalValidator := shadowValidatorPath
	shadowValidatorPath = func(shell string) (string, error) { return exec.LookPath("bash") }
	t.Cleanup(func() { shadowValidatorPath = originalValidator })
	home := t.TempDir()
	t.Setenv("HOME", home)
	files := map[string]string{
		".bash_aliases":                              "alias ok='echo OK'\n",
		".bashrc":                                    "# user startup\n",
		".bash_history":                              "echo private\n",
		".config/alias-lens/config.json":             `{"version":1,"shell":"bash"}`,
		".local/share/alias-lens/usage.json":         "{}\n",
		".local/share/alias-lens/revisions/keep.txt": "private revision\n",
		"dotfiles/.git/index":                        "private index\n",
		"dotfiles/unrelated.txt":                     "keep\n",
	}
	for name, contents := range files {
		path := filepath.Join(home, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	before := shadowTreeManifest(t, home)
	if _, err := inspectCatalogShadow(""); err != nil {
		t.Fatal(err)
	}
	after := shadowTreeManifest(t, home)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("home changed\nbefore: %#v\nafter: %#v", before, after)
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

func TestShadowMissingAliasDoesNotCreateFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	_, err := inspectCatalogShadow("bash")
	if err == nil {
		t.Fatal("missing alias file passed")
	}
	if _, statErr := os.Stat(filepath.Join(home, ".bash_aliases")); !os.IsNotExist(statErr) {
		t.Fatalf("missing alias file was created: %v", statErr)
	}
}

func TestShadowReadOnlyConfigSelection(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configDirectory := filepath.Join(home, ".config", "alias-lens")
	if err := os.MkdirAll(configDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	config := []byte(`{"version":0,"shell":"zsh"}`)
	path := filepath.Join(configDirectory, "config.json")
	if err := os.WriteFile(path, config, 0o600); err != nil {
		t.Fatal(err)
	}
	adapter, err := shadowShellAdapter("")
	if err != nil {
		t.Fatal(err)
	}
	if adapter.Name() != "zsh" {
		t.Fatalf("adapter = %s", adapter.Name())
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(config, after) {
		t.Fatal("shadow migrated config")
	}
	if _, err := os.Stat(path + ".alias-lens.bak"); !os.IsNotExist(err) {
		t.Fatal("shadow created config backup")
	}
}

func TestShadowReadOnlyConfigMatrix(t *testing.T) {
	tests := []struct {
		name, config, shell, errorText string
	}{
		{name: "versionless", config: `{"shell":"zsh"}`, shell: "zsh"},
		{name: "version zero", config: `{"version":0,"shell":"bash"}`, shell: "bash"},
		{name: "version one", config: `{"version":1,"shell":"zsh"}`, shell: "zsh"},
		{name: "empty shell", config: `{"version":1,"shell":""}`, shell: "bash"},
		{name: "negative", config: `{"version":-1}`, errorText: "invalid configuration version"},
		{name: "future", config: `{"version":2}`, errorText: "configuration version unsupported"},
		{name: "null version", config: `{"version":null}`, errorText: "invalid configuration"},
		{name: "null shell", config: `{"shell":null}`, errorText: "invalid configuration"},
		{name: "wrong type", config: `{"version":"one"}`, errorText: "invalid configuration"},
		{name: "duplicate", config: `{"shell":"bash","shell":"zsh"}`, errorText: "duplicate configuration field"},
		{name: "unsupported shell", config: `{"shell":"fish"}`, errorText: "unsupported configured shell"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			directory := filepath.Join(home, ".config", "alias-lens")
			if err := os.MkdirAll(directory, 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(directory, "config.json")
			before := []byte(test.config)
			if err := os.WriteFile(path, before, 0o600); err != nil {
				t.Fatal(err)
			}
			adapter, err := shadowShellAdapter("")
			if test.errorText != "" {
				if err == nil || !strings.Contains(err.Error(), test.errorText) {
					t.Fatalf("error = %v", err)
				}
			} else if err != nil || adapter.Name() != test.shell {
				t.Fatalf("adapter = %v, error = %v", adapter, err)
			}
			after, readErr := os.ReadFile(path)
			if readErr != nil || !bytes.Equal(before, after) {
				t.Fatalf("configuration changed: %v", readErr)
			}
		})
	}
}

func TestShadowShellSelectionMatrix(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configDirectory := filepath.Join(home, ".config", "alias-lens")
	if err := os.MkdirAll(configDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDirectory, "config.json"), []byte(`{"version":99,"shell":"zsh"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	adapter, err := shadowShellAdapter("bash")
	if err != nil || adapter.Name() != "bash" {
		t.Fatalf("explicit selection did not bypass config: %v, %v", adapter, err)
	}
}

func TestShadowNonblockingFileTypeMatrix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix file type test")
	}
	directory := t.TempDir()
	regular := filepath.Join(directory, "regular")
	if err := os.WriteFile(regular, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := readShadowRegular(regular, 2); err != nil || string(got) != "ok" {
		t.Fatalf("regular read = %q, %v", got, err)
	}
	link := filepath.Join(directory, "link")
	if err := os.Symlink(regular, link); err != nil {
		t.Fatal(err)
	}
	if _, err := readShadowRegular(link, 2); err != nil {
		t.Fatalf("leaf symlink failed: %v", err)
	}
	fifo := filepath.Join(directory, "fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if _, err := readShadowRegular(fifo, 2); err == nil {
		t.Fatal("FIFO was accepted")
	}
	if time.Since(started) > time.Second {
		t.Fatal("FIFO read blocked")
	}
	if _, err := readShadowRegular(directory, 2); err == nil {
		t.Fatal("directory was accepted")
	}
	if _, err := readShadowRegular(regular, 1); err == nil {
		t.Fatal("oversized file was accepted")
	}
}

func TestShadowConfigResourceLimits(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	directory := filepath.Join(home, ".config", "alias-lens")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "config.json")
	if err := os.WriteFile(path, bytes.Repeat([]byte(" "), shadowConfigLimit+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := shadowShellAdapter(""); err == nil || !strings.Contains(err.Error(), "configuration unreadable") {
		t.Fatalf("oversized config error = %v", err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() != shadowConfigLimit+1 {
		t.Fatalf("config was changed: %v, %v", info, err)
	}
}

func TestShadowResourceLimits(t *testing.T) {
	tests := []struct {
		name, contents, want string
	}{
		{name: "source bytes", contents: string(bytes.Repeat([]byte("x"), shadowSourceLimit+1)), want: "unreadable"},
		{name: "physical line", contents: string(bytes.Repeat([]byte("x"), shadowLineLimit+1)), want: "line over 1 MiB"},
		{name: "definitions", contents: repeatedShadowLines("alias x%d='true'\n", shadowMaxEntries+1), want: "more than 10000"},
		{name: "ranges", contents: repeatedShadowLines("unsupported%d\n", shadowMaxResults+1), want: "more than 20000"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			path := filepath.Join(home, ".bash_aliases")
			if err := os.WriteFile(path, []byte(test.contents), 0o600); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			_, inspectErr := inspectCatalogShadow("bash")
			if inspectErr == nil || !strings.Contains(inspectErr.Error(), test.want) {
				t.Fatalf("error = %v", inspectErr)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("source changed: %v", err)
			}
		})
	}
}

func repeatedShadowLines(format string, count int) string {
	var output strings.Builder
	for index := 0; index < count; index++ {
		fmt.Fprintf(&output, format, index)
	}
	return output.String()
}

func TestShadowNeverExecutesBashContent(t *testing.T) {
	originalValidator := shadowValidatorPath
	shadowValidatorPath = func(shell string) (string, error) { return exec.LookPath("bash") }
	t.Cleanup(func() { shadowValidatorPath = originalValidator })
	home := t.TempDir()
	t.Setenv("HOME", home)
	sentinel := filepath.Join(home, "sentinel")
	source := "alias dangerous='touch " + sentinel + "'\nfn() { touch " + sentinel + "; }\n"
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := inspectCatalogShadow("bash"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatalf("inspected content executed: %v", err)
	}
}

func TestShadowNeverExecutesZshContent(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is not installed")
	}
	originalValidator := shadowValidatorPath
	shadowValidatorPath = func(shell string) (string, error) { return zsh, nil }
	t.Cleanup(func() { shadowValidatorPath = originalValidator })
	home := t.TempDir()
	t.Setenv("HOME", home)
	sentinel := filepath.Join(home, "sentinel")
	source := "alias dangerous='touch " + sentinel + "'\nfn() { touch " + sentinel + "; }\n"
	if err := os.WriteFile(filepath.Join(home, ".zsh_aliases"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := inspectCatalogShadow("zsh"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatalf("inspected content executed: %v", err)
	}
}

func TestShadowValidatorIsolation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix validator test")
	}
	directory := t.TempDir()
	record := filepath.Join(directory, "record")
	script := filepath.Join(directory, "validator")
	scriptBody := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > '" + record + ".args'\n" +
		"env | sort > '" + record + ".env'\n" +
		"cat > '" + record + ".stdin'\n"
	if err := os.WriteFile(script, []byte(scriptBody), 0o700); err != nil {
		t.Fatal(err)
	}
	originalValidator := shadowValidatorPath
	shadowValidatorPath = func(string) (string, error) { return script, nil }
	t.Cleanup(func() { shadowValidatorPath = originalValidator })
	input := []byte("alias x='echo safe'\n")
	if err := validateShadowSyntax(context.Background(), "bash", input); err != nil {
		t.Fatal(err)
	}
	arguments, err := os.ReadFile(record + ".args")
	if err != nil {
		t.Fatal(err)
	}
	if string(arguments) != "--noprofile\n--norc\n-n\n" {
		t.Fatalf("arguments = %q", arguments)
	}
	environment, err := os.ReadFile(record + ".env")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(environment), "PATH=/usr/bin:/bin") || !strings.Contains(string(environment), "BASH_ENV=/dev/null") || !strings.Contains(string(environment), "LC_ALL=C") {
		t.Fatalf("environment = %q", environment)
	}
	stdin, err := os.ReadFile(record + ".stdin")
	if err != nil || !bytes.Equal(stdin, input) {
		t.Fatalf("stdin = %q, %v", stdin, err)
	}
}

func TestShadowValidatorOutputLimits(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix validator test")
	}
	for _, test := range []struct {
		name, command string
	}{
		{name: "stdout", command: "printf output"},
		{name: "stderr limit", command: "head -c 65537 /dev/zero >&2"},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			script := filepath.Join(directory, "validator")
			if err := os.WriteFile(script, []byte("#!/bin/sh\n"+test.command+"\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			originalValidator := shadowValidatorPath
			shadowValidatorPath = func(string) (string, error) { return script, nil }
			t.Cleanup(func() { shadowValidatorPath = originalValidator })
			if err := validateShadowSyntax(context.Background(), "bash", []byte("alias x='true'\n")); err == nil || !shadowValidationBlocked(err) {
				t.Fatalf("validator error = %v", err)
			}
		})
	}
}

func TestShadowValidatorBisectsFailures(t *testing.T) {
	original := shadowSyntaxValidator
	launches := 0
	shadowSyntaxValidator = func(_ context.Context, _ string, contents []byte) error {
		launches++
		if bytes.Contains(contents, []byte("broken")) {
			return &exec.ExitError{}
		}
		return nil
	}
	t.Cleanup(func() { shadowSyntaxValidator = original })
	results := importShadowSource("bash", []byte("alias good='true'\nalias bad='broken'\nalias also_good='true'\n"))
	validateShadowCandidates(results)
	validateShadowRendered("bash", results)
	statuses := map[string]string{}
	for _, result := range results {
		statuses[result.Name] = result.Status
	}
	if statuses["good"] != "equivalent" || statuses["also_good"] != "equivalent" || statuses["bad"] != "invalid" {
		t.Fatalf("statuses = %#v", statuses)
	}
	if launches != 5 {
		t.Fatalf("bisection launches = %d", launches)
	}
}

func TestShadowValidatorLaunchAndConcurrencyCaps(t *testing.T) {
	original := shadowSyntaxValidator
	launches := 0
	active := 0
	maximumActive := 0
	shadowSyntaxValidator = func(_ context.Context, _ string, _ []byte) error {
		launches++
		active++
		if active > maximumActive {
			maximumActive = active
		}
		active--
		return &exec.ExitError{}
	}
	t.Cleanup(func() { shadowSyntaxValidator = original })
	results := make([]shadowResult, 10000)
	for index := range results {
		value := "broken"
		entry := newShadowEntry("bash", fmt.Sprintf("x%d", index), "command", value, "", EntryMetadata{})
		results[index] = newShadowResult([]byte("source"), "bash", index, index+1, index+1, index+1, &entry)
	}
	validateShadowCandidates(results)
	validateShadowRendered("bash", results)
	if launches != shadowMaxLaunches {
		t.Fatalf("launches = %d", launches)
	}
	if maximumActive != 1 {
		t.Fatalf("validator concurrency = %d", maximumActive)
	}
	blocked := 0
	for _, result := range results {
		if result.Status == "blocked" {
			blocked++
		}
	}
	if blocked == 0 {
		t.Fatal("launch cap did not block unresolved batches")
	}
}

func TestShadowValidatorMatrix(t *testing.T) {
	original := shadowSyntaxValidator
	defer func() { shadowSyntaxValidator = original }()
	for _, test := range []struct {
		name   string
		err    error
		status string
	}{
		{name: "success", status: "equivalent"},
		{name: "syntax", err: &exec.ExitError{}, status: "invalid"},
		{name: "missing", err: os.ErrNotExist, status: "blocked"},
		{name: "timeout", err: context.DeadlineExceeded, status: "blocked"},
	} {
		t.Run(test.name, func(t *testing.T) {
			shadowSyntaxValidator = func(context.Context, string, []byte) error { return test.err }
			results := importShadowSource("bash", []byte("alias x='true'\n"))
			validateShadowCandidates(results)
			validateShadowRendered("bash", results)
			if results[0].Status != test.status {
				t.Fatalf("status = %s", results[0].Status)
			}
		})
	}
}

func TestShadowComparisonIdentityFailures(t *testing.T) {
	results := importShadowSource("bash", []byte("alias x='true'\n"))
	validateShadowCandidates(results)
	for _, mutate := range []func([]byte) []byte{
		func(rendered []byte) []byte {
			return bytes.Replace(rendered, []byte("# al-shadow-origin:"), []byte("# moved-origin:"), 1)
		},
		func(rendered []byte) []byte { return append([]byte("# ordinary\n"), rendered...) },
		func(rendered []byte) []byte { return append(rendered, rendered...) },
	} {
		result := results[0]
		result.rendered = mutate(append([]byte(nil), result.rendered...))
		compareShadowRoundTrip("bash", &result)
		if result.Status != "invalid" {
			t.Fatalf("identity mutation status = %s", result.Status)
		}
	}
}

func TestShadowComparisonFieldMatrix(t *testing.T) {
	results := importShadowSource("bash", []byte("# Original\nalias x='true'\n"))
	validateShadowCandidates(results)
	result := results[0]
	result.rendered = bytes.Replace(result.rendered, []byte("# Original\n"), []byte("# Changed\n"), 1)
	compareShadowRoundTrip("bash", &result)
	if result.Status != "different" || !reflect.DeepEqual(result.DifferentFields, []string{"description"}) {
		t.Fatalf("comparison = %#v", result)
	}
}

func TestShadowValidatorKillsProcessGroup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix process-group test")
	}
	directory := t.TempDir()
	pidPath := filepath.Join(directory, "child.pid")
	script := filepath.Join(directory, "validator")
	contents := "#!/bin/sh\nsleep 30 &\necho $! > '" + pidPath + "'\nwait\n"
	if err := os.WriteFile(script, []byte(contents), 0o700); err != nil {
		t.Fatal(err)
	}
	originalValidator := shadowValidatorPath
	shadowValidatorPath = func(string) (string, error) { return script, nil }
	t.Cleanup(func() { shadowValidatorPath = originalValidator })
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	err := validateShadowSyntax(ctx, "bash", []byte("alias x='true'\n"))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("validator error = %v", err)
	}
	payload, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatal(err)
	}
	var pid int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(payload)), "%d", &pid); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		err = syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("validator child %d survived process-group cleanup: %v", pid, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestShadowDiagnosticTruncation(t *testing.T) {
	report := shadowReport{SchemaVersion: 1, Shell: "bash", Diagnostics: []shadowDiagnostic{}, Results: []shadowResult{}}
	for index := 0; index < 101; index++ {
		report.Results = append(report.Results, shadowResult{Unit: index, Status: "unsupported", StartByte: index, EndByte: index + 1, StartLine: index + 1, EndLine: index + 1, Diagnostics: []shadowDiagnostic{{Code: "unsupported", Message: "safe"}}})
	}
	finalizeShadowReport(&report)
	ordinary := 0
	for _, result := range report.Results {
		ordinary += len(result.Diagnostics)
	}
	if ordinary != 100 || len(report.Diagnostics) != 1 || report.Diagnostics[0].Code != "diagnostics_truncated" || !strings.Contains(report.Diagnostics[0].Message, "1") {
		t.Fatalf("diagnostic limit = %#v, ordinary = %d", report.Diagnostics, ordinary)
	}
}

func TestShadowReportErrorMatrix(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), []byte("alias x='true'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	originalValidator := shadowSyntaxValidator
	originalStdout := shadowStdout
	t.Cleanup(func() {
		shadowSyntaxValidator = originalValidator
		shadowStdout = originalStdout
	})

	shadowSyntaxValidator = func(context.Context, string, []byte) error { panic("private canary") }
	code, err := runCatalogCommand([]string{"shadow", "--shell", "bash", "--json"})
	if code != 2 || err == nil || strings.Contains(err.Error(), "canary") {
		t.Fatalf("panic result = %d, %v", code, err)
	}

	shadowSyntaxValidator = func(context.Context, string, []byte) error { return nil }
	shadowStdout = shortShadowWriter{}
	code, err = runCatalogCommand([]string{"shadow", "--shell", "bash", "--json"})
	if code != 2 || !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write result = %d, %v", code, err)
	}
}

func TestShadowForbiddenCallGraph(t *testing.T) {
	contents, err := os.ReadFile("catalog_shadow.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"loadConfig(", "loadAliases(", "saveConfig(", "saveRevision(", "syncRepository(", "writeAliasFile("} {
		if bytes.Contains(contents, []byte(forbidden)) {
			t.Fatalf("shadow implementation calls %s", forbidden)
		}
	}
}

func TestShadowConcurrentPureReader(t *testing.T) {
	source := []byte("# Safe\nalias x='echo x'\nfn() { echo y; }\n")
	const readers = 32
	var group sync.WaitGroup
	group.Add(readers)
	for index := 0; index < readers; index++ {
		go func() {
			defer group.Done()
			results := importShadowSource("bash", source)
			markShadowDuplicates(results)
			validateShadowCandidates(results)
			if len(results) != 2 {
				t.Errorf("results = %d", len(results))
			}
		}()
	}
	group.Wait()
}

type shortShadowWriter struct{}

func (shortShadowWriter) Write(contents []byte) (int, error) {
	if len(contents) == 0 {
		return 0, nil
	}
	return len(contents) - 1, nil
}

func TestShadowJSONSchemaOmitsPrivateFingerprints(t *testing.T) {
	report := shadowReport{SchemaVersion: 1, Shell: "bash", Diagnostics: []shadowDiagnostic{}, Results: []shadowResult{{Unit: 0, Name: "x", Kind: "command", Status: "equivalent", StartByte: 0, EndByte: 10, StartLine: 1, EndLine: 1, Diagnostics: []shadowDiagnostic{}, origin: "private"}}}
	finalizeShadowReport(&report)
	output, err := renderShadowReport(report, true)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(output, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"source_sha256", "origin_key"} {
		if _, ok := decoded[field]; ok {
			t.Fatalf("report contains %s", field)
		}
	}
	if strings.Contains(string(output), "private") {
		t.Fatal("report contains internal fingerprint")
	}
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
	var decoded struct {
		SchemaVersion int                `json:"schema_version"`
		Shell         string             `json:"shell"`
		Summary       shadowSummary      `json:"summary"`
		Diagnostics   []shadowDiagnostic `json:"diagnostics"`
		Results       []shadowResult     `json:"results"`
	}
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

func shadowGoldenReport() shadowReport {
	report := shadowReport{SchemaVersion: 1, Shell: "bash", Diagnostics: []shadowDiagnostic{}, Results: []shadowResult{
		{Unit: 5, Name: "ok", Kind: "command", Status: "equivalent", StartByte: 50, EndByte: 51, StartLine: 6, EndLine: 6, Diagnostics: []shadowDiagnostic{}},
		{Unit: 4, Name: "unsup", Status: "unsupported", StartByte: 40, EndByte: 41, StartLine: 5, EndLine: 5, Diagnostics: []shadowDiagnostic{}},
		{Unit: 3, Name: "diff", Kind: "command", Status: "different", StartByte: 30, EndByte: 31, StartLine: 4, EndLine: 4, DifferentFields: []string{"description"}, Diagnostics: []shadowDiagnostic{}},
		{Unit: 2, Name: "dup", Kind: "command", Status: "duplicate", StartByte: 20, EndByte: 21, StartLine: 3, EndLine: 3, Diagnostics: []shadowDiagnostic{}},
		{Unit: 1, Name: "block", Kind: "command", Status: "blocked", StartByte: 10, EndByte: 11, StartLine: 2, EndLine: 2, Diagnostics: []shadowDiagnostic{}},
		{Unit: 0, Name: "inv", Kind: "command", Status: "invalid", StartByte: 0, EndByte: 1, StartLine: 1, EndLine: 1, Diagnostics: []shadowDiagnostic{}},
	}}
	finalizeShadowReport(&report)
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
			originalValidator := shadowValidatorPath
			shadowValidatorPath = func(string) (string, error) { return validator, nil }
			t.Cleanup(func() { shadowValidatorPath = originalValidator })
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
			report, err := inspectCatalogShadow(shell)
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

func FuzzShadowImporter(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte("alias x='true'\n"),
		[]byte("x() { echo ok; }\n"),
		[]byte("alias x='unterminated\n"),
		[]byte("# al: tags=git platforms=linux\nalias g='git status'\n"),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, source []byte) {
		if len(source) > 1<<20 || !utf8.Valid(source) || bytes.IndexByte(source, 0) >= 0 {
			t.Skip()
		}
		results := importShadowSource("bash", source)
		if len(results) > shadowMaxResults {
			t.Skip()
		}
		markShadowDuplicates(results)
		validateShadowCandidates(results)
	})
}
