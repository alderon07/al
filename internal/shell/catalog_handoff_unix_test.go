//go:build !windows

package shell_test

import (
	"encoding/json"
	"github.com/alderon07/al/internal/catalog"
	"github.com/alderon07/al/internal/catalogstore"
	"github.com/alderon07/al/internal/shell"
	"os"
	"testing"
)

func TestRuntimeHandoffMatchesSavedShellBytes(t *testing.T) {
	contents, err := os.ReadFile("testdata/catalog_handoff.json")
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{}
	if err := json.Unmarshal(contents, &expected); err != nil {
		t.Fatal(err)
	}
	cases := map[string][]catalogstore.GenerationEntry{
		"empty":    nil,
		"alias":    {{Entry: catalog.Entry{Name: "demo"}, Declaration: "alias demo='printf synthetic'\n"}},
		"function": {{Entry: catalog.Entry{Name: "demo"}, Declaration: "demo() {\n printf '%s\\n' \"$1\"\n}\n"}},
	}
	for _, name := range []string{"bash", "zsh"} {
		for kind, entries := range cases {
			t.Run(name+"/"+kind, func(t *testing.T) {
				actual, err := shell.RuntimeHandoff(name, entries)
				if err != nil || actual != expected[name+"/"+kind] {
					t.Fatalf("handoff bytes changed: %q %v", actual, err)
				}
			})
		}
	}
	if output, err := shell.RuntimeHandoff("fish", cases["alias"]); err == nil || output != "" {
		t.Fatal("unsupported handoff accepted")
	}
}
