//go:build !windows

package shell_test

import (
	"fmt"
	"github.com/alderon07/al/internal/entry"
	"github.com/alderon07/al/internal/shell"
	"strings"
	"testing"
)

var benchmarkFunctions []entry.Alias
var benchmarkShadow []shell.Result
var benchmarkParsedName string
var benchmarkParsedCommand string
var benchmarkParsedOK bool

func BenchmarkShellParse(b *testing.B) {
	for _, name := range []string{"bash", "zsh"} {
		for _, n := range []int{100, 1000, 10000} {
			adapter, err := shell.New(name)
			if err != nil {
				b.Fatal(err)
			}
			b.Run(fmt.Sprintf("%s/aliases/%d", name, n), func(b *testing.B) {
				lines := make([]string, n)
				size := 0
				for i := range lines {
					lines[i] = fmt.Sprintf("alias tool%05d='printf synthetic-%05d'", i, i)
					size += len(lines[i]) + 1
					parsed, _, ok := adapter.ParseAliasDefinition(lines[i])
					if !ok || parsed != fmt.Sprintf("tool%05d", i) {
						b.Fatal("alias parser rejected input")
					}
				}
				b.SetBytes(int64(size))
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					for _, line := range lines {
						benchmarkParsedName, benchmarkParsedCommand, benchmarkParsedOK = adapter.ParseAliasDefinition(line)
					}
				}
				b.StopTimer()
				b.ReportMetric(float64(n), "entries")
			})
			b.Run(fmt.Sprintf("%s/functions/%d", name, n), func(b *testing.B) {
				var text strings.Builder
				for i := 0; i < n; i++ {
					fmt.Fprintf(&text, "function tool%05d {\n printf synthetic-%05d\n}\n", i, i)
				}
				input := text.String()
				if result := adapter.ParseFunctions(input); len(result) != n {
					b.Fatalf("function count=%d", len(result))
				}
				b.SetBytes(int64(len(input)))
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					benchmarkFunctions = adapter.ParseFunctions(input)
				}
				b.StopTimer()
				b.ReportMetric(float64(n), "entries")
			})
			b.Run(fmt.Sprintf("%s/structural/%d", name, n), func(b *testing.B) {
				var text strings.Builder
				for i := 0; i < n; i++ {
					fmt.Fprintf(&text, "alias tool%05d='printf synthetic-%05d'\n", i, i)
				}
				input := []byte(text.String())
				result := shell.ImportShadowSource(name, input)
				if len(result) != n {
					b.Fatalf("structural count=%d", len(result))
				}
				for _, value := range result {
					if value.Entry == nil {
						b.Fatal("structural parser rejected entry")
					}
				}
				b.SetBytes(int64(len(input)))
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					benchmarkShadow = shell.ImportShadowSource(name, input)
				}
				b.StopTimer()
				b.ReportMetric(float64(n), "entries")
			})
		}
	}
}
