package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	neutralcatalog "alias-lens/internal/catalog"
)

func TestCompletionCommandMetadataCoversDetailedHelp(t *testing.T) {
	seen := map[string]bool{}
	for _, spec := range publicCommandSpecs {
		if seen[spec.Name] {
			t.Fatalf("duplicate command specification %q", spec.Name)
		}
		seen[spec.Name] = true
		if spec.Usage == "" {
			t.Errorf("command %q has no detailed help", spec.Name)
		}
	}
	for name := range commandUsage {
		if !seen[name] {
			t.Errorf("detailed help command %q is missing from the command specification", name)
		}
	}
}

func TestGeneratedCompletionIsDeterministicAndParses(t *testing.T) {
	tests := []struct {
		name    string
		program func() string
	}{
		{name: "bash", program: renderBashCompletion},
		{name: "zsh", program: renderZshCompletion},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			first, second := test.program(), test.program()
			if first != second {
				t.Fatal("completion output changed between identical renders")
			}
			for _, wanted := range []string{"completion-candidates", "--command 'entries'", "--prefix"} {
				if !strings.Contains(first, wanted) {
					t.Errorf("completion output does not contain %q", wanted)
				}
			}
			path := filepath.Join(t.TempDir(), "completion."+test.name)
			if err := os.WriteFile(path, []byte(first), 0o600); err != nil {
				t.Fatal(err)
			}
			binary, err := exec.LookPath(test.name)
			if err != nil {
				t.Skipf("%s is not installed", test.name)
			}
			if output, err := exec.Command(binary, "-n", path).CombinedOutput(); err != nil {
				t.Fatalf("%s rejected generated completion: %v\n%s", test.name, err, output)
			}
		})
	}
}

func TestBashCompletionOffersCommandsAndClosedValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "completion.bash")
	if err := os.WriteFile(path, []byte(renderBashCompletion()), 0o600); err != nil {
		t.Fatal(err)
	}
	script := fmt.Sprintf(`source %q
COMP_WORDS=(al completion z)
COMP_CWORD=2
_alias_lens_complete
printf '%%s\n' "${COMPREPLY[@]}"
`, path)
	output, err := exec.Command("bash", "--noprofile", "--norc", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("invoke Bash completion: %v\n%s", err, output)
	}
	if string(output) != "zsh\n" {
		t.Fatalf("completion = %q, want zsh", output)
	}
}

func TestCompletionInstallAndRemoveUsePrivatePlannedFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var output bytes.Buffer
	if err := runCompletionCommand([]string{"install", "bash"}, &output); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".config", "alias-lens", "completion.bash")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(contents, []byte(renderBashCompletion())) {
		t.Fatal("installed completion differs from generated Bash completion")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("completion mode = %v, %v", info, err)
	}
	if !strings.Contains(output.String(), "Enter al setup bash") {
		t.Fatalf("install output omitted the next step: %s", output.String())
	}

	output.Reset()
	if err := runCompletionCommand([]string{"remove", "bash"}, &output); err != nil {
		t.Fatal(err)
	}
	contents, err = os.ReadFile(path)
	if !os.IsNotExist(err) {
		t.Fatalf("removed completion file still exists: %q, %v", contents, err)
	}
	if !strings.Contains(output.String(), "suggestions are removed") {
		t.Fatalf("remove output did not report the result: %s", output.String())
	}
}

func TestCompletionRefusesToOverwriteOrRemoveEditedFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	directory := filepath.Join(home, ".config", "alias-lens")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "completion.bash")
	edited := []byte(renderBashCompletion() + "# my change\n")
	if err := os.WriteFile(path, edited, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"install", "remove"} {
		var output bytes.Buffer
		if err := runCompletionCommand([]string{action, "bash"}, &output); err == nil || !strings.Contains(err.Error(), "changed outside Alias Lens") {
			t.Fatalf("%s error = %v", action, err)
		}
		contents, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(contents, edited) {
			t.Fatalf("%s changed edited completion: %q, %v", action, contents, err)
		}
	}
}

func TestCompletionRefusesEditedShellIntegration(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	adapter := bashShellAdapter{}
	path, err := aliasPathFor(adapter)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# Alias Lens Bash integration\n# edited\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"install", "remove"} {
		if _, err := buildCompletionPlan(action, "bash"); err == nil || !strings.Contains(err.Error(), "al setup --repair bash") {
			t.Fatalf("%s error = %v", action, err)
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "alias-lens", "completion.bash")); !os.IsNotExist(err) {
		t.Fatalf("edited integration caused a completion write: %v", err)
	}
}

func TestCompletionRecognizesHealthyShellIntegration(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	adapter := bashShellAdapter{}
	path, err := aliasPathFor(adapter)
	if err != nil {
		t.Fatal(err)
	}
	contents := "alias gs='git status'\n\n# Alias Lens Bash integration\n" +
		`eval "$(command alias-lens shell-init bash)"` + "\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if state := completionIntegrationState(adapter); state != "ready" {
		t.Fatalf("integration state = %q, want ready", state)
	}
	var output bytes.Buffer
	if err := runCompletionCommand([]string{"install", "bash"}, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Start a new bash shell") || strings.Contains(output.String(), "al setup bash") {
		t.Fatalf("install output did not recognize the healthy integration: %s", output.String())
	}
}

func TestCompletionRejectsProfilesInVersionOneSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	directory := filepath.Join(home, ".config", "alias-lens")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "config.json"), []byte(`{"version":1,"profiles":["work"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if runCompletionCandidates([]string{"--shell", "bash", "--command", "profiles", "--prefix", ""}, &output) {
		t.Fatal("version 1 settings with profiles unexpectedly succeeded")
	}
	if output.Len() != 0 {
		t.Fatalf("invalid settings leaked completion output %q", output.String())
	}
}

func TestShellIntegrationLoadsOnlyInstalledCompletionFile(t *testing.T) {
	for _, test := range []struct {
		name        string
		integration string
		path        string
	}{
		{name: "bash", integration: bashIntegration, path: "$HOME/.config/alias-lens/completion.bash"},
		{name: "zsh", integration: zshIntegration, path: "$HOME/.config/alias-lens/completion.zsh"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if !strings.Contains(test.integration, test.path) || !strings.Contains(test.integration, "-s \"") {
				t.Fatalf("integration does not guard and load %s", test.path)
			}
		})
	}
}

func TestCompletionCandidatesReadLegacyEntriesWithoutRunningThem(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	sentinel := filepath.Join(home, "must-not-exist")
	contents := "alias gs='git status'\n" +
		"alias inert='touch " + sentinel + "'\n" +
		"helper() {\n  alias nested='printf private'\n  printf helper\n}\n"
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if !runCompletionCandidates([]string{"--shell", "bash", "--command", "entries", "--prefix", ""}, &output) {
		t.Fatal("candidate reader failed")
	}
	if got := output.String(); got != "gs\nhelper\ninert\n" {
		t.Fatalf("candidates = %q", got)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatal("reading candidates executed an alias")
	}
}

func TestCompletionCandidatesFailClosedForMalformedPrivateData(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), []byte("alias broken\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if runCompletionCandidates([]string{"--shell", "bash", "--command", "entries", "--prefix", ""}, &output) {
		t.Fatal("malformed alias data unexpectedly succeeded")
	}
	if output.Len() != 0 {
		t.Fatalf("malformed private data leaked output %q", output.String())
	}
}

func TestCompletionCandidatesDoNotCreateAMissingAliasFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	aliasPath := filepath.Join(home, ".bash_aliases")
	var output bytes.Buffer
	if runCompletionCandidates([]string{"--shell", "bash", "--command", "entries", "--prefix", ""}, &output) {
		t.Fatal("missing alias data unexpectedly succeeded")
	}
	if _, err := os.Stat(aliasPath); !os.IsNotExist(err) {
		t.Fatalf("completion created a missing alias file: %v", err)
	}
}

func TestCompletionCandidatesUseResolvedCatalogNames(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configDirectory := filepath.Join(home, ".config", "alias-lens")
	if err := os.MkdirAll(configDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDirectory, "config.json"), []byte(`{"version":2,"profiles":["work"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	stateDirectory := filepath.Join(home, ".local", "state", "alias-lens")
	if err := os.MkdirAll(stateDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	portable := &neutralcatalog.Portable{Program: "git", Args: []string{"status"}, PassArguments: true}
	catalog := neutralcatalog.Catalog{SchemaVersion: neutralcatalog.SchemaVersion2, Entries: []neutralcatalog.Entry{
		{ID: "00000000000000000000000000000001", Name: "always", Kind: "command", Portable: portable},
		{ID: "00000000000000000000000000000002", Name: "work-only", Kind: "command", Portable: portable, When: &neutralcatalog.Conditions{ProfilesAny: []string{"work"}}},
		{ID: "00000000000000000000000000000003", Name: "zsh-only", Kind: "command", Portable: portable, When: &neutralcatalog.Conditions{Shells: []string{"zsh"}}},
	}}
	encoded, diagnostics := neutralcatalog.Encode(catalog)
	if len(diagnostics) > 0 {
		t.Fatalf("encode catalog: %#v", diagnostics)
	}
	if err := os.WriteFile(filepath.Join(configDirectory, "catalog.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	snapshotHash := hashBytes(encoded)
	state := fmt.Sprintf(`{"schema_version":1,"installed_shells":{"bash":{"source_catalog_sha256":%q}}}`, snapshotHash)
	if err := os.WriteFile(filepath.Join(stateDirectory, "catalog-state.json"), []byte(state), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshotDirectory := filepath.Join(stateDirectory, "catalog-snapshots")
	if err := os.MkdirAll(snapshotDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapshotDirectory, snapshotHash+".json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if !runCompletionCandidates([]string{"--shell", "bash", "--command", "entries", "--prefix", ""}, &output) {
		t.Fatal("catalog candidate reader failed")
	}
	if got := output.String(); got != "always\nwork-only\n" {
		t.Fatalf("catalog candidates = %q", got)
	}
}

func TestCompletionCandidatesAreBounded(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var source strings.Builder
	for index := 0; index < completionMaxResults+25; index++ {
		fmt.Fprintf(&source, "alias a%04d='true'\n", index)
	}
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), []byte(source.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if !runCompletionCandidates([]string{"--shell", "bash", "--command", "entries", "--prefix", ""}, &output) {
		t.Fatal("candidate reader failed")
	}
	if got := strings.Count(output.String(), "\n"); got != completionMaxResults {
		t.Fatalf("candidate count = %d, want %d", got, completionMaxResults)
	}
	if output.Len() > completionOutputLimit {
		t.Fatalf("candidate output is %d bytes", output.Len())
	}
}
