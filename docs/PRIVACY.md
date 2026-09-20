# Local data and privacy

Alias Lens runs locally. It does not send aliases, history, usage, revisions, or configuration to an Alias Lens service. Provider and Git operations contact only the provider or remote that the user configured.

Run `al data paths` to print the paths for the active shell and current configuration.

| Path | Contents | Permissions | Removal |
| --- | --- | --- | --- |
| `~/.bash_aliases` or `~/.zsh_aliases` | User aliases, functions, descriptions, and `# al:` metadata | Existing mode; new files use `0600` | Edit or remove manually after `al setup --remove` |
| `<alias file>.alias-lens.bak` | Latest alias contents from before a write | `0600` | Remove manually |
| `~/.config/alias-lens/config.json` | Data format version, shell, local machine profiles, shortcut choice, repository path, provider settings without tokens, sync settings, and tracked paths | Directory `0700`, file `0600` | Remove manually after disabling sync |
| `~/.config/alias-lens/catalog.json` | Portable commands, native Bash or Zsh definitions, descriptions, tags, platforms, and conditions | `0600` | Remove manually after catalog rollback |
| `~/.config/alias-lens/completion.bash` and `completion.zsh` | Generated command and option suggestions; no command implementations | `0600` | Use `al completion remove bash` or `al completion remove zsh` |
| `~/.config/alias-lens/backups/` | Exact settings and catalog bytes saved before a planned change | Directory `0700`, files `0600` | Remove after confirming rollback is not needed |
| `~/.config/alias-lens/transactions/` | Private journals used to recover an interrupted planned change | Directory `0700`, files `0600` | Let the next changing command recover them; do not edit them |
| `~/.config/alias-lens/theme.json` | Selected theme | Directory `0700`, file `0600` | Remove manually to restore the default |
| `~/.local/share/alias-lens/usage.tsv` | Alias name and use timestamp | Directory `0700`, file `0600` | Run `al data clear-usage` |
| `~/.local/share/alias-lens/revisions/` | Timestamped alias-file copies made before edits | Directory `0700`, files `0600` | Run `al data clear-revisions` |
| `~/.local/share/alias-lens/repos/` | User-selected provider repositories cloned by Alias Lens | User-private data directory | Remove manually after disabling sync |
| `~/.local/state/alias-lens/` | Legacy sync hashes, worker lock and log, tracked-file state, private conflict copies, and the onboarding-tour state | Directory `0700`, files `0600` | Disable sync, then remove manually |
| Bash or Zsh startup files | Marked Alias Lens loader and PATH blocks among user-owned settings | Existing mode | Run `al setup --remove` |

Provider credentials stay outside `config.json`. GitHub uses the `gh` credential store. GitLab and Bitbucket tokens come from their documented environment variables. Alias Lens does not record duration, exit status, current directory, or workspace.
