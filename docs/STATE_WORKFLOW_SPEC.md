# State, planning, and portability specification

Status: proposed on 2026-09-18. This document defines acceptance criteria before implementation. It does not authorize code changes. Each work phase needs an approval record before implementation starts.

## Purpose

Alias Lens needs a predictable path from a portable catalog to working shell definitions. The path must stay narrower than a general dotfile manager.

This specification adds a state model, a common plan contract, machine profiles, semantic catalog comparison, one-command bootstrap, shell completions, plain-language output, and selectable shortcut profiles. It extends [the approved shell-neutral architecture](SHELL_NEUTRAL_ARCHITECTURE.md). That document remains authoritative for catalog ownership, shell adapters, generated files, native approval, transactions, synchronization, recovery, and rollback.

Where this document narrows an ambiguous point, it does so explicitly:

- `al status` is observational. It never performs transaction recovery.
- Catalog conditions require schema version 2. Version 1 continues to reject unknown fields.
- Local profile selection requires application configuration version 2.
- The existing `al diff` command keeps its 1.x meaning. Catalog semantic comparison uses `al catalog diff`.
- Plans do not grant permission to write. A mutating command rebuilds and rechecks its plan before it writes.

## Product boundary

Alias Lens manages commands, aliases, shell functions, their metadata, and the shell integration needed to use them. It can synchronize the portable catalog and files that a user explicitly enrolls with `al track`.

Alias Lens does not become a general configuration manager. This work does not add:

- A template language or environment-variable interpolation.
- Lifecycle scripts, install hooks, package installation, or arbitrary setup and condition commands.
- Secret interpolation or password-manager lookup.
- Automatic downloads outside a configured Git provider or repository.
- Management of untracked dotfiles.
- A branch-per-machine workflow.
- Translation of native code from one shell to another.
- Automatic activation of new or changed synced native code.

Directory and Git-workspace scopes remain a separate roadmap item. Machine profiles decide whether an entry is available on a machine. They do not inspect the current directory or repository.

## State model

Alias Lens reports five states. Each name has one meaning across the CLI, the TUI, JSON output, logs, and tests.

| State | Source | Meaning |
| --- | --- | --- |
| Catalog state | `catalog.json` | The portable entries that the user declared. |
| Resolved state | Catalog state plus the selected shell, platform, local profiles, and native approvals | The entries eligible for one shell on this machine. |
| Rendered state | An immutable generated file | The exact native definitions produced from one resolved state. |
| Installed state | The startup loader, active-generation pointer, and recorded installed state | The generation that a new shell process will load. |
| Repository state | The enrolled catalog path and the last successful local and remote hashes | The synchronization relationship for the catalog. |

"Installed" does not mean that the current shell process loaded the generation. Alias Lens cannot prove that from another process. CLI and TUI text must say "installed for new Bash shells" or "restart or reload the shell" instead of claiming that a running shell is current.

Legacy mode has no catalog, resolved, or rendered state. Its declared and installed definitions remain the configured native alias file. Mixed mode follows the per-shell rules in `SHELL_NEUTRAL_ARCHITECTURE.md`.

### State derivation

State derivation is deterministic for these inputs:

- Canonical catalog bytes and schema version.
- Shell adapter and renderer version.
- Platform.
- Sorted local profile names.
- Sorted native-approval records and their content hashes.
- Installed-state record and on-disk hashes.
- Repository base, local, and remote hashes when available locally.

State derivation does not read arbitrary environment variables, run catalog content, contact a network, or use the clock. Callers supply the allowed inputs. A status timestamp can describe a stored event, but it cannot affect the computed state.

## Status contract

The new command is:

```text
al status [--json]
```

`al status` reads local state and prints one summary. It does not acquire the mutation lock, recover a journal, create a missing file, update a last-read time, run a shell validator, start a watcher, refresh a remote, or contact a provider.

If an incomplete transaction exists, `al status` reports `recovery_required`. The next mutating command performs the mandatory recovery defined by the shell-neutral architecture before it builds a new plan. The recovery-aware shell integration check named in that architecture remains separate. The public command uses an observational inspector and does not call that check.

Plain output uses one line per section and gives an action for every nonhealthy result. It uses no Lip Gloss styling. Example:

```text
mode: catalog
catalog: valid, schema 2, 42 entries
bash: installed generation 71c4...; current for this machine
zsh: not installed; run al catalog enable --shell zsh
sync: local changes; run al catalog diff
recovery: not required
```

Plain output uses familiar words. It says that an item is "ready," "needs attention," or "could not be checked." It reserves internal state names such as `render_required` and `approval_required` for JSON. The plain report explains the result and gives the exact next command.

The JSON object uses this version 1 shape:

```json
{
  "schema_version": 1,
  "mode": "catalog",
  "config": {
    "state": "current",
    "schema_version": 2,
    "action": ""
  },
  "catalog": {
    "state": "valid",
    "schema_version": 2,
    "entry_count": 42,
    "catalog_sha256": "..."
  },
  "shells": [
    {
      "name": "bash",
      "state": "current",
      "installed": true,
      "generation_sha256": "...",
      "action": ""
    }
  ],
  "sync": {
    "state": "local_changes",
    "action": "al catalog diff"
  },
  "recovery": {
    "state": "not_required",
    "action": ""
  },
  "summary": {
    "state": "attention",
    "action_count": 1
  }
}
```

Arrays are present even when empty. Optional hashes are omitted when they do not exist.

Version 1 fixes these state values:

| Field | Values |
| --- | --- |
| `mode` | `legacy`, `catalog`, `mixed` |
| `config.state` | `current`, `migration_required`, `invalid`, `unreadable` |
| `catalog.state` | `absent`, `valid`, `invalid`, `unreadable` |
| `shells[].state` | `not_installed`, `current`, `render_required`, `approval_required`, `integration_drift`, `unreadable`, `blocked` |
| `sync.state` | `unconfigured`, `clean`, `local_changes`, `remote_changes`, `diverged`, `conflict`, `offline`, `invalid` |
| `recovery.state` | `not_required`, `recovery_required`, `blocked` |
| `summary.state` | `healthy`, `attention`, `blocked` |

A minor release can add optional object fields. A new state value or changed field meaning requires status schema version 2. Consumers must reject an unsupported schema version instead of interpreting an unknown state.

Exit status `0` means the summary is `healthy`. Exit status `1` means that inspection succeeded but the summary is `attention` or `blocked`, or that an operational read failed. Exit status `2` means invalid command syntax.

The report abbreviates the home and configured repository roots. It never contains catalog arguments, alias values, function bodies, provider tokens, secret findings, conflict bytes, or validator output.

## Unified operation plans

Every new catalog, profile, bootstrap, completion-installation, and rollback mutation uses one internal `OperationPlan`. Existing mutations move to this contract only under separate acceptance criteria that preserve their 1.x behavior.

The public preview command is:

```text
al plan [--json] COMMAND [ARGUMENTS]
```

The first supported planned operations are:

```text
al plan catalog migrate --to 2
al plan catalog enable --shell bash|zsh
al plan catalog rollback [--shell bash|zsh]
al plan config migrate
al plan config profile add NAME
al plan config profile remove NAME
al plan completion install bash|zsh
al plan init SOURCE [--shell bash|zsh] [--catalog-path PATH]
```

`al plan` never changes managed configuration, catalog, repository, generated, startup, alias, state, backup, revision, approval, or active-pointer files. It can create mode `0700` temporary directories and mode `0600` temporary files for rendering and validation. It removes those artifacts before return. A remote `init` plan can contact the named provider and fetch bounded catalog metadata and bytes after it prints the host and repository that it will contact. No other plan operation contacts a network.

An operation plan contains ordered inputs, actions, diagnostics, and required approvals. The public report has this version 1 shape:

```json
{
  "schema_version": 1,
  "operation": "catalog.enable",
  "inputs": [
    {
      "role": "catalog",
      "display_path": "~/.config/alias-lens/catalog.json",
      "sha256": "..."
    }
  ],
  "actions": [
    {
      "sequence": 1,
      "kind": "create",
      "target_role": "generated_file",
      "display_path": "~/.config/alias-lens/generated/bash/71c4....sh",
      "reason": "the resolved Bash definitions changed",
      "risk": "review",
      "planned_sha256": "...",
      "backup": false,
      "reversible": true
    }
  ],
  "approvals": [],
  "diagnostics": [],
  "summary": {
    "action_count": 1,
    "blocked": false
  }
}
```

The closed action kinds are `create`, `replace`, `remove`, `edit_owned_range`, `remove_owned_range`, `activate`, `deactivate`, `record_approval`, `clone`, and `configure`. The closed risk values are `low`, `review`, and `blocked`. An action with `blocked` risk cannot be applied.

Plan schema version 1 fixes those action kinds and risk values. A minor release can add optional object fields. A new action kind, a new risk value, or a changed field meaning requires plan schema version 2.

Each approval report contains `kind`, `entry_id`, `shell`, `implementation_sha256`, and `state`. Version 1 supports only the `native_code` kind and the states `required` and `approved`. Approval reports never contain implementation text. A new approval kind or state requires plan schema version 2.

A plan contains at most 256 inputs, 20,000 actions, 10,000 approvals, and 100 ordinary diagnostics. A final diagnostic records the exact omitted count when diagnostics exceed that limit. The canonical public report cannot exceed 8 MiB. The planner returns a blocked diagnostic instead of a partial plan when another limit is exceeded.

Status and plan JSON emit fields in the order shown in this document. Ordered actions sort by `sequence`; inputs and approvals use the planner's documented role and identity order. Encoding uses UTF-8, two-space indentation, no HTML escaping, and one final line feed. Optional fields are omitted instead of encoded as `null` or an empty sentinel.

`display_path` is for local display. It abbreviates known private roots and never acts as a write target. The internal plan holds descriptor-checked targets, exact expected identities, planned bytes, inverse edits, metadata, and validation commands as required by the shell-neutral architecture. The public report contains hashes, not definition bodies or inverse bytes.

The mutating command always does this work again:

1. Acquire the shared mutation lock.
2. Perform mandatory journal recovery.
3. Rebuild the operation plan from fresh descriptors and hashes.
4. Recheck all native approvals and validation results.
5. Show the changed plan if it differs from the preview.
6. Require approval again when the changed plan contains a `review` action.
7. Apply the plan through the shared transactional writer.

No command accepts a plan report as permission to skip these checks.

Plain plan output uses the columns `ACTION`, `TARGET`, `RISK`, and `REASON`, followed by an approval and diagnostic summary. Terminal control bytes are escaped. JSON output is buffered before one stdout write and follows the error rules already defined for catalog shadow reports.

## Catalog semantic comparison

The existing `al diff` command continues to compare the configured legacy alias file with its repository copy. Its meaning and accepted syntax do not change.

Catalog mode adds:

```text
al catalog diff [--json|--show-code] [--from repository|installed] [--shell bash|zsh]
```

The default source is `repository` when a catalog repository is configured. Otherwise it is `installed`. The default installed shell is the configured shell. `--shell` selects another installed shell.

The `repository` source reads the enrolled repository path. The `installed` source reads the private canonical catalog snapshot referenced by that shell's installed-state record. If the required source or matching snapshot is missing or corrupt, the command reports that semantic comparison is unavailable. It does not reconstruct catalog intent from generated shell text.

Comparison pairs entries by stable ID. It reports these field paths:

- `schema_version` at the catalog root
- `name`
- `kind`
- `description`
- `category`
- `tags`
- `platforms`
- `favorite`
- `portable`
- `native.bash`
- `native.zsh`
- `when.profiles_any`
- `when.profiles_none`
- `when.shells`

Arrays and the complete `portable` object are atomic values. The two native shell implementations are separate fields. A field with the same canonical value on both sides is unchanged.

Plain output shows metadata values. It shows portable arguments and native implementation text only when the user requests entry details in the TUI or runs `al catalog diff --show-code` in an interactive terminal. The default report shows content hashes and byte counts for implementation fields. `--json` never includes implementation text.

All rendered text escapes terminal controls. Semantic comparison never renders, validates, sources, or executes an entry.

`--show-code` and `--json` are mutually exclusive. `--show-code` fails when standard output is not an interactive terminal. Plain and JSON reports are each limited to 8 MiB and 100 ordinary diagnostics. A report-size failure writes no partial JSON.

Semantic diff JSON uses this version 1 shape:

```json
{
  "schema_version": 1,
  "source": "repository",
  "changes": [
    {
      "scope": "entry",
      "entry_id": "87f4d803c44a4d8792c4824f8e0bc3f1",
      "name": "gs",
      "kind": "changed",
      "path": "description",
      "before_sha256": "...",
      "before_bytes": 22,
      "after_sha256": "...",
      "after_bytes": 29
    }
  ],
  "diagnostics": [],
  "summary": {
    "added": 0,
    "deleted": 0,
    "changed": 1
  }
}
```

`source` is `repository` or `installed`. An installed report also includes `shell` after `source`. `scope` is `catalog` or `entry`. `kind` is `added`, `deleted`, or `changed`. A catalog-schema change uses catalog scope and the path `schema_version`. An added or deleted entry uses the path `entry`. Missing before or after fields are omitted. Changes sort by scope, entry ID, and the field-path order in this document. Hashes cover the canonical JSON encoding of one field value. The report never includes a field value or implementation text. New enum values require semantic diff schema version 2.

### Three-way reconciliation

Catalog synchronization compares a base catalog with the local and remote catalogs. The base is the exact last successful shared catalog hash and bytes. If the base bytes are missing, Alias Lens cannot claim a semantic merge. It saves private conflict copies and leaves the live catalog unchanged.

The merge rules are:

- A field changed on one side uses that side's value.
- The same canonical field value changed on both sides uses that value once.
- Different changes to one field create a field conflict.
- A deletion against an unchanged entry deletes the entry.
- A deletion against a modified entry creates an entry conflict.
- Equal additions of one ID add one entry.
- Different additions of one ID create an entry conflict.
- Different IDs with the same final name create a name conflict.
- Any invalid input blocks the merge before the live catalog changes.

After a clean field merge, Alias Lens validates and canonicalizes the complete result. If validation fails, the merge becomes a conflict. Tags, platforms, arguments, and condition lists do not receive element-level merges.

New or changed remote native implementations remain unapproved. A clean catalog merge can update the portable catalog, but Alias Lens keeps the installed generation unchanged until the user approves every changed native implementation needed by that shell. Status reports `approval_required`. Sync never treats `--push`, `--pull`, automatic sync, or a plan flag as native approval.

Automatic sync can fetch and inspect remote catalog state, but it never replaces the live catalog or activates a generation from remote bytes. It records `remote_changes` or a conflict for review. An explicit `al sync --pull` can apply a clean semantic merge through the operation plan. In catalog mode, pull writes the catalog and then reports `render_required`; it does not activate a generation. Legacy-mode sync keeps its existing 1.x behavior.

Conflict copies use mode `0600` beneath the private state directory. Default output names the entry, field, and recovery command without printing implementation text. The TUI can show local, base, and remote values after an explicit detail action. The user can choose local, remote, or edited content per field. Alias Lens validates the complete candidate again before it writes.

## Machine profiles and conditions

Profiles are local names that group machines by purpose, such as `work` or `laptop`. Profile names do not come from hostnames, environment variables, commands, or repository content.

The profile commands are:

```text
al config profile list
al config profile add NAME
al config profile remove NAME
```

A profile name matches `^[a-z][a-z0-9_-]{0,31}$`. Configuration stores at most 32 profiles. Names are unique and sort bytewise.

### Application configuration version 2

Application configuration version 2 adds `profiles` and `shortcut_profile` after `shell` in encoded field order:

```json
{
  "version": 2,
  "profiles": ["laptop", "work"],
  "shortcut_profile": "macos"
}
```

The complete version 2 field order is `version`, `repository`, `alias_file`, `shell`, `profiles`, `shortcut_profile`, `providers`, `auto_sync`, `tracked_files`, and `footer`. Existing field types and validation do not change. An absent or empty `profiles` value means that no profile is active. The writer omits an empty list. `shortcut_profile` is `windows`, `linux`, or `macos`. A saved value always wins. If the field is absent, Alias Lens selects `windows` on native Windows and WSL, `macos` on Darwin, and `linux` on other supported Linux systems. Reading this default does not write configuration.

The version 1 to version 2 migration follows the existing atomic configuration migration contract. It saves the exact version 1 bytes to `config.json.alias-lens.bak`, sets version 2 with no active profiles, preserves every existing setting, and replaces `config.json` only after validation and durable write completion. Existing commands retain the configured migration-on-load contract. `al status` uses a separate nonmutating decoder, reports `migration_required`, and does not trigger the migration. `al config migrate` provides an explicit path for users who want to migrate before another command needs the configuration.

A version 1 configuration that contains `profiles` is invalid. The migration dispatcher inspects the raw version before it decodes version-specific fields, so it cannot accept profile selection under version 1 and silently drop it.

Adding or removing an active profile can change the next rendered state. The command must show a plan that lists the affected entry names and shells before it changes configuration. It does not activate a new generation as part of the configuration write. Status then reports `render_required`, and `al catalog enable --shell SHELL` performs the separate render and activation transaction.

### Catalog schema version 2

Catalog schema version 1 remains exactly as approved in `docs/acceptance/CATALOG_MODEL_PHASE_2.md`. It continues to reject `when` and every other unknown field.

Schema version 2 adds one optional entry field after `platforms` in canonical field order:

```json
{
  "when": {
    "profiles_any": ["work", "laptop"],
    "profiles_none": ["personal"],
    "shells": ["bash", "zsh"]
  }
}
```

Each present list must contain at least one value. Profile lists contain at most 32 valid profile names. `shells` contains only `bash` and `zsh`. Lists deduplicate and sort bytewise. A profile cannot appear in both profile lists. Unknown fields and explicit `null` remain invalid.

Schema version 2 inherits every version 1 type, byte limit, count limit, diagnostic rule, normalization rule, and JSON encoding rule except where this section adds `when`. Canonical object field order is:

1. Root: `schema_version`, `entries`.
2. Entry: `id`, `name`, `kind`, `description`, `category`, `tags`, `platforms`, `when`, `favorite`, `portable`, `native`.
3. `when`: `profiles_any`, `profiles_none`, `shells`.
4. Portable and native objects: the version 1 order.

The decoder reads `schema_version` before it chooses a version-specific type. A version 2 binary follows these preservation rules:

- It decodes and validates a version 1 catalog with the exact version 1 model.
- A common-field edit to a version 1 catalog encodes valid canonical version 1 again.
- It cannot attach `when` to a version 1 typed value.
- Only `al catalog migrate --to 2` changes a version 1 catalog to version 2.
- It decodes and encodes version 2 through the version 2 model.
- It rejects a future version without returning partial typed state.

A version 1 binary rejects version 2. Semantic diff can compare version 1 and version 2 after lifting version 1 into a read-only internal view with absent conditions, but it reports the root `schema_version` change. Three-way reconciliation requires the base, local, and remote catalogs to use the same schema version. A mismatch blocks reconciliation and names the migration command. Sync never upgrades or downgrades a catalog.

Predicates use these rules:

- Existing `platforms` passes when the current platform is present. An absent list passes.
- `profiles_any` passes when at least one named profile is active.
- `profiles_none` passes when none of the named profiles is active.
- `shells` passes when the selected shell is present.
- All present predicates must pass.
- An absent `when` object passes.

Conditions affect resolved state, rendering, search availability, and picker labels. They do not delete catalog entries, change sync membership, or suppress secret scanning. `al scan` and pre-push scanning inspect every entry, including entries excluded on the current machine.

Search and the picker keep excluded entries visible so the user can find and edit them. Available entries rank first. An excluded entry shows `not available` with the failed platform, profile, or shell predicate, and execution is blocked. Machine-readable search can add `available` and `unavailable_reasons` as optional fields under the compatibility policy. Shell selection and `al pick --command` never return an excluded entry.

Alias Lens does not add conditions to schema version 1 in memory and then encode invalid version 1 JSON. New catalogs use schema version 2 only after the version 2 model and migration criteria pass.

The explicit migration command is:

```text
al catalog migrate --to 2
```

Migration changes `schema_version` from `1` to `2` and makes no semantic entry change. It uses the mutation lock, transaction journal, private backup, and revision. It does not activate, render, sync, or push. Version 2 readers continue to read version 1 catalogs during the documented compatibility window. A version 2-capable writer preserves version 1 for a version 1 input until the user runs the migration. Adding a condition to a version 1 catalog requires the explicit migration first.

Future condition types require another catalog schema version. There is no generic expression, template, hostname predicate, environment predicate, file predicate, or executable predicate in version 2.

## Local state and private storage

The storage layout in the shell-neutral architecture gains these private records:

```text
~/.local/state/alias-lens/
├── catalog-state.json
├── native-approvals.json
├── catalog-snapshots/
│   └── <catalog-sha256>.json
└── temporary/
```

`catalog-state.json` has its own schema version. The fields relevant to this specification have this shape:

```json
{
  "schema_version": 1,
  "installed_shells": {
    "bash": {
      "active_generation_sha256": "...",
      "source_catalog_sha256": "...",
      "resolved_state_sha256": "...",
      "loader_sha256": "..."
    }
  },
  "catalog_sync": {
    "repository_path": "alias-lens/catalog.json",
    "base_catalog_sha256": "...",
    "local_catalog_sha256": "...",
    "remote_catalog_sha256": "..."
  }
}
```

Shell keys sort bytewise. Hash fields contain 64 lowercase hexadecimal characters. `repository_path` follows the same clean relative-path rules as tracked repository paths. Fields that have no value are omitted. Transaction, adoption, and startup metadata required by the shell-neutral architecture can add named objects before this schema is approved. The final acceptance document must freeze their exact shape and canonical order rather than use an untyped extension map. This file does not store catalog entries or implementation text.

`native-approvals.json` uses this shape:

```json
{
  "schema_version": 1,
  "approvals": [
    {
      "entry_id": "f98010ca0aa84af69fd4df32ec91726b",
      "shell": "bash",
      "kind": "function",
      "implementation_sha256": "...",
      "renderer": "bash/v1",
      "approved_at": "2026-09-18T14:00:00Z"
    }
  ]
}
```

Approval records sort by shell and then entry ID. `approved_at` uses RFC 3339 UTC and is audit metadata. It does not affect rendering or the approval key. `implementation_sha256` hashes the same unsigned 64-bit big-endian length-framed sequence format used by generation hashes. Its values are the entry kind, native field name, and exact implementation bytes, in that order. The file does not store implementation text. The approval key is the entry ID, shell, kind, implementation hash, and renderer. A catalog-only schema migration does not invalidate unchanged native approval.

`catalog-snapshots/<catalog-sha256>.json` stores exact canonical catalog bytes. Semantic synchronization requires the snapshot named by `base_catalog_sha256`. Installed semantic diff requires the snapshot named by each `source_catalog_sha256`. Alias Lens keeps every snapshot referenced by installed state, sync state, or an incomplete transaction. It removes an older unreferenced snapshot only through a planned private-state cleanup.

The `temporary` directory contains only operation-owned preview and bootstrap artifacts. Each operation uses a new unpredictable child directory with mode `0700`. A successful or normally failed operation removes its child after it closes all descriptors. Recovery preserves an ambiguous child and reports it through `al data paths`.

All directories use mode `0700`, and all files use mode `0600`. Each decoder rejects newer versions and corrupt records without rewriting them. These files remain local and never enter the configured repository. `docs/PRIVACY.md` must list them before release.

## One-command bootstrap

The bootstrap command is:

```text
al init SOURCE [--shell bash|zsh] [--catalog-path PATH] [--apply]
```

`SOURCE` is one of:

- An existing local Git repository path.
- `github:OWNER/REPOSITORY`.
- `gitlab:NAMESPACE/REPOSITORY`.
- `bitbucket:WORKSPACE/REPOSITORY`.

Remote locators use the configured provider host and protocol. They cannot contain user information, a token, a query, a fragment, an absolute filesystem path, or traversal. Provider tokens remain in provider credential stores or documented environment variables. Alias Lens never places them in arguments, output, clone URLs, configuration, or remotes.

The default repository path is `alias-lens/catalog.json`. `--catalog-path` accepts one clean relative path. It rejects absolute paths, `..`, empty components, symbolic-link traversal, and credential-shaped names.

In an interactive terminal, `al init` performs discovery, prints the final plan, and asks whether to apply it. Declining the prompt leaves managed state unchanged. In a noninteractive process, the command prints the plan and exits without applying unless `--apply` is present. The flag does not skip plan construction, hash checks, or native review.

Bootstrap follows this sequence:

1. Resolve or validate the selected shell without changing configuration.
2. Print the exact provider host, repository, and catalog path before a remote read.
3. For a remote source, use `RepoProvider` to read the named file and its immutable revision through the provider's configured HTTPS API. Stream at most 8 MiB, reject cross-host redirects, and do not run Git during preview.
4. For a local source, open the named working-tree file through the descriptor and link checks used for enrolled files. Record its bytes, identity, and repository `HEAD` without changing the index or worktree.
5. Validate the catalog schema, size, secrets, conditions, native structure, and selected-shell render in a private staging directory.
6. Build one operation plan for the managed repository, configuration, approvals, generated file, startup loader, and active pointer.
7. Show every native implementation that needs approval through the explicit native-review view.
8. Acquire the mutation lock, perform recovery, rebuild the plan, and recheck the remote revision and blob hash or the local identity and content hash.
9. For a remote source, create a fresh managed-repository staging directory. Invoke clone with an Alias Lens-owned empty `core.hooksPath`, no checkout, no submodules, a one-commit depth, and partial blob filtering. Use the configured provider protocol without embedding credentials. Do not run checkout, status, add, or a filter command.
10. Read the enrolled catalog blob with Git plumbing that does not apply worktree filters. Populate the index and sparse-worktree bits through plumbing that does not write worktree files. Write only the enrolled catalog path through the shared safe writer, then verify its bytes against the previewed blob. Leave every other tracked path absent from the sparse managed worktree.
11. Apply configuration, generated-file, startup-loader, and active-pointer changes through the transaction protocol from the shell-neutral architecture. Move the managed repository from staging to its final new path only through a recorded transaction action.
12. Run the same local checks used by `al status` and print the action required to start or reload the shell.

`--apply` never approves native code. In an interactive terminal, the user approves native implementations one at a time by entry ID and content hash. In a noninteractive process, bootstrap installs only portable entries and native implementations that already have a matching local approval. If an entry has no eligible implementation, the catalog remains stored but that entry is absent from the generated file. Status reports the pending approvals and unavailable entries.

Remote discovery has a two-minute deadline and an 8 MiB response limit. A managed remote clone has a five-minute deadline and a 256 MiB staging-directory limit. Alias Lens terminates the process group and blocks application when either limit is exceeded. Repository size limits do not weaken the 8 MiB catalog limit.

Provider preview returns the repository's immutable revision, catalog blob hash, and catalog bytes. If a provider cannot supply all three through its HTTPS API, remote bootstrap stops and gives the existing manual repository-configuration path. Application accepts the clone only when its revision and catalog blob match preview. A local source is never copied or recloned. Bootstrap enrolls the existing local repository after the locked identity and hash recheck.

Clone failure, partial-clone refusal, or sparse-index setup failure stops bootstrap. Alias Lens does not fall back to a full clone or a normal checkout. Later sync for this managed sparse repository must keep the same no-hook, no-filter, enrolled-path-only rules.

Bootstrap stops without changing managed state when:

- The repository or catalog path is missing, ambiguous, or invalid.
- The live configuration already names a different repository and the user did not start a separate reconfiguration flow.
- The remote object changes between discovery and locked application.
- The catalog contains a likely secret.
- Rendering or parse-only validation fails.
- A startup target or managed block fails the identity checks in the shell-neutral architecture.
- The plan contains a name collision or unresolved conflict.

A failed operation removes only staging artifacts that the same operation created and can identify exactly. If recovery cannot prove ownership, it leaves private inert artifacts in place and reports them through `al data paths`. It never removes a pre-existing repository directory.

## Shell completions

Alias Lens adds:

```text
al completion bash|zsh
al completion install bash|zsh
al completion remove bash|zsh
```

`al completion SHELL` prints a deterministic completion program to stdout and writes nothing. The program completes public commands, subcommands, closed flag values, configured profile names, catalog entry names where a command accepts one, and Bash or Zsh where a shell name is required.

Completion generation reads one internal command specification shared with help text. Moving help metadata to that specification must preserve the existing 1.x command names, flags, help meaning, exit statuses, and plain output except for the addition of new commands.

Completion code does not contact a network, execute an alias, parse native function bodies, or print implementation text. Dynamic candidates use the private integration command `alias-lens completion-candidates --shell SHELL --command COMMAND --prefix PREFIX`. The command reads the active catalog or legacy alias file through the matching parser. It returns at most 1,000 candidates and 1 MiB of escaped output. A parse, limit, or permission error returns no private fallback text. This integration command can change when completion installation writes a matching completion program, like the existing integration commands in `docs/COMPATIBILITY.md`.

Install and removal use the shared operation plan and writer. They write an Alias Lens-owned completion file. The one Alias Lens-owned shell integration block loads both the active generated definitions and the completion file. Completion installation must not add a second startup-file block. Install and removal preserve unrelated completion settings and refuse an edited integration block. `al setup` does not start installing completions until a separately approved compatibility criterion allows that behavior.

Bash completion supports Apple Bash 3.2 and GNU Bash 5.2. Zsh completion supports Zsh 5.9. Completion installation for one shell does not edit the other shell's files.

## TUI behavior

The Bubble Tea interface remains the default experience.

The TUI adds these views without making the browser endpoint mandatory:

- A status view that renders the same typed status result as `al status`.
- A plan review view with action, target, risk, reason, and affected entry counts.
- A semantic diff view grouped by entry and field.
- A native review view that shows the shell, entry ID, content hash, and escaped implementation text.
- A conflict view that compares base, local, and remote values and supports local, remote, or edited resolution.

The CLI and TUI must not compute separate state, plan, comparison, or merge results. They render the same typed core result. TUI cancellation before final confirmation performs no managed-state mutation. A terminal resize preserves selection and review state. Terminals below the documented minimum size show the existing short size requirement.

Confirmation names the operation and count. A generic "Are you sure?" is not sufficient. Native approval is separate from plan approval.

## Plain-language interface

The CLI and TUI use the same user-facing copy for the same result. JSON field names, stable command names, and internal diagnostic codes keep their exact technical values.

User-facing text follows these rules:

- Lead with what happened. Follow with what the user can do next.
- Say `run` instead of `execute`.
- Say `Press Enter to run` instead of `Enter executes`.
- Say `change` instead of `mutation`.
- Say `compare and combine` instead of `reconcile` when describing sync.
- Say `prepare for Bash` or `prepare for Zsh` instead of `render`.
- Say `ready for new shells` instead of `activated`.
- Say `saved version` instead of `generation` unless a hash is required for support.
- Say `data format` instead of `schema` outside migration help.
- Say `Alias Lens cannot use this file` and give the reason instead of reporting only `invalid`.
- Name the affected alias, shell, or file when that name is safe to print.
- State that files were left unchanged after a failed or cancelled operation.

Messages use this order when all parts apply:

1. Outcome.
2. Short reason.
3. What stayed unchanged.
4. One exact next command or key.

Example:

```text
Alias Lens could not update your aliases because the local and remote commands both changed.
Your current aliases were left unchanged.
Run al catalog diff to review the changes.
```

Advanced help can name catalog state, hashes, and data versions after the plain explanation. Plain output does not require the user to understand Git, JSON, shell parsing, transaction journals, or internal state names.

## Selectable shortcut profiles

Shortcut profiles change TUI keys and the keyboard guide. They do not change shell syntax, catalog portability, or the operating system that Alias Lens supports.

The commands are:

```text
al shortcuts
al shortcuts windows
al shortcuts linux
al shortcuts macos
al shortcuts test
```

`al shortcuts` shows the active style, its source, and its keys. Passing `windows`, `linux`, or `macos` saves that style. A user can choose any profile on any operating system. The first default is Windows on native Windows and WSL, macOS on Darwin, and Linux on other supported Linux systems. Alias Lens detects WSL through a tested platform helper. It does not infer the profile from `$TERM`, terminal brand, shell, or hostname.

Each TUI action has one semantic action ID. Pages handle action IDs, not physical keys. The selected profile maps physical keys to those actions. Help text and footers read from the same map, so displayed shortcuts cannot differ from accepted shortcuts.

The first profile version uses these defaults:

| Action | Windows | Linux | macOS | Terminal-safe fallback |
| --- | --- | --- | --- | --- |
| Run or choose | `Enter` | `Enter` | `Enter` | `Enter` |
| Go back or cancel | `Esc` | `Esc` | `Esc` | `Esc` |
| Find | Type to search | Type to search | Type to search | Type to search |
| Add an alias | `Ctrl+A` | `Ctrl+A` | `Cmd+N` | `Ctrl+A` |
| Edit an alias | `Ctrl+E` | `Ctrl+E` | `Cmd+E` | `Ctrl+E` |
| Save | `Ctrl+S` | `Ctrl+S` | `Cmd+S` | `Enter` in a form |
| Open saved versions | `Ctrl+Z` | `Ctrl+Z` | `Cmd+Z` | `Ctrl+Z` |
| Refresh | `Ctrl+R` | `Ctrl+R` | `Cmd+R` | `Ctrl+R` |
| Open sync | `Ctrl+F` | `Ctrl+F` | `Cmd+Shift+S` | `Ctrl+F` |
| Open themes | `Ctrl+T` | `Ctrl+T` | `Cmd+T` | `Ctrl+T` |
| Open health | `Ctrl+H` | `Ctrl+H` | `Cmd+H` | `Ctrl+H` |
| Open stats | `F2` or `Ctrl+S` | `F2` or `Ctrl+S` | `Cmd+2` | `F2` |
| Open settings | `F3` | `F3` | `Cmd+,` | `F3` |
| Open help | `F1` or `?` | `F1` or `?` | `Cmd+?` or `?` | `?` |

Copy and paste stay owned by the terminal:

- The Windows and Linux guides explain that terminals commonly use `Ctrl+Shift+C` and `Ctrl+Shift+V` for copy and paste. A terminal can provide different bindings.
- The macOS guide shows `Cmd+C` and `Cmd+V` for terminal copy and paste.
- Alias Lens accepts bracketed paste as text input. It does not replace the terminal's clipboard or intercept a copy key globally.
- `Ctrl+C` continues to cancel or quit when the terminal sends it to Alias Lens instead of handling copy.

Many terminals consume Command keys before a TUI can receive them. Alias Lens supports `Cmd` keys only when the terminal sends the Super modifier through its keyboard protocol. The keyboard guide always shows the terminal-safe fallback beside a preferred Command key. `al shortcuts test` prints the active guide without changing the saved shortcut choice. Modified keys that are not Alias Lens actions do not become search or form text. The terminal still delivers bracketed paste as text.

A shortcut cannot replace text entry, block `Esc`, or remove the terminal-safe fallback. Form, confirmation, and text-entry contexts take priority over page-navigation shortcuts. Shortcut changes apply when the next TUI starts.

## Security and privacy rules

All existing safety rules continue to apply. This work also requires these rules:

- Status, plan, diff, merge, bootstrap discovery, completion, and TUI preview never execute an alias or function.
- Secret scanning covers every catalog entry and implementation before a repository commit or push, regardless of local conditions.
- Public plan and JSON reports use hashes and byte counts instead of implementation text.
- Native approval keys contain the entry ID, shell, entry kind, implementation content hash, and renderer version. A change to any value invalidates approval.
- Remote native changes never inherit approval from an entry name or a previous content hash.
- Profile names are not secrets. Alias Lens must still keep the complete local profile list out of repository data and normal logs.
- Temporary bootstrap and validation artifacts use private permissions and do not survive a successful preview.
- A catalog cannot select credential files, run a condition command, read a secret value, or interpolate local data into an implementation.
- Completion functions treat file content as data and escape candidates for the target completion API.
- Network redirects follow the configured-host rule. Cross-host redirects fail.
- Bootstrap does not initialize submodules, Git LFS, hooks, filters, or repository executables.
- Conflict and rollback material stays private and appears in `al data paths`.
- Every new file and field is added to `docs/PRIVACY.md` before release.

## Compatibility and migration

This work follows `docs/COMPATIBILITY.md`.

- `al diff` keeps its current syntax and legacy repository-comparison meaning.
- Existing commands do not gain a required prompt or flag in 1.x.
- New commands can be added in a minor release.
- Plain output can add new sections only where the existing command contract permits it. Existing machine-readable fields are not removed or renamed.
- Catalog version 1 remains strict. Version 2 support uses a separate decoder, validator, normalizer, encoder, fixtures, and comparison matrix.
- App configuration version 2 migrates version 1 through the existing backed-up atomic path.
- Existing installations stay in legacy mode until explicit catalog import, adoption, and enablement.
- Enabling one shell does not change another shell's legacy or catalog source.
- Rollback works offline without reading the current catalog.

Before any 1.x release adds these commands, update the compatibility policy with their stable meanings and JSON contracts.

## Rollback behavior

The transaction and rollback design in the shell-neutral architecture applies to every managed write in this specification.

In addition:

- Profile changes record exact prior configuration bytes and affected resolved-state hashes.
- Catalog schema migration records exact version 1 bytes and can restore them while version 1 remains supported.
- Bootstrap rollback restores adopted native definitions before it deactivates the generated replacement.
- A rollback never contacts a provider or requires a repository.
- A changed target causes a private conflict copy and a manual recovery plan. Alias Lens does not overwrite the changed target.
- Completion removal deletes only an exact recorded managed block and Alias Lens-owned file.
- Semantic conflict resolution creates a catalog revision before it replaces the live catalog.

## Deferred alias packs

Alias packs are deferred until catalog activation, semantic synchronization, profiles, bootstrap, and rollback have completed at least one stable release cycle.

A later pack proposal must use a separate schema version and acceptance review. It must require:

- A pinned pack version and SHA-256 digest.
- A manifest that contains entries and metadata only.
- No scripts, templates, hooks, binaries, package installation, or network calls during installation.
- Preview of every entry and provenance before installation.
- Local approval for each native implementation.
- Stable origin IDs that do not replace user-owned entry IDs.
- Local overrides stored separately from pack content.
- Removal that preserves modified or adopted local entries.
- Secret scanning before storage, commit, or push.
- Offline removal and rollback.

Pack registries, dependency resolution, transitive packs, and automatic updates are out of scope.

## Delivery phases

The phase 4 WSL release-candidate evidence in `docs/testing/PHASE4_MANUAL_CHECKLIST.md` remains the next gate. None of this specification bypasses it.

After that gate, deliver this work in these stages:

1. Foundation: add the pure state resolver, observational status result, operation-plan model, and CLI rendering. Do not add writes.
2. Catalog activation: use the operation plan for the approved catalog preview, import, enablement, and rollback phases from the shell-neutral architecture.
3. Semantic sync: add catalog semantic diff and three-way reconciliation. Keep native changes inactive pending approval.
4. Profiles and shortcuts: add app configuration version 2, catalog schema version 2, explicit catalog migration, profile conditions, and selectable shortcut profiles.
5. Bootstrap: build `al init` on the same planner, writer, repository isolation, native approval, and rollback code.
6. Completions: move command help metadata into one specification, then add Bash and Zsh completion output, installation, and removal.
7. Packs: observe one stable release cycle before proposing alias packs.

Each stage needs its own approved acceptance document or an approved subset of the criteria below. An implementation pull request cites every applicable criterion ID.

## Acceptance criteria and test mapping

Tests use isolated temporary homes and repositories. They set every shell, XDG, history, Git, and provider path explicitly. They never read or change the developer's real files. Terminal criteria run on Ubuntu 24.04 with Bash 5.2 and Zsh 5.9, macOS 14 with Bash 3.2 and Zsh 5.9, and Windows 11 WSL 2 with Ubuntu 24.04 and Bash 5.2 where the behavior applies.

### SW-001 derives state without side effects

| Field | Required evidence |
| --- | --- |
| Initial state | Fixtures cover legacy, catalog, and mixed mode; both shells; missing and corrupt state; changed pointers; pending approvals; and repository hash combinations. Callers inject resolved-entry summaries. |
| Operation | Derive state repeatedly and concurrently from injected inputs. |
| Expected state | Results are deterministic. Derivation performs no file, environment, process, clock, random, network, logging, or global-state access. It never reports a running shell as loaded. Every output state is one of the version 1 values. |
| Automated evidence | `internal/state.TestResolveStateMatrix`, `TestResolveStateDeterministic`, and import-allowlist tests. |
| Approval | Required before foundation implementation. |

### SW-002 keeps status observational

| Field | Required evidence |
| --- | --- |
| Initial state | Temporary homes contain healthy state, every nonhealthy state, an incomplete journal, missing paths, unreadable files, symlinks, and provider credentials in sentinel locations. |
| Operation | Run plain and JSON `al status` twice. |
| Expected state | Output and exit status follow this document. Recursive manifests remain byte-for-byte and metadata-for-metadata unchanged. No lock, recovery, validator, watcher, provider, credential helper, or network double is called. No sentinel value appears. |
| Automated evidence | `cmd/alias-lens.TestStatusMatrix`, `TestStatusJSONGolden`, `TestStatusDoesNotRecover`, `TestStatusForbiddenCallGraph`, and canary redaction tests. |
| Terminal evidence | Plain output in narrow and wide terminals on Ubuntu and macOS. |
| Approval | Required before foundation implementation. |

### SW-003 builds one complete, bounded plan

| Field | Required evidence |
| --- | --- |
| Initial state | Each supported operation uses fixtures for create, replace, exact owned edit, removal, activation, approval, blocked action, changed file, path replacement, permission failure, and maximum-size input. |
| Operation | Build plain and JSON plans repeatedly under randomized map order. |
| Expected state | Action order, hashes, reasons, risks, and summaries are deterministic. The internal plan contains every target and inverse edit. Reports contain no implementation bytes, tokens, or unabridged private roots. Work and output stay within documented byte, action, and diagnostic limits. |
| Automated evidence | `internal/plan.TestPlanMatrix`, `TestPlanDeterministic`, `TestPlanReportRedaction`, `TestPlanLimits`, and golden reports. |
| Approval | Required before foundation implementation. |

### SW-004 rejects a stale or changed plan

| Field | Required evidence |
| --- | --- |
| Failure injection | Change each input, identity, approval, remote object, target, metadata field, or active pointer after preview and at every transaction boundary. |
| Expected state | The mutating command rebuilds the plan under the lock. It either asks for approval of the changed plan or stops. It never accepts a preview report as authorization. Recovery leaves the fully specified old or new state and never leaves an adopted alias missing. |
| Automated evidence | `internal/plan.TestRebuildRejectsStaleInputs` plus the crash-boundary suite required by `SHELL_NEUTRAL_ARCHITECTURE.md`. |
| Approval | Required before the first planned mutation. |

### SW-005 keeps CLI and TUI results identical

| Field | Required evidence |
| --- | --- |
| Operation | Render the same typed status, plan, semantic diff, native review, and conflict result through plain, JSON, and TUI views. Cancel before and after detail views. Resize during review. |
| Expected state | Every view reports the same actions, fields, risks, and counts. Cancellation before final confirmation changes no managed state. Control bytes cannot affect terminal state. |
| Automated evidence | Core-result fixture comparisons, Bubble Tea update tests, terminal-control canaries, and TUI resize tests. |
| Terminal evidence | Narrow, medium, and wide Bash and Zsh sessions. |
| Approval | Required before each TUI view ships. |

### SW-006 preserves schema version boundaries

| Field | Required evidence |
| --- | --- |
| Initial state | Catalog versions 1, 2, and future; config versions 1, 2, and future; unknown fields; duplicate keys; `null`; size boundaries; missing backups; and read-only paths. |
| Operation | Read, plan, migrate, interrupt, recover, and roll back every supported transition. |
| Expected state | Version 1 still rejects `when`. Version 2 follows the exact schema and canonical order. Catalog migration is explicit and semantic-neutral. Config migration preserves all settings and saves exact old bytes. Future versions stop without writes. |
| Automated evidence | `internal/catalog.TestVersion2SchemaMatrix`, `TestV1RejectsV2Fields`, canonical v2 golden tests, `cmd/alias-lens.TestConfigV2MigrationMatrix`, and crash tests. |
| Approval | Required before profiles implementation. |

### SW-007 resolves conditions exactly

| Field | Required evidence |
| --- | --- |
| Initial state | Every absent, matching, nonmatching, duplicate, empty, overlapping, invalid, and boundary condition combined with platform exclusions and native availability. |
| Operation | Resolve entries for both shells, every platform, and profile sets at zero, one, and 32 entries. |
| Expected state | Predicate logic matches this document. Ordering has no effect. Excluded entries remain stored and scanned. No condition reads a hostname, environment value, file, executable, command result, or network response. |
| Automated evidence | `internal/catalog.TestConditionValidationMatrix`, `internal/state.TestConditionResolutionMatrix`, property tests, and forbidden-call tests. |
| Approval | Required before profiles implementation. |

### SW-008 reports semantic changes by stable identity

| Field | Required evidence |
| --- | --- |
| Initial state | Adds, deletes, renames, same-name different-ID entries, every field path, order-only changes, invalid inputs, secrets, and terminal-control canaries. |
| Operation | Run plain, JSON, and detailed catalog diff against repository and installed sources. |
| Expected state | Pairing and field paths follow this document. Default and JSON output redact implementation text. `--show-code` requires an interactive terminal. Comparison executes and validates no entry. Existing `al diff` fixtures remain unchanged. |
| Automated evidence | `internal/catalog.TestSemanticDiffMatrix`, `cmd/alias-lens.TestCatalogDiffGolden`, redaction and no-execution tests, and existing `al diff` regression tests. |
| Approval | Required before semantic-sync implementation. |

### SW-009 merges only nonoverlapping semantic changes

| Field | Required evidence |
| --- | --- |
| Initial state | A complete base, local, and remote matrix covers each rule in this document, missing base bytes, invalid catalogs, name collisions, field collisions, deletes against edits, and post-merge validation failures. |
| Operation | Reconcile through explicit pull, inspect through automatic sync, and resolve conflicts in the TUI. |
| Expected state | Only clean nonoverlapping fields merge. Missing base bytes never trigger a two-way guess. Automatic sync never changes the live catalog. Conflicts leave the live catalog and installed files unchanged and save private copies. Resolution creates a revision and revalidates the complete catalog. |
| Automated evidence | `internal/catalog.TestThreeWayMergeMatrix`, `cmd/alias-lens.TestCatalogSyncConflictSafety`, and TUI conflict tests. |
| Approval | Required before semantic-sync implementation. |

### SW-010 never converts synchronization into native approval

| Field | Required evidence |
| --- | --- |
| Operation | Add or change native implementations through pull, push reconciliation, automatic sync, bootstrap, conflict resolution, rename, and ID reuse attempts. |
| Expected state | Each unrecognized approval key remains inactive. Portable catalog updates can persist, but installed generations do not change until all required native approvals exist. No broad confirmation or flag approves native content. |
| Automated evidence | Native-approval key vectors, `TestRemoteNativeChangeRequiresApproval`, and generated-file hash fixtures. |
| Approval | Required before catalog-activation, semantic-sync, and bootstrap implementation. |

### SW-011 bootstraps without executing repository content

| Field | Required evidence |
| --- | --- |
| Initial state | Local and provider repositories cover valid catalogs, missing paths, redirects, object changes, secrets, native entries, hooks, submodules, LFS, filters, malicious Git configuration, symlinks, traversal, existing configuration, offline operation, and provider failures. |
| Operation | Run preview, interactive apply, noninteractive apply, cancellation, crash, recovery, and rollback. |
| Expected state | Only enrolled catalog contents are materialized in the worktree. Repository metadata and bounded Git objects can be read. Repository code and Git extensions never execute. Credentials stay out of arguments, URLs, files, remotes, and output. `--apply` does not approve native code. A failure preserves all pre-existing state. |
| Automated evidence | `cmd/alias-lens.TestInitMatrix`, process sentinel tests, credential canaries, repository manifest tests, and transaction crash tests. |
| Terminal evidence | Provider bootstrap and native review on Ubuntu, WSL, and macOS with disposable repositories and credentials. |
| Approval | Required before bootstrap implementation. |

### SW-012 generates and installs safe completions

| Field | Required evidence |
| --- | --- |
| Initial state | Every command and flag, malformed catalogs and alias files, hostile names, edited managed blocks, both shells installed, and existing custom completion configuration. |
| Operation | Generate, source, invoke, install, repeat, remove, and roll back completions. |
| Expected state | Static candidates match the command specification. Dynamic candidates stay inert and bounded. Installation is idempotent and shell-specific. Removal preserves unrelated settings and refuses edited blocks. Existing help and command behavior remain unchanged. |
| Automated evidence | Command-spec parity tests, Bash and Zsh completion golden tests, PTY completion tests, mutation-plan tests, and phase 3 baseline comparisons. |
| Terminal evidence | Bash 3.2, Bash 5.2, and Zsh 5.9 completion sessions. |
| Approval | Required before completions implementation. |

### SW-013 preserves privacy, repository isolation, and command compatibility

| Field | Required evidence |
| --- | --- |
| Operation | Run every new command with private canaries, unrelated staged and working-tree files, legacy mode, mixed mode, both configured shells, and no network. Compare the existing stable command suite before and after. |
| Expected state | No canary leaks. Git stages and commits only the enrolled catalog path. Existing `al diff`, setup, sync, import, help, exit status, integration, and JSON fixtures change only where a separately approved criterion names the change. Offline status, local plans, local-source bootstrap, diff, completion, and rollback work. Remote bootstrap reports a network failure without managed-state changes. |
| Automated evidence | Full regression suite, repository isolation tests, secret scans, `make fmt check`, `git diff --check`, and dependency-file hashes. |
| Approval | Required for every stage. |

### SW-014 protects workflow state and snapshots

| Field | Required evidence |
| --- | --- |
| Initial state | Valid, missing, corrupt, truncated, newer-version, symlinked, hard-linked, wrong-owner, wrong-mode, and path-replaced state, approval, and snapshot files. Fixtures include referenced and unreferenced snapshots plus an incomplete transaction. |
| Operation | Read status, compare installed state, approve native content, reconcile, collect unreferenced snapshots, interrupt each write, recover, and roll back. |
| Expected state | Decoders fail closed without rewriting. Approval-key vectors follow the specified framing. Referenced snapshots survive collection. Unreferenced snapshots are removed only by an approved plan. Every file stays private and local. Recovery produces the specified old or new state without losing the current merge base or installed snapshot. |
| Automated evidence | `internal/state.TestWorkflowStateSchemaMatrix`, native-approval known-answer vectors, snapshot reference and collection tests, descriptor-identity tests, and crash-boundary tests. |
| Approval | Required before catalog activation or semantic sync stores workflow state. |

### SW-015 keeps user-facing language clear

| Field | Required evidence |
| --- | --- |
| Initial state | Every success, empty, warning, blocked, conflict, cancellation, and recovery result for the new CLI and TUI views. Fixtures include unsafe names and terminal-control bytes. |
| Operation | Render plain CLI and TUI copy from the same typed results. Compare it with JSON output and internal diagnostics. |
| Expected state | Plain text leads with the outcome, uses the vocabulary in this document, and gives one exact next step. It does not expose internal state names without an explanation. JSON values and diagnostic codes remain stable. Failed operations say that managed files were left unchanged when true. |
| Automated evidence | Shared-copy fixture tests, forbidden-word tests for user-facing strings, terminal-control canaries, and golden output for narrow and wide layouts. |
| Approval | Required before each new CLI or TUI workflow ships. |

### SW-016 supports selectable shortcut profiles

| Field | Required evidence |
| --- | --- |
| Initial state | Native Windows, WSL, macOS, and Linux default detection; Windows, Linux, and macOS profiles on each supported operating system; terminals that deliver and consume Super keys; every form, confirmation, page, text field, and select mode. |
| Operation | Show, save, reload, test, and switch each profile. Invoke every preferred key and fallback through model and PTY tests. |
| Expected state | An absent saved choice resolves to Windows on native Windows and WSL, macOS on Darwin, and Linux on other supported Linux systems. A saved choice wins on every system. Actions, help, and footers use one shared key map. Missing Command-key support leaves every action reachable through its fallback. Copy and paste remain terminal-owned. `Esc` always cancels or goes back. Modified keys that are not actions never enter text. |
| Automated evidence | Platform-default table tests, key-map table tests, help parity tests, config v2 tests, Bubble Tea modifier tests, PTY fallback tests, and tests that `al shortcuts test` does not change the saved shortcut choice. |
| Terminal evidence | Windows Terminal under WSL, one Linux terminal, macOS Terminal, and one macOS terminal with an enhanced keyboard protocol. |
| Approval | Required before shortcut profiles ship. |

## Review record

A second-pass review on 2026-09-18 checked this draft against `AGENTS.md`, `TODO.md`, `docs/COMPATIBILITY.md`, the approved shell-neutral architecture, phase 2 through phase 4 acceptance criteria, and the current command, configuration, and catalog code.

The review found and corrected these blocking ambiguities:

- Catalog and application configuration version 1 could not accept profile fields. The draft now requires explicit version 2 models and version-dispatched decoding.
- A public status command could have triggered journal recovery. The draft now makes `al status` observational and reports `recovery_required`.
- A public plan fingerprint was not reproducible. The draft removes it and requires every mutation to rebuild its plan under the lock.
- Status enum changes could have broken exhaustive JSON consumers. Version 1 values are now fixed.
- Semantic merge had hashes but no durable base bytes. Private canonical snapshots now supply merge and installed comparison inputs.
- Remote bootstrap did not separate preview from repository creation. Preview now uses a bounded provider API read, while application uses a verified no-checkout staged clone.
- Delivery phase numbers conflicted with the existing shell-neutral rollout. This document now uses named delivery stages.

No unresolved product-boundary, compatibility, or safety blocker remains in the draft. Independent approval is still required before any implementation stage starts.

## Review checklist

Before approval, the reviewer must answer these questions in the approval record:

- Does any feature cross the product boundary into general dotfile management or code execution?
- Can any read-only command recover a transaction or make another persistent write?
- Can any version 1 catalog or configuration accept a version 2 field silently?
- Can a stale preview authorize a write?
- Can sync, bootstrap, or a broad confirmation approve native content?
- Can a missing merge base cause a two-way overwrite?
- Can rollback work when the catalog, repository, network, or provider is unavailable?
- Do legacy Bash and Zsh installations keep their files, startup behavior, sync sources, and stable commands until explicit adoption?
- Are paths, catalog bodies, credentials, conflict bytes, and profile lists absent from logs and default machine-readable output?
- Can a person understand each plain result without knowing Git, JSON, shell parsing, or Alias Lens state names?
- Does every shortcut profile keep each action reachable when the terminal consumes its preferred modifier?
- Does each implementation phase cite approved criteria and mapped tests before code changes begin?
