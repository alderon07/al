package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	activeShellEnvironment = "ALIAS_LENS_SHELL"
	historyFileEnvironment = "ALIAS_LENS_HISTORY_FILE"
)

type ShellAdapter interface {
	Name() string
	DisplayName() string
	AliasFilename() string
	HistoryFilename() string
	ValidateEntryName(name, kind string) error
	ParseAliasDefinition(line string) (string, string, bool)
	ParseFunctions(contents string) []Alias
	RenderEntryDefinition(alias Alias) (string, error)
	HistoryCommand(line string) string
	HistoryUsageLine(line string, state *shellHistoryState) (string, time.Time, bool)
	Integration() string
	ExecutionSpec() shellExecutionSpec
	PromptSpec() shellPromptSpec
	BindingSpec() shellBindingSpec
	CheckSyntax(path string) *aliasCheckFinding
	StartupPaths(home, platform string) ([]string, error)
	ConfigureStartup(home, platform, executableDir string) error
	RemoveStartup(home, platform string) error
	StartupStatus(home, platform string) (bool, string)
}

type shellHistoryState struct {
	BashTime time.Time
}

type shellExecutionSpec struct {
	DefinitionCommand string
	ExecuteExpression string
	HistoryCommand    string
}

type shellPromptSpec struct {
	SelectionPrefix      string
	NonEmptyPromptAction string
}

type shellBindingSpec struct {
	Key                string
	DisableEnvironment string
}

func parseHistoryUnix(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}

type bashShellAdapter struct{}
type zshShellAdapter struct{}

func shellAdapter(name string) (ShellAdapter, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "bash":
		return bashShellAdapter{}, nil
	case "zsh":
		return zshShellAdapter{}, nil
	default:
		return nil, fmt.Errorf("unsupported shell %q (use bash or zsh)", name)
	}
}

func activeShellAdapter() ShellAdapter {
	if adapter, err := shellAdapter(os.Getenv(activeShellEnvironment)); err == nil {
		return adapter
	}
	if config, err := loadConfig(); err == nil {
		if adapter, adapterErr := shellAdapter(config.Shell); adapterErr == nil {
			return adapter
		}
	}
	return bashShellAdapter{}
}

func requestedShellAdapter(name string) (ShellAdapter, error) {
	if name != "" {
		return shellAdapter(name)
	}
	if configured := os.Getenv(activeShellEnvironment); configured != "" {
		return shellAdapter(configured)
	}
	if detected := filepath.Base(strings.TrimSpace(os.Getenv("SHELL"))); detected != "." && detected != "" {
		return shellAdapter(detected)
	}
	return activeShellAdapter(), nil
}

func printShellEntry(name string) error {
	definition, err := loadShellEntry(name)
	if err != nil {
		return err
	}
	fmt.Println(definition)
	return nil
}

func loadShellEntry(name string) (string, error) {
	alias, err := loadAliasEntry(name)
	if err != nil {
		return "", err
	}
	return activeShellAdapter().RenderEntryDefinition(alias)
}

func loadAliasEntry(name string) (Alias, error) {
	if !aliasName.MatchString(name) {
		return Alias{}, fmt.Errorf("invalid alias name %q", name)
	}
	aliases, err := loadAliases()
	if err != nil {
		return Alias{}, err
	}
	for _, alias := range aliases {
		if alias.Name == name {
			return alias, nil
		}
	}
	return Alias{}, fmt.Errorf("alias %q is no longer in %s", name, aliasDisplayPath())
}

func shellEntryDefinition(alias Alias) (string, error) {
	return activeShellAdapter().RenderEntryDefinition(alias)
}

func aliasPathFor(adapter ShellAdapter) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, adapter.AliasFilename()), nil
}

func aliasDisplayPath() string {
	return "~/" + activeShellAdapter().AliasFilename()
}

func historyPathFor(adapter ShellAdapter, home string) string {
	if configured := strings.TrimSpace(os.Getenv(historyFileEnvironment)); configured != "" {
		return configured
	}
	return filepath.Join(home, adapter.HistoryFilename())
}

func (bashShellAdapter) Name() string            { return "bash" }
func (bashShellAdapter) DisplayName() string     { return "Bash" }
func (bashShellAdapter) AliasFilename() string   { return ".bash_aliases" }
func (bashShellAdapter) HistoryFilename() string { return ".bash_history" }
func (bashShellAdapter) Integration() string     { return bashIntegration }

func (bashShellAdapter) ValidateEntryName(name, kind string) error {
	return validateLegacyEntryName(name, kind)
}

func (bashShellAdapter) ParseAliasDefinition(line string) (string, string, bool) {
	return parseLegacyAliasDefinition(line)
}

func (bashShellAdapter) ParseFunctions(contents string) []Alias {
	return parseLegacyFunctions(contents)
}

func (adapter bashShellAdapter) RenderEntryDefinition(alias Alias) (string, error) {
	return renderLegacyEntryDefinition(adapter, alias)
}

func (bashShellAdapter) HistoryCommand(line string) string { return line }

func (bashShellAdapter) HistoryUsageLine(line string, state *shellHistoryState) (string, time.Time, bool) {
	if strings.HasPrefix(line, "#") {
		if unix, err := parseHistoryUnix(strings.TrimPrefix(line, "#")); err == nil {
			state.BashTime = time.Unix(unix, 0)
			return "", time.Time{}, true
		}
		return line, time.Time{}, false
	}
	eventTime := state.BashTime
	state.BashTime = time.Time{}
	return line, eventTime, false
}

func (bashShellAdapter) ExecutionSpec() shellExecutionSpec {
	return shellExecutionSpec{DefinitionCommand: `alias-lens shell-entry "$name"`, ExecuteExpression: `builtin eval "$name"`, HistoryCommand: `builtin history -s "$name"`}
}

func (bashShellAdapter) PromptSpec() shellPromptSpec {
	return shellPromptSpec{SelectionPrefix: "$ ", NonEmptyPromptAction: "clear-readline-buffer"}
}

func (bashShellAdapter) BindingSpec() shellBindingSpec {
	return shellBindingSpec{Key: "Ctrl+G", DisableEnvironment: "ALIAS_LENS_NOBIND"}
}

func (bashShellAdapter) StartupPaths(home, platform string) ([]string, error) {
	paths := []string{filepath.Join(home, ".bashrc")}
	if !bashLoginStartupSupported(platform) {
		return paths, nil
	}
	loginPath, err := bashLoginPath(home)
	if err != nil {
		return nil, err
	}
	return append(paths, loginPath), nil
}

func (adapter bashShellAdapter) ConfigureStartup(home, platform, executableDir string) error {
	if err := configureStartupFile(filepath.Join(home, ".bashrc"), ".bash_aliases", bashAliasLoader, home, executableDir); err != nil {
		return err
	}
	if !bashLoginStartupSupported(platform) {
		return nil
	}
	paths, err := adapter.StartupPaths(home, platform)
	if err != nil {
		return err
	}
	loginPath := paths[1]
	contents, err := os.ReadFile(loginPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	updated := []byte(strings.ReplaceAll(string(contents), bashLoginLoader, ""))
	if !hasActiveShellReference(updated, ".bashrc") && !hasActiveShellReference(updated, ".bash_aliases") {
		updated = append(updated, []byte(bashLoginLoader)...)
	}
	if string(updated) == string(contents) {
		return nil
	}
	return writeStartupFile(loginPath, contents, updated)
}

func (adapter bashShellAdapter) RemoveStartup(home, platform string) error {
	if err := removeStartupFileBlocks(filepath.Join(home, ".bashrc"), bashAliasLoader); err != nil {
		return err
	}
	if !bashLoginStartupSupported(platform) {
		return nil
	}
	for _, name := range []string{".bash_profile", ".bash_login", ".profile"} {
		if err := removeStartupFileBlocks(filepath.Join(home, name), bashLoginLoader); err != nil {
			return err
		}
	}
	return nil
}

func (adapter bashShellAdapter) StartupStatus(home, platform string) (bool, string) {
	bashrc, _ := os.ReadFile(filepath.Join(home, ".bashrc"))
	if !hasActiveShellReference(bashrc, ".bash_aliases") {
		return false, ".bashrc does not load .bash_aliases; run al setup bash"
	}
	if !bashLoginStartupSupported(platform) {
		return true, ".bashrc loads .bash_aliases"
	}
	paths, err := adapter.StartupPaths(home, platform)
	if err != nil {
		return false, err.Error()
	}
	loginPath := paths[1]
	login, _ := os.ReadFile(loginPath)
	if !hasActiveShellReference(login, ".bashrc") && !hasActiveShellReference(login, ".bash_aliases") {
		return false, filepath.Base(loginPath) + " does not load Bash aliases; run al setup bash"
	}
	return true, ".bashrc and " + filepath.Base(loginPath) + " load aliases"
}

func (zshShellAdapter) Name() string            { return "zsh" }
func (zshShellAdapter) DisplayName() string     { return "Zsh" }
func (zshShellAdapter) AliasFilename() string   { return ".zsh_aliases" }
func (zshShellAdapter) HistoryFilename() string { return ".zsh_history" }
func (zshShellAdapter) Integration() string     { return zshIntegration }

func (zshShellAdapter) ValidateEntryName(name, kind string) error {
	return validateLegacyEntryName(name, kind)
}

func (zshShellAdapter) ParseAliasDefinition(line string) (string, string, bool) {
	return parseLegacyAliasDefinition(line)
}

func (zshShellAdapter) ParseFunctions(contents string) []Alias {
	return parseLegacyFunctions(contents)
}

func (adapter zshShellAdapter) RenderEntryDefinition(alias Alias) (string, error) {
	return renderLegacyEntryDefinition(adapter, alias)
}

func (zshShellAdapter) HistoryCommand(line string) string {
	if strings.HasPrefix(line, ": ") {
		if separator := strings.IndexByte(line, ';'); separator >= 0 {
			return line[separator+1:]
		}
	}
	return line
}

func (zshShellAdapter) HistoryUsageLine(line string, _ *shellHistoryState) (string, time.Time, bool) {
	eventTime := time.Time{}
	if strings.HasPrefix(line, ": ") {
		if separator := strings.IndexByte(line, ';'); separator >= 0 {
			metadata := strings.TrimPrefix(line[:separator], ": ")
			stamp, _, _ := strings.Cut(metadata, ":")
			if unix, err := parseHistoryUnix(stamp); err == nil {
				eventTime = time.Unix(unix, 0)
			}
			line = line[separator+1:]
		}
	}
	return line, eventTime, false
}

func (zshShellAdapter) ExecutionSpec() shellExecutionSpec {
	return shellExecutionSpec{DefinitionCommand: `alias-lens shell-entry "$name"`, ExecuteExpression: `builtin eval "$name"`, HistoryCommand: `print -s -- "$name"`}
}

func (zshShellAdapter) PromptSpec() shellPromptSpec {
	return shellPromptSpec{SelectionPrefix: "$ ", NonEmptyPromptAction: "zle-send-break"}
}

func (zshShellAdapter) BindingSpec() shellBindingSpec {
	return shellBindingSpec{Key: "Ctrl+G", DisableEnvironment: "ALIAS_LENS_NOBIND"}
}

func (zshShellAdapter) StartupPaths(home, _ string) ([]string, error) {
	path, err := zshStartupPath(home)
	if err != nil {
		return nil, err
	}
	return []string{path}, nil
}

func validateLegacyEntryName(name, kind string) error {
	if !aliasName.MatchString(name) {
		return fmt.Errorf("invalid alias name %q", name)
	}
	if kind == "function" && !functionName.MatchString(name) {
		return fmt.Errorf("invalid function name %q", name)
	}
	return nil
}

func renderLegacyEntryDefinition(adapter ShellAdapter, alias Alias) (string, error) {
	if err := adapter.ValidateEntryName(alias.Name, alias.Type); err != nil {
		return "", err
	}
	if alias.Type == "function" {
		return fmt.Sprintf("%s() {\n%s\n}", alias.Name, alias.Command), nil
	}
	return "alias " + alias.Name + "=" + shellQuote(alias.Command), nil
}

var _ ShellAdapter = bashShellAdapter{}
var _ ShellAdapter = zshShellAdapter{}

func (adapter zshShellAdapter) ConfigureStartup(home, platform, executableDir string) error {
	paths, err := adapter.StartupPaths(home, platform)
	if err != nil {
		return err
	}
	return configureStartupFile(paths[0], ".zsh_aliases", zshAliasLoader, home, executableDir)
}

func (adapter zshShellAdapter) RemoveStartup(home, platform string) error {
	paths, err := adapter.StartupPaths(home, platform)
	if err != nil {
		return err
	}
	return removeStartupFileBlocks(paths[0], zshAliasLoader)
}

func (adapter zshShellAdapter) StartupStatus(home, platform string) (bool, string) {
	paths, err := adapter.StartupPaths(home, platform)
	if err != nil {
		return false, err.Error()
	}
	path := paths[0]
	zshrc, _ := os.ReadFile(path)
	if !hasActiveShellReference(zshrc, ".zsh_aliases") {
		return false, path + " does not load .zsh_aliases; run al setup zsh"
	}
	return true, path + " loads .zsh_aliases"
}

func zshStartupPath(home string) (string, error) {
	directory := strings.TrimSpace(os.Getenv("ZDOTDIR"))
	if directory == "" {
		directory = home
	} else if directory == "~" || strings.HasPrefix(directory, "~/") {
		directory = filepath.Join(home, strings.TrimPrefix(directory, "~/"))
	} else if !filepath.IsAbs(directory) {
		absolute, err := filepath.Abs(directory)
		if err != nil {
			return "", err
		}
		directory = absolute
	}
	return filepath.Join(directory, ".zshrc"), nil
}

func bashLoginPath(home string) (string, error) {
	for _, name := range []string{".bash_profile", ".bash_login", ".profile"} {
		candidate := filepath.Join(home, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
	}
	return filepath.Join(home, ".bash_profile"), nil
}

func bashLoginStartupSupported(platform string) bool {
	return platform == "darwin" || platform == "linux"
}

func ensureStartupFileLoads(path, aliasFilename, block string) error {
	return configureStartupFile(path, aliasFilename, block, "", "")
}

func configureStartupFile(path, aliasFilename, aliasBlock, home, executableDir string) error {
	contents, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	updated := removeStartupPathBlocks(contents)
	updated = []byte(strings.ReplaceAll(string(updated), aliasBlock, ""))
	if executableDir != "" && !startupPathReady(updated, aliasFilename, home, executableDir) {
		updated = append([]byte(startupPathBlock(home, executableDir)), updated...)
	}
	if !hasActiveShellReference(updated, aliasFilename) {
		updated = append(updated, []byte(aliasBlock)...)
	}
	if string(updated) == string(contents) {
		return nil
	}
	return writeStartupFile(path, contents, updated)
}

func removeStartupFileBlocks(path string, blocks ...string) error {
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	updated := removeStartupPathBlocks(contents)
	for _, block := range blocks {
		updated = []byte(strings.ReplaceAll(string(updated), block, ""))
	}
	updated = []byte(strings.TrimLeft(string(updated), "\n"))
	if string(updated) == string(contents) {
		return nil
	}
	return writeStartupFile(path, contents, updated)
}

func removeStartupPathBlocks(contents []byte) []byte {
	text := string(contents)
	const startMarker = "# Keep the Alias Lens executable available in new shells.\n"
	const endMarker = "unset _alias_lens_bin_dir\n"
	for {
		start := strings.Index(text, startMarker)
		if start < 0 {
			break
		}
		if start > 0 && text[start-1] == '\n' {
			start--
		}
		endOffset := strings.Index(text[start:], endMarker)
		if endOffset < 0 {
			break
		}
		end := start + endOffset + len(endMarker)
		text = text[:start] + text[end:]
	}
	return []byte(text)
}

func startupPathReady(contents []byte, aliasFilename, home, directory string) bool {
	candidates := []string{filepath.Clean(directory)}
	if relative, err := filepath.Rel(home, directory); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		relative = filepath.ToSlash(relative)
		candidates = append(candidates, "$HOME/"+relative, "${HOME}/"+relative, "~/"+relative, `"$HOME"/`+shellQuote(relative))
	}
	pathSeen := false
	for _, line := range strings.Split(string(contents), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		for _, candidate := range candidates {
			if strings.Contains(trimmed, candidate) {
				pathSeen = true
				break
			}
		}
		if strings.Contains(trimmed, aliasFilename) {
			return pathSeen
		}
	}
	return pathSeen
}

func startupPathBlock(home, directory string) string {
	value := shellQuote(filepath.Clean(directory))
	if relative, err := filepath.Rel(home, directory); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		value = `"$HOME"/` + shellQuote(filepath.ToSlash(relative))
	}
	return `
# Keep the Alias Lens executable available in new shells.
_alias_lens_bin_dir=` + value + `
case ":$PATH:" in
  *":$_alias_lens_bin_dir:"*) ;;
  *) export PATH="$_alias_lens_bin_dir:$PATH" ;;
esac
unset _alias_lens_bin_dir
`
}

func writeStartupFile(path string, contents, updated []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}
	if len(contents) > 0 {
		if err := os.WriteFile(path+".alias-lens.bak", contents, 0o600); err != nil {
			return err
		}
	}
	return os.WriteFile(path, updated, mode)
}

func hasActiveShellReference(contents []byte, filename string) bool {
	for _, line := range strings.Split(string(contents), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && strings.Contains(trimmed, filename) {
			return true
		}
	}
	return false
}

const bashAliasLoader = `
# >>> Alias Lens Bash alias loader >>>
if [ -f "$HOME/.bash_aliases" ]; then
  . "$HOME/.bash_aliases"
fi
# <<< Alias Lens Bash alias loader <<<
`

const bashLoginLoader = `
# >>> Alias Lens Bash login loader >>>
if [ -n "${BASH_VERSION:-}" ] && [ -f "$HOME/.bashrc" ]; then
  . "$HOME/.bashrc"
fi
# <<< Alias Lens Bash login loader <<<
`

const zshAliasLoader = `
# >>> Alias Lens Zsh alias loader >>>
if [[ -f "$HOME/.zsh_aliases" ]]; then
  source "$HOME/.zsh_aliases"
fi
# <<< Alias Lens Zsh alias loader <<<
`

const bashIntegration = `# Alias Lens Bash integration
unalias al 2>/dev/null || true
# Bash records history timestamps only while HISTTIMEFORMAT is set. An empty
# value keeps the normal history display while preserving dates for stats.
if [ -z "${HISTTIMEFORMAT+x}" ]; then
  HISTTIMEFORMAT=
fi
_alias_lens_flush_history() {
  builtin history -a
}
_alias_lens_execute() {
  local _alias_lens_name="$1" _alias_lens_definition _alias_lens_status
  _alias_lens_definition="$(command env ALIAS_LENS_SHELL=bash alias-lens shell-entry "$_alias_lens_name")" || return
  builtin eval "$_alias_lens_definition" || return
  command env ALIAS_LENS_SHELL=bash alias-lens record-use "$_alias_lens_name" >/dev/null 2>&1
  builtin history -s "$_alias_lens_name"
  builtin history -a
  builtin eval "$_alias_lens_name"
  _alias_lens_status=$?
  return "$_alias_lens_status"
}
al() {
  _alias_lens_flush_history 2>/dev/null || true
  if [ "$#" -eq 0 ]; then
    local _alias_lens_name
    _alias_lens_name="$(command env ALIAS_LENS_SHELL=bash ALIAS_LENS_HISTORY_FILE="${HISTFILE:-$HOME/.bash_history}" alias-lens)" || return
    [ -z "$_alias_lens_name" ] && return
    _alias_lens_execute "$_alias_lens_name"
    return $?
  fi
  if [ "${1-}" = "use" ]; then
    shift
    if [ "${1-}" = "--help" ] || [ "${1-}" = "-h" ]; then
      command env ALIAS_LENS_SHELL=bash ALIAS_LENS_HISTORY_FILE="${HISTFILE:-$HOME/.bash_history}" alias-lens help use
      return
    fi
    local _alias_lens_name
    _alias_lens_name="$(command env ALIAS_LENS_SHELL=bash ALIAS_LENS_HISTORY_FILE="${HISTFILE:-$HOME/.bash_history}" alias-lens pick --execute "$@")" || return
    [ -z "$_alias_lens_name" ] && return
    _alias_lens_execute "$_alias_lens_name"
    return $?
  fi
  command env ALIAS_LENS_SHELL=bash ALIAS_LENS_HISTORY_FILE="${HISTFILE:-$HOME/.bash_history}" alias-lens "$@"
}
_alias_lens_prepare_readline() {
  bind '"\C-x\C-a":abort'
  if [ -n "${READLINE_LINE-}" ]; then
    READLINE_LINE=""
    READLINE_POINT=0
    return
  fi
  _alias_lens_flush_history 2>/dev/null || true
  local _alias_lens_name _alias_lens_definition
  _alias_lens_name="$(command env ALIAS_LENS_SHELL=bash ALIAS_LENS_HISTORY_FILE="${HISTFILE:-$HOME/.bash_history}" ALIAS_LENS_PROMPT_ACCEPT=1 alias-lens)" || return
  [ -z "$_alias_lens_name" ] && return
  _alias_lens_definition="$(command env ALIAS_LENS_SHELL=bash alias-lens shell-entry "$_alias_lens_name")" || return
  builtin eval "$_alias_lens_definition" || return
  command env ALIAS_LENS_SHELL=bash alias-lens record-use "$_alias_lens_name" >/dev/null 2>&1
  READLINE_LINE="$_alias_lens_name"
  READLINE_POINT=${#READLINE_LINE}
  bind '"\C-x\C-a":accept-line'
}
if [ -z "${ALIAS_LENS_NOBIND-}" ]; then
  bind '"\C-x\C-a":abort'
  bind -x '"\C-x\C-g":_alias_lens_prepare_readline'
  bind '"\C-g":"\C-x\C-g\C-x\C-a"'
fi
command env ALIAS_LENS_SHELL=bash ALIAS_LENS_HISTORY_FILE="${HISTFILE:-$HOME/.bash_history}" alias-lens watch --ensure >/dev/null 2>&1
`

const zshIntegration = `# Alias Lens Zsh integration
unalias al 2>/dev/null || true
_alias_lens_flush_history() {
  setopt localoptions extendedhistory
  fc -AI "${HISTFILE:-$HOME/.zsh_history}"
}
_alias_lens_execute() {
  local _alias_lens_name="$1" _alias_lens_definition _alias_lens_status
  _alias_lens_definition="$(command env ALIAS_LENS_SHELL=zsh alias-lens shell-entry "$_alias_lens_name")" || return
  builtin eval "$_alias_lens_definition" || return
  command env ALIAS_LENS_SHELL=zsh alias-lens record-use "$_alias_lens_name" >/dev/null 2>&1
  setopt localoptions extendedhistory
  print -s -- "$_alias_lens_name"
  fc -AI "${HISTFILE:-$HOME/.zsh_history}"
  builtin eval "$_alias_lens_name"
  _alias_lens_status=$?
  return "$_alias_lens_status"
}
al() {
  _alias_lens_flush_history 2>/dev/null || true
  if (( $# == 0 )); then
    local _alias_lens_name
    _alias_lens_name="$(command env ALIAS_LENS_SHELL=zsh ALIAS_LENS_HISTORY_FILE="${HISTFILE:-$HOME/.zsh_history}" alias-lens)" || return
    [[ -z "$_alias_lens_name" ]] && return
    _alias_lens_execute "$_alias_lens_name"
    return $?
  fi
  if [[ "${1-}" == "use" ]]; then
    shift
    if [[ "${1-}" == "--help" || "${1-}" == "-h" ]]; then
      command env ALIAS_LENS_SHELL=zsh ALIAS_LENS_HISTORY_FILE="${HISTFILE:-$HOME/.zsh_history}" alias-lens help use
      return
    fi
    local _alias_lens_name
    _alias_lens_name="$(command env ALIAS_LENS_SHELL=zsh ALIAS_LENS_HISTORY_FILE="${HISTFILE:-$HOME/.zsh_history}" alias-lens pick --execute "$@")" || return
    [[ -z "$_alias_lens_name" ]] && return
    _alias_lens_execute "$_alias_lens_name"
    return $?
  fi
  command env ALIAS_LENS_SHELL=zsh ALIAS_LENS_HISTORY_FILE="${HISTFILE:-$HOME/.zsh_history}" alias-lens "$@"
}
_alias_lens_launch() {
  if [[ -n "$BUFFER" ]]; then
    zle send-break
    return
  fi
  _alias_lens_flush_history 2>/dev/null || true
  local _alias_lens_name _alias_lens_definition
  _alias_lens_name="$(command env ALIAS_LENS_SHELL=zsh ALIAS_LENS_HISTORY_FILE="${HISTFILE:-$HOME/.zsh_history}" ALIAS_LENS_PROMPT_ACCEPT=1 alias-lens)" || return
  [[ -z "$_alias_lens_name" ]] && return
  _alias_lens_definition="$(command env ALIAS_LENS_SHELL=zsh alias-lens shell-entry "$_alias_lens_name")" || return
  builtin eval "$_alias_lens_definition" || return
  command env ALIAS_LENS_SHELL=zsh alias-lens record-use "$_alias_lens_name" >/dev/null 2>&1
  BUFFER="$_alias_lens_name"
  CURSOR=${#BUFFER}
  zle accept-line
}
if [[ -z "${ALIAS_LENS_NOBIND-}" ]]; then
  zle -N _alias_lens_launch
  bindkey '^G' _alias_lens_launch
fi
command env ALIAS_LENS_SHELL=zsh ALIAS_LENS_HISTORY_FILE="${HISTFILE:-$HOME/.zsh_history}" alias-lens watch --ensure >/dev/null 2>&1
`
