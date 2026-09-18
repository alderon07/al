# Local data and privacy

Alias Lens runs locally. It does not send aliases, history, usage, revisions, or configuration to an Alias Lens service. Provider and Git operations contact only the provider or remote that the user configured.

Run `al data paths` to print the paths for the active shell and current configuration.

| Path | Contents | Permissions | Removal |
| --- | --- | --- | --- |
| `~/.bash_aliases` or `~/.zsh_aliases` | User aliases, functions, descriptions, and `# al:` metadata | Existing mode; new files use `0600` | Edit or remove manually after `al setup --remove` |
| `<alias file>.alias-lens.bak` | Latest alias contents from before a write | `0600` | Remove manually |
| `~/.config/alias-lens/config.json` | Schema version, shell, repository path, provider settings without tokens, sync settings, and tracked paths | Directory `0700`, file `0600` | Remove manually after disabling sync |
| `~/.config/alias-lens/config.json.alias-lens.bak` | Configuration from before the last schema migration | `0600` | Remove manually |
| `~/.config/alias-lens/theme.json` | Selected theme | Directory `0700`, file `0600` | Remove manually to restore the default |
| `~/.local/share/alias-lens/usage.tsv` | Alias name and use timestamp | Directory `0700`, file `0600` | Run `al data clear-usage` |
| `~/.local/share/alias-lens/revisions/` | Timestamped alias-file copies made before edits | Directory `0700`, files `0600` | Run `al data clear-revisions` |
| `~/.local/share/alias-lens/repos/` | User-selected provider repositories cloned by Alias Lens | User-private data directory | Remove manually after disabling sync |
| `~/.local/state/alias-lens/` | Sync hashes, worker lock and log, tracked-file state, private conflict copies, and the onboarding-tour state | Directory `0700`, files `0600` | Disable sync, then remove manually |
| Bash or Zsh startup files | Marked Alias Lens loader and PATH blocks among user-owned settings | Existing mode | Run `al setup --remove` |

Provider credentials stay outside `config.json`. GitHub uses the `gh` credential store. GitLab and Bitbucket tokens come from their documented environment variables. Alias Lens does not record duration, exit status, current directory, or workspace.
