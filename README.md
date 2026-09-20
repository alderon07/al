<p align="center">
  <img src="docs/assets/alias-lens-logo.svg" alt="Alias Lens logo: a terminal prompt inside a magnifying lens" width="180">
</p>

<h1 align="center">Alias Lens</h1>

<p align="center"><strong>Your shortcuts should save keystrokes, not become trivia.</strong></p>

<p align="center">
  Find, run, explain, edit, and sync Bash and Zsh aliases without leaving the terminal.
</p>

Aliases are supposed to save time. Then you forget whether checkout is `gco`, `gc`, or the command you made at 2 AM. Alias Lens gives them a searchable home.

```text
$ al search git
ALIAS  COMMAND                          DESCRIPTION
gs     git status                       Show repository status
gl     git log --oneline --graph        Show the compact commit graph
gcb    git checkout -b                  Create and switch to a branch
```

Press `Ctrl+G` at an empty Zsh or Bash 4+ prompt to open the terminal interface. Stock macOS Bash 3.2 users run `al` instead. Pick an alias, press `Enter`, and Alias Lens prints the alias before it runs. No mystery commands.

## Install it

Alias Lens supports Bash and Zsh on Linux, WSL, and macOS.

With Go 1.24.2 or newer:

```bash
go install github.com/alderon07/al/cmd/alias-lens@latest
alias-lens setup
```

Or download the archive for your system from [GitHub Releases](https://github.com/alderon07/al/releases), extract it, and install the binary:

```bash
mkdir -p "$HOME/.local/bin"
install -m 0755 alias-lens "$HOME/.local/bin/alias-lens"
"$HOME/.local/bin/alias-lens" setup
```

To build from source, you need Make and Go 1.24.2 or newer:

```bash
git clone https://github.com/alderon07/al.git
make -C al install
"$HOME/.local/bin/alias-lens" setup
```

Setup keeps a user-installed binary on `PATH`, including after a WSL restart. Start a new shell, then check the installation:

```bash
al --version
```

`al setup` detects your shell. Use `al setup bash` or `al setup zsh` to choose one yourself. Fish, PowerShell, and Command Prompt are not supported yet.

Setup can offer a few optional Git and file-listing aliases. It shows each command before asking and never replaces an existing alias.

To update a source installation:

```bash
cd /path/to/al
git pull --ff-only
make install
```

If an older WSL installation disappears after a restart, repair it once after updating:

```bash
"$HOME/.local/bin/alias-lens" setup --repair bash
```

To repair or remove the generated shell integration later:

```bash
al setup --repair
al setup --remove
```

Removal keeps your aliases, Alias Lens configuration, revisions, and sync repositories.

See [Local data and privacy](docs/PRIVACY.md) for every file Alias Lens creates and the commands that remove usage data or revisions.

Alias Lens is available under the [Apache License 2.0](LICENSE).

## Find the shortcut before you forget it

Open Alias Lens with `Ctrl+G` on Zsh or Bash 4+, or run `al` on any supported shell. Stock macOS Bash 3.2 keeps the normal Readline `Ctrl+G` cancellation behavior. Search by alias, command, description, category, or tag. Fuzzy search still finds a likely match when your memory is one letter off.

The idle screen brings useful aliases back into view. Select one and press `Enter` to run it in the current shell. Press `Tab` to return the alias to the prompt without running it, then edit it or add arguments. New and edited aliases work without restarting the shell.

Use the CLI when you already know what you want:

```bash
al search git
al search --json daily
al use docker
```

## Use the keys that matter

Press `?` inside the TUI for the full searchable keyboard guide.

| Key | Action |
| --- | --- |
| `↑` and `↓` | Select an alias |
| `Enter` | Use the selected alias |
| `Tab` | Return the selected alias to the prompt without running it |
| `Ctrl+A` | Add an alias |
| `Ctrl+E` | Edit the selected alias |
| `Ctrl+D` | Delete the selected alias after confirmation |
| `F2` or `Ctrl+S` | Open usage stats |
| `F3` | Customize the TUI appearance and footer |
| `Ctrl+H` | Show alias health warnings |
| `Ctrl+F` | Show tracked files and sync status |
| `Ctrl+G` | Sync while the TUI is open |
| `Ctrl+T` | Preview and select a theme |
| `Ctrl+Z` | Browse and restore revisions |
| `Esc` | Exit |

The page shortcuts work from the alias list, help, stats, footer settings, themes, revisions, sync status, and alias health. Press the current page's shortcut again to return to the alias list.

Some terminals reserve `Ctrl+S` for flow control, so use `F2` if `Ctrl+S` does not reach Alias Lens.

Alias Lens chooses a familiar keyboard style for your computer: Windows on Windows and WSL, macOS on macOS, and Linux on Linux. See the active style or choose the one you already know:

```bash
al shortcuts
al shortcuts auto
al shortcuts macos
al shortcuts windows
al shortcuts linux
```

You can use any style on any computer. Run `al shortcuts auto` to remove a saved choice and return to the style for the current computer. The macOS style shows Command shortcuts with terminal-safe fallbacks because many terminals keep Command keys for themselves. Copy and paste remain terminal features. They are commonly `Cmd+C` and `Cmd+V` on macOS or `Ctrl+Shift+C` and `Ctrl+Shift+V` in Windows and Linux terminals. `Ctrl+C` still cancels or closes Alias Lens when the terminal sends it to the app.

On Zsh and Bash 4+, `Ctrl+G` keeps its normal cancel behavior when the prompt contains text. Bash 3.2 does not install the picker binding; run `al` instead. Set `ALIAS_LENS_NOBIND=1` before the shell integration loads if you do not want the key binding.

## Add context to cryptic names

Alias Lens edits the command, name, description, category, and tags from one form. Tags become search terms, and favorites appear first in suggestions.

```bash
al meta gs tags=git,daily favorite=true
al meta docker-clean platforms=linux
```

Add descriptions to aliases that do not have one:

```bash
al describe
```

Alias Lens preserves comments that you wrote yourself.

It also finds common Bash and Zsh functions. Add metadata above an alias or function when you prefer to edit the file:

```bash
# al: tags=git,work platforms=linux,wsl favorite=true
# Open the current branch in the browser
gopen() {
  gh browse
}
```

## Let history do the remembering

`al suggest` finds long commands that you have run at least three times:

```bash
al suggest
al suggest add 1
al suggest add 1 myname
```

Suggestions come only from the active shell's local history. Commands that commonly contain credentials, including `ssh`, `curl`, and `export`, are excluded.

Press `F2` in the TUI or run `al stats` to see alias usage:

```bash
al stats
al stats today
al stats --plain week
```

Counts come from your active terminal history. Time periods need shell-history timestamps.

Export aliases or stats as JSON, YAML, or CSV:

```bash
al export aliases --format yaml
al export stats --format csv --period week --output weekly-aliases.csv
```

Preview aliases before you import them:

```bash
al import ~/Downloads/aliases.sh
al import ~/Downloads/aliases.sh --apply
```

The preview reports syntax errors, duplicate names, duplicate commands, conflicts, skipped aliases, and every planned addition. `--apply` stops on a syntax error or name conflict. It adds the accepted aliases in one backed-up write.

## Catch broken aliases before they catch you

Run the checker without sourcing or executing the alias file:

```bash
al check
al check --strict
```

It reports malformed aliases, duplicate names, invalid metadata, likely secrets, missing executables, and syntax errors. Diagnostics show line numbers without printing secret values.

Aliases that contain risky commands open a review screen before execution. The screen explains why Alias Lens stopped and shows the full command.

## Make it look like your terminal

Open the theme picker with `Ctrl+T`, or choose a theme by name:

```bash
al theme
al theme tokyo-night
al theme --check
```

Moving through the picker previews each theme. Press `Enter` to save it or `Esc` to keep the previous theme.

Press `F3` in the TUI to personalize the brand name, brand art, interface markers, footer message, footer icon, and footer alignment. Use Left and Right to choose between full, compact, text-only, or hidden brand art; symbol, ASCII, or no markers; and left, center, or right footer alignment. The default symbol style uses ordinary Unicode characters and does not require a Nerd Font.

You can also change the footer message and icon from the command line:

```bash
al config footer-message 'Built with {icon} by Sam'
al config footer-icon spark
```

`{icon}` marks the icon position. Built-in choices include `heart`, `spark`, `brand`, `alias`, `command`, `stats`, `sync`, and `theme`. Use `none` to hide it. Prefix one emoji with `emoji:` to use it as the icon. A custom pixel icon is a quoted 4×2 bitmap with `#` for filled pixels and `.` for empty pixels:

```bash
al config footer-icon 'emoji:🚀'
al config footer-icon '#..#/.##.'
al config footer-reset
```

## Sync aliases without babysitting Git

Connect a repository from GitHub, GitLab, or Bitbucket:

```bash
al repo github
al repo gitlab
al repo bitbucket
```

Alias Lens guides you through sign-in and shows repositories where you can push. GitHub uses GitHub CLI, GitLab uses GitLab CLI, and Bitbucket asks for a scoped API token in a hidden prompt. Provider tokens are not written to the Alias Lens configuration.

To use a repository that already exists on your computer:

```bash
al repo /path/to/dotfiles
```

Sync locally or push the change:

```bash
al sync
al sync --push
```

Choosing a repository enables automatic sync. Check or change it with:

```bash
al autosync status
al autosync disable
al autosync enable
```

Alias Lens syncs only the active alias file unless you explicitly track another file:

```bash
al track ~/.gitconfig
al track ~/.config/starship.toml shell/starship.toml
al untrack ~/.gitconfig
```

Environment files, keys, and credential-shaped filenames cannot be tracked. Alias Lens scans the alias file for likely secrets before every push.

If the local and remote copies both changed, Alias Lens keeps the live file untouched and saves private conflict copies. Run `al diff` to compare them.

## Undo the oops

Alias Lens saves a private revision before each edit and before it restores another version.

```bash
al history
al undo
al undo 20260909T120000.000000000Z
```

The `.alias-lens.bak` file beside your alias file contains the latest backup.

## Keep private commands private

- Searching, explaining, checking, and syncing never execute an alias.
- Running an alias always requires an explicit selection or command.
- History, revisions, conflict copies, and exports stay on your computer.
- Git sync pushes only the active alias file and files that you chose with `al track`.
- Secret scans run before every remote push.

Inspect or clear local data without removing aliases or configuration:

```bash
al data paths
al data clear-usage
al data clear-revisions
```

Alias Lens honors `NO_COLOR`. If `TERM=dumb`, it prints a plain alias list instead of starting the interactive interface. See [Local data and privacy](docs/PRIVACY.md) for file contents, permissions, and removal steps.

## Fix a strange installation

Run the diagnostic command:

```bash
al doctor
```

It checks the installed binary, shell integration, active alias file, Git connection, provider sign-in, and sync repository.

Use the built-in help for command syntax and examples:

```bash
al help
al help sync
al repo --help
```

If you prefer a browser view, run `al --web` and open `http://127.0.0.1:8787`. The terminal interface remains the default.
