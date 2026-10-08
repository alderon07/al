# Complete catalog workflow and bootstrap

Approved for implementation by the user on 2026-10-03. This plan supersedes conflicting pre-release lifecycle details. Native platform evidence remains a release gate.

## Product contract

`al init SOURCE [--shell bash|zsh] [--catalog-path PATH] [--apply]` and `al catalog enable [--shell bash|zsh] [--apply]` compose discovery, validation, exact native-code review, exact ownership review, final confirmation, and transactional installation. Focused preview, import, review, approve, adopt, rollback, diff, status, and plan commands remain available. Interactive decisions stay in memory until final application. Noninteractive application cannot invent native approval or ownership. Cancellation before application changes no managed state.

Catalog and app configuration remain format 2. Public status, plan, and semantic diff remain version 1. Active installation uses Bash and Zsh renderer version 2; shadow version 1 outputs keep their existing meaning. Approval version 2 binds ID, shell, name, kind, implementation hash, and renderer. Unsupported development formats are rejected without migration.

Initially adopted native definitions remain unchanged as fallbacks. Subsequent reviewed enablement updates exact enrolled ranges. Rename, deletion, and intentional condition exclusion affect those fallbacks too. Unrelated edits require fresh review; no fuzzy relocation is allowed. Original enrollment bytes remain available for offline rollback. New catalog-only entries do not gain native fallbacks automatically. A pending or unavailable replacement of any installed entry blocks the complete replacement generation. Initial installation can omit valid unapproved or unavailable entries; unsafe declarations block it.

Startup sources native definitions once, then evaluates only complete verified output from a pinned read-only helper. The helper checks pointer, package, structural declarations, and expected native-input hash, without consulting editable catalog/config or performing recovery, writes, validators, or network calls. Missing or invalid artifacts leave native fallbacks intact. Dynamic or ambiguous startup routes require explicit placement. Known owned source blocks can be replaced; proven single user source statements are preserved with a generation-only block. One block is required per startup path, once per process route. Bash login precedence and Zsh discovery are preserved.

All managed mutations share the persistent OS-backed state-directory mutation lock. Sessions pass explicitly through nested operations. Multi-target transactions name exact roots and targets, use target-local temporary files, retain private inverse payloads, and record forward and recovery progress. Recovery restores native bytes before pointer, startup blocks, and installed state. Shell file metadata and link semantics must be preserved or the write refused.

Generation identity covers renderer, shell, platform, source snapshot, declarations, exact installed IDs, confirmed approval keys, executable resolutions, and expected native-input hash. Complete file hashes are separate. Portable executables resolve to absolute external paths in Go from explicitly captured PATH. Invalid/relative/empty search paths are rejected. All declarations are structurally checked before isolated parse-only validation; discovery never executes them.

Catalog sync owns one path and exact merge base. Native sync is for configured legacy shells only. Automatic sync never replaces or activates a catalog. Explicit pull performs semantic combination and preserves conflicts privately. Push scans all decoded entries and outgoing enrolled history, commits only the selected path, and never force-pushes. An uncertain push is reconciled through its pinned durable intent.

Remote init previews through provider HTTPS APIs, pins immutable revision and blob, and uses a restricted staged no-checkout, no-hook, no-submodule, depth-one, blob-filtered clone. Only the enrolled file is materialized through plumbing. Discovery is bounded to two minutes and 8 MiB; clone to five minutes and 256 MiB. Unsupported capabilities stop with a manual local enrollment path. No full-clone fallback is allowed.

## Delivery order and review

1. Freeze acceptance contracts and persistent formats.
2. Implement strict storage, shared sessions, multi-target transactions, and recovery.
3. Implement adapter validation, rendering, startup installation, ownership, and offline rollback.
4. Implement catalog editing, installed entry facade, TUI review, picker, and completion membership.
5. Connect semantic sync and mixed-mode dispatch.
6. Implement local init, then bounded provider and managed-repository remote init.
7. Complete documentation, end-to-end verifier, and evidence.

Each bounded stage uses medium-effort coding, a separate high-effort review, medium-effort fixes, and repeated high-effort review until acceptance, security, correctness, and compatibility findings are resolved. No competing writers edit the same files. Run `make fmt check` after each behavior stage. Preserve the existing user edit to AGENTS.md. Add no explanatory code comments or unrelated dependencies.

## Acceptance mapping

| Criterion | Required behavior and automated evidence |
| --- | --- |
| CL-001 / SW-004 / SW-014 | One lock; exact target roots; symlink/hard-link and metadata policy; forward and recovery crash matrix, including a second crash and promotion. `internal/transaction` workflow tests and command mutation-session tests. |
| CL-002 | Native and generated loading once for each supported startup route; exact pointer/package checks; readonly and masked-name handoff; fallback drift; inherited/static/dynamic ZDOTDIR; Bash precedence; new-shell PTY tests at transaction boundaries. |
| CL-003 | Exact per-entry approval and ownership; no executable discovery; retained and refreshed fallbacks; immutable original rollback baseline; cancellation and stale decisions; approval, adoption, and rollback tests plus PTY review. |
| CL-004 / SW-005 | Stable typed edits with revisions; shared declared/installed/native facade; no pending candidate execution; exact installed completion membership; CLI/TUI parity and narrow/wide PTY tests. |
| CL-005 / SW-008 / SW-009 / SW-010 / SW-013 | Semantic merge from saved bytes; private conflicts; mixed mode; no automatic catalog replacement or activation; path-only commits and uncertain push reconciliation. Temporary repository isolation and sync PTY tests. |
| CB-001 / SW-011 | Local working-tree preview preserves index/worktree and pins HEAD, file bytes, identities. Init preview, cancellation, stale-source, and local PTY tests. |
| CB-002 / SW-011 | Same-host HTTPS immutable discovery with synthetic credentials, bounded responses, changed-object refusal, supported provider hosts, access filtering, and SSH fallback tests. |
| CB-003 / SW-011 | Restricted clone, filters/hooks/submodules remain inert, enrolled path only, capability refusal, process-group cancellation, size/deadline bounds, promotion and repeated recovery tests. |
| CL-006 / SW-012 / SW-015 | Disposable-home preview/import/review/adopt/enable/edit/sync/conflict/rollback/init workflow; accurate help/privacy/data paths; `make fmt check`; synthetic end-to-end verifier. |

Tests set HOME, shell, XDG, history, Git, and provider paths to disposable data. No production secrets, aliases, or repositories enter fixtures, logs, evidence, or screenshots. Native Ubuntu, macOS, and WSL restart evidence remains separately recorded before release.

## Implementation map

Related records and operations share modules in the implementation. Existing codec, semantic merge, state observation, and transaction primitives remain reusable.

| Area | Modules and reason |
| --- | --- |
| Persistence | `internal/catalogstore/types.go`, `codec.go`, `validate.go`, `generation.go`, `private_unix.go`, `private_windows.go`, and `sync.go`: strict private records, canonical generation identity, safe reads, and enrollment/push intent. Synthetic format vectors live in package tests. |
| Transactions | `internal/transaction/workflow*.go`: coordination, exact targets, durable journal, ordered recovery, and platform descriptor operations. `cmd/alias-lens/mutation.go` supplies the shared session to legacy and catalog writers. |
| Installation | `catalog_lifecycle.go`, `catalog_adapter.go`, `catalog_native.go`, `catalog_startup.go`, `catalog_loader.go`, `catalog_review.go`, and `catalog_rollback.go`: validation, executable resolution, safe loading, exact review, fallback ownership, and offline baseline restore. Startup route decisions are adapter methods in `shell.go`; `shell_handoff.go` handles legacy handoff. |
| Editing and observation | `catalog_writer.go`, `catalog_entries.go`, `catalog_status.go`, `catalog_check.go`, and `tui_catalog.go`: typed edits and revisions, exact installed membership, shared status/health, terminal review, and conflict choices. Existing search, export, completion, metadata, revision, and TUI callers use those services. |
| Synchronization | `catalog_sync.go`, `managed_git.go`, `managed_git_unix.go`, `managed_git_windows.go`, and `repository_managed.go`: semantic pull/conflicts, safe path commits, pinned uncertain pushes, controlled transport, bounded cloning, and journaled promotion. Existing repository/autosync modules dispatch enrolled units. |
| Bootstrap and commands | `init.go`, `providers_catalog.go`, `catalog_commands.go`, and `command_spec.go`: local/immutable remote enrollment, common guided review, shared command/help/completion metadata, and CLI routing. |
| Evidence | Corresponding package tests, `catalog_sync_remote_test.go`, `autosync_units_test.go`, `init_pty_test.go`, catalog/TUI/startup PTYs, `scripts/verify-catalog-workflow.sh`, and `docs/testing/evidence/CATALOG_REVIEW.md` record the acceptance and review gates. |
