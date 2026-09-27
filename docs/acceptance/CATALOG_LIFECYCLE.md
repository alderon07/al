# Complete catalog mode for Bash and Zsh

Status: proposed for review on 2026-09-27. This document defines the remaining opt-in catalog workflow and its acceptance criteria. Implementation of activation and rollback starts after review of the transaction contract and its test mapping. The phase 4 WSL restart run and native platform checks remain release gates.

This document implements the decisions in [Shell-neutral alias architecture](../SHELL_NEUTRAL_ARCHITECTURE.md) and [State, planning, and portability specification](../STATE_WORKFLOW_SPEC.md). Those documents control any case this one does not cover. The existing `al catalog preview`, `import`, `shadow`, and `diff` commands keep their meanings.

## Goal and current state

The complete opt-in workflow lets a user import aliases, review native implementations, enable catalog mode for Bash or Zsh, edit catalog entries, sync the catalog, and roll back offline. The Bubble Tea interface remains the default way to browse and edit entries. Existing installations stay in legacy mode until the user enables a shell explicitly.

The current code has catalog format 2, import, shadow inspection, semantic comparison, condition resolution, rendering, native approval keys, state reporting, a three-way merge function, and a journal for private files under one directory. It has no catalog loader, active generation, adoption command, native review command, catalog editor, catalog sync path, or catalog rollback. `applyPrivatePlan` cannot cover a startup file and a native alias file in one transaction. Activation must use a new shared workflow transaction before either file changes.

One-command bootstrap and shell completion installation use the same planned writer when they need it. They do not determine whether catalog mode is usable, so they follow the core workflow. Fish, automatic migration, and making catalog mode the default for new installations follow the separate rollout gates in the architecture.

Catalog format 2 permits portable external commands and Bash or Zsh native implementations. A typed `change_directory` entry is a separate catalog format change. It needs its own path semantics, renderer contract, and acceptance criteria before any shell adapter supports it.

## Ownership and paths

| Path | Owner and purpose |
| --- | --- |
| `~/.config/alias-lens/catalog.json` | User's declared catalog. The source for catalog edits and catalog sync. |
| `~/.config/alias-lens/generated/<shell>/<hash>.<ext>` | Immutable Alias Lens output for one shell. |
| `~/.config/alias-lens/generated/<shell>/active` | Private regular file containing one generation hash. The startup loader reads it. |
| `~/.bash_aliases` or `~/.zsh_aliases` | User's native file. Unadopted entries and the current `al` shell integration remain here. |
| Bash or Zsh startup file | User's file containing one exact Alias Lens loader block. |
| `~/.local/state/alias-lens/catalog-state.json` | Local installed-shell and catalog-sync hashes. No command bodies. |
| `~/.local/state/alias-lens/native-approvals.json` | Local content-bound approvals. No command bodies. |
| `~/.local/state/alias-lens/adoptions.json` | Local, inactive removal intents. No command bodies. |
| `~/.local/state/alias-lens/catalog-snapshots/` | Canonical catalog bytes needed for installed comparison and merge bases. |
| `~/.local/state/alias-lens/rollback/` | Private inverse edits and exact original bytes needed for offline rollback. |
| `~/.local/state/alias-lens/transactions/` | Recovery journals for multi-path workflow changes. |

Alias Lens synchronizes only `catalog.json` for catalog mode. It does not synchronize generations, approvals, profiles, adoption intents, rollback records, or local state. A legacy shell keeps its native alias file as a separate sync unit.

### Installed state record

`catalog-state.json` keeps schema version 1. Each `installed_shells` value has the existing `active_generation_sha256`, `source_catalog_sha256`, `resolved_state_sha256`, and `loader_sha256` fields. It gains `rollback_record_id`, `native_after_sha256`, and `startup_after_sha256`. The first points to a private rollback record. The hashes identify the exact native and startup files after activation. An empty `rollback_record_id` is omitted. The state decoder rejects unknown fields, duplicate shell keys, unsupported versions, invalid hashes, and unsafe record IDs before any mutation.

`adoptions.json` has schema version 1 and an `intents` array sorted by shell, source path, and byte offset. Each intent contains `entry_id`, `shell`, `source_path`, `source_sha256`, `start_byte`, `end_byte`, `range_sha256`, and `implementation_sha256`. Offsets refer to the exact source bytes inspected by `al catalog shadow`. The file contains no source lines. A source hash or catalog implementation change invalidates the intent. Invalid intents do not block unrelated catalog entries.

A rollback record contains the shell, operation ID, native path and expected post-enable hash, startup path and exact managed-block hashes, the previous pointer value or absence, the previous installed-state record or absence, and paths and hashes of private inverse payloads. The payloads contain exact removed native byte ranges and the prior owned startup block. A rollback record never depends on the current catalog, a repository, or a network response. Its format, field order, and checksum vectors are frozen in a golden test before activation code lands.

## Shell startup and activation

The versioned catalog loader first sources the selected shell's existing native alias file. It then reads the Alias Lens-owned `active` pointer, accepts exactly 64 lowercase hexadecimal characters and a final line feed, and sources only the matching regular generated file beneath that shell's generated directory. The pointer and generated file must not be symbolic links. Bash reads Bash files; Zsh reads Zsh files. No catalog data becomes a source path. The existing `al` function remains available from the native file.

The loader keeps unadopted native definitions available when the pointer is absent or invalid. Adopted definitions are different: after their native ranges are removed, a missing pointer or generated file can make those names unavailable in a new shell. Native removal remains blocked until the design specifies and tests a durable startup fallback for those removed definitions, including missing or unreadable pointer and generation files. A readable but corrupt generation needs an explicit validation and recovery policy; a shell loader must not claim to detect arbitrary corruption without one. An offline rollback record alone is not a startup fallback.

The loader sources the generated file after the native file, but source order alone does not settle a name collision. Bash and Zsh expand aliases while reading commands; an old native alias can mask a generated function or change how its declaration parses. Without an approved adoption intent, a catalog entry that collides with a native name stays out of the generated file. For an adopted name, the adapter must prove a shell-specific handoff that leaves the native definition usable until the generated replacement is defined and then gives the replacement precedence. If it cannot prove that handoff, enablement blocks that name and leaves the native definition intact. New-shell PTY tests verify the name before pointer replacement, between pointer replacement and native removal, and after native removal.

Adoption can also change definitions that are not adopted. Bash expands aliases while reading a function declaration. If a surviving native function used an alias that enablement removes, that function may have a different body when a new shell reads it before the generated file. An exact byte range and a working replacement for the adopted name do not prove that its removal is safe. The adapter needs a conservative source-order and dependency policy; ambiguous dependencies block automatic removal and require a manual migration. It must not execute user definitions to discover dependencies.

Activation must inspect the existing startup load path. A user's startup file may already source the native alias file before or after the managed block. Adding a loader that sources it again can repeat top-level effects, and sourcing it later can override the generated definitions. An unmanaged or ambiguous source path blocks activation with a manual placement instruction. Alias Lens does not delete the user's source lines. Catalog names that would replace `al`, `alias-lens`, or Alias Lens integration helpers also block activation; approval of a native body does not authorize replacement of the control commands.

The product reports that an installed generation is ready for new shells. A running shell keeps the definitions it already loaded until the user reloads or starts a new shell.

`al catalog enable --shell bash|zsh` is repeatable. It resolves the current catalog for one shell and platform, excludes entries outside local profiles, and keeps unapproved native implementations out of the generation. Before removing an adopted native definition, the plan proves that the same entry is present in the validated generation and that the startup fallback is durable. An unapproved or unavailable entry stays in the native file. Portable commands resolve to external executables without shell lookup. The current renderer emits `command 'program'`; this does not yet establish a stable executable identity. Before activation, the adapter must resolve the external executable for the selected machine, render its validated absolute path, and include that resolution in the generation identity. The adapter verifies that each native declaration has no extra top-level command or declaration redirection, then checks the complete generated file with the target shell's isolated parse-only mode. Syntax checking alone cannot prove that a declaration is inert when sourced. A renderer or validation failure leaves the prior pointer in place. Re-enabling with unchanged inputs makes no file edit. A later enablement carries forward the original offline rollback record and adds inverse payloads for any newly adopted entries.

If the named shell has no Alias Lens setup, the plan includes its own startup integration and names every file it will change. `al catalog enable` never modifies the other shell's paths. An edited Alias Lens loader block blocks replacement and shows `al setup --repair` only after a separate review of that block.

## One workflow transaction

All Alias Lens managed writes use one operating-system-backed lock at `~/.local/state/alias-lens/mutation.lock`. This includes legacy alias edits, settings, setup, completion installation, catalog edits, approvals, adoptions, activation, rollback, and both sync modes. The automatic sync worker uses the same lock. The existing time-based `sync.lock` can remain a worker-liveness signal, but it cannot authorize a write or replace the mutation lock. The current config-directory lock moves to this shared boundary before activation ships.

The current private-file journal remains available for settings and completion files. A new workflow coordinator extends its recovery rules across the Alias Lens config directory, state directory, and explicitly selected user-owned shell files. The journal stores exact target identities, hashes, modes, link policy, planned hashes, inverse payload references, and write order. It rejects any path outside those named roots. Alias Lens creates the journal and fsyncs it before the first target write. Each completed write and directory fsync gets a checksummed record.

Activation follows this order under the lock:

1. Recover or stop on an earlier incomplete workflow transaction. Rebuild the plan and compare every input and path identity with its preview.
2. Create the private catalog snapshot and rollback payloads. Back up each mutable file before changing it.
3. Write and validate the immutable generated file. Leave the current pointer untouched.
4. Replace only the exact owned startup block. It continues to source the native alias file.
5. Atomically write the active pointer. At this point new shells can load the generation.
6. Remove only approved adoption ranges whose generated replacements are active. Recheck the native file immediately before writing.
7. Atomically write the installed-state record and mark the journal committed after every target directory is durable.

Without a committed record, recovery restores the old state in dependency order: native definitions first, active pointer second, startup block third, and installed-state record last. It compares every current target with the recorded old or planned hash before acting. If a target has an unrecognized hash, missing backup, changed link, or ambiguous metadata, recovery leaves the active pointer and native file in the safest observed state, saves private conflict material, and reports `recovery_required`. It does not guess from timestamps. A committed record is finalized without replaying writes.

| Last durable boundary | State a new shell can see | Recovery action |
| --- | --- | --- |
| Before pointer replacement | Native definitions remain available. | Restore the prior owned loader block if it changed; keep the old pointer. |
| After pointer replacement, before native removal | Native definitions and the new generation are both available. | Restore the old pointer, then the prior loader block. |
| After native removal, before installed-state write | The new generation supplies adopted names. | Restore the removed native bytes, then the old pointer and loader block. |
| After installed-state write, before commit record | The new generation supplies adopted names. | Restore native bytes, pointer, loader block, and state record in that order. |
| After commit record | The new generation is installed. | Finish cleanup without replaying any target write. |

At any boundary, an unexpected target hash stops automatic recovery. If native bytes were removed and the new pointer still names a validated generation, recovery keeps that pointer until the original native bytes can be restored. The journal and private backups remain available for manual repair.

Normal rollback runs as another planned transaction. It restores adopted native ranges before it deactivates the generated pointer. It then restores the previous owned startup block and installed-state record. A changed user file blocks rollback before its first write and produces a private conflict copy and a manual recovery plan. The catalog can be absent or corrupt during rollback.

## Adoption and native review

`al catalog adopt NAME --shell bash|zsh` shows the catalog entry, native source path, line range, and the hash of the exact source span. It records only the inactive intent after the user confirms that exact entry. It does not edit the native file. It rejects a duplicate name, an ambiguous parser range, an implementation mismatch, a changed source hash, and an entry without a renderable replacement. Enablement rechecks that the replacement is present in the validated generation before it removes the native range.

`al catalog review --shell bash|zsh` shows pending native implementations one at a time. `al catalog approve NAME --shell bash|zsh` opens that review for one entry. The review includes the entry name, kind, shell, ID, implementation hash, and escaped complete implementation text in an interactive terminal. Approval requires a separate confirmation for that exact key. Neither command accepts a broad yes flag or approval from noninteractive input. The approval record uses the existing entry ID, shell, kind, implementation hash, and renderer key. A changed body, kind, ID, or renderer requires a new review. Sync and bootstrap never create approval records.

## Catalog editing and execution

When a shell is in catalog mode, the TUI edits `catalog.json` through a catalog writer that preserves unrelated entries and validates the complete document before an atomic write. Add, edit, delete, metadata, favorites, and profiles use the same typed catalog result. Each change creates a private revision and leaves the installed generation unchanged until `al catalog enable --shell SHELL` applies it.

The TUI shows installed, pending, unavailable, and native-only entries distinctly. The normal picker and `al shell-entry` use the installed snapshot for catalog-managed names and the native file for unadopted names. Pending entries cannot run through the picker. A changed catalog command cannot silently run its previous installed body under the new label. The TUI uses the same state, plan, diff, approval, and conflict results as the CLI, keeps selection on resize, and requires a specific final confirmation for file changes. The optional browser view remains optional and read-only unless it already has an explicitly approved write route.

## Catalog synchronization

The catalog is one sync unit with its own repository path, local hash, remote hash, and exact saved base snapshot. Push scans every portable argument and native implementation, including entries excluded on this machine. It stages and commits only the enrolled catalog path. A local alias file continues as a separate unit for any legacy shell.

Automatic sync can fetch and compare a remote catalog, but it does not replace the live catalog or activate a generation. It may push a local catalog only when the stored remote base still matches and the secret scan passes. An explicit `al sync --pull` performs the approved three-way merge by stable entry ID when the selected shell uses catalog mode. A selected legacy shell keeps the native pull behavior. If base bytes are missing, the catalogs are invalid, fields collide, or names collide, Alias Lens saves private base, local, and remote copies and leaves the live catalog unchanged. A clean merge writes a private revision and the new catalog, then reports that `al catalog enable --shell SHELL` is needed. Remote native content remains unapproved.

## Delivery and acceptance criteria

Each stage cites the SW criteria in `STATE_WORKFLOW_SPEC.md`. Automated tests use temporary homes and repositories. The implementation adds no code comments, and all terminal behavior gets PTY coverage. The full `make fmt check` gate runs after each behavior stage. Manual Ubuntu, WSL, and macOS checks remain release evidence.

### CL-001: one lock and recovery boundary

| Field | Required evidence |
| --- | --- |
| Initial state | Legacy Bash and Zsh files, both startup files, private config and state, symlinks, hard links, edited blocks, and an unrelated staged Git file in a temporary home. |
| Operation | Run two concurrent catalog or sync changes. Kill the writer after each journal, backup, temporary-file, rename, and directory-fsync boundary; then invoke the next mutating command. |
| Expected state | One writer holds the lock. Recovery reaches the exact old or new state. It restores native definitions before deactivating a pointer. An ambiguous target blocks recovery and preserves private evidence. Unrelated paths and staged files keep their hashes and metadata. |
| Automated evidence | `internal/transaction` crash-boundary matrix and `cmd/alias-lens` concurrent workflow tests. |
| Approval | Reviewer and date required before activation implementation. Maps to SW-004, SW-013, and SW-014. |

### CL-002: generated Bash and Zsh startup

| Field | Required evidence |
| --- | --- |
| Initial state | Clean and existing Bash and Zsh setups, Bash login-file precedence variants, Zsh `ZDOTDIR`, a native function that refers to an adopted alias, unmanaged startup sources before and after the managed block, native aliases and functions that collide with catalog aliases and functions, protected integration names, portable entries, a pending native approval, and adopted names whose generated files or pointers are later removed. |
| Operation | Preview and enable each shell twice, start new interactive and login shells in a PTY at every activation boundary, attempt a dependent or protected-name adoption, remove or corrupt the pointer and generated file, change `PATH`, then roll back offline. |
| Expected state | Only the selected shell changes. Native-only and `al` definitions remain available. An ambiguous native dependency or startup load path blocks activation without sourcing a file twice. Protected integration names never become catalog declarations. A collision without adoption keeps native behavior. An adopted name is callable before and after the handoff, including when startup must use its durable fallback. A readable corrupt generation follows the explicit validation and recovery policy. The generated implementation wins only after its declaration succeeds. The pointer accepts only a valid generation name. Portable entries execute the resolved external file despite a changed `PATH`. Repeated enablement makes no extra edit. Rollback restores the exact prior owned block and native definitions. Existing startup text and metadata remain intact. |
| Automated evidence | Loader golden tests, isolated validator tests, Bash 3.2 and 5.2 PTY tests, Zsh 5.9 PTY tests, and startup manifest comparisons. |
| Terminal evidence | Ubuntu, WSL, and macOS startup and rollback sessions. |
| Approval | Reviewer and date required. Maps to SW-004, SW-010, SW-013, and SW-014. |

### CL-003: adoption and native approval

| Field | Required evidence |
| --- | --- |
| Initial state | Safe and ambiguous definitions, duplicate names, stale source bytes, edited native implementations, and native code with a top-level side-effect sentinel. |
| Operation | Preview, import, review, approve, adopt, enable, change the approved body, and roll back. |
| Expected state | Preview and review execute nothing. Adoption intent changes no native file. Enablement removes only a byte-exact approved range with an active generated replacement. Changed native content stays inactive. Rollback restores the removed range without reading the catalog. |
| Automated evidence | Parser range tests, approval-key vectors, no-execution sentinels, file-manifest tests, and PTY review tests. |
| Approval | Reviewer and date required. Maps to SW-003, SW-004, SW-010, SW-013, and SW-014. |

### CL-004: one catalog editing and execution model

| Field | Required evidence |
| --- | --- |
| Initial state | Legacy mode, catalog mode, mixed Bash and Zsh modes, pending edits, missing implementations, and an installed snapshot that differs from the current catalog. |
| Operation | Add, edit, delete, favorite, search, pick, and run through CLI and Bubble Tea at narrow and wide PTY widths. |
| Expected state | Legacy commands keep their existing behavior. Catalog edits change only catalog bytes and private revisions. A pending body never runs through the picker. Installed entries run the recorded implementation. UI labels and status match typed CLI results. |
| Automated evidence | Catalog-writer tests, CLI parity tests, picker and shell-entry PTY tests, and TUI resize tests. |
| Approval | Reviewer and date required. Maps to SW-005, SW-010, SW-013, and SW-015. |

### CL-005: semantic sync and mixed mode

| Field | Required evidence |
| --- | --- |
| Initial state | Valid base, local, and remote catalogs; missing base; field and name collisions; native changes; unrelated staged files; Bash-catalog with Zsh-legacy and the reverse. |
| Operation | Run automatic comparison, explicit pull, push, conflict review, and offline retry. |
| Expected state | Auto sync never changes the live catalog. Explicit pull merges only nonoverlapping fields and never activates a generation. Conflict copies stay private. Push scans every entry and commits only the enrolled catalog path. Legacy sync keeps its selected native file. |
| Automated evidence | Three-way merge matrix, repository isolation and canary tests, mixed-mode sync tests, and TUI conflict PTY tests. |
| Approval | Reviewer and date required. Maps to SW-008, SW-009, SW-010, SW-013, and SW-014. |

### CL-006: complete user workflow

| Field | Required evidence |
| --- | --- |
| Initial state | A disposable home with Bash and Zsh aliases and no catalog, plus a second disposable machine repository. |
| Operation | Preview, import, review, adopt, enable Bash, edit and re-enable, enable Zsh, sync, pull a clean change, resolve a conflict, and roll back each shell. |
| Expected state | Every step names the next available command. Both shells keep working after each new shell starts. Rollback works with network disabled and catalog removed. `al data paths`, `docs/PRIVACY.md`, help, and compatibility docs name the new records and commands. |
| Automated evidence | End-to-end PTY workflow, `make fmt check`, `git diff --check`, and private-data scan of staged files before commit. |
| Terminal evidence | Ubuntu, WSL, and macOS evidence in the portable workflow checklist. |
| Approval | Reviewer and date required before a release-ready claim. Maps to SW-005, SW-013, and SW-015. |
