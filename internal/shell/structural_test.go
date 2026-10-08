//go:build !windows

package shell_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"github.com/alderon07/al/internal/catalog"
	"github.com/alderon07/al/internal/shell"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStructuralBoundariesAndPrivateJSON(t *testing.T) {
	source := []byte("# Synthetic\nalias demo='printf synthetic'\n")
	results := shell.ImportShadowSource("bash", source)
	if len(results) != 1 || results[0].Entry == nil || results[0].StartByte != 0 || results[0].EndByte != len(source) {
		t.Fatalf("parse: %#v", results)
	}
	shell.ValidateShadowCandidates(results)
	shell.CompareShadowRoundTrip("bash", &results[0])
	if results[0].Status != "equivalent" {
		t.Fatalf("roundtrip: %#v", results)
	}
	value, err := json.Marshal(results[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"Origin", "Entry", "Rendered", "origin", "entry", "rendered", "synthetic"} {
		if bytes.Contains(value, []byte(private)) {
			t.Fatalf("private JSON: %s", value)
		}
	}
	called := false
	resolver := func(string) (string, error) { called = true; return "", nil }
	if err := shell.ValidateSyntax(context.Background(), "../bash", source, resolver); err == nil || called {
		t.Fatal("unsupported shell reached resolver")
	}
	if _, err := shell.TrustedShadowShell("../bash"); err == nil {
		t.Fatal("trusted traversal accepted")
	}
	if shell.ValidateCatalogNativeDeclaration("fish", source) == nil || shell.ValidateCatalogDeclaration("fish", catalog.Entry{Name: "demo"}, source) == nil {
		t.Fatal("unsupported declarations accepted")
	}
	if _, _, err := shell.CatalogStartupPlacement(nil, "fish", "/synthetic", "/synthetic/.aliases", nil); err == nil {
		t.Fatal("unsupported startup accepted")
	}
	if shell.CatalogLoaderBlock("bash';printf injected;#", "", "", "", false) != "" {
		t.Fatal("unsafe loader generated")
	}
	if len(shell.ImportShadowSource("bash", bytes.Repeat([]byte("x"), 1<<20+1))) != 1 || shell.ImportShadowSource("fish", source)[0].Entry != nil {
		t.Fatal("source bounds")
	}
}

func TestCatalogZDOTDIRRulesAndReadOnlyParsing(t *testing.T) {
	home := t.TempDir()
	directory := t.TempDir()
	t.Setenv("ZDOTDIR", "")
	envPath := filepath.Join(home, ".zshenv")
	contents := []byte("export ZDOTDIR=" + shell.Quote(directory) + "\n")
	if err := os.WriteFile(envPath, contents, 0600); err != nil {
		t.Fatal(err)
	}
	path, err := shell.CatalogZshStartupPath(home)
	if err != nil || path != filepath.Join(directory, ".zshrc") {
		t.Fatalf("static ZDOTDIR: %s %v", path, err)
	}
	actual, _ := os.ReadFile(envPath)
	if !bytes.Equal(actual, contents) {
		t.Fatal("parser wrote .zshenv")
	}
	if err := os.WriteFile(envPath, []byte("export ZDOTDIR=$(printf synthetic)\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := shell.CatalogZshStartupPath(home); err == nil {
		t.Fatal("dynamic ZDOTDIR accepted")
	}
	t.Setenv("ZDOTDIR", "relative")
	if _, err := shell.CatalogZshStartupPath(home); err == nil {
		t.Fatal("relative catalog ZDOTDIR accepted")
	}
	if path, err := shell.ZshStartupPath(home); err != nil || !filepath.IsAbs(path) || !strings.HasSuffix(path, "relative/.zshrc") {
		t.Fatalf("legacy rule changed: %s %v", path, err)
	}
}

func TestInvalidNativeSourceCannotHideControlMasks(t *testing.T) {
	for _, prefix := range [][]byte{[]byte("# \xff\n"), []byte("# \x00\n")} {
		source := append(prefix, []byte("alias harmless='printf harmless' builtin='printf unsafe'\n")...)
		if err := shell.ValidateCatalogNativeControls("bash", source); err == nil {
			t.Fatal("invalid source concealed control assignment")
		}
		results := shell.ImportShadowSource("bash", source)
		if len(results) != 1 || results[0].Entry != nil || results[0].EndByte != len(source) {
			t.Fatalf("refusal range: %#v", results)
		}
	}
}

func TestNativeStructuralGrammarRefusesEscapesAndUnsupportedContexts(t *testing.T) {
	bodies := map[string]string{
		"escaped quote":                "\n: \\'\n}\nprintf harmless_sentinel\n{\n: \\'\n",
		"literal braces":               "\nprintf {\n}\nprintf harmless_sentinel\nfunction other {\nprintf }\n",
		"ANSI C quote":                 "\n" + `printf $'escaped\' brace }'` + "\n",
		"nested double substitution":   "\nprintf \"$(printf '%s' \"}\")\"\n",
		"command substitution":         "\nprintf $(printf '}')\n",
		"process substitution":         "\ncat <(printf '}')\n",
		"parameter expansion":          "\nprintf ${HOME:-'}'}\n",
		"brace expansion":              "\nprintf {one,two}\n",
		"backtick":                     "\nprintf `printf '}'`\n",
		"arithmetic substitution":      "\nprintf $(( $(printf 8) << 2 ))\n",
		"arithmetic variable":          "\nprintf $((value << 2))\n",
		"legacy arithmetic":            "\nprintf $[8 + 2]\n",
		"nested group":                 "\n{ printf safe; }\n",
		"escaped newline before close": "\nprintf \\\n}\nprintf harmless_sentinel\n{\n",
		"dynamic delimiter":            "\ncat <<$'EOF'\n}\nEOF\n",
		"reviewed heredoc joining":     "\ncat <<EOF\nEO\\\nF\n}\nprintf 'harmless_sentinel\\n'\nfunction other {\nEOF\n",
		"reviewed heredoc quote":       "\ncat <<EOF \"\n\"\nEOF\n}\nprintf 'harmless_sentinel\\n'\nfunction other {\n# \"\nEOF\n",
		"unsupported here string":      "\ncat <<< 'literal input'\n",
		"unsupported input overlap":    "\ncat <<<< 'literal input'\n",
		"repeated here strings":        "\ncat <<< 'first' <<< 'second'\n",
	}
	for _, name := range []string{"bash", "zsh"} {
		for label, body := range bodies {
			t.Run(name+"/"+label, func(t *testing.T) {
				source := []byte("synthetic() {" + body + "}\n")
				if err := shell.ValidateCatalogNativeDeclaration(name, source); err == nil {
					t.Fatal("unsafe or unsupported declaration accepted")
				}
			})
		}
	}
}

func TestNativeHereStringOperatorBoundary(t *testing.T) {
	declaration := []byte("function synthetic {\ncat <<<EOF\n}\nbuiltin printf 'harmless_sentinel\\n'\nfunction other {\nEOF\n}\n")
	for _, name := range []string{"bash", "zsh"} {
		t.Run(name, func(t *testing.T) {
			if err := shell.ValidateCatalogNativeDeclaration(name, declaration); err == nil {
				t.Fatal("reviewed here-string declaration accepted")
			}
			results := shell.ImportShadowSource(name, declaration)
			if len(results) != 1 || results[0].Entry != nil || results[0].EndByte != len(declaration) {
				t.Fatalf("unsupported declaration range changed: %#v", results)
			}
			results = shell.ImportShadowSource(name, []byte("cat <<< 'EOF'\nalias next='true'\n"))
			if len(results) != 2 || results[0].Entry != nil || results[1].Name != "next" {
				t.Fatalf("here-string suffix was treated as heredoc: %#v", results)
			}
		})
	}
}

func TestNativeStructuralGrammarPreservesBodyAndFollowingAlias(t *testing.T) {
	bodies := map[string]string{
		"quoted variable":                  "\n printf '%s' \"$HOME\"\n",
		"escaped quote":                    "\n printf '%s' \\'\n",
		"escaped braces":                   "\n printf '%s' \\{ \\}\n",
		"quoted braces":                    "\n printf '%s' '{' \"}\"\n",
		"numeric arithmetic":               "\n printf '%s' $((8 << 2))\n",
		"single line":                      " printf '%s' body; ",
		"quoted heredoc backslash":         "\ncat <<'EOF'\nEO\\\nF\n} literal data {\nEOF\n",
		"heredoc trailing quoted argument": "\ncat <<EOF \"argument\"\n} literal data {\nEOF\n",
		"quoted input operator":            "\nprintf '%s' '<<< literal'\n",
		"tab stripping heredoc":            "\ncat <<-EOF\n\t} literal data {\n\tEOF\n",
	}
	for _, delimiter := range []string{"EOF", "'EOF'", `\EOF`, "'EO'F", `"EO"F`} {
		bodies["heredoc "+delimiter] = "\n cat <<" + delimiter + "\n} braced data {\nEOF\n"
	}
	for _, name := range []string{"bash", "zsh"} {
		for label, body := range bodies {
			t.Run(name+"/"+label, func(t *testing.T) {
				declaration := "synthetic() {" + body + "} # trailing\n"
				if err := shell.ValidateCatalogNativeDeclaration(name, []byte(declaration)); err != nil {
					t.Fatal(err)
				}
				results := shell.ImportShadowSource(name, []byte(declaration+"alias next='true'\n"))
				if len(results) != 2 || results[0].Entry == nil || results[1].Name != "next" || results[0].EndByte != len(declaration) {
					t.Fatalf("source range changed: %#v", results)
				}
				if actual := *results[0].Entry.Native[name].FunctionBody; actual != body {
					t.Fatalf("body changed: %q", actual)
				}
			})
		}
	}
}

func TestStructuralOriginsBindEntireSourceAndRanges(t *testing.T) {
	for _, name := range []string{"bash", "zsh"} {
		for _, source := range [][]byte{
			[]byte("# Synthetic\nalias first='printf synthetic'\nfunction second {\n printf synthetic\n}\n"),
			[]byte("alias first='printf synthetic'\nalias second='printf alternate'\n"),
		} {
			digest := sha256.Sum256(source)
			results := shell.ImportShadowSource(name, source)
			if len(results) != 2 {
				t.Fatalf("%s result count=%d", name, len(results))
			}
			for _, result := range results {
				var framed bytes.Buffer
				for _, value := range [][]byte{[]byte(name), digest[:], binary.BigEndian.AppendUint64(nil, uint64(result.StartByte)), binary.BigEndian.AppendUint64(nil, uint64(result.EndByte))} {
					framed.Write(binary.BigEndian.AppendUint64(nil, uint64(len(value))))
					framed.Write(value)
				}
				origin := sha256.Sum256(framed.Bytes())
				want := hex.EncodeToString(origin[:])
				if result.Entry == nil || result.Origin != want || result.Entry.ID != want[:32] {
					t.Fatalf("%s origin does not bind complete input and range", name)
				}
			}
			changed := append(append([]byte{}, source...), []byte("# trailing synthetic\n")...)
			if shell.ImportShadowSource(name, changed)[0].Origin == results[0].Origin {
				t.Fatal("changed source reused an origin")
			}
		}
	}
}
