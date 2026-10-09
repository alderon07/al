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

Catalog preview prints actionable import diagnostics without shell source. Its JSON uses the existing version 1 diagnostic fields for detailed reasons and guidance. Diagnostic wording and codes can become more specific; inspection statuses, source ranges, and catalog shadow output retain their existing meanings.

Native approval record format 2 binds entry ID, shell, name, kind, implementation hash, and renderer. Generation, installed, ownership, and rollback records have independently versioned private formats. Multi-target workflow journals use format 2, while existing private-file journals retain format 1 recovery. Unsupported formats are refused without rewriting them.

The guided commands are `init SOURCE` and `catalog enable`, with optional `--shell`, `--startup-path`, and `--apply`; init also accepts `--catalog-path`. Explicit startup enrollment supports narrowly proven placement, including relocation of one exact native source block after other static literal loads. The plan discloses reordering; those other files remain user-owned and uninspected. Dynamic or duplicate native routes and unsupported immediate control flow remain refused. Inspection and recovery retain `catalog review`, `catalog approve NAME`, `catalog adopt NAME`, `catalog rollback`, and `catalog recover`. `al plan [--json]` previews enablement, rollback, and init without applying changes. `sync --catalog` selects catalog synchronization independently of the configured legacy shell. See the [storage](acceptance/CATALOG_STORAGE.md), [transaction](acceptance/CATALOG_TRANSACTION.md), and [shell runtime](acceptance/CATALOG_SHELL_RUNTIME.md) contracts for the exact private formats and accepted startup grammar.

Startup-name checks use actual declaration names and final source order. Earlier defaults require proof that reviewed enrolled native aliases overwrite them. Explicit relocation guards native and catalog definition parsing against aliases from earlier loads and restores the previous alias-expansion option. It preserves the external alias table and does not inspect external functions or freeze later alias expansion. Verified re-enable retains the managed source placement without changing record formats. See the [startup placement contract](acceptance/CATALOG_STARTUP_DEFINITIONS.md).

Explicit enrollment also recognizes existence-guarded opaque loads using an escaped dot and directory variables with preceding top-level literal assignments. Variable resolution is a static route hint; the loaded scripts' effects on variables and functions remain trusted user configuration. Planning neither executes those scripts nor claims to verify their runtime targets. Immediate opaque eval also requires explicit placement so initialization code runs before the catalog overlay. Visible ambiguous assignments, dynamic expressions, mismatched guards, extra commands, and native-file route hints remain refused.

Startup inspection refuses unproven unquoted brace contexts and unsupported function headers, including declarations embedded in command lists. Quoted literal braces and proven deferred helper bodies remain supported. Refusal happens before enrollment, so initial acceptance and later managed-block discovery use compatible boundaries.

Interactive catalog review groups pending exact native approvals and eligible fallback enrollments into one displayed batch. Users can confirm the batch or review entries individually, followed by final save or application confirmation. Existing matching records are reused. Batch review changes no approval keys, ownership identities, freshness checks, storage formats, or noninteractive approval requirements. The terminal view wraps the review and requires reaching its end before batch approval.

Native catalog enrollment accepts a bounded grammar for Bash and Zsh declarations. Command and process substitutions, ANSI-C quoted strings, unquoted brace contexts, braced parameter expansions such as `${HOME}`, escaped command newlines, here-strings, and arithmetic involving variables are unsupported and require simplifying the definition before enrollment. Ordinary `$HOME` and quoted literal braces remain supported.

Installation separately inspects retained native helpers. Ordinary command substitutions, braced parameter expansions, and continuations separated by horizontal whitespace on both sides can remain native without approval or enrollment. Compound control blocks, heredocs, process substitutions, and continuations that join tokens remain unsupported. Inspection uses the same boundaries for collisions, fallback refresh, and the read-only startup helper; it runs no native code. Exact literal legacy shell-init invocations are relocated through the installation plan and restored by offline rollback.

Alias dependencies use whole alias-eligible command words, including commands inside substitutions. Flags, ordinary arguments, paths, quoted or escaped literals, comments, assignments, and redirection targets do not create dependencies. The pure dependency policy runs during planning and immutable generation loading. Trailing-blank alias replacements remain unsupported. See the [dependency acceptance contract](acceptance/CATALOG_ALIAS_DEPENDENCIES.md).

Bash permits a simple alias replacement whose initial command is its own name. That exemption does not apply to functions, subsequent commands, or nested substitutions. Zsh self replacements remain conservatively blocked pending runtime evidence.

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
