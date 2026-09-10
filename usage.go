package main

import (
	"fmt"
	"strings"
)

const usageText = `Alias Lens manages Bash and Zsh aliases from a terminal interface.

Usage:
  al                         Open the alias browser; Enter executes the selection
  al COMMAND [ARGUMENTS]     Run a command without opening the browser
  al help [COMMAND]          Explain all commands or one command

Find and use aliases:
  pick       Select an alias and print its name or command; never executes it
  use        Select and immediately execute an alias; requires al setup
  suggest    Find repeated commands in local shell history and optionally add one
  meta       Set tags, supported platforms, or favorite status on an alias
  describe   Add generated comments to aliases that do not have descriptions

Protect and recover aliases:
  scan       Report likely secrets by type and line number; hides secret values
  history    List private revisions created before Alias Lens changes the file
  undo       Restore a revision after saving the current alias file first
  doctor     Diagnose the binary, shell integration, Git, providers, and sync
  setup      Detect Bash or Zsh, back up shell files, and install integration

Configure Git sync:
  repo       Choose or clone a Git repository and enable automatic sync
  config     Show settings or configure repository providers and clone protocols
  track      Add one non-secret config file to automatic sync
  untrack    Stop syncing a tracked file; does not delete either copy
  sync       Copy and commit aliases locally, or explicitly push or pull
  diff       Compare local and repository aliases without changing either file
  autosync   Enable, disable, or show the background sync status
  watch      Run one automatic-sync reconciliation cycle in the foreground

Other commands:
  shell-init Print Bash or Zsh integration; normally called by al setup
  --web      Start the optional local web interface on 127.0.0.1:8787
  --version  Print the installed version

Run "al help COMMAND" or "al COMMAND --help" for syntax, effects, and examples.
`

var commandUsage = map[string]string{
	"pick": `Usage: al pick [--command] [QUERY]

Open a terminal picker, optionally filtered by QUERY. By default, the selected
alias name is printed to standard output. --command prints the underlying shell
command instead. This command only prints a selection and never executes it.

Examples:
  al pick
  al pick git
  al pick --command git
`,
	"use": `Usage: al use [QUERY]

Open the picker and immediately execute the selected alias command. This action
is provided by the shell integration, so run "al setup" and start a new shell
shell first. This is the same Enter behavior as plain "al", with an optional
starting query. Use "al pick" when you want a result without executing it.
`,
	"suggest": `Usage:
  al suggest
  al suggest add NUMBER [NAME]

Read the active shell's private history file and list repeated long commands that
do not already have aliases. Commands likely to contain credentials are excluded.
The add form writes the numbered suggestion to the active alias file.
`,
	"meta": `Usage: al meta ALIAS key=value [key=value ...]

Write search and display metadata above an existing alias or shell function.
Supported keys are tags, collections, platforms, and favorite. Alias Lens saves
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

Check the installed binary, alias file, shell startup loading, integration, Git,
sync repository, automatic sync state, provider credentials, and SSH access.
Failed checks print the command or action that should fix them.
`,
	"setup": `Usage: al setup [bash|zsh]

Detect the current Bash or Zsh shell and install the function used by "al" and
"al use", plus the Ctrl+G binding. Pass a shell name to override detection.
Bash uses ~/.bash_aliases and ~/.bashrc; Zsh uses ~/.zsh_aliases and ~/.zshrc.
On macOS, Bash login-shell precedence is preserved. Files are backed up before
editing, and missing alias files are created with mode 0600. In an interactive
terminal, optional developer aliases are explained and require confirmation.
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
`,
	"config": `Usage:
  al config
  al config shell bash|zsh
  al config provider github [HOST]
  al config provider bitbucket WORKSPACE
  al config provider gitlab [HOST]
  al config protocol PROVIDER auto|ssh|https
  al config disable PROVIDER

With no arguments, print the effective Alias Lens configuration without tokens.
The other forms select a shell, enable a provider, choose its Git clone protocol,
or disable it.
GitHub uses the gh CLI. Bitbucket and GitLab read tokens from environment
variables and never store them in config.json.
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

Run one automatic-sync reconciliation cycle in the foreground. The cycle pulls
with fast-forward-only Git behavior, compares saved hashes, and then safely
pulls or pushes when only one side changed. If both sides changed, it saves
private conflict copies and leaves the live alias file unchanged.
`,
	"shell-init": `Usage: al shell-init bash|zsh

Print the selected shell's function and Ctrl+G binding. This command does not
edit shell files by itself. "al setup" installs the correct output safely.
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
	if command == "version" || command == "-v" {
		command = "--version"
	}
	text, ok := commandUsage[command]
	if !ok {
		fmt.Printf("Unknown command %q.\n\n", command)
		printUsage()
		return
	}
	fmt.Print(text)
}
