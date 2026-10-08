# Catalog lifecycle acceptance

Approved for implementation on 2026-10-03. [CATALOG_PLAN.md](../CATALOG_PLAN.md) defines the complete delivery order. [CATALOG_BOOTSTRAP.md](CATALOG_BOOTSTRAP.md), [CATALOG_STORAGE.md](CATALOG_STORAGE.md), [CATALOG_TRANSACTION.md](CATALOG_TRANSACTION.md), [CATALOG_SHELL_RUNTIME.md](CATALOG_SHELL_RUNTIME.md), and [CATALOG_SYNC.md](CATALOG_SYNC.md) freeze the detailed contracts and test mappings. Native macOS and WSL evidence remains a release gate.

## Guided operation

`al init SOURCE [--shell bash|zsh] [--catalog-path PATH] [--apply]` and `al catalog enable [--shell bash|zsh] [--apply]` compose discovery, structural validation, exact native review, ownership review, final plan confirmation, and application. Individual review, approve, adopt, rollback, and plan commands remain available. Decisions stay in memory until final confirmation. Cancellation saves none. Noninteractive application can use existing matching decisions and cannot approve new code or take over an unrelated collision.

Approval binds entry ID, shell, name, kind, native implementation hash, and renderer. Changes to any bound value require new review. Ownership binds the exact native input identity, complete file hash, and source range. An unrelated native edit requires renewed ownership review.

## Retained native fallbacks

Adoption keeps the original native declaration and records permission to manage that exact fallback. Later reviewed enablement refreshes it; deliberate rename, deletion, and profile exclusion update or remove only its enrolled range. New catalog-only entries gain no fallback automatically. The original pre-catalog native and startup bytes remain in an immutable offline rollback record independently of later refreshes.

A missing helper, pointer, or generation leaves the sourced native definitions available. Native drift declines the overlay and directs the user to `al catalog enable`. A failed replacement for an installed entry blocks the entire re-enable operation and preserves the previous generation and fallbacks. Initial installation may omit well-formed new entries needing approval or an unavailable executable; malformed data and unsafe declarations block application.

## Local records and rendering

| Path | Contract |
| --- | --- |
| `~/.config/alias-lens/catalog.json` | Declared catalog, format 2; only portable catalog data is synchronized. |
| `~/.config/alias-lens/generated/<shell>/<id>.{sh,json}` | Immutable complete shell file and canonical manifest. |
| `~/.config/alias-lens/generated/<shell>/active` | Exactly 64 lowercase hexadecimal bytes and one line feed. |
| `~/.local/state/alias-lens/catalog-installed.json` | Private format 1 installed records, generation and rollback identity, all startup routes. |
| `~/.local/state/alias-lens/native-approvals.json` | Private approval format 2, exact content/name-bound keys. |
| `~/.local/state/alias-lens/adoptions.json` | Private format 1 exact ownership ranges and fallback bytes. |
| `~/.local/state/alias-lens/catalog-snapshots/` | Canonical source snapshots referenced by installed packages. |
| `~/.local/state/alias-lens/rollback/` | Original enrollment baseline and private inverse payloads. |
| `~/.local/state/alias-lens/workflows/` | Durable multi-target workflow format 2 journals and recovery progress. |
| `~/.local/state/alias-lens/mutation.lock` | One persistent OS-backed mutation lock. |
| `~/.config/alias-lens/catalog-sync.json` | Exact repository path, saved semantic base, pinned revision, and push intent. |
| `~/.config/alias-lens/catalog-conflicts/` | Private base/local/remote conflict bundles. |

The active renderer IDs are `bash/v2` and `zsh/v2`. Shadow renderer v1 meanings and golden outputs remain stable. Public status, plan, and semantic-diff JSON remain version 1. Private codecs reject duplicate and unknown keys, unsupported versions, invalid hashes/IDs/timestamps, unsafe paths, and oversized documents.

A generation identity covers the source catalog hash, machine resolution inputs, exact installed entry IDs and declarations, approvals, executable paths, and expected native-input hash. Complete file bytes have their own digest. Timestamps are excluded from identity. Immutable files under an existing identity cannot be replaced with different bytes.

Portable commands resolve an external absolute executable in Go using captured `PATH`; empty and relative components are rejected. Application rechecks the selected path. Native declarations require structural validation as one intended declaration without extra top-level code, redirection, or definition-time expansion. Protected names and ambiguous alias dependencies block application. Isolated shell syntax validation supplements this structural check.

## Startup routes and handoff

`ShellAdapter` discovers and plans startup paths. Bash preserves login-file precedence and records both login and nonlogin routes. Zsh accepts inherited or narrowly parsed static `ZDOTDIR`; dynamic paths require explicit enrollment. Replace an exact known Alias Lens source block in place, or preserve one proven user source/Ubuntu guard and insert the generation-only block immediately afterward. Duplicate, dynamic, conditional, or unreachable routes require placement guidance.

The pinned read-only helper verifies and buffers the complete immutable package, private paths, pointer, declaration constraints, and live native-input hash before emitting any definition bytes. Failure emits none. The helper reads no editable catalog/configuration, runs no validator process, performs no recovery or writes, and starts no network work or watcher. The shell evaluates only a complete successful result. A replacement function must be defined successfully before a masking alias is removed. Readonly conflicts are preflighted and shell options are preserved.

Installed means ready for new shells. A running shell keeps its previously loaded definitions. Pickers, `shell-entry`, and completions use exact installed membership. A pending changed implementation cannot run under an installed label.

## Shared application and recovery

All managed writers use one explicit mutation session. No lock is held while displaying confirmation. A manifest names private roots and exact enrolled user/repository targets; it never authorizes broad writes beneath the home directory. Replacements use temporary files beside their targets, so separate mounted filesystems remain supported. Unsupported metadata and changed identities cause refusal.

Activation prepares validated artifacts and baseline rollback material, then records confirmed decisions, refreshes owned fallbacks, installs startup blocks, replaces the pointer, and records installed state. Recovery restores native content before changing the pointer, startup next, and installed state last. Recovery journals each step and resulting identity so a second crash can resume without mistaking an inverse rename for an external edit. Unrecognized bytes or identities preserve artifacts and stop automatic recovery.

Offline rollback is another displayed transaction. It restores the original enrollment baseline before deactivation. It can restore a deliberately renamed or deleted alias; confirmation explains this. It works without catalog, network, or generation availability. Unrelated user drift blocks automatic overwrite.

## Catalog edits and synchronization

Catalog-mode edits preserve stable IDs and unrelated fields, write private revisions, and leave installed generations unchanged until enablement. CLI and Bubble Tea share installed/pending/unavailable/native results, exact review decisions, plans, and conflict results. The terminal remains the default interface.

Catalog synchronization owns one enrolled path and a saved merge base. A catalog shell's local fallback is not a second sync unit. Legacy shells retain native sync. Explicit pull combines semantic fields through a plan, saves private conflict bundles on ambiguity, and never grants approval or activates entries. Automatic sync may inspect remote changes and safely push unchanged-base local changes, but cannot replace the live catalog. Push scans all entries and outgoing enrolled-path history, commits only that path, and preserves unrelated index/worktree state. A durable pinned push intent handles uncertain results through remote inspection; local recovery cannot reverse a remote push.

## Delivery evidence

Medium implementation is followed by separate high review, medium corrections, and repeated high review. Run `make fmt check` after each behavior stage. [CATALOG_REVIEW.md](../testing/evidence/CATALOG_REVIEW.md) records findings and sanitized verification. `scripts/verify-catalog-workflow.sh` uses disposable homes and actual Bash/Zsh PTYs. Required native release evidence is tracked separately and cannot be inferred from cross-compilation.

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
| Expected state | Preview and review execute nothing. Ownership enrollment changes no native file. Initial enablement retains enrolled native fallbacks. Reviewed re-enable refreshes only exact owned ranges; deliberate rename, deletion, or exclusion is displayed before editing those ranges. Native drift requires renewed ownership review. Rollback restores the enrollment baseline without reading the catalog. |
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
