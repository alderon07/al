package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

var version = "dev"

//go:embed web/*
var web embed.FS

type Alias struct {
	Name        string   `json:"name"`
	Command     string   `json:"command"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Issues      []string `json:"issues,omitempty"`
	Usage       int      `json:"usage,omitempty"`
	Type        string   `json:"type,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Platforms   []string `json:"platforms,omitempty"`
	Favorite    bool     `json:"favorite,omitempty"`
}

func main() {
	if len(os.Args) == 1 {
		runTUI()
		return
	}
	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Printf("alias-lens %s\n", version)
	case "--web":
		runWeb()
	case "repo":
		if len(os.Args) == 2 {
			if err := runRepoPicker(""); err != nil {
				fmt.Fprintln(os.Stderr, "Alias Lens:", err)
			}
			return
		}
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "Usage: al repo [/path/to/dotfiles]")
			return
		}
		if os.Args[2] == "github" || os.Args[2] == "bitbucket" || os.Args[2] == "gitlab" {
			if err := runRepoPicker(os.Args[2]); err != nil {
				fmt.Fprintln(os.Stderr, "Alias Lens:", err)
			}
			return
		}
		if err := configureRepository(os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
			return
		}
		fmt.Println("Alias Lens will sync only .bash_aliases in", os.Args[2])
	case "config":
		if err := runConfigCommand(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	case "pick":
		commandOnly := false
		arguments := os.Args[2:]
		if len(arguments) > 0 && arguments[0] == "--command" {
			commandOnly = true
			arguments = arguments[1:]
		}
		if len(arguments) > 1 {
			fmt.Fprintln(os.Stderr, "Usage: al pick [--command] [QUERY]")
			return
		}
		query := ""
		if len(arguments) == 1 {
			query = arguments[0]
		}
		if err := runAliasPicker(query, commandOnly); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	case "shell-init":
		if len(os.Args) != 3 || os.Args[2] != "bash" {
			fmt.Fprintln(os.Stderr, "Usage: al shell-init bash")
			return
		}
		printBashIntegration()
	case "suggest":
		if err := runHistorySuggestions(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	case "scan":
		if len(os.Args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: al scan")
			return
		}
		if err := runSecretScan(); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	case "meta":
		if err := runMetadataCommand(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	case "history":
		if len(os.Args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: al history")
			return
		}
		if err := runRevisionHistory(); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	case "undo":
		if len(os.Args) > 3 {
			fmt.Fprintln(os.Stderr, "Usage: al undo [REVISION]")
			return
		}
		revision := "latest"
		if len(os.Args) == 3 {
			revision = os.Args[2]
		}
		if err := restoreRevision(revision); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	case "doctor":
		if len(os.Args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: al doctor")
			return
		}
		if err := runDoctor(); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	case "setup":
		if len(os.Args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: al setup")
			return
		}
		if err := runSetup(); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	case "autosync":
		if err := runAutoSyncCommand(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	case "watch":
		if len(os.Args) == 3 && os.Args[2] == "--ensure" {
			config, err := loadConfig()
			if err == nil && config.AutoSync.Enabled {
				err = ensureWatchProcess()
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, "Alias Lens:", err)
			}
			return
		}
		daemon := len(os.Args) == 3 && os.Args[2] == "--daemon"
		if len(os.Args) > 3 || (len(os.Args) == 3 && !daemon) {
			fmt.Fprintln(os.Stderr, "Usage: al watch")
			return
		}
		if err := runWatch(daemon); err != nil && !daemon {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	case "track", "untrack":
		if err := runTrackCommand(os.Args[2:], os.Args[1] == "untrack"); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	case "sync":
		if len(os.Args) > 3 || (len(os.Args) == 3 && os.Args[2] != "--push" && os.Args[2] != "--pull") {
			printUsage()
			return
		}
		if len(os.Args) == 3 && os.Args[2] == "--pull" {
			message, err := pullRepository()
			if err != nil {
				fmt.Fprintln(os.Stderr, "Alias Lens:", err)
				return
			}
			fmt.Println(message)
			return
		}
		push := len(os.Args) == 3 && os.Args[2] == "--push"
		message, err := syncRepository(push)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
			return
		}
		fmt.Println(message)
	case "diff":
		if len(os.Args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: al diff")
			return
		}
		if err := showRepositoryDiff(); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage: al [--web | --version | pick | repo | config | sync]")
	fmt.Println("  al               Open Alias Lens in the terminal")
	fmt.Println("  al --web         Start the optional browser interface")
	fmt.Println("  al --version     Print the installed Alias Lens version")
	fmt.Println("  al repo          Pick from writable GitHub, Bitbucket, and GitLab repositories")
	fmt.Println("  al repo PROVIDER Limit the picker to one configured provider")
	fmt.Println("  al repo PATH     Choose an existing local Git repository")
	fmt.Println("  al config        Show safe provider configuration and setup commands")
	fmt.Println("  al pick          Print an alias selected in the terminal picker")
	fmt.Println("  al use           Run a selected alias after loading shell integration")
	fmt.Println("  al shell-init bash  Print Bash integration")
	fmt.Println("  al suggest       Suggest aliases from local Bash history")
	fmt.Println("  al scan          Check aliases for likely secrets")
	fmt.Println("  al meta          Add tags, platforms, and favorite metadata")
	fmt.Println("  al history       List recoverable alias revisions")
	fmt.Println("  al undo          Restore the latest or a selected revision")
	fmt.Println("  al doctor        Check shell, Git, providers, SSH, and sync setup")
	fmt.Println("  al setup         Install Bash integration and the Ctrl+G binding")
	fmt.Println("  al autosync      Enable, disable, or inspect automatic sync")
	fmt.Println("  al watch         Run one pull/reconcile/push cycle")
	fmt.Println("  al track         Add an explicit config file to automatic sync")
	fmt.Println("  al untrack       Remove a config file from automatic sync")
	fmt.Println("  al sync          Commit only .bash_aliases to that repository")
	fmt.Println("  al sync --push   Commit and explicitly push it")
	fmt.Println("  al sync --pull   Pull and import non-conflicting remote aliases")
	fmt.Println("  al diff          Compare local and tracked aliases by name")
}

func printBashIntegration() {
	_, _ = os.Stdout.WriteString(`# Alias Lens shell integration
unalias al 2>/dev/null || true
al() {
  if [ "$#" -eq 0 ]; then
    local _alias_lens_name _alias_lens_line _alias_lens_prompt
    _alias_lens_name="$(command alias-lens)" || return
    [ -z "$_alias_lens_name" ] && return
    if [[ $- != *i* ]]; then
      printf '%s\n' "$_alias_lens_name"
      return
    fi
    _alias_lens_prompt="${PS1-}"
    IFS= read -e -r -i "$_alias_lens_name" -p "${_alias_lens_prompt@P}" _alias_lens_line || return
    [ -n "$_alias_lens_line" ] && builtin eval -- "$_alias_lens_line"
    return
  fi
  if [ "${1-}" = "use" ]; then
    shift
    local _alias_lens_command
    _alias_lens_command="$(command alias-lens pick --command "$@")" || return
    [ -n "$_alias_lens_command" ] && builtin eval "$_alias_lens_command"
    return
  fi
  command alias-lens "$@"
}
_alias_lens_insert() {
  local _alias_lens_name
  _alias_lens_name="$(command alias-lens pick)" || return
  [ -z "$_alias_lens_name" ] && return
  READLINE_LINE="${READLINE_LINE:0:READLINE_POINT}${_alias_lens_name}${READLINE_LINE:READLINE_POINT}"
  READLINE_POINT=$((READLINE_POINT + ${#_alias_lens_name}))
}
bind -x '"\C-g":_alias_lens_insert'
command alias-lens watch --ensure >/dev/null 2>&1
`)
}

func runWeb() {
	if len(os.Args) > 2 {
		printUsage()
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/aliases", aliasesHandler)

	static, err := fs.Sub(web, "web")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServer(http.FS(static)))

	address := "127.0.0.1:8787"
	fmt.Printf("Alias Lens web mode is running at http://%s\n", address)
	fmt.Println("Reading aliases from ~/.bash_aliases")
	if err := http.ListenAndServe(address, mux); err != nil {
		panic(err)
	}
}

func aliasesHandler(w http.ResponseWriter, r *http.Request) {
	aliases, err := loadAliases()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(aliases)
}

func loadAliases() ([]Alias, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("find home directory: %w", err)
	}
	path := filepath.Join(home, ".bash_aliases")
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if createErr := ensureAliasFileExists(path); createErr != nil {
			return nil, createErr
		}
		contents, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var aliases []Alias
	var notes []string
	metadata := EntryMetadata{}
	for _, line := range strings.Split(string(contents), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			if parsed, ok := parseMetadataComment(line); ok {
				metadata = parsed
				continue
			}
			note := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
			if strings.Contains(note, "===") || strings.Contains(note, "---") || isSectionHeading(note) {
				notes = nil
				continue
			}
			if note != "" {
				notes = append(notes, note)
			}
			continue
		}

		name, command, ok := parseAliasDefinition(line)
		if !ok {
			notes = nil
			metadata = EntryMetadata{}
			continue
		}
		description := describe(name, command)
		if len(notes) > 0 {
			description = notes[len(notes)-1]
		}
		alias := Alias{
			Name:        name,
			Command:     command,
			Description: description,
			Category:    category(command),
			Type:        "alias",
		}
		applyMetadata(&alias, metadata)
		aliases = append(aliases, alias)
		notes = nil
		metadata = EntryMetadata{}
	}
	aliases = append(aliases, parseFunctions(string(contents))...)

	sort.Slice(aliases, func(i, j int) bool { return strings.ToLower(aliases[i].Name) < strings.ToLower(aliases[j].Name) })
	annotateUsage(aliases, loadHistoryCounts())
	annotateHealth(aliases)
	return aliases, nil
}

func annotateHealth(aliases []Alias) {
	counts := make(map[string]int, len(aliases))
	for _, alias := range aliases {
		counts[alias.Name]++
	}
	for index := range aliases {
		alias := &aliases[index]
		if counts[alias.Name] > 1 {
			alias.Issues = append(alias.Issues, "duplicate definition")
		}
		if isDangerousCommand(alias.Command) {
			alias.Issues = append(alias.Issues, "review before running")
		}
		if !platformSupported(alias.Platforms) {
			alias.Issues = append(alias.Issues, "not for "+currentPlatform())
		}
		fields := strings.Fields(alias.Command)
		if len(fields) > 0 && shouldCheckExecutable(fields[0]) {
			if _, err := exec.LookPath(fields[0]); err != nil {
				alias.Issues = append(alias.Issues, "missing executable: "+fields[0])
			}
		}
	}
}

func isDangerousCommand(command string) bool {
	lower := strings.ToLower(command)
	patterns := []string{"--force", "reset --hard", "clean -fd", "rm -rf", "chmod -r", "chown -r"}
	for _, pattern := range patterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return strings.Contains(command, "branch -D")
}

func shouldCheckExecutable(command string) bool {
	builtins := map[string]bool{
		".": true, "alias": true, "cd": true, "command": true, "echo": true, "export": true,
		"for": true, "if": true, "local": true, "printf": true, "pwd": true, "read": true,
		"return": true, "source": true, "test": true, "type": true,
	}
	return !builtins[command] && !strings.ContainsAny(command, "$()`")
}

func parseAliasDefinition(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "alias ") {
		return "", "", false
	}
	definition := strings.TrimSpace(strings.TrimPrefix(trimmed, "alias "))
	name, value, found := strings.Cut(definition, "=")
	name = strings.TrimSpace(name)
	value = strings.TrimSpace(value)
	if !found || name == "" || value == "" {
		return "", "", false
	}
	if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
		value = strings.TrimSuffix(strings.TrimPrefix(value, "'"), "'")
		value = strings.ReplaceAll(value, "'\\''", "'")
	} else if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
		if decoded, err := strconv.Unquote(value); err == nil {
			value = decoded
		} else {
			value = strings.TrimSuffix(strings.TrimPrefix(value, "\""), "\"")
		}
	}
	return name, value, true
}

func isSectionHeading(note string) bool {
	return len(note) <= 30 && note == strings.ToUpper(note)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func category(command string) string {
	first := strings.Fields(command)
	if len(first) == 0 {
		return "custom"
	}
	switch first[0] {
	case "git":
		return "git"
	case "docker":
		return "docker"
	case "eza", "ls":
		return "files"
	case "npm", "pnpm", "bun", "go":
		return "dev"
	default:
		return "custom"
	}
}

func describe(name, command string) string {
	fields := strings.Fields(command)
	if len(fields) >= 2 && fields[0] == "git" {
		return "Runs git " + strings.Join(fields[1:], " ")
	}
	if len(fields) >= 2 && fields[0] == "docker" {
		return "Runs docker " + strings.Join(fields[1:], " ")
	}
	if len(fields) > 0 {
		return "Runs " + fields[0]
	}
	return "Custom command for " + name
}
