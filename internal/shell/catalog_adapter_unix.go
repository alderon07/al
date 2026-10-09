//go:build !windows

package shell

import (
	"bytes"
	"context"
	"fmt"
	"github.com/alderon07/al/internal/catalogstore"
	"regexp"
	"strings"
	"time"

	neutralcatalog "github.com/alderon07/al/internal/catalog"
)

var catalogNameTokens = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_.-]*`)

var catalogProtectedNames = func() map[string]bool {
	names := map[string]bool{}
	for _, name := range strings.Fields("alias bind bg break builtin cd command compdef compgen complete compopt continue declare dirs disown echo enable eval exec exit export false help fc fg functions getopts hash jobs kill let local logout mapfile popd pushd printf pwd read readarray readonly rehash return set shift source test type suspend times trap true typeset ulimit umask unalias unset wait whence where which zcompile zformat zle zmodload zparseopts zprof zpty zregexparse zsocket zstyle emulate getcap setcap limit unlimit integer float autoload disable log print pushln sched vared if then else elif fi case esac for select while until do done in function time coproc repeat nocorrect noglob [[ ]]") {
		names[name] = true
	}
	return names
}()

func CatalogProtectedName(name string) bool {
	if name == "al" || name == "alias-lens" || strings.HasPrefix(name, "_alias_lens") {
		return true
	}
	return catalogProtectedNames[name]
}

func ValidateCatalogNames(value neutralcatalog.Catalog) error {
	for _, entry := range value.Entries {
		if CatalogProtectedName(entry.Name) {
			return fmt.Errorf("catalog name %q is reserved for shell control; rename it before al catalog enable", entry.Name)
		}
		if entry.Portable != nil && !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(entry.Name) {
			return fmt.Errorf("%s requires a portable function name; rename it before al catalog enable", entry.Name)
		}
	}
	return nil
}

func ValidateCatalogDeclaration(shell string, entry neutralcatalog.Entry, declaration []byte) error {
	if shell != "bash" && shell != "zsh" {
		return fmt.Errorf("unsupported shell %q (use bash or zsh)", shell)
	}

	if CatalogProtectedName(entry.Name) {
		return fmt.Errorf("catalog name %q is reserved for shell control; rename it before al catalog enable", entry.Name)
	}
	if len(declaration) == 0 || len(declaration) > shadowSourceLimit {
		return fmt.Errorf("catalog declaration has an unsupported size")
	}
	results := ImportShadowSource(shell, declaration)
	if len(results) != 1 || results[0].Entry == nil || results[0].StartByte != 0 || results[0].EndByte != len(declaration) || results[0].Name != entry.Name {
		return fmt.Errorf("%s needs one complete literal declaration; edit it before al catalog enable", entry.Name)
	}
	if _, ok := entry.Native[shell]; ok {
		expected, err := catalogstore.Declaration(entry, shell, "")
		if err != nil || !bytes.Equal([]byte(expected), declaration) {
			return fmt.Errorf("%s declaration changed during structural validation", entry.Name)
		}
	}

	return nil
}

func ValidateCatalogNativeControls(shell string, native []byte) error {
	if err := validateSource(native); err != nil {
		return err
	}

	if shell != "bash" && shell != "zsh" {
		return fmt.Errorf("unsupported shell %q (use bash or zsh)", shell)
	}

	controls := map[string]bool{}
	for _, name := range strings.Fields(". : [ [[ alias autoload bind bindkey builtin case command declare do done else esac eval exec export false fc fi for function history if in local print printf return select set setopt shift source then true typeset unalias unset until while zle") {
		controls[name] = true
	}
	adapter, err := New(shell)
	if err != nil {
		return err
	}
	source, err := CatalogNativeReviewSource(adapter, native)
	if err != nil {
		return err
	}
	units, err := InspectCatalogNativeSource(shell, source)
	if err != nil {
		return err
	}
	for _, unit := range units {
		blocked := unit.Kind == "command" && controls[unit.Name] || unit.Kind == "function" && (unit.Name == "builtin" || unit.Name == "command")
		if blocked {
			return nativeBoundaryError(unit.StartLine, unit.EndLine, "declaration masks catalog handoff control")
		}
	}

	return nil
}

func ValidateCatalogDependencyPolicy(shell string, entries []neutralcatalog.Entry, declarations [][]byte, native []byte) error {
	if shell != "bash" && shell != "zsh" {
		return fmt.Errorf("unsupported shell %q (use bash or zsh)", shell)
	}
	if err := ValidateCatalogNativeControls(shell, native); err != nil {
		return err
	}
	if len(entries) != len(declarations) {
		return fmt.Errorf("catalog declarations are incomplete")
	}
	names := map[string]bool{}
	aliases := map[string]bool{}
	for _, entry := range entries {
		if names[entry.Name] {
			return fmt.Errorf("catalog declarations contain duplicate names")
		}
		names[entry.Name] = true
		if implementation, ok := entry.Native[shell]; ok && implementation.AliasValue != nil {
			aliases[entry.Name] = true
		}
	}
	adapter, err := New(shell)
	if err != nil {
		return err
	}
	reviewSource, err := CatalogNativeReviewSource(adapter, native)
	if err != nil {
		return err
	}
	survivors, err := InspectCatalogNativeSource(shell, reviewSource)
	if err != nil {
		return err
	}
	for _, result := range survivors {
		if result.Kind == "command" {
			implementation := result.Entry.Native[shell]
			if implementation.AliasValue != nil && len(*implementation.AliasValue) > 0 && strings.ContainsAny((*implementation.AliasValue)[len(*implementation.AliasValue)-1:], " \t") {
				return fmt.Errorf("trailing-blank native alias is unsupported; remove the trailing blank before al catalog enable")
			}
			aliases[result.Name] = true
		}
	}
	for index, entry := range entries {
		if err := ValidateCatalogDeclaration(shell, entry, declarations[index]); err != nil {
			return err
		}
		if implementation, ok := entry.Native[shell]; ok {
			text := ""
			if implementation.FunctionBody != nil {
				text = *implementation.FunctionBody
			}
			if implementation.AliasValue != nil {
				text = *implementation.AliasValue
			}
			if err := rejectCatalogAliasDependencies(shell, entry.Name, text, aliases, implementation.AliasValue != nil); err != nil {
				return err
			}
		}
		if _, nativeImplementation := entry.Native[shell]; !nativeImplementation && entry.Portable != nil {
			if err := rejectCatalogPortableAliasDependencies(entry.Name, aliases); err != nil {
				return err
			}
		}
	}
	for _, result := range survivors {
		if result.Kind == "command" {
			implementation := result.Entry.Native[shell]
			if implementation.AliasValue != nil {
				if err := rejectCatalogAliasDependencies(shell, result.Name, *implementation.AliasValue, aliases, true); err != nil {
					return nativeBoundaryError(result.StartLine, result.EndLine, "retained alias has an ambiguous or unsupported alias dependency")
				}
			}
		}
		if result.Kind == "function" {
			if err := rejectCatalogAliasDependencies(shell, result.Name, result.Body, aliases, false); err != nil {
				return nativeBoundaryError(result.StartLine, result.EndLine, "retained function has an ambiguous dependency on an alias")
			}
		}
	}

	return nil
}

func ValidateCatalogDeclarations(shell string, entries []neutralcatalog.Entry, declarations [][]byte, native []byte, validator func(context.Context, string, []byte) error) error {
	if err := ValidateCatalogDependencyPolicy(shell, entries, declarations, native); err != nil {
		return err
	}
	if validator == nil {
		validator = func(ctx context.Context, name string, data []byte) error { return ValidateSyntax(ctx, name, data, nil) }
	}
	var combined bytes.Buffer
	for _, declaration := range declarations {
		combined.Write(declaration)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := validator(ctx, shell, combined.Bytes()); err != nil {
		return fmt.Errorf("catalog declarations failed isolated %s parse-only validation", shell)
	}
	return nil
}

func rejectCatalogPortableAliasDependencies(name string, aliases map[string]bool) error {
	for _, token := range []string{"function", "builtin", "exec"} {
		if aliases[token] {
			return fmt.Errorf("%s has an ambiguous dependency on alias %s; migrate it manually before al catalog enable", name, token)
		}
	}
	return nil
}

func ValidateCatalogNativeDeclaration(shell string, declaration []byte) error {
	if shell != "bash" && shell != "zsh" {
		return fmt.Errorf("unsupported shell %q (use bash or zsh)", shell)
	}

	results := ImportShadowSource(shell, declaration)
	if len(results) != 1 || results[0].Entry == nil {
		return fmt.Errorf("native implementation must be one complete declaration in the supported shell grammar; simplify quoting and continuations before al catalog approve")
	}
	if CatalogProtectedName(results[0].Name) || results[0].StartByte != 0 || results[0].EndByte != len(declaration) {
		return fmt.Errorf("native implementation escapes its declaration or uses a protected name")
	}
	return nil
}
