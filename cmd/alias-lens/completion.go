package main

import (
	"github.com/alderon07/al/internal/presentation"

	"fmt"
	"io"

	"strings"

	workflowplan "github.com/alderon07/al/internal/plan"
)

type completionRule struct {
	Path    []string
	Values  []string
	Dynamic string
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
		{Path: []string{"config"}, Values: []string{"shell", "provider", "protocol", "disable", "profile", "footer-message", "footer-icon", "footer-reset"}},
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
		{Path: []string{"plan"}, Values: []string{"--json", "config", "completion", "catalog", "init"}},
		{Path: []string{"init"}, Values: []string{"--shell", "--startup-path", "--catalog-path", "--apply"}},
		{Path: []string{"init", "--shell"}, Values: shells},
		{Path: []string{"plan", "init"}, Values: []string{"--shell", "--startup-path", "--catalog-path"}},
		{Path: []string{"plan", "init", "--shell"}, Values: shells},
		{Path: []string{"catalog", "enable"}, Values: []string{"--shell", "--startup-path", "--apply"}},
		{Path: []string{"catalog", "enable", "--shell"}, Values: shells},
		{Path: []string{"catalog", "plan"}, Values: []string{"--shell", "--startup-path", "--json"}},
		{Path: []string{"catalog", "plan", "--shell"}, Values: shells},
		{Path: []string{"catalog", "review"}, Values: []string{"--shell", "--startup-path"}},
		{Path: []string{"catalog", "review", "--shell"}, Values: shells},
		{Path: []string{"catalog", "approve"}, Values: []string{"--shell", "--startup-path"}},
		{Path: []string{"catalog", "approve", "--shell"}, Values: shells},
		{Path: []string{"catalog", "adopt"}, Values: []string{"--shell", "--startup-path"}},
		{Path: []string{"catalog", "adopt", "--shell"}, Values: shells},
		{Path: []string{"catalog", "rollback"}, Values: []string{"--shell", "--apply"}},
		{Path: []string{"catalog", "rollback", "--shell"}, Values: shells},
		{Path: []string{"plan", "catalog", "enable"}, Values: []string{"--shell", "--startup-path"}},
		{Path: []string{"plan", "catalog", "enable", "--shell"}, Values: shells},
		{Path: []string{"plan", "catalog", "rollback"}, Values: []string{"--shell", "--startup-path"}},
		{Path: []string{"plan", "catalog", "rollback", "--shell"}, Values: shells},
		{Path: []string{"plan", "catalog"}, Values: []string{"enable", "rollback"}},
		{Path: []string{"plan", "config"}, Values: []string{"profile"}},
		{Path: []string{"plan", "config", "profile"}, Values: []string{"add", "remove"}},
		{Path: []string{"plan", "config", "profile", "remove"}, Dynamic: "profiles"},
		{Path: []string{"plan", "completion"}, Values: []string{"install", "remove"}},
		{Path: []string{"plan", "completion", "install"}, Values: shells},
		{Path: []string{"plan", "completion", "remove"}, Values: shells},
		{Path: []string{"catalog"}, Values: []string{"preview", "import", "shadow", "diff", "enable", "plan", "review", "approve", "adopt", "rollback", "recover"}},
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
		{Path: []string{"diff"}, Values: []string{"--tui"}},
		{Path: []string{"catalog", "diff", "--from"}, Values: []string{"repository", "installed"}},
		{Path: []string{"catalog", "diff", "--shell"}, Values: shells},
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
		{Path: []string{"sync"}, Values: []string{"--push", "--pull", "--catalog", "--apply", "--resolve"}},
		{Path: []string{"autosync"}, Values: []string{"enable", "disable", "status"}},
		{Path: []string{"shortcuts"}, Values: []string{"auto", "windows", "linux", "macos", "test", "set", "reset", "reset-all"}},
		{Path: []string{"shortcuts", "set"}, Values: shortcutCompletionActions()},
		{Path: []string{"shortcuts", "reset"}, Values: shortcutCompletionActions()},
		{Path: []string{"theme"}, Values: append([]string{"--check"}, presentation.ThemeNames()...)},
		{Path: []string{"pick"}, Values: []string{"--command"}, Dynamic: "entries"},
		{Path: []string{"pick", "--command"}, Dynamic: "entries"},
		{Path: []string{"use"}, Dynamic: "entries"},
		{Path: []string{"search"}, Values: []string{"--json", "--global"}, Dynamic: "entries"},
		{Path: []string{"search", "--json"}, Values: []string{"--global"}, Dynamic: "entries"},
		{Path: []string{"search", "--global"}, Values: []string{"--json"}, Dynamic: "entries"},
		{Path: []string{"context"}, Values: []string{"list", "add", "remove"}},
		{Path: []string{"context", "add"}, Dynamic: "entries"},
		{Path: []string{"context", "remove"}, Dynamic: "entries"},
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
	if err := applicationServices().ApplyCompletion(preview, action, shell, completionScript(shell)); err != nil {
		return err
	}
	if action == "install" {
		adapter, _ := applicationServices().ShellAdapter(shell)
		if applicationServices().CompletionIntegrationState(shell) == "ready" {
			_, err = fmt.Fprintf(output, "%s suggestions are installed. Start a new %s shell to use them.\n", adapter.DisplayName(), shell)
		} else {
			_, err = fmt.Fprintf(output, "%s suggestions are saved. Enter al setup %s so new shells can load them.\n", adapter.DisplayName(), shell)
		}
	} else {
		adapter, _ := applicationServices().ShellAdapter(shell)
		_, err = fmt.Fprintf(output, "%s suggestions are removed. Start a new %s shell to finish.\n", adapter.DisplayName(), shell)
	}
	return err
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
	contents, err := applicationServices().CompletionCandidates(arguments[1], arguments[3], arguments[5])
	if err != nil {
		return false
	}
	_, err = output.Write(contents)
	return err == nil
}
func completionScript(shell string) []byte {
	if shell == "bash" {
		return []byte(renderBashCompletion())
	}
	return []byte(renderZshCompletion())
}
func buildCompletionPlan(action, shell string) (workflowplan.OperationPlan, error) {
	return applicationServices().PreviewCompletion(action, shell, completionScript(shell))
}
