package state

import (
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func healthyInput() Inputs {
	return Inputs{
		Mode:    ModeCatalog,
		Config:  ConfigObservation{Present: true, SchemaVersion: 2, CurrentSchemaVersion: 2},
		Catalog: CatalogObservation{Present: true, SchemaVersion: 2, EntryCount: 42, SHA256: "catalog"},
		Shells: []ShellObservation{{Name: "bash", Resolved: ResolvedSummary{SHA256: "resolved", EligibleEntries: 40}, Installed: &InstalledObservation{
			GenerationSHA256: "generation", ResolvedStateSHA256: "resolved", RecordedLoaderSHA256: "loader",
			ActiveGenerationSHA256: "generation", OnDiskGenerationSHA256: "generation", OnDiskLoaderSHA256: "loader",
		}}},
		Sync: SyncObservation{Configured: true, BaseSHA256: "same", LocalSHA256: "same", RemoteSHA256: "same"},
	}
}

func TestResolveStateMatrix(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Inputs)
		want any
		get  func(Report) any
	}{
		{"legacy", func(i *Inputs) { i.Mode = ModeLegacy; i.Catalog = CatalogObservation{} }, ModeLegacy, func(r Report) any { return r.Mode }},
		{"mixed", func(i *Inputs) { i.Mode = ModeMixed }, ModeMixed, func(r Report) any { return r.Mode }},
		{"missing config uses defaults", func(i *Inputs) { i.Config.Present = false; i.Config.SchemaVersion = 0 }, ConfigCurrent, func(r Report) any { return r.Config.State }},
		{"zsh", func(i *Inputs) { i.Shells[0].Name = "zsh" }, "zsh", func(r Report) any { return r.Shells[0].Name }},
		{"missing state", func(i *Inputs) { i.Shells[0].Installed = nil }, ShellNotInstalled, func(r Report) any { return r.Shells[0].State }},
		{"corrupt state", func(i *Inputs) { i.Shells[0].Unreadable = true }, ShellUnreadable, func(r Report) any { return r.Shells[0].State }},
		{"changed pointer", func(i *Inputs) { i.Shells[0].Installed.ActiveGenerationSHA256 = "changed" }, ShellIntegrationDrift, func(r Report) any { return r.Shells[0].State }},
		{"changed loader", func(i *Inputs) { i.Shells[0].Installed.OnDiskLoaderSHA256 = "changed" }, ShellIntegrationDrift, func(r Report) any { return r.Shells[0].State }},
		{"pending approval", func(i *Inputs) { i.Shells[0].Resolved.PendingApprovals = 1 }, ShellApprovalRequired, func(r Report) any { return r.Shells[0].State }},
		{"render needed", func(i *Inputs) { i.Shells[0].Resolved.SHA256 = "new" }, ShellRenderRequired, func(r Report) any { return r.Shells[0].State }},
		{"local changes", func(i *Inputs) { i.Sync.LocalSHA256 = "local" }, SyncLocalChanges, func(r Report) any { return r.Sync.State }},
		{"remote changes", func(i *Inputs) { i.Sync.RemoteSHA256 = "remote" }, SyncRemoteChanges, func(r Report) any { return r.Sync.State }},
		{"diverged", func(i *Inputs) { i.Sync.LocalSHA256 = "local"; i.Sync.RemoteSHA256 = "remote" }, SyncDiverged, func(r Report) any { return r.Sync.State }},
		{"recovery", func(i *Inputs) { i.Recovery.Required = true }, RecoveryRequired, func(r Report) any { return r.Recovery.State }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := healthyInput()
			test.edit(&input)
			got := test.get(Resolve(input))
			if got != test.want {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestResolveStateDeterministic(t *testing.T) {
	input := healthyInput()
	input.Shells = append(input.Shells, ShellObservation{Name: "zsh"})
	want := Resolve(input)
	const workers = 64
	var group sync.WaitGroup
	errors := make(chan string, workers)
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for range 100 {
				if got := Resolve(input); !reflect.DeepEqual(got, want) {
					errors <- "nondeterministic result"
					return
				}
			}
		}()
	}
	group.Wait()
	close(errors)
	for message := range errors {
		t.Error(message)
	}
}

func TestStateImportAllowlist(t *testing.T) {
	allowed := map[string]bool{"bytes": true, "encoding/json": true, "fmt": true, "sort": true, "strings": true}
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
					t.Errorf("production import %q can access ambient state", path)
				}
			}
		}
	}
}

func TestStatusRenderingAndJSON(t *testing.T) {
	report := Resolve(healthyInput())
	plain := RenderPlain(report)
	if !strings.Contains(plain, "installed for new Bash shells") || strings.Contains(plain, "loaded") {
		t.Fatalf("unsafe shell claim in:\n%s", plain)
	}
	if strings.Contains(plain, "schema") || !strings.Contains(plain, "Settings: ready") {
		t.Fatalf("plain output uses internal format language:\n%s", plain)
	}
	encoded, err := EncodeJSON(report)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(encoded, []byte{'\n'}) || bytes.Contains(encoded, []byte("\\u003c")) {
		t.Fatalf("noncanonical JSON: %q", encoded)
	}
	if ExitCode(report) != 0 {
		t.Fatalf("healthy exit code = %d", ExitCode(report))
	}
	report = Resolve(func() Inputs { value := healthyInput(); value.Recovery.Required = true; return value }())
	if ExitCode(report) != 1 || !strings.Contains(RenderPlain(report), "needs attention") {
		t.Fatal("attention report did not produce actionable output")
	}
}

func TestStatusEscapesTerminalControls(t *testing.T) {
	input := healthyInput()
	input.Shells[0].Name = "ba\x1b[2Jsh"
	plain := RenderPlain(Resolve(input))
	if strings.ContainsRune(plain, '\x1b') || !strings.Contains(plain, `\x1b`) {
		t.Fatalf("control byte was not escaped: %q", plain)
	}
}
