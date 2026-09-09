# al — Alias Lens

**Remember less. Move faster.**

Alias Lens is a terminal-first productivity tool for discovering, understanding, and maintaining the shell aliases in `~/.bash_aliases`. Launch it with two letters from any directory:

```bash
al
```

It opens an interactive Bubble Tea interface directly in the terminal. A browser is never required.

## What it does

- **Surfaces useful aliases automatically.** The idle screen suggests safe shortcuts you may have forgotten.
- **Searches as quickly as you type.** Enter one letter to see every matching prefix—`g` shows all aliases beginning with `g`—or keep typing to search commands and descriptions.
- **Catches mistakes.** Fuzzy matching recognizes a misspelled alias and ranks likely corrections.
- **Explains every shortcut.** Alias Lens shows the command and a plain-language description derived from nearby comments and known command patterns.
- **Edits aliases safely.** Add, update, and delete entries without leaving the terminal. Related commands are kept close together in `.bash_aliases`.
- **Checks alias health.** Find missing executables, risky destructive commands, duplicate definitions, and stale paths.
- **Fits your terminal.** Switch among Phosphor, JetBrains Darcula, Dracula, and Catppuccin Mocha themes.
- **Tracks aliases with Git.** Choose a writable GitHub repository from the terminal or connect an existing local dotfiles repository. Syncing commits only the configured alias file.
- **Offers an optional web view.** Run a local browser interface when useful; the terminal experience remains the default.

Alias Lens operates locally. It does not upload or execute the commands in your alias file.

## Terminal controls

- `↑` / `↓`: select an alias
- `Enter`: reveal its full command
- `Ctrl+A`: add an alias
- `Ctrl+E`: edit the selected alias
- `Ctrl+D`: delete the selected alias after confirmation
- `Ctrl+H`: show aliases with health warnings
- `Ctrl+G`: commit aliases to the configured repository
- `Ctrl+T`: switch themes
- `Ctrl+R`: reload aliases and theme configuration
- `Esc`: exit

New aliases are inserted beside commands with the same tool and subcommand. Before every write, Alias Lens saves the previous file as `~/.bash_aliases.alias-lens.bak`.

## Themes

Built-in themes are Phosphor, JetBrains Darcula, Dracula, and Catppuccin Mocha. `Ctrl+T` cycles through them and saves the selection to `~/.config/alias-lens/theme.json`. You can also set `preset` to `phosphor`, `darcula`, `dracula`, or `catppuccin`. Add any color field from `theme.go` to override that part of the preset.

## Git repository tracking

Authenticate GitHub CLI once, then open the interactive picker:

```bash
gh auth login
al repo
```

The picker requests repositories visible to your account, keeps only non-archived repositories where you have push access, and clones your selection into `~/.local/share/alias-lens/repos/`. You can instead connect an existing local Git repository:

```bash
al repo /path/to/dotfiles
```

Sync changes locally or explicitly push them:

```bash
al sync
al sync --push
```

`al sync` copies and commits only the `alias_file` configured in `~/.config/alias-lens/config.json`. Pushing requires the explicit `--push` flag. Your repository may contain any other configuration files.

## Optional browser view

Run `al --web` and open `http://127.0.0.1:8787`.

## Build from source

Alias Lens requires Go 1.24 or newer.

```bash
git clone https://github.com/alderon07/al.git
cd al
go test ./...
go build -o al .
install -Dm755 al ~/.local/bin/alias-lens
```

Add this line to `~/.bash_aliases`, then start a new shell or run `source ~/.bash_aliases`:

```bash
alias al="$HOME/.local/bin/alias-lens"
```

## Local files

| File | Purpose |
| --- | --- |
| `~/.bash_aliases` | Alias source managed by the app |
| `~/.bash_aliases.alias-lens.bak` | Most recent pre-edit backup |
| `~/.config/alias-lens/config.json` | Repository and sync settings |
| `~/.config/alias-lens/theme.json` | Selected theme and color overrides |

These files are intentionally excluded from this project's version control.
