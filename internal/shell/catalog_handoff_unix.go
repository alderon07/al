//go:build !windows

package shell

import (
	"alias-lens/internal/catalogstore"
	"fmt"
	"strings"
)

func RuntimeHandoff(shell string, entries []catalogstore.GenerationEntry) (string, error) {
	if shell != "bash" && shell != "zsh" {
		return "", fmt.Errorf("unsupported shell %q (use bash or zsh)", shell)
	}
	var output strings.Builder
	output.WriteString("if function _alias_lens_catalog_handoff {\n  :\n")
	if shell == "bash" && len(entries) > 0 {
		output.WriteString("  builtin local _alias_lens_readonly_functions _alias_lens_readonly_line _alias_lens_readonly_name\n  _alias_lens_readonly_functions=\"$(builtin declare -Fr)\" || return 1\n  while IFS= builtin read -r _alias_lens_readonly_line; do\n    _alias_lens_readonly_name=\"${_alias_lens_readonly_line##* }\"\n")
		for _, item := range entries {
			fmt.Fprintf(&output, "    if builtin test \"$_alias_lens_readonly_name\" = %s; then return 1; fi\n", QuoteShadow(item.Entry.Name))
		}
		output.WriteString("  done <<< \"$_alias_lens_readonly_functions\"\n")
	}
	if shell == "zsh" {
		output.WriteString("  [[ ${(t)functions} != *readonly* && ${(t)aliases} != *readonly* ]] || return 1\n")
	}
	for _, item := range entries {
		declaration := item.Declaration
		if strings.HasPrefix(declaration, "alias ") {
			fmt.Fprintf(&output, "  builtin %s", strings.TrimSuffix(declaration, "\n")+" || return 1\n")
		} else {
			fmt.Fprintf(&output, "  if builtin eval %s; then builtin unalias -- %s 2>/dev/null || :; else return 1; fi\n", QuoteShadow(declaration), QuoteShadow(item.Entry.Name))
		}
	}
	output.WriteString("}; then\nif \\_alias_lens_catalog_handoff; then\n  builtin unset -f _alias_lens_catalog_handoff\n  builtin true\nelse\n  builtin unset -f _alias_lens_catalog_handoff\n  builtin false\nfi\nelse\n  builtin false\nfi\n")
	return output.String(), nil
}
