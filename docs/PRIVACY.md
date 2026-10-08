# Local data and privacy

Alias Lens runs locally. It does not send aliases, history, usage, revisions, or configuration to an Alias Lens service. Provider and Git operations contact only the provider or remote that the user configured.

Run `al data paths` to print the paths for the active shell and current configuration.

| Path | Contents | Permissions | Removal |
| --- | --- | --- | --- |
| `~/.bash_aliases` or `~/.zsh_aliases` | User aliases, functions, descriptions, and `# al:` metadata | Existing mode; new files use `0600` | Edit or remove manually after `al setup --remove` |
| `<alias file>.alias-lens.bak` | Latest alias contents from before a write | `0600` | Remove manually |
| `~/.config/alias-lens/config.json` | Data format version, shell, local machine profiles, shortcut profile and overrides, repository path, provider settings without tokens, sync settings, and tracked paths | Directory `0700`, file `0600` | Remove manually after disabling sync |
| `~/.config/alias-lens/catalog.json` | Portable commands, native Bash or Zsh definitions, descriptions, tags, platforms, and conditions | `0600` | Remove manually after catalog rollback |
| `~/.config/alias-lens/generated/<shell>/` | Immutable shell definitions, canonical manifest, exact installed entries, profiles, approvals, native-input hash and local executable paths | Directories `0700`, files `0600` | Roll back the shell before removing referenced packages |
| `~/.config/alias-lens/catalog-sync.json` | Enrolled repository/path, saved semantic base, pinned revision/blob, and uncertain push intent | `0600` | Resolve pending push and disable sync before removing |
| `~/.config/alias-lens/catalog-conflicts/` | Separate base/local/remote catalog copies and semantic conflict metadata | Directories `0700`, files `0600` | Remove resolved bundles after confirming no reference remains |
| `~/.config/alias-lens/contexts.json` | Explicit alias marks, local project or folder paths, and command digests | Directory `0700`, file `0600` | Run `al context remove NAME --all` for each marked alias |
| `~/.config/alias-lens/completion.bash` and `completion.zsh` | Generated command and option suggestions; no command implementations | `0600` | Use `al completion remove bash` or `al completion remove zsh` |
| `~/.config/alias-lens/backups/` | Exact settings and catalog bytes saved before a planned change | Directory `0700`, files `0600` | Remove after confirming rollback is not needed |
| `~/.config/alias-lens/transactions/` | Private journals used to recover an interrupted planned change | Directory `0700`, files `0600` | Let the next changing command recover them; do not edit them |
| `~/.config/alias-lens/theme.json` | Selected theme | Directory `0700`, file `0600` | Remove manually to restore the default |
| `~/.local/share/alias-lens/usage.tsv` | Alias name and use timestamp | Directory `0700`, file `0600` | Run `al data clear-usage` |
| `~/.local/share/alias-lens/revisions/` | Timestamped alias-file copies made before edits | Directory `0700`, files `0600` | Run `al data clear-revisions` |
| `~/.local/share/alias-lens/repos/` | User-selected provider repositories cloned by Alias Lens | User-private data directory | Remove manually after disabling sync |
| `~/.local/state/alias-lens/catalog-installed.json` | Per-shell generation/rollback identity and every startup route | `0600` | Use `al catalog rollback --shell SHELL` |
| `~/.local/state/alias-lens/native-approvals.json` | Exact native content/name approval keys and UTC approval times | `0600` | Remove manually to require new approval |
| `~/.local/state/alias-lens/adoptions.json` | Exact source ranges, native fallback bytes and ownership hashes | `0600` | Roll back before removing ownership records |
| `~/.local/state/alias-lens/rollback/` | Original native and startup enrollment bytes and existence records | Directories `0700`, files `0600` | Preserve while a shell is installed; ordinary revision clearing keeps these |
| `~/.local/state/alias-lens/catalog-snapshots/` | Canonical catalog snapshots referenced by installed packages | Directory `0700`, files `0600` | Preserve referenced snapshots |
| `~/.local/state/alias-lens/native-snapshots/` | Exact native input used to verify an installed package during renewed review | Directory `0700`, files `0600` | Preserve while its installed generation is referenced |
| `~/.local/state/alias-lens/catalog-revisions/` | Catalog copies made before editing | Directory `0700`, files `0600` | Clear ordinary revisions; keep enrollment baselines |
| `~/.local/state/alias-lens/workflows/` | Durable multi-target journal, inverse payloads and recovery progress | Directories `0700`, files `0600` | Let the next mutation recover; preserve incomplete journals |
| `~/.local/state/alias-lens/mutation.lock` | Persistent OS-backed lock for every managed writer | `0600` | Keep while Alias Lens is installed |
| `~/.local/state/alias-lens/watch-worker.lock` | Persistent OS-backed lock preventing duplicate background workers; each cycle also takes the mutation lock | `0600` | Keep while Alias Lens is installed |
| `~/.local/state/alias-lens/catalog-stages/` | Operation-owned remote staging intents and pinned directory identities | Directory `0700`, files `0600` | Let recovery clean proven owned artifacts; preserve ambiguous stages |
| `~/.local/share/alias-lens/catalog-repos/` | Restricted managed repository containing the enrolled catalog path | Private directories and catalog | Disable enrollment before manual removal |
| `~/.local/state/alias-lens/` | Legacy sync hashes, tracked-file state, private conflict copies, and the onboarding-tour state | Directory `0700`, files `0600` | Disable sync, then remove manually |
| Bash or Zsh startup files | Marked Alias Lens loader and PATH blocks among user-owned settings | Existing mode | Run `al setup --remove` |

Catalog installation adds private artifacts under `~/.config/alias-lens/generated/<shell>/` and `~/.local/state/alias-lens/`. Generation manifests contain exact installed implementations, approvals, machine profiles, native-input hashes, and local executable paths. The state directory holds approval and ownership records, enrollment baselines, startup routes, installed records, snapshots, staging intents, and multi-target recovery journals. Catalog sync bases, conflict bundles, and push intents live under the configuration directory. Directories use `0700`; files use `0600`. Managed mutations share the persistent `mutation.lock` in the state directory.

Keep artifacts referenced by an active generation, rollback record, sync base, or incomplete transaction. Run catalog rollback before removing catalog installation data. Clearing ordinary revisions must preserve catalog enrollment baselines. Native fallbacks and all installation artifacts are local; automatic catalog sync owns only the explicitly enrolled catalog path.

Provider credentials stay outside `config.json`. GitHub uses the `gh` credential store. GitLab and Bitbucket tokens come from their documented environment variables. Alias Lens does not record duration, exit status, or a history of where commands ran. `al context add` saves only the paths the user explicitly marks, and those marks are never synced.
