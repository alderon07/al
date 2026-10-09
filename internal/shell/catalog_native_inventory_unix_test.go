//go:build !windows

package shell

import (
	"bytes"
	"context"
	neutralcatalog "github.com/alderon07/al/internal/catalog"
	"github.com/alderon07/al/internal/catalogstore"
	"strings"
	"testing"
)

func TestCatalogNativeInventoryRetainsOpaqueFunctions(t *testing.T) {
	source := []byte("alias before='printf synthetic'\nfunction retained() {\n local value=\"$(printf '%s' \"${HOME##*/}\")\"\n printf '%s\\n' \"${value:-synthetic}\"\n # } fake boundary\n printf '%s' '}'\n}\nalias after='printf trailing'\n")
	for _, shell := range []string{"bash", "zsh"} {
		t.Run(shell, func(t *testing.T) {
			units, err := InspectCatalogNativeSource(shell, source)
			if err != nil {
				t.Fatal(err)
			}
			if len(units) != 3 || units[1].Name != "retained" || units[1].Entry != nil || units[2].Name != "after" {
				t.Fatalf("unexpected inventory: %#v", units)
			}
			if units[1].StartLine != 2 || units[1].EndLine != 7 {
				t.Fatalf("wrong boundary: %#v", units[1])
			}
			if !bytes.Equal(source[units[1].StartByte:units[1].EndByte], source[bytes.Index(source, []byte("function retained")):bytes.Index(source, []byte("alias after"))]) {
				t.Fatal("function bytes changed")
			}
			if err := ValidateCatalogNativeControls(shell, source); err != nil {
				t.Fatal(err)
			}
			strict := ImportShadowSource(shell, source)
			for _, unit := range strict {
				if unit.Name == "retained" && unit.Entry != nil {
					t.Fatal("opaque function became importable")
				}
			}
		})
	}
}

func TestCatalogNativeInventoryFailsClosed(t *testing.T) {
	fixtures := []string{
		"fixture() { printf \"$(printf x)\" >\n}\n", "fixture() { printf \"$(printf x)\" >; }\n", "fixture() { printf \"$(printf x)\" # comment\n | printf x; }\n", "fixture() { ! if true; then printf \"$(printf x)\"; fi; }\n", "fixture() { }\n",
		"if() { printf \"$(printf synthetic)\"; }\n", "then() { printf \"$(printf synthetic)\"; }\n", "fi() { printf \"$(printf synthetic)\"; }\n", "for() { printf \"$(printf synthetic)\"; }\n", "while() { printf \"$(printf synthetic)\"; }\n", "case() { printf \"$(printf synthetic)\"; }\n",
		"fixture() {\n printf \"$(printf synthetic)\"\n cat <\\\n<EOF\n}\nother() {\nEOF\n}\n",
		"fixture() {\n if true; then\n printf \"$(printf synthetic)\"\n}\n",
		"fixture() { printf \"$(printf x)\"; | printf x; }\n",
		"fixture() { printf \"$(printf x)\" &&\n}\n",
		"fixture() { if; then printf \"$(printf x)\"; fi; }\n",
		"alias fixture='builtin'\nfixture() { :; }\n", "alias dynamic=\"$HOME\"\n", "alias fixture='printf safe'; printf tail\n", "export EXAMPLE=synthetic # helper()\n", "'fake() { synthetic; }'\n",
		"bad-name() { printf safe; }\n", "fixture() { printf safe; } > synthetic\n", "fixture() { printf safe; }; printf tail\n",
		"fixture() { printf \"$(printf safe\"; }\n", "fixture() { printf \"${value\"; }\n", "fixture() { printf 'unterminated; }\n",
		"fixture() { cat <<END\n}\nEND\n}\n", "fixture() { printf `printf safe`; }\n", "fixture() { cat <(printf safe); }\n",
		"fixture() { (printf safe); }\n", "fixture() { { printf safe; }; }\n", "fixture() { case safe in x) printf safe;; esac; }\n",
		"fixture() { printf $((1+1)); }\n", "fixture() { printf ${items[0]}; }\n", "fixture() { printf ${value:-\"synthetic\"}; }\n",
		"fixture() { printf safe\n", "fixture() { printf safe }\n", "fixture() { printf \\\n safe }\n",
	}
	for _, shell := range []string{"bash", "zsh"} {
		for index, source := range fixtures {
			if _, err := InspectCatalogNativeSource(shell, []byte(source)); err == nil {
				t.Fatalf("%s accepted fixture %d", shell, index)
			} else if !strings.Contains(err.Error(), "native lines ") || strings.Contains(err.Error(), "EXAMPLE") {
				t.Fatalf("unbounded private error: %v", err)
			}
		}
	}
}

func TestCatalogNativeInventoryQuotedBoundariesAndNestedSubstitution(t *testing.T) {
	source := []byte("retained() {\n printf '%s' \"$(printf '%s' \"$(printf '%s' \"${value:-synthetic}\")\")\"\n printf '%s' '} alias fake=secret'\n printf '%s' synthetic\n}\n")
	for _, shell := range []string{"bash", "zsh"} {
		units, err := InspectCatalogNativeSource(shell, source)
		if err != nil || len(units) != 1 {
			t.Fatalf("%s: %v %#v", shell, err, units)
		}
	}
	deep := []byte("fixture() { printf \"" + strings.Repeat("$(printf ", 66) + "safe" + strings.Repeat(")", 66) + "\"; }\n")
	if _, err := InspectCatalogNativeSource("bash", deep); err == nil {
		t.Fatal("nesting bound not enforced")
	}
	for _, source := range []string{"alias command='printf safe'\n", "command() { printf \"$(printf safe)\"; }\n", "builtin() { printf safe; }\n"} {
		if err := ValidateCatalogNativeControls("bash", []byte(source)); err == nil {
			t.Fatal("control mask accepted")
		}
	}
}

func TestCatalogLiteralShellInitIntegrationBoundaries(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		adapter, err := New(shell)
		if err != nil {
			t.Fatal(err)
		}
		prefix := "retained() { printf '%s' \"$(printf '%s' \"${HOME##*/}\")\"; }\n"
		for _, command := range []string{"alias-lens", "command alias-lens"} {
			form := "eval \"$(" + command + " shell-init " + shell + ")\"\n"
			source := []byte(prefix + form + "alias trailing='printf synthetic'\n")
			stripped, start, err := CatalogRemoveOwnedIntegration(adapter, source)
			if err != nil || start != len(prefix) || string(stripped) != prefix+"alias trailing='printf synthetic'\n" {
				t.Fatalf("route proof: %v %d", err, start)
			}
			masked, err := CatalogNativeReviewSource(adapter, source)
			if err != nil {
				t.Fatal(err)
			}
			if len(masked) != len(source) {
				t.Fatal("mask shifted source coordinates")
			}
			if err := ValidateCatalogNativeControls(shell, source); err != nil {
				t.Fatal(err)
			}
			if _, _, err := CatalogRemoveOwnedIntegration(adapter, []byte(prefix+form+form)); err == nil {
				t.Fatal("duplicate route accepted")
			}
			hidden := []byte("fixture() {\n" + form + "}\n")
			if _, start, err := CatalogRemoveOwnedIntegration(adapter, hidden); err != nil || start >= 0 {
				t.Fatal("hidden route relocated")
			}
		}
		for _, source := range []string{"eval \"$(alias-lens shell-init $SHELL)\"\n", "eval \"$(alias-lens shell-init fish)\"\n", "eval \"$(alias-lens shell-init " + shell + ")\"; printf tail\n", "if true; then\neval \"$(alias-lens shell-init " + shell + ")\"\nfi\n"} {
			if err := ValidateCatalogNativeControls(shell, []byte(source)); err == nil {
				t.Fatal("unproven integration accepted")
			}
		}
	}
}

func TestCatalogRetainedFunctionDependencies(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		value := "printf synthetic"
		entry := neutralcatalog.Entry{ID: strings.Repeat("1", 32), Name: "fixture", Kind: "command", Native: map[string]neutralcatalog.NativeImplementation{shell: {AliasValue: &value}}}
		declaration, err := catalogstore.Declaration(entry, shell, "")
		if err != nil {
			t.Fatal(err)
		}
		called := false
		validator := func(context.Context, string, []byte) error { called = true; return nil }
		if err := ValidateCatalogDeclarations(shell, []neutralcatalog.Entry{entry}, [][]byte{[]byte(declaration)}, []byte("retained() { printf '%s' \"$(fixture)\"; }\n"), validator); err == nil || !strings.Contains(err.Error(), "native lines 1-1") || called {
			t.Fatalf("dependency accepted or validator ran: %v", err)
		}
		if err := ValidateCatalogDeclarations(shell, []neutralcatalog.Entry{entry}, [][]byte{[]byte(declaration)}, []byte("retained() { printf '%s' \"$(printf '%s' \"${HOME##*/}\")\"; }\n"), validator); err != nil || !called {
			t.Fatalf("unrelated retained function refused: %v", err)
		}
	}
}

func TestCatalogIntegrationCandidateBound(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		adapter, _ := New(shell)
		source := []byte("fixture() {\n" + strings.Repeat("eval \"$(alias-lens shell-init "+shell+")\"\n", 1000) + "}\n")
		if _, _, err := CatalogRemoveOwnedIntegration(adapter, source); err == nil || !strings.Contains(err.Error(), "candidate count exceeds") {
			t.Fatalf("candidate work not bounded: %v", err)
		}
		hidden := []byte("fixture() {\n cat <\\\n<EOF\n}\neval \"$(alias-lens shell-init " + shell + ")\"\nother() {\nEOF\n}\n")
		if _, start, err := CatalogRemoveOwnedIntegration(adapter, hidden); err == nil && start >= 0 {
			t.Fatal("hidden heredoc route removed")
		}
	}
}

func TestCatalogSeparatedContinuations(t *testing.T) {
	source := []byte("fixture() {\n local value=\"$(printf '%s' \"${HOME##*/}\")\"\n printf '%s' \\\n  \"$value\" \\\n  synthetic\n}\nalias following='printf safe'\n")
	for _, shell := range []string{"bash", "zsh"} {
		units, err := InspectCatalogNativeSource(shell, source)
		if err != nil || len(units) != 2 || units[0].EndLine != 6 || units[1].Name != "following" {
			t.Fatalf("separated continuation: %s %v %#v", shell, err, units)
		}
		for _, body := range []string{"cat < \\\n <EOF", "cat <\\\n<EOF", "i\\\nf true", "printf safe \\\n| printf safe", "if|printf synthetic", "fi&&printf synthetic", "in", "function fixture", "[[ -n synthetic ]", "printf safe;; printf safe"} {
			if _, err := InspectCatalogNativeSource(shell, []byte("fixture() {\n printf \"$(printf safe)\"\n"+body+"\n}\n")); err == nil {
				t.Fatalf("unproven continuation/control accepted: %s", shell)
			}
		}
	}
}
