//go:build !windows

package shell

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "source ") && !strings.HasPrefix(line, ". ") {
		return false
	}
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
	return CatalogStartupPlacementWithNative(contents, shell, home, nativePath, names, nil)
}

func CatalogStartupPlacementWithNative(contents []byte, shell, home, nativePath string, names, authoritativeAliases map[string]bool) ([]byte, bool, int, error) {
	return CatalogStartupPlacementPolicy(contents, shell, home, nativePath, names, authoritativeAliases, false)
}

func CatalogStartupPlacementPolicy(contents []byte, shell, home, nativePath string, names, authoritativeAliases map[string]bool, explicit bool) ([]byte, bool, int, error) {
	if err := validateSource(contents); err != nil {
		return nil, false, 0, startupPlacementError(1, "startup source bounds are unsupported")
	}
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
	literalMask, err := startupMask(contents, false)
	if err != nil {
		return nil, false, 0, err
	}
	literalMask, err = startupDeferredMask(contents, literalMask)
	if err != nil {
		return nil, false, 0, err
	}
	ownedStart := -1
	count := 0
	for offset, attempts := 0, 0; offset < len(contents); {
		next := strings.Index(text[offset:], owned)
		if next < 0 {
			break
		}
		attempts++
		if attempts > 32 {
			return nil, false, 0, fmt.Errorf("too many owned startup candidates; review al setup --repair")
		}
		start := offset + next
		offset = start + len(owned)
		if !bytes.Contains(literalMask[start:start+len(owned)], []byte("Alias Lens "+friendlyShellName(shell)+" alias loader")) {
			continue
		}
		if !startupOwnedVisible(contents, literalMask, start, "Alias Lens "+friendlyShellName(shell)+" alias loader") {
			return nil, false, 0, startupPlacementError(1, "nested owned startup block is unproven")
		}
		ownedStart = start
		count++
		if count > 1 {
			return nil, false, 0, fmt.Errorf("duplicate owned startup blocks; review al setup --repair")
		}
	}
	if count > 1 {
		return nil, false, 0, fmt.Errorf("duplicate owned startup blocks; review al setup --repair")
	}
	if ownedStart >= 0 {
		insertion = ownedStart
		text = text[:ownedStart] + text[ownedStart+len(owned):]
	}
	literalMask, err = startupMask([]byte(text), false)
	if err != nil {
		return nil, false, 0, err
	}
	literalMask, err = startupDeferredMask([]byte(text), literalMask)
	if err != nil {
		return nil, false, 0, err
	}
	if bytes.Contains(literalMask, []byte("Alias Lens "+friendlyShellName(shell)+" alias loader")) {
		return nil, false, 0, fmt.Errorf("edited owned startup block; review al setup --repair")
	}
	if bytes.Contains(literalMask, []byte(catalogLoaderStart)) || bytes.Contains(literalMask, []byte(catalogLoaderEnd)) {
		return nil, false, 0, fmt.Errorf("existing catalog block requires installed startup verification")
	}
	preservedText := text
	analysis, err := startupAnalysisMask([]byte(text))
	if err != nil {
		return nil, false, 0, err
	}
	for _, guard := range []string{"case $- in\n    *i*) ;;\n      *) return;;\nesac", "case $- in\n    *i*) ;;\n      *) return ;;\nesac"} {
		if start := strings.Index(text, guard); start >= 0 && strings.HasPrefix(strings.TrimSpace(string(analysis[start:start+len(guard)])), "case $- in") {
			for index := start; index < start+len(guard); index++ {
				if analysis[index] != '\n' {
					analysis[index] = ' '
				}
			}
		}
	}
	text = string(analysis)
	lines := strings.Split(text, "\n")
	originalLines := strings.Split(preservedText, "\n")
	preservedBytes := []byte(preservedText)
	sourceLines := SourceLines(preservedBytes)
	endLine := func(index int) int {
		if index < len(sourceLines) {
			return sourceLines[index].End
		}
		return len(preservedText)
	}
	sourceCount := 0
	routeStart, routeEnd := -1, -1
	otherSources := false
	depth := 0
	declarations := []startupDeclaration{}
	skipThrough := -1
	staticHints := map[string]string{}
	seenHints := map[string]bool{}
	for index, raw := range lines {
		if index <= skipThrough {
			continue
		}
		line := strings.TrimSpace(raw)
		original := strings.TrimSpace(originalLines[index])
		if line != "" && !strings.HasPrefix(line, "#") && index < len(sourceLines) {
			name, open, ok := nativeFunctionHeader(preservedBytes, sourceLines[index].Start)
			if ok {
				close, reason := (nativeBoundaryScanner{source: preservedBytes}).region(open+1, '}', 0)
				if reason != "" {
					return nil, false, 0, startupPlacementError(index+1, "deferred function boundary is unsupported")
				}
				closingLine := sort.Search(len(sourceLines), func(i int) bool { return sourceLines[i].End > close })
				if closingLine >= len(sourceLines) {
					return nil, false, 0, startupPlacementError(index+1, "deferred function boundary is unproven")
				}
				tail := strings.TrimSpace(preservedText[close+1 : sourceLines[closingLine].End])
				if tail != "" && !strings.HasPrefix(tail, "#") {
					return nil, false, 0, startupPlacementError(index+1, "function declaration has an unsupported tail")
				}
				declarations = append(declarations, startupDeclaration{name: name, kind: "function", line: index + 1, end: endLine(closingLine)})
				skipThrough = closingLine
				continue
			}
		}
		if line != "" && !strings.HasPrefix(line, "#") && strings.ContainsAny(line, "{}") {
			return nil, false, 0, startupPlacementError(index+1, "unproven unquoted brace syntax needs startup repair")
		}
		if line != "" && !strings.HasPrefix(line, "#") && startupUnprovenFunctionHeader(line) {
			return nil, false, 0, startupPlacementError(index+1, "unproven startup function header needs explicit repair")
		}
		if explicit && depth == 0 && startupGuardedOpaqueSource(originalLines[index], raw, shell, home, nativePath, staticHints) {
			otherSources = true
			continue
		}
		if startupQuotedControl(originalLines[index], raw) {
			return nil, false, 0, startupPlacementError(index+1, "quoted or escaped startup control needs explicit placement")
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if explicit {
			if !startupUpdateHints(original, depth, home, staticHints, seenHints) {
				return nil, false, 0, startupPlacementError(index+1, "visible HOME mutation makes startup placement unproven")
			}
		}
		if CatalogStaticSource(original, home, nativePath) {
			if depth != 0 {
				return nil, false, 0, fmt.Errorf("nested native source needs explicit startup placement")
			}
			sourceCount++
			insertion = endLine(index)
			routeStart, routeEnd = sourceLines[index].Start, insertion
			sourceNative = false
			continue
		}
		guard := strings.HasPrefix(original, "if [ -f ") && strings.HasSuffix(original, " ]; then") && CatalogStaticSource("source "+strings.TrimSuffix(strings.TrimPrefix(original, "if [ -f "), " ]; then"), home, nativePath)
		zguard := strings.HasPrefix(original, "if [[ -f ") && strings.HasSuffix(original, " ]]; then") && CatalogStaticSource("source "+strings.TrimSuffix(strings.TrimPrefix(original, "if [[ -f "), " ]]; then"), home, nativePath)
		if guard || (shell == "zsh" && zguard) {
			if depth != 0 || index+2 >= len(lines) || !CatalogStaticSource(strings.TrimSpace(originalLines[index+1]), home, nativePath) || strings.TrimSpace(lines[index+2]) != "fi" {
				return nil, false, 0, fmt.Errorf("native source guard needs explicit startup placement")
			}
			lines[index] = ""
			lines[index+1] = ""
			lines[index+2] = ""
			sourceCount++
			insertion = endLine(index + 2)
			routeStart, routeEnd = sourceLines[index].Start, insertion
			sourceNative = false
			continue
		}
		if strings.Contains(line, filepath.Base(nativePath)) || startupControlWord(line) == "source" || startupControlWord(line) == "." {
			if !explicit || !startupStaticOtherSource(original, shell, home, nativePath) {
				return nil, false, 0, startupPlacementError(index+1, "dynamic or unenrolled startup source needs explicit placement")
			}
			otherSources = true
			continue
		}
		for _, start := range startupCommandStarts(raw) {
			segment := strings.TrimSpace(raw[start:])
			originalSegment := strings.TrimSpace(originalLines[index][start:])
			word := strings.Fields(segment)
			if len(word) > 0 && (word[0] == "builtin" || word[0] == "command") {
				word = word[1:]
			}
			if len(word) > 0 && word[0] == "eval" {
				if strings.Contains(originalSegment, ".bash_aliases") || strings.Contains(originalSegment, ".zsh_aliases") || strings.Contains(originalSegment, "source ") {
					return nil, false, 0, startupPlacementError(index+1, "evaluated startup source route is unproven")
				}
				if !explicit {
					return nil, false, 0, startupPlacementError(index+1, "opaque startup evaluation needs explicit --startup-path enrollment")
				}
				otherSources = true
				continue
			}
			if strings.HasPrefix(segment, "alias ") || strings.HasPrefix(segment, "alias\t") || strings.HasPrefix(segment, "builtin alias ") {
				name, ok := startupLiteralAlias(originalSegment)
				if !ok {
					return nil, false, 0, startupPlacementError(index+1, "startup alias definition is unproven")
				}
				declarations = append(declarations, startupDeclaration{name: name, kind: "command", line: index + 1, end: endLine(index)})
			}
			if start > 0 && startupControlWord(segment) != "" {
				return nil, false, 0, startupPlacementError(index+1, "startup command list needs explicit placement")
			}
		}

		if startupControlWord(line) != "" {
			return nil, false, 0, startupPlacementError(index+1, "startup control flow needs explicit placement")
		}

		if strings.HasPrefix(line, "if ") || strings.HasPrefix(line, "case ") || strings.HasPrefix(line, "for ") || strings.HasPrefix(line, "while ") {
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
	if explicit && otherSources && sourceCount != 1 {
		return nil, false, 0, startupPlacementError(1, "explicit startup enrollment needs exactly one proven native source route")
	}
	if explicit && routeStart >= 0 {
		route := preservedText[routeStart:routeEnd]
		rest := preservedText[:routeStart] + preservedText[routeEnd:]
		if rest != "" && !strings.HasSuffix(rest, "\n") {
			rest += "\n"
		}
		preservedText = rest + route
		insertion = len(preservedText)
	}
	for _, declaration := range declarations {
		if startupMasksControl(declaration.name, declaration.kind) {
			return nil, false, 0, startupPlacementError(declaration.line, "startup declaration masks catalog handoff control")
		}
		if names[declaration.name] && !(declaration.kind == "command" && authoritativeAliases[declaration.name] && declaration.end <= insertion) {
			return nil, false, 0, startupPlacementError(declaration.line, "startup defines a catalog name; remove the late override before al catalog enable")
		}
	}
	return []byte(preservedText), sourceNative, insertion, nil
}

func CatalogZshStartupPath(home string) (string, error) {
	return catalogZshStartupPath(home, defaultRuntime(Runtime{}))
}
func catalogZshStartupPath(home string, runtime Runtime) (string, error) {
	directory := runtime.Environment("ZDOTDIR")
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

func CatalogNativeReviewSource(adapter Adapter, native []byte) ([]byte, error) {
	stripped, start, err := CatalogRemoveOwnedIntegration(adapter, native)
	if err != nil {
		return nil, err
	}
	masked := append([]byte{}, native...)
	if start >= 0 {
		for index := start; index < start+len(native)-len(stripped); index++ {
			if masked[index] != '\n' {
				masked[index] = ' '
			}
		}
	}
	return masked, nil
}

func CatalogRemoveOwnedIntegration(adapter Adapter, native []byte) ([]byte, int, error) {
	type candidate struct{ start, length int }
	candidates := []candidate{}
	forms := append([]string{}, adapter.KnownIntegrationForms()...)
	forms = append(forms, fmt.Sprintf("eval \"$(alias-lens shell-init %s)\"\n", adapter.Name()), fmt.Sprintf("eval \"$(command alias-lens shell-init %s)\"\n", adapter.Name()))
	for _, form := range forms {
		block := []byte(form)
		for offset := 0; offset < len(native); {
			index := bytes.Index(native[offset:], block)
			if index < 0 {
				break
			}
			position := offset + index
			offset = position + len(block)
			if position == 0 || native[position-1] == '\n' {
				candidates = append(candidates, candidate{position, len(block)})
				if len(candidates) > 32 {
					return nil, 0, fmt.Errorf("native integration candidate count exceeds inspection bounds; run al setup --repair")
				}
			}
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].start < candidates[j].start })
	start, length := -1, 0
	for _, candidate := range candidates {
		prefix := append([]byte{}, native[:candidate.start]...)
		if start >= 0 {
			for index := start; index < start+length; index++ {
				if prefix[index] != '\n' {
					prefix[index] = ' '
				}
			}
		}
		if _, err := InspectCatalogNativeSource(adapter.Name(), prefix); err != nil {
			continue
		}
		if start >= 0 {
			return nil, 0, fmt.Errorf("duplicate native integration; run al setup --repair")
		}
		start, length = candidate.start, candidate.length
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
