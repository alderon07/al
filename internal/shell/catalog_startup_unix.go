//go:build !windows

package shell

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const catalogLoaderStart = "# >>> Alias Lens catalog v2 >>>"
const catalogLoaderEnd = "# <<< Alias Lens catalog v2 <<<"

func CatalogLoaderBlock(shell, nativePath, executable, generatedRoot string, sourceNative bool) string {
	if shell != "bash" && shell != "zsh" {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n" + catalogLoaderStart + "\n")
	if sourceNative {
		fmt.Fprintf(&b, "if [ -f %s ]; then\n  . %s\nfi\n", QuoteShadow(nativePath), QuoteShadow(nativePath))
	}
	fmt.Fprintf(&b, "if _alias_lens_catalog_buffer=$(%s catalog-loader --shell %s --root %s --native %s 2>/dev/null); then\n", QuoteShadow(executable), QuoteShadow(shell), QuoteShadow(generatedRoot), QuoteShadow(nativePath))
	fmt.Fprintf(&b, "  builtin eval \"$_alias_lens_catalog_buffer\"\nelse\n  builtin printf 'Alias Lens catalog unavailable; run al catalog enable --shell %s\\n' >&2\nfi\nbuiltin unset _alias_lens_catalog_buffer\n"+catalogLoaderEnd+"\n", shell)
	return b.String()
}

func CatalogStaticSource(line, home, path string) bool {
	candidates := []string{path}
	for _, name := range []string{".bash_aliases", ".zsh_aliases"} {
		logical := filepath.Join(home, name)
		resolved, err := filepath.EvalSymlinks(logical)
		if err == nil && resolved == path {
			candidates = append(candidates, logical)
		}
	}
	line = strings.TrimSpace(line)
	for _, candidate := range candidates {
		for _, verb := range []string{".", "source"} {
			for _, operand := range []string{QuoteShadow(candidate), `"` + candidate + `"`, candidate, `"$HOME/` + strings.TrimPrefix(candidate, home+"/") + `"`, `"${HOME}/` + strings.TrimPrefix(candidate, home+"/") + `"`, "~/" + strings.TrimPrefix(candidate, home+"/")} {
				if line == verb+" "+operand {
					return true
				}
			}
		}
	}
	return false
}

func CatalogStartupPlacement(contents []byte, shell, home, nativePath string, names map[string]bool) ([]byte, bool, error) {
	preserved, load, _, err := CatalogStartupPlacementAt(contents, shell, home, nativePath, names)
	return preserved, load, err
}

func CatalogStartupPlacementAt(contents []byte, shell, home, nativePath string, names map[string]bool) ([]byte, bool, int, error) {
	if shell != "bash" && shell != "zsh" {
		return nil, false, 0, fmt.Errorf("unsupported shell %q (use bash or zsh)", shell)
	}

	text := string(contents)
	insertion := -1
	owned := BashAliasLoader
	if shell == "zsh" {
		owned = ZshAliasLoader
	}
	sourceNative := true
	if strings.Count(text, owned) > 1 {
		return nil, false, 0, fmt.Errorf("duplicate owned startup blocks; review al setup --repair")
	}
	if index := strings.Index(text, owned); index >= 0 {
		insertion = index
	}
	text = strings.Replace(text, owned, "", 1)
	if strings.Contains(text, "Alias Lens "+friendlyShellName(shell)+" alias loader") {
		return nil, false, 0, fmt.Errorf("edited owned startup block; review al setup --repair")
	}
	if strings.Contains(text, catalogLoaderStart) || strings.Contains(text, catalogLoaderEnd) {
		return nil, false, 0, fmt.Errorf("existing catalog block requires installed startup verification")
	}
	preservedText := text
	text = strings.Replace(text, "case $- in\n    *i*) ;;\n      *) return;;\nesac", "", 1)
	text = strings.Replace(text, "case $- in\n    *i*) ;;\n      *) return ;;\nesac", "", 1)
	lines := strings.Split(text, "\n")
	originalLines := strings.Split(preservedText, "\n")
	endLine := func(index int) int {
		return min(len(preservedText), len(strings.Join(originalLines[:index+1], "\n"))+1)
	}
	sourceCount := 0
	depth := 0
	for index, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if CatalogStaticSource(line, home, nativePath) {
			if depth != 0 {
				return nil, false, 0, fmt.Errorf("nested native source needs explicit startup placement")
			}
			sourceCount++
			insertion = endLine(index)
			sourceNative = false
			continue
		}
		guard := strings.HasPrefix(line, "if [ -f ") && strings.HasSuffix(line, " ]; then") && CatalogStaticSource("source "+strings.TrimSuffix(strings.TrimPrefix(line, "if [ -f "), " ]; then"), home, nativePath)
		zguard := strings.HasPrefix(line, "if [[ -f ") && strings.HasSuffix(line, " ]]; then") && CatalogStaticSource("source "+strings.TrimSuffix(strings.TrimPrefix(line, "if [[ -f "), " ]]; then"), home, nativePath)
		if guard || (shell == "zsh" && zguard) {
			if depth != 0 || index+2 >= len(lines) || !CatalogStaticSource(strings.TrimSpace(lines[index+1]), home, nativePath) || strings.TrimSpace(lines[index+2]) != "fi" {
				return nil, false, 0, fmt.Errorf("native source guard needs explicit startup placement")
			}
			lines[index] = ""
			lines[index+1] = ""
			lines[index+2] = ""
			sourceCount++
			insertion = endLine(index + 2)
			sourceNative = false
			continue
		}
		if strings.Contains(line, filepath.Base(nativePath)) || strings.HasPrefix(line, "source ") || strings.HasPrefix(line, ". ") {
			return nil, false, 0, fmt.Errorf("dynamic startup source needs explicit startup placement")
		}
		for _, token := range catalogNameTokens.FindAllString(line, -1) {
			if names[token] && (strings.Contains(line, "alias ") || strings.Contains(line, "()") || strings.Contains(line, "function ")) {
				return nil, false, 0, fmt.Errorf("startup defines a catalog name; remove the late override before al catalog enable")
			}
		}
		for _, token := range catalogNameTokens.FindAllString(line, -1) {
			if token == "return" || token == "exit" || token == "exec" {
				return nil, false, 0, fmt.Errorf("startup control flow needs explicit placement")
			}
		}
		if strings.HasPrefix(line, "return") || strings.HasPrefix(line, "exit") || strings.HasPrefix(line, "exec ") {
			return nil, false, 0, fmt.Errorf("startup control flow needs explicit placement")
		}
		if strings.HasPrefix(line, "if ") || strings.HasPrefix(line, "case ") || strings.HasPrefix(line, "for ") || strings.HasPrefix(line, "while ") || strings.Contains(line, "() {") {
			depth++
		}
		if line == "fi" || line == "esac" || line == "done" || line == "}" {
			depth--
			if depth < 0 {
				return nil, false, 0, fmt.Errorf("startup control flow is ambiguous")
			}
		}
	}
	if depth != 0 || sourceCount > 1 {
		return nil, false, 0, fmt.Errorf("startup source route is ambiguous; use explicit placement")
	}
	if insertion < 0 {
		insertion = len(preservedText)
	}
	return []byte(preservedText), sourceNative, insertion, nil
}

func CatalogZshStartupPath(home string) (string, error) {
	directory := os.Getenv("ZDOTDIR")
	if directory != "" && !filepath.IsAbs(directory) {
		return "", fmt.Errorf("ZDOTDIR must be absolute; set it before al catalog enable")
	}
	if directory == "" {
		directory = home
	}
	zshenv := filepath.Join(directory, ".zshenv")
	contents, err := readRegularFile(zshenv, shadowSourceLimit)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	assignment := regexp.MustCompile(`^(?:export )?ZDOTDIR=(?:'([^']+)'|"([^"$` + "`" + `]+)"|(/[A-Za-z0-9_./-]+))$`)
	found := false
	for _, raw := range strings.Split(string(contents), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.Contains(line, "ZDOTDIR") {
			return "", fmt.Errorf("unsupported .zshenv route; enroll the intended file with al catalog enable --startup-path PATH")
		}
		match := assignment.FindStringSubmatch(line)
		if match == nil || found {
			return "", fmt.Errorf("dynamic ZDOTDIR needs al catalog enable --startup-path PATH")
		}
		found = true
		for _, value := range match[1:] {
			if value != "" {
				directory = value
			}
		}
		if !filepath.IsAbs(directory) {
			return "", fmt.Errorf("static ZDOTDIR must be absolute")
		}
	}
	return filepath.Join(directory, ".zshrc"), nil
}

func CatalogLoginLoadsBashrc(contents []byte, home string) bool {
	text := string(contents)
	if strings.Count(text, BashLoginLoader) == 1 {
		return strings.Count(strings.Replace(text, BashLoginLoader, "", 1), ".bashrc") == 0 && !strings.Contains(text, ".bash_aliases")
	}
	guard := "if [ -n \"$BASH_VERSION\" ]; then\n    if [ -f \"$HOME/.bashrc\" ]; then\n        . \"$HOME/.bashrc\"\n    fi\nfi"
	if strings.Count(text, guard) == 1 {
		return strings.Count(strings.Replace(text, guard, "", 1), ".bashrc") == 0 && !strings.Contains(text, ".bash_aliases")
	}
	count := 0
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if CatalogStaticSource(trimmed, home, filepath.Join(home, ".bashrc")) {
			count++
			continue
		}
		for _, token := range catalogNameTokens.FindAllString(trimmed, -1) {
			if token == "if" || token == "for" || token == "while" || token == "case" || token == "return" || token == "exit" || token == "exec" || token == "source" {
				return false
			}
		}
		if strings.Contains(trimmed, ".bashrc") {
			return false
		}
	}
	return count == 1 && !strings.Contains(text, ".bash_aliases")
}

func CatalogPinnedIntegration(adapter Adapter, executable string) (string, error) {
	text := adapter.Integration()
	declarations := regexp.MustCompile(`(?m)^([A-Za-z_][A-Za-z0-9_]*)\(\) \{$`)
	text = declarations.ReplaceAllString(text, "function $1 {")
	text = strings.Replace(text, "unalias al 2>/dev/null || true\n", "", 1)
	for _, result := range ImportShadowSource(adapter.Name(), []byte(text)) {
		if result.Name == "al" && result.Entry != nil {
			declaration := text[result.StartByte:result.EndByte]
			guarded := "if " + declaration + "then builtin unalias al 2>/dev/null || :; fi\n"
			text = text[:result.StartByte] + guarded + text[result.EndByte:]
			break
		}
	}
	text = strings.ReplaceAll(text, " alias-lens", " "+QuoteShadow(executable))
	lines := strings.Split(text, "\n")
	kept := []string{}
	for _, line := range lines {
		if strings.Contains(line, " watch --ensure ") {
			continue
		}
		kept = append(kept, line)
	}
	text = strings.Join(kept, "\n")
	return text + "\n", nil
}

func (bashShellAdapter) CatalogStartupRoute(path string, contents []byte, home string) (string, bool) {
	if filepath.Base(path) == ".bashrc" {
		return "bash-nonlogin", false
	}
	if CatalogLoginLoadsBashrc(contents, home) {
		return "via-bashrc", true
	}
	return "bash-login", false
}
func (zshShellAdapter) CatalogStartupRoute(string, []byte, string) (string, bool) {
	return "zsh-interactive", false
}

func CatalogRemoveOwnedIntegration(adapter Adapter, native []byte) ([]byte, int, error) {
	start, length := -1, 0
	for _, form := range adapter.KnownIntegrationForms() {
		block := []byte(form)
		for offset := 0; offset < len(native); {
			index := bytes.Index(native[offset:], block)
			if index < 0 {
				break
			}
			position := offset + index
			offset = position + len(block)
			if position > 0 && native[position-1] != '\n' {
				continue
			}
			proven := true
			for _, prefix := range ImportShadowSource(adapter.Name(), native[:position]) {
				if prefix.Entry == nil {
					proven = false
				}
			}
			if !proven {
				continue
			}
			if start >= 0 {
				return nil, 0, fmt.Errorf("duplicate native integration; run al setup --repair")
			}
			start, length = position, len(block)
		}
	}
	if start < 0 {
		return append([]byte{}, native...), -1, nil
	}
	return append(append([]byte{}, native[:start]...), native[start+length:]...), start, nil
}

func friendlyShellName(name string) string {
	if name == "bash" {
		return "Bash"
	}
	if name == "zsh" {
		return "Zsh"
	}
	return name
}
