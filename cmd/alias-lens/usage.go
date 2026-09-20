package main

import (
	"fmt"
	"strings"
)

const usageText = `Alias Lens manages Bash and Zsh aliases from a terminal interface.

Usage:
  Ctrl+G                     Open from an empty Zsh or Bash 4+ prompt
  al                         Open the same browser without using the shortcut
  al COMMAND [ARGUMENTS]     Run a command without opening the browser
  al help [COMMAND]          Explain all commands or one command

Find and use aliases:
  pick       Select an alias and print its name or command; never runs it
  use        Select and immediately run an alias; requires al setup
  search     Print aliases that match a name, command, description, or tag
  stats      Rank aliases used directly or launched through Alias Lens
  export     Export aliases or usage stats as JSON, YAML, or CSV
  import     Preview aliases from a file, then apply them in one safe write
  suggest    Find repeated commands in local shell history and optionally add one
  meta       Set tags, supported platforms, or favorite status on an alias
  describe   Add generated comments to aliases that do not have descriptions

Protect and recover aliases:
  status     Show what is ready and what needs attention without changing files
  plan       Preview a catalog or settings change without applying it
  check      Check alias syntax and details without running the file
  scan       Report likely secrets by type and line number; hides secret values
  history    List private revisions created before Alias Lens changes the file
  undo       Restore a revision after saving the current alias file first
  doctor     Diagnose the binary, shell integration, Git, providers, and sync
  setup      Install, repair, or remove the Bash or Zsh integration
  data       List local data paths or clear usage data and private revisions
  catalog    Inspect catalog migration safety without changing shell files

Configure Git sync:
  repo       Choose or clone a Git repository and enable automatic sync
  config     Show settings or configure repository providers and clone protocols
  track      Add one non-secret config file to automatic sync
  untrack    Stop syncing a tracked file; does not delete either copy
  sync       Copy and commit aliases locally, or explicitly push or pull
  diff       Compare local and repository aliases without changing either file
  autosync   Enable, disable, or show the background sync status
  watch      Check once for alias changes that need to sync

Other commands:
  theme      List dark themes or select one by preset name
  shortcuts  Show or choose Windows, Linux, or macOS keyboard shortcuts
  completion Print Bash or Zsh completion code
  shell-init Print Bash or Zsh integration; normally called by al setup
  --web      Start the optional local web interface on 127.0.0.1:8787
  --version  Print the installed version

Run "al help COMMAND" or "al COMMAND --help" for syntax, effects, and examples.
`

var commandUsage = map[string]string{
	"status": `Usage: al status [--json]

Show whether settings, the portable catalog, shell setup, synchronization, and
recovery state are ready. This command only reads local files. It does not
create files, migrate settings, repair an interrupted change, or contact a
provider. Exit status 0 means everything is ready. Status 1 means one or more
items need attention.
`,
	"plan": `Usage: al plan [--json] COMMAND [ARGUMENTS]

Preview a catalog or settings change without applying it. The report lists each
file action, why it is needed, its review level, and whether it can be undone.
Planning can use private temporary files for validation, but removes them before
returning and does not change managed files.

Available previews:
  al plan config migrate
  al plan config profile add NAME
  al plan config profile remove NAME
  al plan catalog migrate --to 2
  al plan completion install bash
  al plan completion remove zsh
`,
	"catalog": `Usage:
  al catalog preview [--from bash|zsh] [--json]
  al catalog import --from bash|zsh
  al catalog shadow [--shell bash|zsh] [--json]
  al catalog diff [--json|--show-code|--web] [--from repository|installed] [--shell bash|zsh]
  al catalog migrate --to 2

Preview checks which entries can move into a portable catalog and changes no
files. Import copies safe entries into an inactive catalog. It keeps your native
alias file and shell setup unchanged. Shadow prints the lower-level safety report.
It never writes a catalog, alias file, startup file, configuration, or sync state.
Exit status 0 means every inspected entry is equivalent. Status 1 means at least
one entry needs attention. Status 2 means Alias Lens could not inspect the file.

Catalog diff compares entries by their stable identity and reports which details
changed without printing command or function text. --show-code displays exact
private text only in an interactive terminal. It leaves both files unchanged.

Catalog migrate saves a private backup, then updates a version 1 catalog to data
format 2 without changing its entries or preparing shell files.
`,
	"pick": `Usage: al pick [--command] [QUERY]

Open a terminal picker, optionally filtered by QUERY. By default, the selected
alias name is printed to standard output. --command prints the underlying shell
command instead. This command only prints a selection and never runs it.
Inside an integration-launched picker, Tab returns the alias to the prompt for
editing instead of accepting it.

Examples:
  al pick
  al pick git
  al pick --command git
`,
	"use": `Usage: al use [QUERY]

Open the picker and immediately run the selected alias command. This action
is provided by the shell integration, so run "al setup" and start a new shell
first. This is the same Enter behavior as plain "al", with an optional
starting query. Press Tab to return the alias to the prompt without running it.
Use "al pick" when you want a result without running it.
`,
	"suggest": `Usage:
  al suggest
  al suggest add NUMBER [NAME]

Read the active shell's private history file and list repeated long commands that
do not already have aliases. Commands likely to contain credentials are excluded.
The add form writes the numbered suggestion to the active alias file.
`,
	"search": `Usage: al search [--json] [QUERY]

Print every alias that matches QUERY across its name, command, description, tags,
category, and platform metadata. An empty query lists every alias. Output is plain
tab-separated text by default. --json emits the complete Alias objects.

Examples:
  al search git
  al search --json daily
`,
	"stats": `Usage: al stats [--plain] [all|today|week|month|year]

Rank aliases found in the active terminal history. The default period is all.
Today starts at local midnight. Week and year mean the previous 7 days and 12
months. A terminal opens the interactive dashboard; --plain prints rows.
The shell integration flushes the current session before reading the history
file. Time-based periods require timestamped Bash or Zsh history.
`,
	"export": `Usage: al export aliases|stats [--format json|yaml|csv] [--period PERIOD] [--output PATH]

Export aliases or ranked usage stats without running alias commands. JSON is
the default format. Stats periods are all, today, week, and year. --output writes
the export atomically with private file permissions; otherwise output goes to
standard output.

Examples:
  al export aliases --format yaml
  al export stats --format csv --period week --output weekly-aliases.csv
`,
	"import": `Usage: al import FILE [--apply]

Preview aliases from a Bash or Zsh file. The preview reports syntax problems,
duplicate names, duplicate commands, conflicts, skipped aliases, and planned
additions without writing. --apply refuses blocking problems and adds all
accepted aliases in one backed-up atomic replacement.
`,
	"meta": `Usage: al meta ALIAS key=value [key=value ...]

Write search and display metadata above an existing alias or shell function.
Supported keys are tags, collections, category, platforms, and favorite. Alias Lens saves
a backup and private revision before changing the active alias file.

Examples:
  al meta gs tags=git,daily favorite=true
  al meta docker-clean platforms=linux,wsl
`,
	"describe": `Usage: al describe

Add an action-oriented description above every alias that does not already have
one. The command also improves older generated comments that merely repeat the
command. Custom comments, commands, metadata, and functions are unchanged.
Alias Lens writes the file once and saves a backup and private revision first.
`,
	"scan": `Usage: al scan

Read the active alias file and report likely credentials by type and line number.
Secret values are never printed. Alias Lens runs this check before every push.
`,
	"check": `Usage: al check [--strict]

Check the active alias file without loading or running it. Alias Lens
checks definitions, duplicate names, metadata, multiline aliases, likely
secrets, and missing executables. It also runs the configured shell in syntax-
only mode. Diagnostics never include the source line or a secret value.

Exit status 0 means that the file has no errors. Warnings also fail when
--strict is set. Exit status 1 means that validation failed. Exit status 2 means
that Alias Lens could not read the file or configuration.
`,
	"history": `Usage: al history

List the timestamp, local time, and size of each private alias revision. Alias
Lens creates these revisions before it changes or restores the active alias file.
`,
	"undo": `Usage: al undo [REVISION]

Restore the newest private revision, or the exact revision ID shown by
"al history". The current alias file is backed up and saved as another revision
before the restore.
`,
	"doctor": `Usage: al doctor

Check the installed binary, alias file syntax, shell startup loading, integration, Git,
sync repository, automatic sync state, provider credentials, and SSH access.
Failed checks print the command or action that should fix them.
`,
	"setup": `Usage:
  al setup [bash|zsh]
  al setup --repair [bash|zsh]
  al setup --remove [bash|zsh]

Detect the current Bash or Zsh shell and install the function used by "al" and
"al use". Zsh and Bash 4+ also get a prompt-aware Ctrl+G launcher. Ctrl+G opens
Alias Lens when the prompt is empty and keeps its cancel behavior when the
prompt contains text. Bash 3.2 keeps normal Readline cancellation; run "al".
--repair restores missing generated integration and removes duplicate generated
blocks. --remove removes only the Alias Lens integration. It keeps aliases,
configuration, revisions, and repositories.
Set ALIAS_LENS_NOBIND=1 before the integration loads to disable the binding.
Pass a shell name to override detection.
Bash uses ~/.bash_aliases, ~/.bashrc, and the existing login file. Zsh uses
~/.zsh_aliases and ~/.zshrc.
Setup adds a binary installed under the home directory to the shell's PATH.
This keeps Alias Lens available after a WSL restart. Bash login-file precedence
is preserved on Linux, WSL, and macOS. Files are backed up before editing, and
missing alias files are created with mode 0600. In an interactive terminal,
Alias Lens explains optional developer aliases and asks before adding them.
Fish, PowerShell, and Command Prompt are not supported.
`,
	"repo": `Usage:
  al repo
  al repo github|bitbucket|gitlab
  al repo /path/to/dotfiles

Choose the Git repository that stores the alias file. With no argument, show
writable repositories from every configured provider. A provider argument
limits that picker. Remote selections are cloned under Alias Lens local data.
A path selects an existing local Git repository.

Selecting a repository updates Alias Lens configuration and enables automatic
sync. It does not add provider tokens to the repository or configuration file.
"al repo github" starts GitHub CLI sign-in when needed, then opens the picker.
"al repo gitlab" does the same through GitLab CLI. "al repo bitbucket" reads
a scoped API token without echo and keeps it only for the current picker.
`,
	"config": `Usage:
  al config
  al config shell bash|zsh
  al config provider github [HOST]
  al config provider bitbucket WORKSPACE
  al config provider gitlab [HOST]
  al config protocol PROVIDER auto|ssh|https
  al config disable PROVIDER
  al config migrate
  al config profile list
  al config profile add NAME
  al config profile remove NAME
  al config footer-message MESSAGE
  al config footer-icon ICON
  al config footer-reset

With no arguments, print the effective Alias Lens configuration without tokens.
The other forms select a shell, enable a provider, choose its Git clone protocol,
manage local machine profiles, update the settings data format, disable a
provider, or customize the TUI footer. Profile changes show their plan, save a
private backup, and do not prepare new shell definitions automatically.
Use {icon} in MESSAGE to place the icon.
ICON may be heart, spark, brand, alias, command, stats, sync, theme, none, one
emoji written as emoji:VALUE, or a custom 4x2 bitmap such as #..#/.##. Quote
messages, emoji values, and custom bitmaps in a shell.
GitHub uses the gh CLI. Bitbucket and GitLab read tokens from environment
variables and never store them in config.json.
`,
	"data": `Usage:
  al data paths
  al data clear-usage
  al data clear-revisions

List every local path Alias Lens uses, delete the private usage log, or delete
private alias revisions. Clear commands do not remove aliases, configuration,
shell startup settings, repositories, conflict copies, or latest backups.
`,
	"track": `Usage: al track SOURCE [REPOSITORY_PATH]

Enroll one extra local config file in automatic sync. REPOSITORY_PATH chooses
where it lives inside the configured Git repository and defaults to the source
filename. Credential-shaped files, keys, and environment files are rejected.

Example:
  al track ~/.config/starship.toml shell/starship.toml
`,
	"untrack": `Usage: al untrack SOURCE

Remove SOURCE from the tracked-file registry. This stops future automatic sync
for the file. It does not delete the local file or its repository copy.
`,
	"sync": `Usage:
  al sync
  al sync --push
  al sync --pull

"al sync" copies the active alias file into the configured repository and creates a
commit containing only that alias file. It does not push. --push also sends the
commit to the Git remote after a secret scan. --pull uses "git pull --ff-only"
and imports remote-only aliases. Conflicting definitions stop the import.
`,
	"diff": `Usage: al diff

Compare aliases and shell functions in the live alias file with the configured
repository copy. Print names that exist on only one side and both command texts
for changed definitions. Neither file is modified.
`,
	"autosync": `Usage:
  al autosync status
  al autosync enable
  al autosync disable

Show automatic-sync state, start the background worker, or tell the worker to
stop after its current check. Enabling requires a configured repository.
Automatic sync may pull, commit, and push aliases and explicitly tracked files.
`,
	"watch": `Usage: al watch

Check once for alias changes that need to sync. Alias Lens safely downloads or
uploads when only one side changed. If both sides changed, it saves private
copies for comparison and leaves the alias file you use unchanged.
`,
	"theme": `Usage: al theme [PRESET|--check]

List every built-in dark theme and mark the active one. Pass a preset name to
save it immediately. --check prints the selected theme's text and control
contrast ratios. Ctrl+T opens a live-preview theme picker inside the TUI.

Examples:
  al theme
  al theme tokyo-night
`,
	"shortcuts": `Usage: al shortcuts [auto|windows|linux|macos|test]

Show the keyboard style Alias Lens uses. Without a saved choice, Alias Lens
chooses Windows on Windows and WSL, macOS on macOS, and Linux on Linux. You can
choose any style on any computer. Use auto to remove a saved choice and return
to the style for the current computer. The test option shows the active
shortcuts without changing your shortcut choice.

Examples:
  al shortcuts
  al shortcuts macos
  al shortcuts auto
  al shortcuts test
`,
	"completion": `Usage:
  al completion bash|zsh
  al completion install bash|zsh
  al completion remove bash|zsh

Print a deterministic completion program for Bash or Zsh. The program completes
commands, flags, shell names, configured profile names, and entry names. Dynamic
candidates are read only from local Alias Lens files. Completion never contacts
a provider, runs an alias, or prints command and function implementations.

Install saves the generated program in Alias Lens's private settings directory.
Remove deletes that owned file so new shells stop loading suggestions. Neither
command changes aliases or another shell's settings.
`,
	"shell-init": `Usage: al shell-init bash|zsh

Print the selected shell's functions and prompt integration. Zsh and Bash 4+
install a prompt-aware Ctrl+G binding. Bash 3.2 keeps normal cancellation and
uses "al". This command does not edit shell files by itself. "al setup" installs
the correct output safely. Set ALIAS_LENS_NOBIND=1 to skip the binding.
`,
	"--web": `Usage: al --web

Start the optional browser interface at http://127.0.0.1:8787. The server binds
only to the local computer and reads aliases from the active shell's alias file.
`,
	"--version": `Usage: al --version

Print the installed Alias Lens version.
`,
}

func isHelpFlag(argument string) bool {
	return argument == "--help" || argument == "-h"
}

func printUsage() {
	fmt.Print(usageText)
}

func printCommandUsage(command string) {
	command = strings.ToLower(command)
	spec, ok := lookupCommandSpec(command)
	if !ok {
		fmt.Printf("Unknown command %q.\n\n", command)
		printUsage()
		return
	}
	fmt.Print(spec.Usage)
}
