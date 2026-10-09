//go:build !windows

package shell

import (
	"strings"
	"testing"
)

func TestCatalogAliasDependencyWords(t *testing.T) {
	aliases := map[string]bool{"fixture": true, "other": true}
	positives := []string{"printf --fixture fixture /fixture fixture/path", "./fixture --fixture", "fi\"xture\" safe", "2>fixture printf fixture", "printf 2>fixture fixture", "printf fixture 2>fixture", "printf fixture \\\n  fixture", "printf 'fixture' \"fixture\"", "printf safe # fixture\n", "EXAMPLE=fixture printf fixture", "printf fixture > fixture", "printf fixture 2>fixture", "'fixture' --fixture", "\\fixture --fixture", "fix'ture' safe"}
	negatives := []string{"other --fixture", "2>fixture other", "other 2>fixture", "printf safe | 2>fixture other", "EXAMPLE=fixture 2>fixture other", "printf safe\nother", "printf safe \\\n  | other", "printf safe | fixture", "printf safe && fixture", "printf safe; fixture", "EXAMPLE=fixture other", "printf \"$(fixture)\"", "EXAMPLE=\"$(fixture)\" printf safe", "printf safe > \"$(fixture)\"", "printf \"${value:-$(fixture)}\"", "printf \"$(printf \"$(fixture)\")\"", "printf safe; fixture", "printf fixture ", "if true; then fixture; fi", "fi\\\nxture", "printf `fixture`"}
	for _, shell := range []string{"bash", "zsh"} {
		for index, text := range positives {
			if err := rejectCatalogAliasDependencies(shell, "fixture", text, aliases, true); err != nil {
				t.Fatalf("%s positive%d: %v", shell, index, err)
			}
		}
		for index, text := range negatives {
			if err := rejectCatalogAliasDependencies(shell, "fixture", text, aliases, true); err == nil {
				t.Fatalf("%s negative%d accepted", shell, index)
			}
		}
		if shell == "bash" {
			if err := rejectCatalogAliasDependencies(shell, "fixture", "fixture --fixture", aliases, true); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := rejectCatalogAliasDependencies(shell, "fixture", "fixture --fixture", aliases, true); err == nil {
				t.Fatal("unproven Zsh self replacement allowed")
			}
		}
		if err := rejectCatalogAliasDependencies(shell, "fixture", "fixture --fixture", aliases, false); err == nil {
			t.Fatal("function received self replacement exemption")
		}
		for _, text := range []string{"printf --fixture fixture", "printf 'fixture'", "EXAMPLE=fixture printf safe", "printf safe >fixture"} {
			if err := rejectCatalogAliasDependencies(shell, "fixture", text, aliases, false); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := rejectCatalogAliasDependencies("zsh", "fixture", "command -p fixture", aliases, false); err == nil {
		t.Fatal("zsh precommand dependency hidden")
	}
	if err := rejectCatalogAliasDependencies("bash", "fixture", "command fixture", aliases, false); err != nil {
		t.Fatal("Bash noneligible builtin argument rejected", err)
	}
}

func TestCatalogAliasDependencyBounds(t *testing.T) {
	source := strings.Repeat("$(printf ", 66) + "fixture" + strings.Repeat(")", 66)
	if err := rejectCatalogAliasDependencies("bash", "fixture", source, map[string]bool{"fixture": true}, true); err == nil {
		t.Fatal("nested dependency bound bypassed")
	}
	for _, source := range []string{"printf 'unfinished", "printf \"$(fixture\"", "printf safe >", "printf safe <<END\nfixture\nEND", "printf <(fixture)", "printf $((1+1))"} {
		if err := rejectCatalogAliasDependencies("bash", "fixture", source, map[string]bool{"fixture": true}, true); err == nil {
			t.Fatal("unsupported syntax accepted")
		}
	}
}

func FuzzCatalogAliasDependencyInspection(f *testing.F) {
	for _, source := range []string{"printf --fixture fixture", "EXAMPLE=\"$(fixture)\" printf safe", "printf '%s' \"${value:-$(fixture)}\"", "printf safe \\\n  fixture", "printf '\x00'", "printf \\"} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		for _, shell := range []string{"bash", "zsh"} {
			_ = rejectCatalogAliasDependencies(shell, "fixture", source, map[string]bool{"fixture": true}, true)
		}
	})
}
