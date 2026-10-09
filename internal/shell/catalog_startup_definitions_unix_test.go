//go:build !windows

package shell

import (
	"bytes"
	"strings"
	"testing"
)

func TestCatalogStartupLiteralDefinitionsAndCoordinates(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		t.Run(shell, func(t *testing.T) {
			path := "/synthetic/." + shell + "_aliases"
			guard := "if [ -f \"$HOME/." + shell + "_aliases\" ]; then\n . \"$HOME/." + shell + "_aliases\"\nfi\n"
			prefix := "case $- in\n    *i*) ;;\n      *) return;;\nesac\nalias fixture='printf earlier'\nalias unrelated=\"printf fixture --fixture\"\nPS1='fixture alias fixture=not-a-definition'\n"
			source := []byte(prefix + guard + "printf --fixture fixture\n")
			preserved, load, insertion, err := CatalogStartupPlacementWithNative(source, shell, "/synthetic", path, map[string]bool{"fixture": true}, map[string]bool{"fixture": true})
			if err != nil || load || insertion != len(prefix+guard) || !bytes.Equal(source, preserved) {
				t.Fatalf("placement coordinates: %v %v %d", err, load, insertion)
			}
			for _, authority := range []map[string]bool{nil, {"other": true}} {
				if _, _, _, err := CatalogStartupPlacementWithNative(source, shell, "/synthetic", path, map[string]bool{"fixture": true}, authority); err == nil || !strings.Contains(err.Error(), "startup line 5:") {
					t.Fatalf("unowned/function alias mask accepted: %v", err)
				}
			}
			if _, _, _, err := CatalogStartupPlacementWithNative([]byte(guard+"alias fixture='printf late'\n"), shell, "/synthetic", path, map[string]bool{"fixture": true}, map[string]bool{"fixture": true}); err == nil {
				t.Fatal("late override accepted")
			}
			deferred := "deferred() {\n alias fixture='printf deferred'\n return\n}\nPS1='multi\nalias fixture=quoted\nfixture() { fake; }\n'\n"
			if _, _, _, err := CatalogStartupPlacementWithNative([]byte(deferred+guard), shell, "/synthetic", path, map[string]bool{"fixture": true}, nil); err != nil {
				t.Fatal("deferred/quoted definition treated as immediate", err)
			}
			conditional := "if [ -n \"$TERM\" ]; then\n alias fixture='printf conditional'\nfi\n"
			if _, _, _, err := CatalogStartupPlacementWithNative([]byte(conditional+guard), shell, "/synthetic", path, map[string]bool{"fixture": true}, map[string]bool{"fixture": true}); err != nil {
				t.Fatal(err)
			}
			for _, suffix := range []string{"printf safe; alias fixture='printf late'\n", "EXAMPLE=safe alias fixture='printf late'\n", "fixture() { printf late; }\n", "source \"$DYNAMIC\"\n"} {
				if _, _, _, err := CatalogStartupPlacementWithNative([]byte(guard+suffix), shell, "/synthetic", path, map[string]bool{"fixture": true}, map[string]bool{"fixture": true}); err == nil {
					t.Fatal("late/unproven declaration accepted")
				}
			}
		})
	}
}

func TestCatalogExplicitStartupRelocation(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		path := "/synthetic/." + shell + "_aliases"
		guard := "if [ -f \"$HOME/." + shell + "_aliases\" ]; then\n . \"$HOME/." + shell + "_aliases\"\nfi\n"
		prefix := "alias fixture='printf earlier'\n"
		rest := "if [ -f /synthetic/other ]; then\n . /synthetic/other\nfi\ndeferred() { return; }\n"
		source := []byte(prefix + guard + rest)
		names := map[string]bool{"fixture": true}
		authority := map[string]bool{"fixture": true}
		if _, _, _, err := CatalogStartupPlacementWithNative(source, shell, "/synthetic", path, names, authority); err == nil {
			t.Fatal("default implicitly enrolled another source")
		}
		preserved, load, insertion, err := CatalogStartupPlacementPolicy(source, shell, "/synthetic", path, names, authority, true)
		if err != nil || load || insertion != len(preserved) || string(preserved) != prefix+rest+guard {
			t.Fatalf("explicit route relocation: %v %s", err, preserved)
		}
		for _, other := range []string{"source \"$DYNAMIC\"\n", "source /synthetic/.zsh_aliases\n", "source /synthetic/.bash_aliases; printf tail\n", "eval 'source /synthetic/.bash_aliases'\n"} {
			if _, _, _, err := CatalogStartupPlacementPolicy([]byte(guard+other), shell, "/synthetic", path, names, authority, true); err == nil {
				t.Fatalf("unproven explicit source accepted: %s", other)
			}
		}
		if _, _, _, err := CatalogStartupPlacementPolicy([]byte(guard+guard+rest), shell, "/synthetic", path, names, authority, true); err == nil {
			t.Fatal("duplicate native route accepted")
		}
	}
}

func TestCatalogExplicitNativeManagedSegment(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		route := []byte("if [ -f \"$HOME/." + shell + "_aliases\" ]; then\n . \"$HOME/." + shell + "_aliases\"\nfi\n")
		contents := append([]byte("printf preserved\n"), route...)
		before, moved, err := CatalogExplicitNativeSourceTail(contents, shell, "/synthetic", "/synthetic/."+shell+"_aliases")
		if err != nil || string(before) != "printf preserved\n" || !bytes.Equal(moved, route) {
			t.Fatal("exact source tail lost", err)
		}
		block, err := CatalogGuardExplicitBlock(shell, CatalogLoaderBlock(shell, "/synthetic/."+shell+"_aliases", "/synthetic/alias-lens", "/synthetic/generated", false), route)
		if err != nil {
			t.Fatal(err)
		}
		captured, err := CatalogExplicitNativeRoute([]byte(block))
		if err != nil || !bytes.Equal(captured, route) {
			t.Fatal("managed route lost", err)
		}
		if !strings.Contains(block, "\n\\builtin ") || strings.Index(block, "\n\\builtin ") > strings.Index(block, string(route)) {
			t.Fatal("alias option disable not a prior standalone statement")
		}
		if _, err := CatalogExplicitNativeRoute([]byte(block + catalogExplicitNativeEnd)); err == nil {
			t.Fatal("duplicate marker accepted")
		}
	}
}

func TestCatalogStartupKnownControlsAndQuotedCommands(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		for _, statement := range []string{"alias builtin='printf unsafe'\n", "command() { printf unsafe; }\n", "'alias' fixture='printf unsafe'\n", "printf safe; 'alias' fixture='printf unsafe'\n", "\\alias fixture='printf unsafe'\n", "'source' '/synthetic/opaque'\n"} {
			if _, _, _, err := CatalogStartupPlacementWithNative([]byte(statement), shell, "/synthetic", "/synthetic/."+shell+"_aliases", map[string]bool{"fixture": true}, map[string]bool{"fixture": true}); err == nil {
				t.Fatal("unproven/known startup control accepted")
			}
		}
	}
}

func TestCatalogQuotedOwnedLoaderCannotImpersonateStartupRoute(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		owned := BashAliasLoader
		if shell == "zsh" {
			owned = ZshAliasLoader
		}
		guard := "if [ -f \"$HOME/." + shell + "_aliases\" ]; then\n . \"$HOME/." + shell + "_aliases\"\nfi\n"
		source := []byte("PS1='" + owned + "'\n" + guard + "printf preserved\n")
		preserved, load, position, err := CatalogStartupPlacementWithNative(source, shell, "/synthetic", "/synthetic/."+shell+"_aliases", nil, nil)
		if err != nil || load || !bytes.Equal(preserved, source) || position != len(source)-len("printf preserved\n") {
			t.Fatalf("quoted owned route altered source/coordinates: %v", err)
		}
		nested := []byte("case safe in\n safe)\n" + owned + ";;\nesac\n")
		if _, _, _, err := CatalogStartupPlacementWithNative(nested, shell, "/synthetic", "/synthetic/."+shell+"_aliases", nil, nil); err == nil {
			t.Fatal("nested owned route accepted")
		}
	}
}

func TestCatalogExplicitScalarOpaqueRoutes(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		path := "/synthetic/." + shell + "_aliases"
		native := "source " + QuoteShadow(path) + "\n"
		assignment := "export DIR=\"$HOME/static\"\n"
		route := "[ -s \"$DIR/extra\" ] && \\. \"$DIR/extra\" # optional comment\n"
		source := []byte(native + assignment + route + "[ -f \"${DIR}/followup\" ] && source \"${DIR}/followup\"\n")
		preserved, _, _, err := CatalogStartupPlacementPolicy(source, shell, "/synthetic", path, nil, nil, true)
		if err != nil || !strings.HasSuffix(string(preserved), native) || !bytes.Contains(preserved, []byte(assignment+route)) {
			t.Fatal("static scalar route rejected/changed", err)
		}
		for _, mutation := range []string{"DIR='/other'\n", "true; DIR='/other'\n", "builtin export DIR='/other'\n", "DIR+=suffix\n", "printf -v DIR '%s' /other\n", "unset 'DIR'\n", "read \"DIR\"\n", "printf -v 'DIR' '%s' /other\n", "unset \"$UNKNOWN\"\n", "HOME='/other'\n", "unset 'HOME'\n"} {
			if _, _, _, err := CatalogStartupPlacementPolicy([]byte(native+assignment+mutation+route), shell, "/synthetic", path, nil, nil, true); err == nil {
				t.Fatalf("visible mutation accepted: %s", mutation)
			}
		}
		for _, invalid := range []string{"if true; then\n" + assignment + "fi\n" + route, "export DIR=\"$(printf /synthetic)\"\n" + route, assignment + "[ -s \"$DIR/a\" ] && \\. \"$DIR/b\"\n", assignment + "[ -s \"$DIR/extra\" ] && \\. \"$DIR/extra\"; printf tail\n", "export DIR=\"$HOME\"\n[ -s \"$DIR/." + shell + "_aliases\" ] && \\. \"$DIR/." + shell + "_aliases\"\n"} {
			if _, _, _, err := CatalogStartupPlacementPolicy([]byte(native+invalid), shell, "/synthetic", path, nil, nil, true); err == nil {
				t.Fatalf("unproven scalar route accepted: %s", invalid)
			}
		}
	}
}

func TestCatalogImmediateOpaqueEvalNeedsExplicitEnrollment(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		path := "/synthetic/." + shell + "_aliases"
		native := "source " + QuoteShadow(path) + "\n"
		for _, statement := range []string{"eval 'synthetic'\n", "builtin eval 'synthetic'\n", "X=synthetic eval 'synthetic'\n", "printf synthetic; eval 'synthetic'\n", "printf synthetic | eval 'synthetic'\n"} {
			source := []byte(native + statement)
			if _, _, _, err := CatalogStartupPlacementPolicy(source, shell, "/synthetic", path, nil, nil, false); err == nil {
				t.Fatal("opaque immediate evaluation accepted automatically")
			}
			if _, _, _, err := CatalogStartupPlacementPolicy(source, shell, "/synthetic", path, nil, nil, true); err != nil {
				t.Fatal("explicit opaque trust rejected", err)
			}
		}
	}
}

func TestCatalogManagedMarkersIgnoreQuotedLiterals(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		route := []byte("source '/synthetic/." + shell + "_aliases'\n")
		block, err := CatalogGuardExplicitBlock(shell, CatalogLoaderBlock(shell, "/synthetic/."+shell+"_aliases", "/synthetic/alias-lens", "/synthetic/generated", false), route)
		if err != nil {
			t.Fatal(err)
		}
		fake := "PS2='" + catalogLoaderStart + "\n" + catalogLoaderEnd + "\n" + catalogExplicitNativeStart + catalogExplicitNativeEnd + "'\n"
		source := []byte(fake + block)
		start, end, err := CatalogStartupBlockSpan(source)
		if err != nil || string(source[start:end]) != block {
			t.Fatal("quoted markers selected", err)
		}
		captured, err := CatalogExplicitNativeRoute(source)
		if err != nil || !bytes.Equal(captured, route) {
			t.Fatal("quoted native markers selected", err)
		}
		if strings.Contains(block, "BASHOPTS") {
			t.Fatal("guard requires Bash newer than 3.2")
		}
	}
}

func TestCatalogScalarTargetsAndInertArguments(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		path := "/synthetic/." + shell + "_aliases"
		native := "source " + QuoteShadow(path) + "\n"
		initial := "export DIR=\"$HOME/static\"\n"
		route := "[ -s \"$DIR/extra\" ] && \\. \"$DIR/extra\"\n"
		for _, harmless := range []string{"export HOME_TOOLS=/x\n", "export PATH=\"$HOME/bin:$PATH\"\n", "export OTHER=/x # HOME assignment is absent\n", "printf '%s' DIR=/literal\n", "printf '%s' export\n", "printf '%s' unset DIR\n", "printf '%s' read\n"} {
			if _, _, _, err := CatalogStartupPlacementPolicy([]byte(native+initial+harmless+route), shell, "/synthetic", path, nil, nil, true); err != nil {
				t.Fatalf("inert target/argument rejected: %s: %v", harmless, err)
			}
		}
	}
}

func TestCatalogDeferredManagedMarkersAndInlineDefinitions(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		route := []byte("source '/synthetic/." + shell + "_aliases'\n")
		fake := []byte("deferred() {\n" + catalogLoaderStart + "\n" + catalogLoaderEnd + "\n" + catalogExplicitNativeStart + catalogExplicitNativeEnd + ":\n}\n")
		inline := []byte("deferred_inline() { printf synthetic; alias fixture='printf deferred'; }\n")
		before := append(append([]byte{}, fake...), inline...)
		if _, _, _, err := CatalogStartupPlacementAt(append(append([]byte{}, before...), route...), shell, "/synthetic", "/synthetic/."+shell+"_aliases", map[string]bool{"fixture": true}); err != nil {
			t.Fatal("deferred definitions treated as immediate", err)
		}
		block, err := CatalogGuardExplicitBlock(shell, CatalogLoaderBlock(shell, "/synthetic/."+shell+"_aliases", "/synthetic/alias-lens", "/synthetic/generated", false), route)
		if err != nil {
			t.Fatal(err)
		}
		full := append(before, []byte(block)...)
		if _, _, err := CatalogStartupBlockSpan(full); err != nil {
			t.Fatal("deferred markers broke block refresh", err)
		}
		captured, err := CatalogExplicitNativeRoute(full)
		if err != nil || !bytes.Equal(captured, route) {
			t.Fatal("deferred markers broke route refresh", err)
		}
	}
}

func TestCatalogUnsupportedLateFunctionHeadersRefused(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		for _, header := range []string{"fixture ( ) { printf synthetic; }\n", "function\tfixture { printf synthetic; }\n", "printf synthetic; fixture() { printf late; }\n", "printf synthetic; fixture ( ) { printf late; }\n", "printf synthetic; function fixture { printf late; }\n"} {
			source := []byte("source '/synthetic/." + shell + "_aliases'\n" + header)
			if _, _, _, err := CatalogStartupPlacementAt(source, shell, "/synthetic", "/synthetic/."+shell+"_aliases", map[string]bool{"fixture": true}); err == nil {
				t.Fatal("unproven late header accepted")
			}
		}
	}
}

func TestCatalogUnprovenBraceContextRejectedBeforeEnrollment(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		path := "/synthetic/." + shell + "_aliases"
		for _, statement := range []string{"printf '%s' {\n", "printf '%s' }\n"} {
			source := []byte(statement + "source " + QuoteShadow(path) + "\n")
			for _, explicit := range []bool{false, true} {
				if _, _, _, err := CatalogStartupPlacementPolicy(source, shell, "/synthetic", path, nil, nil, explicit); err == nil {
					t.Fatal("unproven brace context enrolled")
				}
			}
		}
		quoted := []byte("printf '%s' '{'\nsource " + QuoteShadow(path) + "\n")
		if _, _, _, err := CatalogStartupPlacementAt(quoted, shell, "/synthetic", path, nil); err != nil {
			t.Fatal("quoted brace argument refused", err)
		}
	}
}
