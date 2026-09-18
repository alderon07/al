# Alias Lens compatibility policy

Alias Lens stores a schema version in `~/.config/alias-lens/config.json`. It migrates older supported configurations before use and saves the previous file as `config.json.alias-lens.bak`. Alias Lens refuses to open a configuration written by a newer unsupported version.

## Stable commands and flags

Starting with 1.0, the following user commands keep their meaning throughout the 1.x series:

- Alias work: `pick`, `use`, `search`, `suggest`, `stats`, `export`, `import`, `meta`, and `describe`.
- Safety and recovery: `check`, `scan`, `history`, `undo`, `doctor`, and `setup`.
- Configuration and sync: `repo`, `config`, `data`, `track`, `untrack`, `sync`, `diff`, `autosync`, and `watch`.
- Interface and system commands: `theme`, `catalog shadow`, `shell-init`, `--web`, and `--version`.

The stable flags are `pick --command`, `search --json`, `stats --plain`, `export --format`, `export --period`, `export --output`, `import --apply`, `check --strict`, `setup --repair`, `setup --remove`, `sync --push`, `sync --pull`, `theme --check`, and `catalog shadow --shell` and `--json`. A minor release can add a command, flag, accepted value, or optional output field. It cannot change the meaning of an existing one.

`shell-entry`, `record-use`, `pick --execute`, and `watch --ensure` are integration commands. They can change when `al setup --repair` installs a matching integration. The browser HTML, CSS, JavaScript, and local API are not part of the 1.0 contract.

## Stable metadata and exports

Alias Lens 1.x reads the `# al:` fields `tags`, `collections`, `category`, `platforms`, and `favorite`. A minor release can add a field. It cannot reuse an existing field name for a different value.

Alias exports contain top-level `kind`, `exported_at`, and `aliases` fields. Each alias can contain `name`, `command`, `description`, `category`, `type`, `tags`, `platforms`, and `favorite`. Stats exports add the top-level `period` field and the per-alias `count` field. CSV header names and YAML field names follow the same contract. Minor releases can add fields. They do not remove or rename fields in 1.x.

## Stable configuration

Configuration schema version 1 contains `version`, `repository`, `alias_file`, `shell`, `providers`, `auto_sync`, and `tracked_files`. Provider entries contain `enabled`, `host`, `protocol`, and `workspaces`. Automatic sync contains `enabled` and `interval_seconds`. Tracked-file entries contain `source` and `repository_path`.

Migrations preserve repository, shell, provider, automatic-sync, and tracked-file settings. Alias Lens writes the old bytes to `config.json.alias-lens.bak` before it replaces the configuration. It refuses a schema version newer than the installed binary supports.

`al setup --repair` can replace generated shell integration. It does not change user aliases. `al setup --remove` removes generated integration only. It keeps aliases and Alias Lens data.

## Breaking changes

A 1.x release deprecates a command or flag before removal and keeps the replacement available for at least one minor release. Removal, field renaming, and semantic changes require a new major version. The release notes name every breaking change and its replacement.

If a future configuration cannot migrate without losing information, Alias Lens stops and leaves both the live file and its backup unchanged. It never guesses at a destructive migration.
