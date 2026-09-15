package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	Integration() string
	ConfigureStartup(home, platform string) error
	StartupStatus(home, platform string) (bool, string)
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

func printAliasReceipt(name string) error {
	alias, err := loadAliasEntry(name)
	if err != nil {
		return err
	}
	fmt.Println(formatAliasReceipt(alias))
	return nil
}

func formatAliasReceipt(alias Alias) string {
	return fmt.Sprintf("Alias Lens ran [%s]: %s", alias.Name, strconv.QuoteToGraphic(alias.Command))
}

func loadShellEntry(name string) (string, error) {
	alias, err := loadAliasEntry(name)
	if err != nil {
		return "", err
	}
	return shellEntryDefinition(alias)
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
	if !aliasName.MatchString(alias.Name) {
		return "", fmt.Errorf("invalid alias name %q", alias.Name)
	}
	if alias.Type == "function" {
		if !functionName.MatchString(alias.Name) {
			return "", fmt.Errorf("invalid function name %q", alias.Name)
		}
		return fmt.Sprintf("%s() {\n%s\n}", alias.Name, alias.Command), nil
	}
	return "alias " + alias.Name + "=" + shellQuote(alias.Command), nil
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

func (bashShellAdapter) ConfigureStartup(home, platform string) error {
	if err := ensureStartupFileLoads(filepath.Join(home, ".bashrc"), ".bash_aliases", bashAliasLoader); err != nil {
		return err
	}
	if platform != "darwin" {
		return nil
	}
	loginPath, err := bashLoginPath(home)
	if err != nil {
		return err
	}
	contents, err := os.ReadFile(loginPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if hasActiveShellReference(contents, ".bashrc") || hasActiveShellReference(contents, ".bash_aliases") {
		return nil
	}
	return appendStartupBlock(loginPath, contents, bashLoginLoader)
}

func (bashShellAdapter) StartupStatus(home, platform string) (bool, string) {
	bashrc, _ := os.ReadFile(filepath.Join(home, ".bashrc"))
	if !hasActiveShellReference(bashrc, ".bash_aliases") {
		return false, ".bashrc does not load .bash_aliases; run al setup bash"
	}
	if platform != "darwin" {
		return true, ".bashrc loads .bash_aliases"
	}
	loginPath, err := bashLoginPath(home)
	if err != nil {
		return false, err.Error()
	}
	login, _ := os.ReadFile(loginPath)
	if !hasActiveShellReference(login, ".bashrc") && !hasActiveShellReference(login, ".bash_aliases") {
		return false, filepath.Base(loginPath) + " does not load Bash aliases; run al setup bash"
	}
	return true, ".bashrc and the Bash login file load aliases"
}

func (zshShellAdapter) Name() string            { return "zsh" }
func (zshShellAdapter) DisplayName() string     { return "Zsh" }
func (zshShellAdapter) AliasFilename() string   { return ".zsh_aliases" }
func (zshShellAdapter) HistoryFilename() string { return ".zsh_history" }
func (zshShellAdapter) Integration() string     { return zshIntegration }

func (zshShellAdapter) ConfigureStartup(home, _ string) error {
	path, err := zshStartupPath(home)
	if err != nil {
		return err
	}
	return ensureStartupFileLoads(path, ".zsh_aliases", zshAliasLoader)
}

func (zshShellAdapter) StartupStatus(home, _ string) (bool, string) {
	path, err := zshStartupPath(home)
	if err != nil {
		return false, err.Error()
	}
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

func ensureStartupFileLoads(path, aliasFilename, block string) error {
	contents, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if hasActiveShellReference(contents, aliasFilename) {
		return nil
	}
	return appendStartupBlock(path, contents, block)
}

func appendStartupBlock(path string, contents []byte, block string) error {
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
	return os.WriteFile(path, append(contents, []byte(block)...), mode)
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
# Load personal Bash aliases.
if [ -f "$HOME/.bash_aliases" ]; then
  . "$HOME/.bash_aliases"
fi
`

const bashLoginLoader = `
# Load Bash configuration for login shells.
if [ -n "${BASH_VERSION:-}" ] && [ -f "$HOME/.bashrc" ]; then
  . "$HOME/.bashrc"
fi
`

const zshAliasLoader = `
# Load personal Zsh aliases.
if [[ -f "$HOME/.zsh_aliases" ]]; then
  source "$HOME/.zsh_aliases"
fi
`

const bashIntegration = `# Alias Lens Bash integration
unalias al 2>/dev/null || true
_alias_lens_report() {
  local _alias_lens_receipt="$1" _alias_lens_status="$2"
  if [ "$_alias_lens_status" -ne 0 ]; then
    _alias_lens_receipt="$_alias_lens_receipt (exit $_alias_lens_status)"
  fi
  printf '\n%s\n' "$_alias_lens_receipt"
}
_alias_lens_flush_history() {
  local HISTTIMEFORMAT='%s '
  builtin history -a
}
_alias_lens_execute() {
  local _alias_lens_name="$1" _alias_lens_definition _alias_lens_receipt _alias_lens_status HISTTIMEFORMAT='%s '
  _alias_lens_definition="$(command env ALIAS_LENS_SHELL=bash alias-lens shell-entry "$_alias_lens_name")" || return
  _alias_lens_receipt="$(command env ALIAS_LENS_SHELL=bash alias-lens entry-summary "$_alias_lens_name")" || return
  builtin eval "$_alias_lens_definition" || return
  command env ALIAS_LENS_SHELL=bash alias-lens record-use "$_alias_lens_name" >/dev/null 2>&1
  builtin history -s "$_alias_lens_name"
  builtin history -a
  builtin eval "$_alias_lens_name"
  _alias_lens_status=$?
  _alias_lens_report "$_alias_lens_receipt" "$_alias_lens_status"
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
_alias_lens_launch() {
  if [ -n "${READLINE_LINE-}" ]; then
    READLINE_LINE=""
    READLINE_POINT=0
    return
  fi
  _alias_lens_flush_history 2>/dev/null || true
  local _alias_lens_name
  _alias_lens_name="$(command env ALIAS_LENS_SHELL=bash ALIAS_LENS_HISTORY_FILE="${HISTFILE:-$HOME/.bash_history}" alias-lens)" || return
  [ -z "$_alias_lens_name" ] && return
  _alias_lens_execute "$_alias_lens_name"
}
if [ -z "${ALIAS_LENS_NOBIND-}" ]; then
  bind -x '"\C-g":_alias_lens_launch'
fi
command env ALIAS_LENS_SHELL=bash ALIAS_LENS_HISTORY_FILE="${HISTFILE:-$HOME/.bash_history}" alias-lens watch --ensure >/dev/null 2>&1
`

const zshIntegration = `# Alias Lens Zsh integration
unalias al 2>/dev/null || true
_alias_lens_report() {
  local _alias_lens_receipt="$1" _alias_lens_status="$2"
  if [[ "$_alias_lens_status" -ne 0 ]]; then
    _alias_lens_receipt="$_alias_lens_receipt (exit $_alias_lens_status)"
  fi
  printf '\n%s\n' "$_alias_lens_receipt"
}
_alias_lens_flush_history() {
  setopt localoptions extendedhistory
  fc -AI "${HISTFILE:-$HOME/.zsh_history}"
}
_alias_lens_execute() {
  local _alias_lens_name="$1" _alias_lens_definition _alias_lens_receipt _alias_lens_status
  _alias_lens_definition="$(command env ALIAS_LENS_SHELL=zsh alias-lens shell-entry "$_alias_lens_name")" || return
  _alias_lens_receipt="$(command env ALIAS_LENS_SHELL=zsh alias-lens entry-summary "$_alias_lens_name")" || return
  builtin eval "$_alias_lens_definition" || return
  command env ALIAS_LENS_SHELL=zsh alias-lens record-use "$_alias_lens_name" >/dev/null 2>&1
  setopt localoptions extendedhistory
  print -s -- "$_alias_lens_name"
  fc -AI "${HISTFILE:-$HOME/.zsh_history}"
  builtin eval "$_alias_lens_name"
  _alias_lens_status=$?
  _alias_lens_report "$_alias_lens_receipt" "$_alias_lens_status"
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
  local _alias_lens_name _alias_lens_status
  _alias_lens_name="$(command env ALIAS_LENS_SHELL=zsh ALIAS_LENS_HISTORY_FILE="${HISTFILE:-$HOME/.zsh_history}" alias-lens)" || return
  [[ -z "$_alias_lens_name" ]] && return
  _alias_lens_execute "$_alias_lens_name"
  _alias_lens_status=$?
  zle reset-prompt
  return "$_alias_lens_status"
}
if [[ -z "${ALIAS_LENS_NOBIND-}" ]]; then
  zle -N _alias_lens_launch
  bindkey '^G' _alias_lens_launch
fi
command env ALIAS_LENS_SHELL=zsh ALIAS_LENS_HISTORY_FILE="${HISTFILE:-$HOME/.zsh_history}" alias-lens watch --ensure >/dev/null 2>&1
`
