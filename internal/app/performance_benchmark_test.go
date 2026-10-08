//go:build !windows

package app

import (
	"alias-lens/internal/catalog"
	"alias-lens/internal/catalogstore"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var benchmarkEntries []Alias

func benchmarkAliases(n int) []Alias {
	values := make([]Alias, n)
	for i := range values {
		values[i] = Alias{Name: fmt.Sprintf("tool%05d", i), Command: fmt.Sprintf("printf synthetic-%05d", i), Description: "Inspect synthetic repository status", Category: "dev", Type: "alias", Tags: []string{"repository", "status"}, Favorite: i%10 == 0, Usage: i % 26}
	}
	return values
}

func benchmarkServices(b *testing.B, name string) (*Services, string) {
	b.Helper()
	home, err := filepath.EvalSymlinks(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	if err = os.Chmod(home, 0700); err != nil {
		b.Fatal(err)
	}
	service := NewServices(Dependencies{HomeDir: func() (string, error) { return home, nil }, WorkingDir: func() (string, error) { return home, nil }, Environment: func(key string) string {
		switch key {
		case activeShellEnvironment:
			return name
		case "PATH":
			return "/usr/bin:/bin"
		}
		return ""
	}})
	config := service.Settings.Defaults()
	config.Shell = name
	if err = service.Settings.Save(config); err != nil {
		b.Fatal(err)
	}
	return service, home
}

func BenchmarkEntrySearch(b *testing.B) {
	for _, n := range []int{100, 1000, 10000} {
		for _, query := range []struct {
			name, text string
			empty      bool
		}{{"prefix", "t", false}, {"semantic", "repository", false}, {"typo", "tpol00042", false}, {"no-match", "zzzzzzzzzzzzzzzzzzzz", true}} {
			b.Run(fmt.Sprintf("%s/%d", query.name, n), func(b *testing.B) {
				service, _ := benchmarkServices(b, "bash")
				aliases := benchmarkAliases(n)
				data, err := json.Marshal(aliases)
				if err != nil {
					b.Fatal(err)
				}
				result := service.FilterEntries(aliases, query.text, nil)
				if (len(result) == 0) != query.empty {
					b.Fatalf("unexpected search count=%d", len(result))
				}
				if query.name == "prefix" || query.name == "semantic" {
					if len(result) != n {
						b.Fatalf("search count=%d want=%d", len(result), n)
					}
				}
				if query.name == "typo" {
					found := false
					for _, value := range result {
						found = found || value.Name == "tool00042"
					}
					if !found {
						b.Fatal("typo target absent")
					}
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					benchmarkEntries = service.FilterEntries(aliases, query.text, nil)
				}
				b.StopTimer()
				b.ReportMetric(float64(n), "entries")
				b.ReportMetric(float64(len(data)), "input-B")
			})
		}
	}
}

func BenchmarkEntrySuggestions(b *testing.B) {
	for _, n := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			service, _ := benchmarkServices(b, "bash")
			aliases := benchmarkAliases(n)
			data, err := json.Marshal(aliases)
			if err != nil {
				b.Fatal(err)
			}
			ranked := service.SuggestedEntries(aliases, nil)
			if len(ranked) != n || !ranked[0].Favorite {
				b.Fatal("suggestion ranking invalid")
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkEntries = service.SuggestedEntries(aliases, nil)
			}
			b.StopTimer()
			b.ReportMetric(float64(n), "entries")
			b.ReportMetric(float64(len(data)), "input-B")
		})
	}
}

func benchmarkNativeEntries(b *testing.B, service *Services, home, name string, n int, installed bool) int64 {
	b.Helper()
	adapter, err := service.ShellAdapter(name)
	if err != nil {
		b.Fatal(err)
	}
	var source, history strings.Builder
	if !installed {
		for i := 0; i < n; i++ {
			fmt.Fprintf(&source, "# Synthetic repository status\n# al: tags=repository,status favorite=%t\nalias tool%05d='printf synthetic-%05d'\n", i%10 == 0, i, i)
		}
	}
	for i := 0; i < n; i++ {
		if name == "zsh" {
			fmt.Fprintf(&history, ": 1700000000:0;tool%05d\n", i)
		} else {
			fmt.Fprintf(&history, "#1700000000\ntool%05d\n", i)
		}
	}
	for path, data := range map[string][]byte{filepath.Join(home, adapter.AliasFilename()): []byte(source.String()), filepath.Join(home, adapter.HistoryFilename()): []byte(history.String())} {
		if err = os.WriteFile(path, data, 0600); err != nil {
			b.Fatal(err)
		}
	}
	return int64(source.Len() + history.Len())
}

func benchmarkInstalledCatalog(b *testing.B, service *Services, home, name string, n int) int64 {
	b.Helper()
	adapter, err := service.ShellAdapter(name)
	if err != nil {
		b.Fatal(err)
	}
	value := catalog.Catalog{SchemaVersion: 2, Entries: make([]catalog.Entry, n)}
	manifest := catalogstore.GenerationManifest{Version: 1, Shell: name, Renderer: name + "/v2", Platform: service.currentPlatform(), NativePath: filepath.Join(home, adapter.AliasFilename()), NativePolicy: "regular-user", SourceSHA256: "", NativeInputSHA256: catalogstore.Hash(nil), Profiles: []string{}, Entries: make([]catalogstore.GenerationEntry, 0, n), IncludedEntryIDs: []string{}, Confirmations: []catalogstore.ApprovalKey{}, ExecutableResolutions: make([]catalogstore.ExecutableResolution, 0, n)}
	var body strings.Builder
	for i := range value.Entries {
		entry := catalog.Entry{ID: fmt.Sprintf("%032x", i+1), Name: fmt.Sprintf("tool%05d", i), Kind: "command", Description: "Synthetic repository status", Favorite: i%10 == 0, Portable: &catalog.Portable{Program: "printf", Args: []string{fmt.Sprintf("synthetic-%05d", i)}, PassArguments: true}}
		declaration, err := catalogstore.Declaration(entry, name, "/usr/bin/printf")
		if err != nil {
			b.Fatal(err)
		}
		value.Entries[i] = entry
		manifest.Entries = append(manifest.Entries, catalogstore.GenerationEntry{Entry: entry, Declaration: declaration})
		manifest.ExecutableResolutions = append(manifest.ExecutableResolutions, catalogstore.ExecutableResolution{EntryID: entry.ID, Program: "printf", Path: "/usr/bin/printf"})
		body.WriteString(declaration)
	}
	declared, problems := catalog.Encode(value)
	if len(problems) != 0 {
		b.Fatal(problems)
	}
	manifest.SourceSHA256 = catalogstore.Hash(declared)
	manifest, encoded, err := catalogstore.BuildGeneration(manifest, []byte(body.String()))
	if err != nil {
		b.Fatal(err)
	}
	pointer, err := catalogstore.EncodePointer(manifest.ID)
	if err != nil {
		b.Fatal(err)
	}
	records, err := catalogstore.Encode(catalogstore.InstalledFile{Version: 1, Records: []catalogstore.InstalledState{{Shell: name, GenerationID: manifest.ID, Renderer: name + "/v2", NativeInputSHA256: manifest.NativeInputSHA256}}})
	if err != nil {
		b.Fatal(err)
	}
	root := service.catalogGeneratedRoot(name)
	inputs := map[string][]byte{service.localCatalogPath(): declared, filepath.Join(root, manifest.ID+".json"): encoded, filepath.Join(root, manifest.ID+".sh"): []byte(body.String()), filepath.Join(root, "active"): pointer, service.catalogInstalledPath(): records}
	size := int64(0)
	for path, data := range inputs {
		if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			b.Fatal(err)
		}
		if err = os.WriteFile(path, data, 0600); err != nil {
			b.Fatal(err)
		}
		size += int64(len(data))
	}
	if verified, err := service.readCatalogGeneration(root, manifest.NativePath, name); err != nil || len(verified.Entries) != n {
		b.Fatalf("installed generation count=%d err=%v", len(verified.Entries), err)
	}
	return size
}

func BenchmarkEntryLoad(b *testing.B) {
	for _, name := range []string{"bash", "zsh"} {
		for _, kind := range []string{"native", "installed-catalog"} {
			for _, n := range []int{100, 1000, 10000} {
				b.Run(fmt.Sprintf("%s/%s/%d", name, kind, n), func(b *testing.B) {
					service, home := benchmarkServices(b, name)
					installed := kind == "installed-catalog"
					size := benchmarkNativeEntries(b, service, home, name, n, installed)
					if installed {
						size += benchmarkInstalledCatalog(b, service, home, name, n)
					}
					values, err := service.Entries()
					if err != nil || len(values) != n {
						b.Fatalf("entry count=%d err=%v", len(values), err)
					}
					if installed {
						for _, value := range values {
							if value.CatalogState != "installed" {
								b.Fatal("catalog membership not installed")
							}
						}
					}
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						benchmarkEntries, err = service.Entries()
						if err != nil {
							b.Fatal(err)
						}
					}
					b.StopTimer()
					b.ReportMetric(float64(n), "entries")
					b.ReportMetric(float64(size), "fixture-B")
				})
			}
		}
	}
}
