# Alias Lens compatibility policy

Alias Lens stores a schema version in `~/.config/alias-lens/config.json`. Before 1.0, only the current schema is supported. Alias Lens refuses older and newer formats without changing the file.

## Stable commands and flags

Starting with 1.0, the following user commands keep their meaning throughout the 1.x series:

- Alias work: `pick`, `use`, `search`, `context`, `suggest`, `stats`, `export`, `import`, `meta`, and `describe`.
- Safety and recovery: `check`, `scan`, `history`, `undo`, `doctor`, and `setup`.
- Configuration and sync: `repo`, `config`, `data`, `track`, `untrack`, `sync`, `diff`, `autosync`, and `watch`.
- Interface and system commands: `status`, `plan`, `theme`, `shortcuts`, `catalog preview`, `catalog import`, `catalog shadow`, `catalog diff`, `completion`, `shell-init`, `--web`, and `--version`.

The stable flags are `pick --command`, `search --json`, `search --global`, `context add --repo`, `context add --directory`, `context remove --all`, `stats --plain`, `export --format`, `export --period`, `export --output`, `import --apply`, `check --strict`, `setup --repair`, `setup --remove`, `sync --push`, `sync --pull`, `theme --check`, `status --json`, `catalog shadow --shell`, `catalog shadow --json`, `catalog diff --json`, `catalog diff --show-code`, `catalog diff --web`, `catalog diff --from`, and `catalog diff --shell`. A minor release can add a command, flag, accepted value, or optional output field. It cannot change the meaning of an existing one.

`shell-entry`, `catalog-loader`, `record-use`, `pick --execute`, `completion-candidates`, and `watch --ensure` are integration commands. They can change when Alias Lens installs matching shell integration. The browser HTML, CSS, JavaScript, and local API are not part of the 1.0 contract.

## Catalog contracts

Catalog and application configuration remain at format 2. Public status, operation-plan, and semantic-diff JSON remain at version 1. Active catalog installation uses `bash/v2` and `zsh/v2`; shadow-mode reports retain their existing renderer meanings and outputs.

Native approval record format 2 binds entry ID, shell, name, kind, implementation hash, and renderer. Generation, installed, ownership, and rollback records have independently versioned private formats. Multi-target workflow journals use format 2, while existing private-file journals retain format 1 recovery. Unsupported formats are refused without rewriting them.

The guided commands are `init SOURCE` and `catalog enable`, with optional `--shell`, `--startup-path`, and `--apply`; init also accepts `--catalog-path`. Explicit startup enrollment resolves routes that static discovery cannot prove. Inspection and recovery retain `catalog review`, `catalog approve NAME`, `catalog adopt NAME`, `catalog rollback`, and `catalog recover`. `al plan [--json]` previews enablement, rollback, and init without applying changes. `sync --catalog` selects catalog synchronization independently of the configured legacy shell. See the [storage](acceptance/CATALOG_STORAGE.md), [transaction](acceptance/CATALOG_TRANSACTION.md), and [shell runtime](acceptance/CATALOG_SHELL_RUNTIME.md) contracts for the exact private formats and accepted startup grammar.

Native catalog enrollment accepts a bounded grammar for Bash and Zsh declarations. Command and process substitutions, ANSI-C quoted strings, unquoted brace contexts, braced parameter expansions such as `${HOME}`, escaped command newlines, here-strings, and arithmetic involving variables are unsupported and require simplifying the definition before enrollment. Ordinary `$HOME` and quoted literal braces remain supported.

## Stable metadata and exports

Alias Lens 1.x reads the `# al:` fields `tags`, `collections`, `category`, `platforms`, and `favorite`. A minor release can add a field. It cannot reuse an existing field name for a different value.

Alias exports contain top-level `kind`, `exported_at`, and `aliases` fields. Each alias can contain `name`, `command`, `description`, `category`, `type`, `tags`, `platforms`, and `favorite`. Stats exports add the top-level `period` field and the per-alias `count` field. CSV header names and YAML field names follow the same contract. Minor releases can add fields. They do not remove or rename fields in 1.x.

## Stable configuration

Configuration schema version 2 contains `version`, `repository`, `alias_file`, `shell`, `profiles`, `shortcut_profile`, `shortcuts`, `providers`, `auto_sync`, `tracked_files`, `appearance`, and `footer`. Provider entries contain `enabled`, `host`, `protocol`, and `workspaces`. Automatic sync contains `enabled` and `interval_seconds`. Tracked-file entries contain `source` and `repository_path`.

Tracked sources support owned symlinks when the target and its parent directories pass source trust checks. Unsafe file or parent permissions, write-granting macOS ACLs, unverified ACL metadata, and hardlinks are refused. Source trust changes do not prevent settings from loading or removing the enrollment with `al untrack FILE`.

Alias Lens refuses any configuration schema other than the current version. This keeps pre-1.0 development formats out of the runtime. A future released migration must preserve user settings and back up the exact original bytes.

`al setup --repair` can replace generated shell integration. It does not change user aliases. `al setup --remove` removes generated integration only. It keeps aliases and Alias Lens data.

## Breaking changes

A 1.x release deprecates a command or flag before removal and keeps the replacement available for at least one minor release. Removal, field renaming, and semantic changes require a new major version. The release notes name every breaking change and its replacement.

If a future configuration cannot migrate without losing information, Alias Lens must stop and leave the live file unchanged. It must never guess at a destructive migration.
