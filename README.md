# Alias Lens

**Remember less. Move faster.**

Alias Lens is a terminal-first Bash and Zsh alias manager for searching, explaining, editing, and syncing shell shortcuts. It combines fuzzy search, command history suggestions, alias health checks, private revisions, and conflict-aware Git sync in a Bubble Tea TUI.

Launch it with two letters from any directory:

```bash
al
```

It opens an interactive Bubble Tea interface directly in the terminal. A browser is never required.

## Install locally

Alias Lens requires Bash or Zsh, Make, and Go 1.24 or newer. Clone the repository and install the binary in `~/.local/bin`:

```bash
git clone https://github.com/alderon07/al.git && make -C al install && "$HOME/.local/bin/alias-lens" setup
```

Make sure `~/.local/bin` is on `PATH`, then start a new shell. Setup detects Bash or Zsh, installs the matching integration, and tells you which shell it configured. After restarting, run `al --version` to verify the installation.

`alias-lens setup` uses `$SHELL` to select Bash or Zsh. Pass `bash` or `zsh` explicitly when you need an override. Bash uses `~/.bash_aliases` and `~/.bashrc`; Zsh uses `~/.zsh_aliases` and `.zshrc` in `$ZDOTDIR` or your home directory. Before editing an existing alias or startup file, Alias Lens creates a private backup and a timestamped alias revision. It replaces an existing alias named `al` because Alias Lens uses that command. WSL follows the normal Bash path. On macOS, Bash setup respects `.bash_profile`, `.bash_login`, and `.profile` precedence. Fish, PowerShell, and Command Prompt are not supported yet.

During interactive setup, Alias Lens offers a small set of optional Git and file-listing aliases. It prints every alias and explains what it does before asking. The default answer is no. If you accept, setup skips existing alias names and commands rather than replacing them.

To update a local installation, pull the repository and rebuild the binary:

```bash
cd /path/to/al
git pull --ff-only
make install
```

## Build and development commands

Run `make` or `make help` to print the command guide. The main targets are:

| Command | What it does |
| --- | --- |
| `make fmt` | Rewrites every top-level Go source file with `gofmt`. |
| `make fmt-check` | Reports files that need formatting without changing them. |
| `make test` | Runs the Go test suite. |
| `make vet` | Runs Go's static analyzer to find suspicious code. |
| `make build` | Builds `alias-lens` at `/tmp/alias-lens-release`. |
| `make diff-check` | Checks the current Git diff for whitespace errors. |
| `make check` | Runs the non-modifying format check, tests, static analysis, build, and diff check used by CI. |
| `make install` | Runs the tests, builds the binary, and copies it to `~/.local/bin/alias-lens`. It does not run `alias-lens setup` or edit shell files. |

Override the output or install location on the command line:

```bash
make build OUTPUT=./alias-lens
make install PREFIX=/usr/local
```

Homebrew packaging is in progress. Release maintainers can follow [the release and Homebrew checklist](docs/RELEASING.md).

## What it does

- **Surfaces useful aliases automatically.** The idle screen suggests safe shortcuts you may have forgotten.
- **Searches as quickly as you type.** Enter one letter to see every matching prefix—`g` shows all aliases beginning with `g`—or keep typing to search commands and descriptions.
- **Catches mistakes.** Fuzzy matching recognizes a misspelled alias and ranks likely corrections.
- **Explains every shortcut.** Alias Lens shows the command and a plain-language description derived from nearby comments and known command patterns.
- **Edits aliases safely.** Add, update, and delete entries without leaving the terminal. Related commands are kept close together in the active alias file.
- **Checks alias health.** Find missing executables, risky destructive commands, duplicate definitions, and stale paths.
- **Fits your terminal.** Use every dark theme shipped with Codex, plus the original Phosphor and JetBrains Darcula presets.
- **Tracks aliases with Git.** Choose a writable GitHub, Bitbucket, or GitLab repository from the terminal, or connect an existing local dotfiles repository. Syncing commits only the configured alias file.
- **Offers an optional web view.** Run a local browser interface when useful; the terminal experience remains the default.

Alias Lens operates locally. It does not upload or execute the commands in your alias file.

If the active alias file does not exist, Alias Lens restores it from the configured repository. If the repository does not contain a copy, Alias Lens creates an empty file with mode `0600`.

## Command help

Run `al help` to see every command grouped by purpose. Use `al help COMMAND` or `al COMMAND --help` to see a command's syntax, file changes, sync behavior, and examples.

```bash
al help
al help sync
al repo --help
```

The help text marks commands that execute aliases, edit shell files, create Git commits, push to a remote, or enable automatic sync.

## Terminal controls

- `?`: open or close the keyboard guide
- `↑` / `↓`: select an alias
- `Enter`: close the TUI and execute the selected alias
- `Ctrl+A`: add an alias
- `Ctrl+E`: edit the selected alias
- `Ctrl+D`: delete the selected alias after confirmation
- `Ctrl+H`: show aliases with health warnings
- `Ctrl+F`: show tracked config files and their sync status
- `Ctrl+G`: commit aliases to the configured repository
- `Ctrl+T`: open the theme picker; moving through it previews each theme
- `Ctrl+R`: reload aliases and theme configuration
- `Esc`: exit

When the active alias file is empty, Alias Lens shows the file and detected shell instead of an empty search result. Press `Enter` or `Ctrl+A` to create the first alias.

## Use an alias from the picker

Install the detected shell integration once, then start a new shell:

```bash
al setup
```

Run `al`, select an alias, and press `Enter`. Alias Lens closes and executes the selected alias by name. It does not place the underlying command on the terminal line. The TUI labels this action as `execute` so the alias does not run by surprise. `al use QUERY` does the same thing with an initial search. Press `Ctrl+G` at a Bash or Zsh prompt to insert an alias at the current cursor without running it.

Scripts can use `al pick` to return an alias name. Add `--command` to return its command instead:

```bash
al pick
al pick --command git
```

## Turn repeated commands into aliases

`al suggest` reads only the active shell's local history: `~/.bash_history` for Bash or `~/.zsh_history` for Zsh. It understands Zsh's extended-history prefix and lists long commands that appear at least three times and do not already have aliases.

```bash
al suggest
al suggest add 1
al suggest add 1 myname
```

Alias Lens does not suggest commands that commonly contain credentials, such as `ssh`, `curl`, or `export` commands.

## Add metadata and shell functions

Alias Lens discovers common Bash and Zsh function syntax as well as aliases. It labels functions in search results. Add structured metadata immediately above an alias or function:

```bash
# al: tags=git,work platforms=linux,wsl favorite=true
# Open the current branch in the browser
gopen() {
  gh browse
}
```

Tags act as collections and become search terms. Favorites rank first on the suggestion screen. An unsupported `platforms` value appears in alias health. Update metadata without editing the file directly:

```bash
al meta gs tags=git,daily favorite=true
al meta docker-clean platforms=linux
```

Supported metadata fields are `tags`, `collections`, `platforms`, and `favorite`.

Add generated comments to aliases that do not already have descriptions:

```bash
al describe
```

This adds action-oriented descriptions and improves older generated comments that merely repeat the command. It preserves custom comments and writes the alias file once after creating a backup and private revision.

New aliases are inserted beside commands with the same tool and subcommand. Before every write, Alias Lens saves the previous file beside it with the `.alias-lens.bak` suffix.

## Themes

Alias Lens includes the 27 dark theme families shipped with Codex: Absolutely, Ayu, Catppuccin, Codex, Dracula, Everforest, GitHub, Gruvbox, Linear, Lobster, Material, Matrix, Monokai, Night Owl, Nord, Notion, One, Oscurange, Raycast, Rose Pine, Sentry, Solarized, Temple, Tokyo Night, Vercel, VS Code Plus, and Xcode. Phosphor and JetBrains Darcula remain available as Alias Lens originals.

List preset names or select one directly:

```bash
al theme
al theme tokyo-night
```

`Ctrl+T` opens the theme picker. Move through the list to preview themes immediately, press `Enter` to save, or press `Esc` to restore the previous theme. You can still override individual color fields in `~/.config/alias-lens/theme.json`.

## Git repository tracking

GitHub is enabled by default. Authenticate GitHub CLI once, then open the interactive picker:

```bash
gh auth login
al repo
```

Add Bitbucket Cloud or GitLab to the same picker through the provider configuration layer:

```bash
# Bitbucket Cloud: repeat this for each workspace you want to search
al config provider bitbucket YOUR_WORKSPACE
export BITBUCKET_API_TOKEN="..."

# GitLab.com, or pass a self-managed GitLab hostname as the final argument
al config provider gitlab
export GITLAB_TOKEN="..."

# See the effective configuration. Tokens are never included.
al config
```

Bitbucket tokens need repository read access to discover repositories. GitLab tokens need API read access. Git push permissions remain controlled by the credentials used by Git itself.

The picker requests repositories visible to your enabled providers and keeps only non-archived repositories where you can push. It clones selections beneath `~/.local/share/alias-lens/repos/<provider>/`.

Clone transport defaults to `auto`: Alias Lens checks for an authenticated, non-interactive SSH connection and prefers the SSH URL when available. Otherwise it uses the provider's HTTPS or CLI flow. Override this per provider when needed:

```bash
al config protocol github ssh
al config protocol bitbucket https
al config protocol gitlab auto
```

You can limit the picker to one provider with `al repo github`, `al repo bitbucket`, or `al repo gitlab`. Disable a connection with `al config disable PROVIDER`.

You can instead connect an existing local Git repository:

```bash
al repo /path/to/dotfiles
```

Sync changes locally or explicitly push them:

```bash
al sync
al sync --push
```

`al sync` copies and commits only the `alias_file` configured in `~/.config/alias-lens/config.json`. Manual sync requires the explicit `--push` flag. Automatic sync pushes after a safe reconciliation when you enable it. Before any push, Alias Lens scans for likely credentials and blocks the push when it finds one. Run `al scan` to see finding types and line numbers without printing secret values.

## Keep aliases synchronized automatically

Choosing a repository enables automatic sync. `al setup` starts one background worker from the shell integration. The worker checks every 15 seconds by default. A lock prevents multiple shells from starting competing workers.

```bash
al autosync status
al autosync disable
al autosync enable
al watch
```

`al watch` runs one reconciliation cycle in the foreground. Each cycle pulls with Git's fast-forward-only mode before it considers a push. Local-only changes are scanned, committed, and pushed. Remote-only changes create a revision and then update the active alias file atomically.

Track other configuration files explicitly. Alias Lens rejects environment files, keys, and credential-shaped filenames:

```bash
al track ~/.gitconfig
al track ~/.config/starship.toml shell/starship.toml
al untrack ~/.gitconfig
```

Each tracked file has independent hashes, backups, and private conflict copies. Alias Lens stages and commits only the file that changed.

If both files changed since the last successful cycle, Alias Lens does not overwrite either version. It stores private conflict copies in `~/.local/state/alias-lens/conflicts/`, reports `conflict` in `al autosync status`, and waits until you make the local and tracked files match. Network failures report `offline` and retry on the next cycle.

Pull and compare aliases across machines:

```bash
al diff
al sync --pull
```

Pull uses Git's fast-forward-only mode. Alias Lens imports remote-only aliases and preserves local-only aliases. If the same name has different commands, it stops and asks you to inspect both values with `al diff`.

Your repository may contain other configuration files. Alias Lens does not stage or commit them.

## Recover an earlier version

Every edit stores the previous file in the private revision directory at `~/.local/share/alias-lens/revisions/`. The alias file's `.alias-lens.bak` companion still holds the most recent backup.

```bash
al history
al undo
al undo 20260909T120000.000000000Z
```

Restoring a revision saves the current file as another revision first.

## Diagnose the installation

Run `al doctor` to check the executable, active alias-file loading, shell integration, Git, the sync repository, provider credentials, and SSH fallback behavior.

Check the installed build with `al --version`.

## Optional browser view

Run `al --web` and open `http://127.0.0.1:8787`.

## Local files

| File | Purpose |
| --- | --- |
| `~/.bash_aliases` or `~/.zsh_aliases` | Alias source selected for Bash or Zsh |
| `<alias-file>.alias-lens.bak` | Most recent pre-edit backup |
| `~/.config/alias-lens/config.json` | Repository and sync settings |
| `~/.config/alias-lens/theme.json` | Selected theme and color overrides |
| `~/.local/share/alias-lens/revisions/` | Timestamped private alias revisions |
| `~/.local/state/alias-lens/sync-state.json` | Automatic sync hashes and status |
| `~/.local/state/alias-lens/conflicts/` | Private local and remote conflict copies |

These files are intentionally excluded from this project's version control.
