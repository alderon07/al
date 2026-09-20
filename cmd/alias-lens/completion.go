package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	neutralcatalog "alias-lens/internal/catalog"
	workflowplan "alias-lens/internal/plan"
)

const (
	completionSourceLimit = 8 << 20
	completionOutputLimit = 1 << 20
	completionMaxResults  = 1000
)

type commandSpec struct {
	Name    string
	Summary string
	Usage   string
}

type completionRule struct {
	Path    []string
	Values  []string
	Dynamic string
}

var completionCandidateName = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,254}$`)

// publicCommandSpecs is the shared index for command help and shell completion.
// Detailed help remains in commandUsage so existing 1.x text stays byte-for-byte
// compatible while the command index has one source of names and summaries.
var publicCommandSpecs = newPublicCommandSpecs()

func newPublicCommandSpecs() []commandSpec {
	rows := [][2]string{
		{"pick", "Select an alias without using it"},
		{"use", "Select and use an alias through the shell integration"},
		{"search", "Find aliases by name or metadata"},
		{"stats", "Show alias usage"},
		{"export", "Export aliases or usage data"},
		{"import", "Preview or import aliases from a file"},
		{"suggest", "Find repeated commands in shell history"},
		{"meta", "Edit alias metadata"},
		{"describe", "Add missing alias descriptions"},
		{"check", "Check alias syntax without using it"},
		{"scan", "Find likely secrets without showing their values"},
		{"history", "List private alias revisions"},
		{"undo", "Restore a private alias revision"},
		{"doctor", "Diagnose the Alias Lens installation"},
		{"setup", "Install, repair, or remove shell integration"},
		{"data", "Show or clear private local data"},
		{"status", "Show what is ready without changing files"},
		{"plan", "Preview a change without applying it"},
		{"catalog", "Inspect shell-neutral catalog migration"},
		{"repo", "Configure a Git repository"},
		{"config", "Show or change Alias Lens settings"},
		{"track", "Add a file to automatic sync"},
		{"untrack", "Remove a file from automatic sync"},
		{"sync", "Synchronize aliases with the configured repository"},
		{"diff", "Compare local and repository aliases"},
		{"autosync", "Configure background synchronization"},
		{"watch", "Check once for changes that need to sync"},
		{"theme", "Show or select a terminal theme"},
		{"shortcuts", "Show or select keyboard shortcuts"},
		{"completion", "Print Bash or Zsh completion code"},
		{"shell-init", "Print shell integration code"},
		{"--web", "Start the optional local browser"},
		{"--version", "Print the installed version"},
	}
	result := make([]commandSpec, 0, len(rows))
	for _, row := range rows {
		result = append(result, commandSpec{Name: row[0], Summary: row[1], Usage: commandUsage[row[0]]})
	}
	return result
}

func lookupCommandSpec(name string) (commandSpec, bool) {
	if name == "version" || name == "-v" {
		name = "--version"
	}
	for _, spec := range publicCommandSpecs {
		if spec.Name == name {
			return spec, true
		}
	}
	return commandSpec{}, false
}

func completionRules() []completionRule {
	commands := make([]string, 0, len(publicCommandSpecs))
	for _, spec := range publicCommandSpecs {
		commands = append(commands, spec.Name)
	}
	commands = append(commands, "help")
	shells := []string{"bash", "zsh"}
	providers := []string{"github", "bitbucket", "gitlab"}
	periods := []string{"all", "today", "week", "month", "year"}
	return []completionRule{
		{Values: commands},
		{Path: []string{"help"}, Values: commands},
		{Path: []string{"completion"}, Values: append([]string{"install", "remove"}, shells...)},
		{Path: []string{"completion", "install"}, Values: shells},
		{Path: []string{"completion", "remove"}, Values: shells},
		{Path: []string{"shell-init"}, Values: shells},
		{Path: []string{"setup"}, Values: append([]string{"--repair", "--remove"}, shells...)},
		{Path: []string{"setup", "--repair"}, Values: shells},
		{Path: []string{"setup", "--remove"}, Values: shells},
		{Path: []string{"config"}, Values: []string{"shell", "provider", "protocol", "disable", "migrate", "profile", "footer-message", "footer-icon", "footer-reset"}},
		{Path: []string{"config", "shell"}, Values: shells},
		{Path: []string{"config", "provider"}, Values: providers},
		{Path: []string{"config", "protocol"}, Values: providers},
		{Path: []string{"config", "protocol", "github"}, Values: []string{"auto", "ssh", "https"}},
		{Path: []string{"config", "protocol", "bitbucket"}, Values: []string{"auto", "ssh", "https"}},
		{Path: []string{"config", "protocol", "gitlab"}, Values: []string{"auto", "ssh", "https"}},
		{Path: []string{"config", "disable"}, Values: providers},
		{Path: []string{"config", "profile"}, Values: []string{"list", "add", "remove"}},
		{Path: []string{"config", "profile", "remove"}, Dynamic: "profiles"},
		{Path: []string{"data"}, Values: []string{"paths", "clear-usage", "clear-revisions"}},
		{Path: []string{"status"}, Values: []string{"--json"}},
		{Path: []string{"plan"}, Values: []string{"--json", "config", "catalog", "completion"}},
		{Path: []string{"plan", "config"}, Values: []string{"migrate", "profile"}},
		{Path: []string{"plan", "config", "profile"}, Values: []string{"add", "remove"}},
		{Path: []string{"plan", "config", "profile", "remove"}, Dynamic: "profiles"},
		{Path: []string{"plan", "catalog"}, Values: []string{"migrate"}},
		{Path: []string{"plan", "catalog", "migrate"}, Values: []string{"--to"}},
		{Path: []string{"plan", "catalog", "migrate", "--to"}, Values: []string{"2"}},
		{Path: []string{"plan", "completion"}, Values: []string{"install", "remove"}},
		{Path: []string{"plan", "completion", "install"}, Values: shells},
		{Path: []string{"plan", "completion", "remove"}, Values: shells},
		{Path: []string{"catalog"}, Values: []string{"preview", "import", "shadow", "diff", "migrate"}},
		{Path: []string{"catalog", "preview"}, Values: []string{"--json", "--from", "--shell"}},
		{Path: []string{"catalog", "preview", "--from"}, Values: shells},
		{Path: []string{"catalog", "preview", "--shell"}, Values: shells},
		{Path: []string{"catalog", "import"}, Values: []string{"--from"}},
		{Path: []string{"catalog", "import", "--from"}, Values: shells},
		{Path: []string{"catalog", "shadow"}, Values: []string{"--json", "--shell"}},
		{Path: []string{"catalog", "shadow", "--shell"}, Values: shells},
		{Path: []string{"catalog", "shadow", "--json"}, Values: []string{"--shell"}},
		{Path: []string{"catalog", "shadow", "--json", "--shell"}, Values: shells},
		{Path: []string{"catalog", "diff"}, Values: []string{"--json", "--show-code", "--web", "--from", "--shell"}},
		{Path: []string{"catalog", "diff", "--from"}, Values: []string{"repository", "installed"}},
		{Path: []string{"catalog", "diff", "--shell"}, Values: shells},
		{Path: []string{"catalog", "migrate"}, Values: []string{"--to"}},
		{Path: []string{"catalog", "migrate", "--to"}, Values: []string{"2"}},
		{Path: []string{"repo"}, Values: providers},
		{Path: []string{"suggest"}, Values: []string{"add"}},
		{Path: []string{"stats"}, Values: append([]string{"--plain"}, periods...)},
		{Path: []string{"export"}, Values: []string{"aliases", "stats", "--format", "--period", "--output"}},
		{Path: []string{"export", "--format"}, Values: []string{"json", "yaml", "csv"}},
		{Path: []string{"export", "--period"}, Values: periods},
		{Path: []string{"export", "aliases"}, Values: []string{"--format", "--output"}},
		{Path: []string{"export", "stats"}, Values: []string{"--format", "--period", "--output"}},
		{Path: []string{"export", "aliases", "--format"}, Values: []string{"json", "yaml", "csv"}},
		{Path: []string{"export", "stats", "--format"}, Values: []string{"json", "yaml", "csv"}},
		{Path: []string{"export", "stats", "--period"}, Values: periods},
		{Path: []string{"import"}, Values: []string{"--apply"}},
		{Path: []string{"check"}, Values: []string{"--strict"}},
		{Path: []string{"sync"}, Values: []string{"--push", "--pull"}},
		{Path: []string{"autosync"}, Values: []string{"enable", "disable", "status"}},
		{Path: []string{"shortcuts"}, Values: []string{"auto", "windows", "linux", "macos", "test"}},
		{Path: []string{"theme"}, Values: append([]string{"--check"}, themeOrder...)},
		{Path: []string{"pick"}, Values: []string{"--command"}, Dynamic: "entries"},
		{Path: []string{"pick", "--command"}, Dynamic: "entries"},
		{Path: []string{"use"}, Dynamic: "entries"},
		{Path: []string{"search"}, Values: []string{"--json"}, Dynamic: "entries"},
		{Path: []string{"search", "--json"}, Dynamic: "entries"},
		{Path: []string{"meta"}, Dynamic: "entries"},
	}
}

func runCompletionCommand(arguments []string, output io.Writer) error {
	if len(arguments) == 2 && (arguments[0] == "install" || arguments[0] == "remove") {
		return changeCompletion(arguments[0], arguments[1], output)
	}
	if len(arguments) != 1 {
		return fmt.Errorf("usage: al completion bash|zsh | al completion install|remove bash|zsh")
	}
	var program string
	switch arguments[0] {
	case "bash":
		program = renderBashCompletion()
	case "zsh":
		program = renderZshCompletion()
	default:
		return fmt.Errorf("unsupported shell %q (use bash or zsh)", arguments[0])
	}
	_, err := io.WriteString(output, program)
	return err
}

func changeCompletion(action, shell string, output io.Writer) error {
	preview, err := buildCompletionPlan(action, shell)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(output, workflowplan.RenderPlain(preview)); err != nil {
		return err
	}
	if len(preview.Actions) == 0 {
		_, err = fmt.Fprintln(output, "Nothing needed to change.")
		return err
	}
	path, err := completionInstallPath(shell)
	if err != nil {
		return err
	}
	if err := applyPrivatePlan(filepath.Dir(path), preview, func() (workflowplan.OperationPlan, error) {
		return buildCompletionPlan(action, shell)
	}); err != nil {
		return err
	}
	if action == "install" {
		adapter, _ := shellAdapter(shell)
		if completionIntegrationState(adapter) == "ready" {
			_, err = fmt.Fprintf(output, "%s suggestions are installed. Start a new %s shell to use them.\n", adapter.DisplayName(), shell)
		} else {
			_, err = fmt.Fprintf(output, "%s suggestions are saved. Enter al setup %s so new shells can load them.\n", adapter.DisplayName(), shell)
		}
	} else {
		adapter, _ := shellAdapter(shell)
		_, err = fmt.Fprintf(output, "%s suggestions are removed. Start a new %s shell to finish.\n", adapter.DisplayName(), shell)
	}
	return err
}

func completionIntegrationState(adapter ShellAdapter) string {
	path, err := aliasPathFor(adapter)
	if err != nil {
		return "missing"
	}
	contents, err := readRegularFile(path, completionSourceLimit)
	if err != nil {
		return "missing"
	}
	marker := "# Alias Lens " + adapter.DisplayName() + " integration"
	evalLine := `eval "$(command alias-lens shell-init ` + adapter.Name() + `)"`
	lines := strings.Split(string(contents), "\n")
	recognizableLines := 0
	healthyPairs := 0
	for index, line := range lines {
		switch {
		case line == marker:
			recognizableLines++
			if index+1 < len(lines) && lines[index+1] == evalLine {
				healthyPairs++
			}
		case line == evalLine:
			recognizableLines++
		case strings.Contains(line, marker), strings.Contains(line, "alias-lens shell-init "+adapter.Name()):
			recognizableLines++
		}
	}
	if healthyPairs == 1 && recognizableLines == 2 {
		return "ready"
	}
	if recognizableLines > 0 {
		return "edited"
	}
	return "missing"
}

func buildCompletionPlan(action, shell string) (workflowplan.OperationPlan, error) {
	if action != "install" && action != "remove" {
		return workflowplan.OperationPlan{}, fmt.Errorf("choose install or remove")
	}
	path, err := completionInstallPath(shell)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	adapter, _ := shellAdapter(shell)
	if completionIntegrationState(adapter) == "edited" {
		return workflowplan.OperationPlan{}, fmt.Errorf("the Alias Lens section in your %s alias file was edited; enter al setup --repair %s before changing suggestions", shell, shell)
	}
	current, readErr := readRegularFile(path, completionOutputLimit)
	kind := workflowplan.ActionReplace
	backup := true
	if errors.Is(readErr, os.ErrNotExist) {
		current = nil
		kind = workflowplan.ActionCreate
		backup = false
	} else if readErr != nil {
		return workflowplan.OperationPlan{}, readErr
	}
	planned := []byte{}
	reason := "remove shell suggestions without changing aliases"
	operation := "completion.remove"
	if action == "install" {
		operation = "completion.install"
		reason = "install shell suggestions generated from the Alias Lens command list"
		if shell == "bash" {
			planned = []byte(renderBashCompletion())
		} else {
			planned = []byte(renderZshCompletion())
		}
	}
	expectedManaged := []byte(renderBashCompletion())
	if shell == "zsh" {
		expectedManaged = []byte(renderZshCompletion())
	}
	if current != nil && !bytes.Equal(current, expectedManaged) {
		return workflowplan.OperationPlan{}, fmt.Errorf("the saved %s suggestions were changed outside Alias Lens; move or remove %s yourself, then try again", shell, displayPrivatePath(path))
	}
	inputs := []workflowplan.Input{}
	if current != nil {
		inputs = append(inputs, planInput("completion_file", path, current))
	}
	if action == "remove" && current == nil {
		return workflowplan.Build(operation, inputs, nil, nil, []workflowplan.Diagnostic{{Code: "no_change", Message: "Shell suggestions are already in the requested state."}}), nil
	}
	if action == "install" && bytes.Equal(current, planned) {
		return workflowplan.Build(operation, inputs, nil, nil, []workflowplan.Diagnostic{{Code: "no_change", Message: "Shell suggestions are already in the requested state."}}), nil
	}
	if action == "remove" {
		kind = workflowplan.ActionRemove
		backup = true
	}
	return workflowplan.Build(operation, inputs, []workflowplan.Action{{Sequence: 1, Kind: kind, TargetRole: "completion_file", DisplayPath: displayPrivatePath(path), Reason: reason, Risk: workflowplan.RiskLow, PlannedSHA256: hashBytes(planned), Backup: backup, Reversible: true, Target: plannedTarget(path, current, planned)}}, nil, nil), nil
}

func completionInstallPath(shell string) (string, error) {
	if _, err := shellAdapter(shell); err != nil {
		return "", err
	}
	path, err := configPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "completion."+shell), nil
}

func renderBashCompletion() string {
	var output strings.Builder
	output.WriteString(`# Alias Lens Bash completion. Generated by "al completion bash".
_alias_lens_complete() {
  local cur path candidate i
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]-}"
  path=""
  i=1
  while [ "$i" -lt "$COMP_CWORD" ]; do
    if [ -n "$path" ]; then path="$path|${COMP_WORDS[$i]}"; else path="${COMP_WORDS[$i]}"; fi
    i=$((i + 1))
  done
  case "$path" in
`)
	for _, rule := range completionRules() {
		fmt.Fprintf(&output, "    %s)\n", shellSingleQuote(strings.Join(rule.Path, "|")))
		if len(rule.Values) > 0 {
			fmt.Fprintf(&output, "      while IFS= read -r candidate; do COMPREPLY[${#COMPREPLY[@]}]=\"$candidate\"; done < <(compgen -W %s -- \"$cur\")\n", shellSingleQuote(strings.Join(rule.Values, " ")))
		}
		if rule.Dynamic != "" {
			fmt.Fprintf(&output, "      while IFS= read -r candidate; do [ -n \"$candidate\" ] && COMPREPLY[${#COMPREPLY[@]}]=\"$candidate\"; done < <(command alias-lens completion-candidates --shell bash --command %s --prefix \"$cur\" 2>/dev/null)\n", shellSingleQuote(rule.Dynamic))
		}
		output.WriteString("      ;;\n")
	}
	output.WriteString(`  esac
}
complete -F _alias_lens_complete al alias-lens
`)
	return output.String()
}

func renderZshCompletion() string {
	var output strings.Builder
	output.WriteString(`# Alias Lens Zsh completion. Generated by "al completion zsh".
_alias_lens_complete() {
  local cur path candidate
  local -a reply static_candidates
  integer i
  cur="${words[CURRENT]-}"
  path=""
  i=2
  while (( i < CURRENT )); do
    if [[ -n "$path" ]]; then path="$path|${words[i]}"; else path="${words[i]}"; fi
    (( i++ ))
  done
  case "$path" in
`)
	for _, rule := range completionRules() {
		fmt.Fprintf(&output, "    %s)\n", shellSingleQuote(strings.Join(rule.Path, "|")))
		if len(rule.Values) > 0 {
			output.WriteString("      static_candidates=(")
			for _, value := range rule.Values {
				output.WriteByte(' ')
				output.WriteString(shellSingleQuote(value))
			}
			output.WriteString(" )\n      for candidate in \"${static_candidates[@]}\"; do [[ \"$candidate\" == \"${cur}\"* ]] && reply+=(\"$candidate\"); done\n")
		}
		if rule.Dynamic != "" {
			fmt.Fprintf(&output, "      while IFS= read -r candidate; do [[ -n \"$candidate\" ]] && reply+=(\"$candidate\"); done < <(command alias-lens completion-candidates --shell zsh --command %s --prefix \"$cur\" 2>/dev/null)\n", shellSingleQuote(rule.Dynamic))
		}
		output.WriteString("      ;;\n")
	}
	output.WriteString(`  esac
  (( ${#reply[@]} )) && compadd -Q -- "${reply[@]}"
}
if ! (( $+functions[compdef] )); then
  autoload -Uz compinit
  compinit
fi
compdef _alias_lens_complete al alias-lens
`)
	return output.String()
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

// runCompletionCandidates is intentionally quiet. Completion must not copy a
// parse or permission error, and especially not private source text, into the
// user's prompt.
func runCompletionCandidates(arguments []string, output io.Writer) bool {
	if len(arguments) != 6 || arguments[0] != "--shell" || arguments[2] != "--command" || arguments[4] != "--prefix" {
		return false
	}
	adapter, err := shellAdapter(arguments[1])
	if err != nil || len(arguments[5]) > 256 || !utf8.ValidString(arguments[5]) {
		return false
	}
	var candidates []string
	switch arguments[3] {
	case "entries":
		candidates, err = completionEntryCandidates(adapter)
	case "profiles":
		candidates, err = completionProfileCandidates()
	default:
		return false
	}
	if err != nil {
		return false
	}
	prefix := arguments[5]
	filtered := candidates[:0]
	for _, candidate := range candidates {
		if strings.HasPrefix(candidate, prefix) {
			filtered = append(filtered, candidate)
		}
		if len(filtered) == completionMaxResults {
			break
		}
	}
	var buffer bytes.Buffer
	for _, candidate := range filtered {
		if !completionCandidateName.MatchString(candidate) || buffer.Len()+len(candidate)+1 > completionOutputLimit {
			return false
		}
		buffer.WriteString(candidate)
		buffer.WriteByte('\n')
	}
	_, err = output.Write(buffer.Bytes())
	return err == nil
}

func completionEntryCandidates(adapter ShellAdapter) ([]string, error) {
	active, err := completionCatalogActive(adapter.Name())
	if err != nil {
		return nil, err
	}
	if !active {
		aliasPath, err := aliasPathFor(adapter)
		if err != nil {
			return nil, err
		}
		contents, err := readCompletionSource(aliasPath)
		if err != nil {
			return nil, err
		}
		return completionLegacyEntries(contents, adapter)
	}
	value, err := readInstalledCatalogSnapshot(adapter.Name())
	if err != nil {
		return nil, err
	}
	return completionCatalogEntries(value, adapter.Name())
}

func completionCatalogActive(shell string) (bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, err
	}
	path := filepath.Join(home, ".local", "state", "alias-lens", "catalog-state.json")
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("catalog state is not a regular file")
	}
	contents, err := readObservedPrivateFile(path, completionSourceLimit)
	if err != nil {
		return false, err
	}
	var state struct {
		SchemaVersion   int                        `json:"schema_version"`
		InstalledShells map[string]json.RawMessage `json:"installed_shells"`
	}
	if err := json.Unmarshal(contents, &state); err != nil || state.SchemaVersion != 1 {
		return false, fmt.Errorf("catalog state is invalid")
	}
	_, active := state.InstalledShells[shell]
	return active, nil
}

func completionCatalogEntries(catalog neutralcatalog.Catalog, shell string) ([]string, error) {
	profiles, err := readCompletionProfiles()
	if err != nil {
		return nil, err
	}
	activeProfiles := make(map[string]bool, len(profiles))
	for _, profile := range profiles {
		activeProfiles[profile] = true
	}
	var names []string
	for _, entry := range catalog.Entries {
		if !completionCatalogEntryActive(entry, shell, activeProfiles) {
			continue
		}
		names = append(names, entry.Name)
	}
	return uniqueCompletionNames(names), nil
}

func completionCatalogEntryActive(entry neutralcatalog.Entry, shell string, profiles map[string]bool) bool {
	if entry.Portable == nil {
		if _, ok := entry.Native[shell]; !ok {
			return false
		}
	}
	if !platformSupported(entry.Platforms) {
		return false
	}
	if entry.When == nil {
		return true
	}
	if len(entry.When.Shells) > 0 && !containsString(entry.When.Shells, shell) {
		return false
	}
	if len(entry.When.ProfilesAny) > 0 {
		matched := false
		for _, profile := range entry.When.ProfilesAny {
			matched = matched || profiles[profile]
		}
		if !matched {
			return false
		}
	}
	for _, profile := range entry.When.ProfilesNone {
		if profiles[profile] {
			return false
		}
	}
	return true
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func completionLegacyEntries(contents []byte, adapter ShellAdapter) ([]string, error) {
	var names []string
	inFunction := false
	seen := map[string]bool{}
	for index, line := range strings.Split(string(contents), "\n") {
		trimmed := strings.TrimSpace(line)
		if inFunction {
			if trimmed == "}" || strings.HasPrefix(trimmed, "};") {
				inFunction = false
			}
			continue
		}
		if name, _, ok := adapter.ParseAliasDefinition(line); ok {
			if checkAliasValueSyntax(line, index+1) != nil || seen[name] {
				return nil, fmt.Errorf("alias file contains an invalid or duplicate alias")
			}
			seen[name] = true
			names = append(names, name)
			continue
		}
		if strings.HasPrefix(trimmed, "alias ") {
			return nil, fmt.Errorf("alias file contains a malformed alias")
		}
		if match := functionStart.FindStringSubmatch(line); match != nil {
			if seen[match[1]] {
				return nil, fmt.Errorf("alias file contains a duplicate entry")
			}
			seen[match[1]] = true
			names = append(names, match[1])
			remainder := strings.TrimSpace(match[2])
			inFunction = !strings.Contains(remainder, "}")
		}
	}
	if inFunction {
		return nil, fmt.Errorf("alias file contains an incomplete function")
	}
	return uniqueCompletionNames(names), nil
}

func uniqueCompletionNames(names []string) []string {
	sort.Strings(names)
	result := names[:0]
	for _, name := range names {
		if !completionCandidateName.MatchString(name) {
			continue
		}
		if len(result) == 0 || result[len(result)-1] != name {
			result = append(result, name)
		}
	}
	return result
}

func completionProfileCandidates() ([]string, error) {
	profiles, err := readCompletionProfiles()
	if err != nil {
		return nil, err
	}
	return uniqueCompletionNames(profiles), nil
}

func readCompletionProfiles() ([]string, error) {
	observed, err := observeConfig()
	if err != nil {
		return nil, err
	}
	return append([]string(nil), observed.Config.Profiles...), nil
}

func readCompletionSource(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > completionSourceLimit {
		return nil, fmt.Errorf("completion source is not a bounded regular file")
	}
	contents, err := io.ReadAll(io.LimitReader(file, completionSourceLimit+1))
	if err != nil || len(contents) > completionSourceLimit || !utf8.Valid(contents) {
		return nil, fmt.Errorf("completion source could not be read safely")
	}
	return contents, nil
}
