package catalog

import (
	"fmt"
	"testing"
)

var benchmarkCatalog Catalog
var benchmarkResolved []ResolvedEntry

func benchmarkCatalogInput(b *testing.B, n int) (Catalog, []byte) {
	b.Helper()
	value := Catalog{SchemaVersion: SchemaVersion, Entries: make([]Entry, n)}
	for i := range value.Entries {
		value.Entries[i] = Entry{ID: fmt.Sprintf("%032x", i+1), Name: fmt.Sprintf("tool%05d", i), Kind: "command", Description: "Inspect synthetic repository status", Tags: []string{"repository", "status"}, Favorite: i%10 == 0, Portable: &Portable{Program: "printf", Args: []string{fmt.Sprintf("synthetic-%05d", i)}, PassArguments: true}}
	}
	data, problems := Encode(value)
	if len(problems) != 0 {
		b.Fatal(problems)
	}
	return value, data
}

func BenchmarkCatalogDecode(b *testing.B) {
	for _, n := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			_, data := benchmarkCatalogInput(b, n)
			value, problems := Decode(data)
			if len(problems) != 0 || len(value.Entries) != n {
				b.Fatalf("decode count=%d diagnostics=%v", len(value.Entries), problems)
			}
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkCatalog, _ = Decode(data)
			}
			b.StopTimer()
			b.ReportMetric(float64(n), "entries")
		})
	}
}

func BenchmarkCatalogResolve(b *testing.B) {
	for _, n := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			value, data := benchmarkCatalogInput(b, n)
			context := ResolveContext{Shell: "bash", Platform: "linux", Profiles: []string{"work"}}
			resolved, problems := Resolve(value, context)
			if len(problems) != 0 || len(resolved) != n {
				b.Fatalf("resolve count=%d diagnostics=%v", len(resolved), problems)
			}
			for _, entry := range resolved {
				if !entry.Available {
					b.Fatal(entry.UnavailableReasons)
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkResolved, _ = Resolve(value, context)
			}
			b.StopTimer()
			b.ReportMetric(float64(n), "entries")
			b.ReportMetric(float64(len(data)), "input-B")
		})
	}
}
