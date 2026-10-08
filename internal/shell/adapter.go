package shell

import (
	"alias-lens/internal/entry"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	activeShellEnvironment = "ALIAS_LENS_SHELL"
	historyFileEnvironment = "ALIAS_LENS_HISTORY_FILE"
)

type Adapter interface {
	Name() string
	DisplayName() string
	AliasFilename() string
	HistoryFilename() string
	ValidateEntryName(name, kind string) error
	ParseAliasDefinition(line string) (string, string, bool)
	ParseFunctions(contents string) []entry.Alias
	RenderEntryDefinition(alias entry.Alias) (string, error)
	HistoryCommand(line string) string
	HistoryUsageLine(line string, state *HistoryState) (string, time.Time, bool)
	Integration() string
	KnownIntegrationForms() []string
	ExecutionSpec() ExecutionSpec
	PromptSpec() PromptSpec
	BindingSpec() BindingSpec
	CheckSyntax(path string) *CheckFinding
	StartupPaths(home, platform string) ([]string, error)
	CatalogStartupPaths(home, platform string, explicit []string) ([]string, error)
	CatalogStartupRoute(path string, contents []byte, home string) (string, bool)
	PlanConfigureStartup(home, platform, executableDir string) ([]StartupEdit, error)
	PlanRemoveStartup(home, platform string) ([]StartupEdit, error)
	StartupStatus(home, platform string) (bool, string)
}

type HistoryState struct {
	BashTime time.Time
}

type ExecutionSpec struct {
	DefinitionCommand string
	ExecuteExpression string
	HistoryCommand    string
}

type PromptSpec struct {
	SelectionPrefix      string
	NonEmptyPromptAction string
}

type BindingSpec struct {
	Key                string
	DisableEnvironment string
}

type bashShellAdapter struct{}
type zshShellAdapter struct{ runtime Runtime }

type Runtime struct {
	Environment func(string) string
	WorkingDir  func() (string, error)
}

func defaultRuntime(runtime Runtime) Runtime {
	if runtime.Environment == nil {
		runtime.Environment = os.Getenv
	}
	if runtime.WorkingDir == nil {
		runtime.WorkingDir = os.Getwd
	}
	return runtime
}

func New(name string) (Adapter, error) { return NewWithRuntime(name, Runtime{}) }

func NewWithRuntime(name string, runtime Runtime) (Adapter, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "bash":
		return bashShellAdapter{}, nil
	case "zsh":
		return zshShellAdapter{runtime: defaultRuntime(runtime)}, nil
	default:
		return nil, fmt.Errorf("unsupported shell %q (use bash or zsh)", name)
	}
}

func parseHistoryUnix(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}
func (bashShellAdapter) Name() string            { return "bash" }
func (bashShellAdapter) DisplayName() string     { return "Bash" }
func (bashShellAdapter) AliasFilename() string   { return ".bash_aliases" }
func (bashShellAdapter) HistoryFilename() string { return ".bash_history" }
func (bashShellAdapter) Integration() string     { return GuardedIntegration(BashIntegration) }
func (bashShellAdapter) ValidateEntryName(name, kind string) error {
	return ValidateLegacyEntryName(name, kind)
}
func (bashShellAdapter) ParseAliasDefinition(line string) (string, string, bool) {
	return ParseLegacyAliasDefinition(line)
}
func (bashShellAdapter) ParseFunctions(contents string) []entry.Alias {
	return ParseLegacyFunctions(contents)
}
func (adapter bashShellAdapter) RenderEntryDefinition(alias entry.Alias) (string, error) {
	return RenderLegacyEntryDefinition(adapter, alias)
}
func (bashShellAdapter) HistoryCommand(line string) string { return line }
func (bashShellAdapter) HistoryUsageLine(line string, state *HistoryState) (string, time.Time, bool) {
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
func (bashShellAdapter) ExecutionSpec() ExecutionSpec {
	return ExecutionSpec{DefinitionCommand: `alias-lens shell-entry "$name"`, ExecuteExpression: `builtin eval "$name"`, HistoryCommand: `builtin history -s "$name"`}
}
func (bashShellAdapter) PromptSpec() PromptSpec {
	return PromptSpec{SelectionPrefix: "$ ", NonEmptyPromptAction: "clear-readline-buffer"}
}
func (bashShellAdapter) BindingSpec() BindingSpec {
	return BindingSpec{Key: "Ctrl+G", DisableEnvironment: "ALIAS_LENS_NOBIND"}
}
func (bashShellAdapter) StartupPaths(home, platform string) ([]string, error) {
	paths := []string{filepath.Join(home, ".bashrc")}
	if !BashLoginStartupSupported(platform) {
		return paths, nil
	}
	loginPath, err := BashLoginPath(home)
	if err != nil {
		return nil, err
	}
	return append(paths, loginPath), nil
}
func (adapter bashShellAdapter) StartupStatus(home, platform string) (bool, string) {
	bashrc, _ := os.ReadFile(filepath.Join(home, ".bashrc"))
	if !HasActiveShellReference(bashrc, ".bash_aliases") {
		return false, ".bashrc does not load .bash_aliases; run al setup bash"
	}
	if !BashLoginStartupSupported(platform) {
		return true, ".bashrc loads .bash_aliases"
	}
	paths, err := adapter.StartupPaths(home, platform)
	if err != nil {
		return false, err.Error()
	}
	loginPath := paths[1]
	login, _ := os.ReadFile(loginPath)
	if !HasActiveShellReference(login, ".bashrc") && !HasActiveShellReference(login, ".bash_aliases") {
		return false, filepath.Base(loginPath) + " does not load Bash aliases; run al setup bash"
	}
	return true, ".bashrc and " + filepath.Base(loginPath) + " load aliases"
}
func (zshShellAdapter) Name() string            { return "zsh" }
func (zshShellAdapter) DisplayName() string     { return "Zsh" }
func (zshShellAdapter) AliasFilename() string   { return ".zsh_aliases" }
func (zshShellAdapter) HistoryFilename() string { return ".zsh_history" }
func (zshShellAdapter) Integration() string     { return GuardedIntegration(ZshIntegration) }
func (zshShellAdapter) ValidateEntryName(name, kind string) error {
	return ValidateLegacyEntryName(name, kind)
}
func (zshShellAdapter) ParseAliasDefinition(line string) (string, string, bool) {
	return ParseLegacyAliasDefinition(line)
}
func (zshShellAdapter) ParseFunctions(contents string) []entry.Alias {
	return ParseLegacyFunctions(contents)
}
func (adapter zshShellAdapter) RenderEntryDefinition(alias entry.Alias) (string, error) {
	return RenderLegacyEntryDefinition(adapter, alias)
}
func (zshShellAdapter) HistoryCommand(line string) string {
	if strings.HasPrefix(line, ": ") {
		if separator := strings.IndexByte(line, ';'); separator >= 0 {
			return line[separator+1:]
		}
	}
	return line
}
func (zshShellAdapter) HistoryUsageLine(line string, _ *HistoryState) (string, time.Time, bool) {
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
func (zshShellAdapter) ExecutionSpec() ExecutionSpec {
	return ExecutionSpec{DefinitionCommand: `alias-lens shell-entry "$name"`, ExecuteExpression: `builtin eval "$name"`, HistoryCommand: `print -s -- "$name"`}
}
func (zshShellAdapter) PromptSpec() PromptSpec {
	return PromptSpec{SelectionPrefix: "$ ", NonEmptyPromptAction: "zle-send-break"}
}
func (zshShellAdapter) BindingSpec() BindingSpec {
	return BindingSpec{Key: "Ctrl+G", DisableEnvironment: "ALIAS_LENS_NOBIND"}
}
func (adapter zshShellAdapter) StartupPaths(home, _ string) ([]string, error) {
	path, err := zshStartupPath(home, defaultRuntime(adapter.runtime))
	if err != nil {
		return nil, err
	}
	return []string{path}, nil
}
func ValidateLegacyEntryName(name, kind string) error {
	if !aliasName.MatchString(name) {
		return fmt.Errorf("invalid alias name %q", name)
	}
	if kind == "function" && !functionName.MatchString(name) {
		return fmt.Errorf("invalid function name %q", name)
	}
	return nil
}
func RenderLegacyEntryDefinition(adapter Adapter, alias entry.Alias) (string, error) {
	if err := adapter.ValidateEntryName(alias.Name, alias.Type); err != nil {
		return "", err
	}
	if alias.Type == "function" {
		return fmt.Sprintf("%s() {\n%s\n}", alias.Name, alias.Command), nil
	}
	return "alias " + alias.Name + "=" + Quote(alias.Command), nil
}
func (adapter zshShellAdapter) StartupStatus(home, platform string) (bool, string) {
	paths, err := adapter.StartupPaths(home, platform)
	if err != nil {
		return false, err.Error()
	}
	path := paths[0]
	zshrc, _ := os.ReadFile(path)
	if !HasActiveShellReference(zshrc, ".zsh_aliases") {
		return false, path + " does not load .zsh_aliases; run al setup zsh"
	}
	return true, path + " loads .zsh_aliases"
}
func ZshStartupPath(home string) (string, error) {
	return zshStartupPath(home, defaultRuntime(Runtime{}))
}
func zshStartupPath(home string, runtime Runtime) (string, error) {
	directory := strings.TrimSpace(runtime.Environment("ZDOTDIR"))
	if directory == "" {
		directory = home
	} else if directory == "~" || strings.HasPrefix(directory, "~/") {
		directory = filepath.Join(home, strings.TrimPrefix(directory, "~/"))
	} else if !filepath.IsAbs(directory) {
		working, err := runtime.WorkingDir()
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(working) {
			return "", fmt.Errorf("working directory must be absolute")
		}
		absolute := filepath.Clean(filepath.Join(working, directory))
		directory = absolute
	}
	return filepath.Join(directory, ".zshrc"), nil
}
func BashLoginPath(home string) (string, error) {
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
func BashLoginStartupSupported(platform string) bool {
	return platform == "darwin" || platform == "macos" || platform == "linux"
}
func RemoveStartupPathBlocks(contents []byte) []byte {
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
func StartupPathReady(contents []byte, aliasFilename, home, directory string) bool {
	candidates := []string{filepath.Clean(directory)}
	if relative, err := filepath.Rel(home, directory); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		relative = filepath.ToSlash(relative)
		candidates = append(candidates, "$HOME/"+relative, "${HOME}/"+relative, "~/"+relative, `"$HOME"/`+Quote(relative))
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
func StartupPathBlock(home, directory string) string {
	value := Quote(filepath.Clean(directory))
	if relative, err := filepath.Rel(home, directory); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		value = `"$HOME"/` + Quote(filepath.ToSlash(relative))
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
func HasActiveShellReference(contents []byte, filename string) bool {
	for _, line := range strings.Split(string(contents), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && strings.Contains(trimmed, filename) {
			return true
		}
	}
	return false
}
func catalogExplicitStartupPaths(explicit []string) ([]string, error) {
	paths := []string{}
	seen := map[string]bool{}
	for _, path := range explicit {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || seen[path] {
			return nil, fmt.Errorf("--startup-path needs a unique absolute startup file")
		}
		seen[path] = true
		paths = append(paths, path)
	}
	return paths, nil
}
func (adapter bashShellAdapter) CatalogStartupPaths(home, platform string, explicit []string) ([]string, error) {
	if len(explicit) > 0 {
		return catalogExplicitStartupPaths(explicit)
	}
	return adapter.StartupPaths(home, platform)
}
func (adapter zshShellAdapter) CatalogStartupPaths(home, platform string, explicit []string) ([]string, error) {
	if len(explicit) > 0 {
		return catalogExplicitStartupPaths(explicit)
	}
	path, err := catalogZshStartupPath(home, defaultRuntime(adapter.runtime))
	if err != nil {
		return nil, err
	}
	return []string{path}, nil
}
func (adapter bashShellAdapter) KnownIntegrationForms() []string {
	return []string{adapter.Integration(), BashIntegration}
}
func (adapter zshShellAdapter) KnownIntegrationForms() []string {
	return []string{adapter.Integration(), ZshIntegration}
}

const BashAliasLoader = `
# >>> Alias Lens Bash alias loader >>>
if [ -f "$HOME/.bash_aliases" ]; then
  . "$HOME/.bash_aliases"
fi
# <<< Alias Lens Bash alias loader <<<
`

const BashLoginLoader = `
# >>> Alias Lens Bash login loader >>>
if [ -n "${BASH_VERSION:-}" ] && [ -f "$HOME/.bashrc" ]; then
  . "$HOME/.bashrc"
fi
# <<< Alias Lens Bash login loader <<<
`

const ZshAliasLoader = `
# >>> Alias Lens Zsh alias loader >>>
if [[ -f "$HOME/.zsh_aliases" ]]; then
  source "$HOME/.zsh_aliases"
fi
# <<< Alias Lens Zsh alias loader <<<
`

const BashIntegration = `# Alias Lens Bash integration
unalias al 2>/dev/null || true
if [ -s "$HOME/.config/alias-lens/completion.bash" ]; then
  . "$HOME/.config/alias-lens/completion.bash"
fi
# Bash records history timestamps only while HISTTIMEFORMAT is set. An empty
# value keeps the normal history display while preserving dates for stats.
if [ -z "${HISTTIMEFORMAT+x}" ]; then
  HISTTIMEFORMAT=
fi
_alias_lens_flush_history() {
  builtin history -a
}
_alias_lens_edit_notice() {
  if [ "${BASH_VERSINFO[0]:-0}" -lt 4 ]; then
    printf 'Selected %s. Bash 3.2 cannot insert it into the prompt; type the alias to edit it.\n' "$1" >&2
  else
    printf 'Selected %s. Use Ctrl+G from an empty prompt to edit it.\n' "$1" >&2
  fi
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
    case "$_alias_lens_name" in
      __alias_lens_edit__:*) _alias_lens_edit_notice "${_alias_lens_name#__alias_lens_edit__:}"; return ;;
    esac
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
    case "$_alias_lens_name" in __alias_lens_edit__:*) _alias_lens_edit_notice "${_alias_lens_name#__alias_lens_edit__:}"; return ;; esac
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
  local _alias_lens_edit=0
  case "$_alias_lens_name" in
    __alias_lens_edit__:*) _alias_lens_edit=1; _alias_lens_name="${_alias_lens_name#__alias_lens_edit__:}" ;;
  esac
  _alias_lens_definition="$(command env ALIAS_LENS_SHELL=bash alias-lens shell-entry "$_alias_lens_name")" || return
  builtin eval "$_alias_lens_definition" || return
  READLINE_LINE="$_alias_lens_name"
  READLINE_POINT=${#READLINE_LINE}
  if [ "$_alias_lens_edit" -eq 0 ]; then
    command env ALIAS_LENS_SHELL=bash alias-lens record-use "$_alias_lens_name" >/dev/null 2>&1
    bind '"\C-x\C-a":accept-line'
  fi
}
_alias_lens_bash_major="${BASH_VERSINFO[0]:-0}"
if [ "$_alias_lens_bash_major" -lt 4 ]; then
  case "$(bind -s 2>/dev/null)" in
    *'"\C-g": "\C-x\C-g\C-x\C-a"'*) bind '"\C-g":abort' ;;
  esac
elif [ -z "${ALIAS_LENS_NOBIND-}" ]; then
  bind '"\C-x\C-a":abort'
  bind -x '"\C-x\C-g":_alias_lens_prepare_readline'
  bind '"\C-g":"\C-x\C-g\C-x\C-a"'
fi
unset _alias_lens_bash_major
command env ALIAS_LENS_SHELL=bash ALIAS_LENS_HISTORY_FILE="${HISTFILE:-$HOME/.bash_history}" alias-lens watch --ensure >/dev/null 2>&1
`

const ZshIntegration = `# Alias Lens Zsh integration
unalias al 2>/dev/null || true
if [[ -s "$HOME/.config/alias-lens/completion.zsh" ]]; then
  source "$HOME/.config/alias-lens/completion.zsh"
fi
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
    if [[ "$_alias_lens_name" == __alias_lens_edit__:* ]]; then
      print -u2 -- "Selected ${_alias_lens_name#__alias_lens_edit__:}. Use Ctrl+G from an empty prompt to edit it."
      return
    fi
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
    if [[ "$_alias_lens_name" == __alias_lens_edit__:* ]]; then
      print -u2 -- "Selected ${_alias_lens_name#__alias_lens_edit__:}. Use Ctrl+G from an empty prompt to edit it."
      return
    fi
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
  local _alias_lens_edit=0
  if [[ "$_alias_lens_name" == __alias_lens_edit__:* ]]; then
    _alias_lens_edit=1
    _alias_lens_name="${_alias_lens_name#__alias_lens_edit__:}"
  fi
  _alias_lens_definition="$(command env ALIAS_LENS_SHELL=zsh alias-lens shell-entry "$_alias_lens_name")" || return
  builtin eval "$_alias_lens_definition" || return
  BUFFER="$_alias_lens_name"
  CURSOR=${#BUFFER}
  if (( !_alias_lens_edit )); then
    command env ALIAS_LENS_SHELL=zsh alias-lens record-use "$_alias_lens_name" >/dev/null 2>&1
    zle accept-line
  fi
}
if [[ -z "${ALIAS_LENS_NOBIND-}" ]]; then
  zle -N _alias_lens_launch
  bindkey '^G' _alias_lens_launch
fi
command env ALIAS_LENS_SHELL=zsh ALIAS_LENS_HISTORY_FILE="${HISTFILE:-$HOME/.zsh_history}" alias-lens watch --ensure >/dev/null 2>&1
`

func ParseLegacyAliasDefinition(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "alias ") {
		return "", "", false
	}
	definition := strings.TrimSpace(strings.TrimPrefix(trimmed, "alias "))
	name, value, found := strings.Cut(definition, "=")
	name = strings.TrimSpace(name)
	value = strings.TrimSpace(value)
	if !found || name == "" || value == "" || !aliasName.MatchString(name) {
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

func ParseLegacyFunctions(contents string) []entry.Alias {
	lines := strings.Split(contents, "\n")
	var functions []entry.Alias
	var notes []string
	metadata := entry.EntryMetadata{}
	for index := 0; index < len(lines); index++ {
		trimmed := strings.TrimSpace(lines[index])
		if parsed, ok := entry.ParseMetadataComment(lines[index]); ok {
			metadata = parsed
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			note := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
			if note != "" && !entry.IsSectionHeading(note) {
				notes = append(notes, note)
			}
			continue
		}
		match := functionStart.FindStringSubmatch(lines[index])
		if match == nil {
			if trimmed != "" {
				notes = nil
				metadata = entry.EntryMetadata{}
			}
			continue
		}
		name := match[1]
		body := strings.TrimSpace(match[2])
		if closeIndex := strings.LastIndex(body, "}"); closeIndex >= 0 {
			body = body[:closeIndex]
		} else {
			var bodyLines []string
			if body != "" {
				bodyLines = append(bodyLines, body)
			}
			for index++; index < len(lines); index++ {
				if strings.TrimSpace(lines[index]) == "}" {
					break
				}
				bodyLines = append(bodyLines, strings.TrimSpace(lines[index]))
			}
			body = strings.Join(bodyLines, " ")
		}
		body = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(body), ";"))
		description := "Shell function"
		if len(notes) > 0 {
			description = notes[len(notes)-1]
		}
		function := entry.Alias{Name: name, Command: body, Description: description, Category: entry.Category(body), Type: "function"}
		entry.ApplyMetadata(&function, metadata)
		functions = append(functions, function)
		notes = nil
		metadata = entry.EntryMetadata{}
	}
	return functions
}

func GuardedIntegration(text string) string {
	text = strings.Replace(text, "unalias al 2>/dev/null || true\n", "", 1)
	declarations := regexp.MustCompile(`(?m)^([A-Za-z_][A-Za-z0-9_]*)\(\) \{$`)
	text = declarations.ReplaceAllString(text, "function $1 {")
	start := strings.Index(text, "function al {\n")
	if start < 0 {
		return text
	}
	end := strings.Index(text[start:], "\n}\n")
	if end < 0 {
		return text
	}
	end += start + len("\n}\n")
	return text[:start] + "if " + text[start:end] + "then builtin unalias al 2>/dev/null || :; fi\n" + text[end:]
}

func ShellEntryHandoff(adapter Adapter, entry entry.Alias) (string, error) {
	declaration, err := adapter.RenderEntryDefinition(entry)
	if err != nil {
		return "", err
	}
	if entry.Type != "function" {
		return declaration, nil
	}
	declaration = strings.Replace(declaration, entry.Name+"() {", "function "+entry.Name+" {", 1)
	return "if builtin eval " + Quote(declaration) + "; then builtin unalias -- " + entry.Name + " 2>/dev/null || :; else builtin false; fi", nil
}

func Quote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

var aliasName = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
var functionName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var functionStart = regexp.MustCompile(`^\s*(?:function\s+)?([A-Za-z_][A-Za-z0-9_]*)(?:\s*\(\s*\))?\s*\{(.*)$`)
