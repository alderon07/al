# Shell-neutral alias architecture proposal

Status: architecture approved on 2026-09-16. This document does not authorize implementation, an automatic migration, or a support claim for another shell. Each implementation phase remains blocked by the acceptance criteria gate below.

Review record: independently reviewed by `gpt-5.6-sol` at high reasoning effort. The final review found no unresolved P0 or P1 architecture blockers and approved the document only after the safety corrections were incorporated.

## Decision

Alias Lens will store entries that it manages in a versioned, shell-neutral catalog. Each shell adapter will render that catalog into a private native file. Existing Bash and Zsh alias files will remain user-owned unless the user adopts an entry through an explicit migration command.

The catalog will share metadata and simple command intent across shells. It will not translate arbitrary shell code. Complex entries can contain separate native implementations for each shell.

## Safety requirements

Protecting the user's shell is more important than completing a migration. Alias Lens must stop without changing files when it cannot prove that a write is safe.

The implementation must follow these rules:

- Keep catalog mode disabled for existing installations until the user enables it.
- Never edit a shell file because it exists. Setup must detect that shell or receive its name from the user.
- Never source one shell's file from another shell.
- Never execute an entry while importing, parsing, validating, rendering, previewing, or syncing it.
- Write generated definitions to Alias Lens-owned files. Do not regenerate an entire user-owned startup or alias file.
- Add startup integration only inside exact Alias Lens markers.
- Refuse to remove a managed block when its contents no longer match the recorded block hash. Show the difference instead.
- Preserve unrelated text, comments, line endings, permissions, ownership, and supported filesystem metadata. Apply the explicit link policy below instead of assuming an atomic rename preserves links.
- Acquire one operating-system-backed mutation lock before any catalog, generated-file, startup-file, native-file, configuration, sync, adoption, enablement, repair, removal, or rollback mutation. Never steal this lock based only on elapsed time.
- Recheck each source-file hash immediately before writing. If the file changed after preview, cancel the operation.
- Create a private backup and timestamped revision before replacing or removing any pre-existing mutable target file. New transaction artifacts, backups, revisions, temporary files, and immutable generations are exempt because they have no prior user state to preserve.
- Use no-follow, descriptor-based file checks where the operating system supports them. Recheck path identity and metadata before committing a write.
- Write a temporary file in the target directory, set and verify its metadata, flush it, rename it atomically, and flush the directory.
- Validate every generated file with the target shell's parse-only mode before replacement.
- Keep the previous generated file active when rendering or validation fails.
- Make migration, enablement, repair, and removal idempotent.
- Provide a rollback command before catalog mode can become the default.

No migration command may delete an original definition during import or adoption. Deletion requires approved adoption intent and occurs only during transactional enablement, after a preview names the file and line range and the replacement is active.

### Filesystem link and metadata policy

Alias Lens-owned catalog, state, generated, lock, backup, and revision paths must not be symbolic links or hard links. Their parent directories must resolve beneath the expected Alias Lens configuration or state directory. Alias Lens refuses non-regular files, broken links, owner mismatches, and path changes detected between planning and application.

A user-owned alias or startup path may itself be a symbolic link. Alias Lens records and rechecks the link identity and target, requires the resolved target to be a regular file owned by the user, and writes the resolved target without replacing the link. A broken or retargeted link stops the operation. Parent-directory symlinks must resolve to the same recorded directory and stay within the user's home unless the user explicitly enrolled an external path. Hard-linked user files are read-only to Alias Lens until a design can preserve their link semantics safely.

A mutation plan records file type, device and inode identity, link target, link count, owner, group, mode, and supported access-control or extended-attribute metadata. If the platform cannot preserve required metadata, Alias Lens refuses the write and gives a manual recovery path.

The mutation lock serializes Alias Lens processes. Descriptor and identity checks detect an uncooperative external writer through the final check, but portable filesystems do not provide an atomic compare-and-swap rename against an expected inode. A different program can still replace a path between that check and Alias Lens's rename. Alias Lens documents this boundary, keeps the interval minimal, retains the backup and journal for recovery, and does not claim impossible cross-process exclusion. Platform-specific stronger primitives may be added only with a tested conservative fallback.

For Zsh, an inherited `ZDOTDIR` is accepted as discovery input. Alias Lens may recognize a narrowly parsed static assignment in `.zshenv`, but it never executes `.zshenv` to discover a path. Dynamic assignments require `al setup zsh --startup-file <path>` and an explicit preview confirmation. Confirming an external path records its resolved directory as enrolled state; later changes require confirmation again.

## Storage layout

Alias Lens will keep the catalog separate from application settings:

```text
~/.config/alias-lens/
├── config.json
├── catalog.json
└── generated/
    ├── bash/
    │   ├── active
    │   └── <generation-hash>.sh
    ├── zsh/
    │   ├── active
    │   └── <generation-hash>.zsh
    └── <future-shell>/
        ├── active
        └── <generation-hash>.<native-extension>

~/.local/state/alias-lens/
├── catalog-state.json
├── mutation.lock
├── transactions/
│   └── <operation-id>.journal
└── revisions/
```

`catalog.json`, generated files, and journals use mode `0600` where the operating system supports Unix permissions. Their directories use mode `0700`. `catalog-state.json` records per-shell installed state, the last successful catalog hash, generation hashes, active-generation pointers, installed startup blocks, and source hashes used by the last migration.

Only `catalog.json` is portable. Sync must not commit generated files or local state. The secret scan must inspect portable commands and every native implementation before a push.

## Catalog model

Every entry has a stable ID. A rename changes the name but keeps usage data and sync identity.

```json
{
  "schema_version": 1,
  "entries": [
    {
      "id": "87f4d803c44a4d8792c4824f8e0bc3f1",
      "name": "gs",
      "kind": "command",
      "description": "Show repository status",
      "category": "git",
      "tags": ["daily", "git"],
      "platforms": ["linux", "macos", "wsl"],
      "favorite": true,
      "portable": {
        "program": "git",
        "args": ["status", "--short", "--branch"],
        "pass_arguments": true
      },
      "native": {}
    }
  ]
}
```

The writer must sort entries and metadata fields deterministically. Unknown schema versions must fail without rewriting the catalog.

### Portable commands

A portable command contains an executable name, an argument array, and an argument-passing rule. It cannot contain a pipe, redirection, command substitution, variable assignment, wildcard expansion, or shell builtin that changes shell state.

This restriction is intentional. An argument array can represent `git status --short` without sharing quoting rules. It cannot safely represent a Bash pipeline or a command such as `cd` that must change the current shell.

The catalog validator rejects NUL bytes, newlines in names or executable paths, empty executable names, names invalid for any selected target shell, and entries over documented size limits. Empty arguments, Unicode, whitespace, quotes, leading dashes, and wildcard characters in arguments remain literal data. `pass_arguments: true` appends the caller's arguments exactly once, including an empty argument; `false` appends none. A matching native implementation takes precedence over a portable implementation for that shell. Platform exclusions prevent rendering and execution on excluded platforms.

Each adapter renders portable commands with shell-native literal quoting and without evaluating catalog text. Invocation preserves the child process exit status and signal result. The model applies a versioned token denylist as a usability filter, but the adapter owns the safety boundary: it resolves the program to an external executable without shell lookup and rejects aliases, functions, builtins, reserved words, hashed commands, and paths supplied by catalog data. Acceptance tests must define the exact behavior for each supported shell rather than assume Bash and Zsh are equivalent.

### Native implementations

An entry can define a structured native implementation when portable execution cannot preserve its behavior:

```json
{
  "id": "f98010ca0aa84af69fd4df32ec91726b",
  "name": "cproj",
  "kind": "function",
  "native": {
    "bash": {
      "function_body": "cd \"$HOME/code/projects/$1\""
    },
    "zsh": {
      "function_body": "cd \"$HOME/code/projects/$1\""
    }
  }
}
```

A native implementation is not an unrestricted startup-file fragment. A command entry stores exactly one `alias_value`. A function entry stores exactly one `function_body`. The adapter supplies the declared name, delimiters, and complete native declaration. The model rejects a generic source field and a field that does not match the entry kind. Only an adapter has enough shell grammar knowledge to reject an imported whole declaration, leading or trailing top-level commands, declaration redirection, and unsafe definition-time expansion. The adapter parses an accepted declaration into this structure without executing it, then renders the structure with the entry's validated name and kind.

Alias Lens must not infer a missing native implementation from another shell's source. The TUI will label the missing implementation and block execution on that shell. A newly synced or changed native implementation remains inactive until the user reviews and accepts it locally. Import, validation, rendering, preview, sync, and startup tests use sentinels to prove that native content cannot cause a top-level side effect.

## Separate plans from file writes

Shell adapters must describe changes without applying them. The core writer will validate and apply the plan.

```go
type ShellAdapter interface {
	Name() string
	Discover(context DiscoveryContext) (ShellFiles, error)
	ValidateName(name string, kind EntryKind) []Diagnostic
	Import(path string, contents []byte) ImportResult
	Render(catalog Catalog) RenderResult
	Validate(path string, contents []byte) []Diagnostic
	RenderInvocation(entry Entry) (Invocation, error)
	PromptInsertion(entry Entry) (PromptText, error)
	StartupPlan(home string, generatedPath string) MutationPlan
	RemovalPlan(home string, state InstalledState) MutationPlan
	Integration() string
	Bindings() BindingPlan
	History(home string) HistorySource
}
```

`DiscoveryContext` contains the selected shell, operating system, home, and an allowlisted set of shell-specific values such as `ZDOTDIR`. Adapters must not read arbitrary environment variables during discovery.

`MutationPlan` will contain every target path, expected content hash and path identity, original filesystem metadata, planned content, inverse edit, backup path, and validation command. Preview commands print this plan. Only the shared writer can apply it.

This split keeps path checks, locking, backups, hash checks, atomic replacement, and rollback out of individual adapters. An adapter cannot write an unexpected file.

The current global Bash-shaped parser, writer, syntax checker, history reader, execution code, prompt insertion, name validation, and key bindings must move behind `ShellAdapter` before another shell is marked as supported. Installed state is recorded per shell. Setting up or removing one shell must not change another shell's integration or legacy sync source.

## Generated files and startup integration

Each successful render creates an immutable generated file named by a generation hash. The generation hash is SHA-256 over an unsigned 64-bit big-endian length-framed sequence containing renderer ID (`bash/v1` or `zsh/v1`), platform, the native-approval input hash as 32 raw digest bytes, and rendered definition-body bytes, in that order. The body excludes the generated-file header, which prevents a circular hash. For renderer `bash/v1`, platform `linux`, the raw SHA-256 digest of empty bytes as the approval hash, and body bytes `# body\n`, the generation hash is `06fc5ce2f0aaa98290cf5ceecb582c406e0b4249606891779927bcf8f86bc205`. A generated file begins with a warning and records the generation hash, source catalog hash, shell, renderer version, platform, and native-approval input used to build it:

```text
# Generated by Alias Lens. Do not edit.
# generation-sha256: ...
# catalog-sha256: ...
# renderer: bash/v1
# platform: linux
# native-approval-sha256: ...
```

The shell startup file receives one small loader block. The loader uses versioned begin and end markers. Alias Lens records the exact block and its hash in local state.

The loader resolves one Alias Lens-owned active-generation pointer. The pointer is a regular private file, not a symlink, and contains only a fixed-length lowercase hexadecimal generation hash. The loader rejects any other value and sources only the matching filename beneath that shell's generated directory. Updating that pointer is the only activation point.

A durable transaction journal starts with an atomically created and fsynced manifest containing the operation ID, original hashes and metadata, planned hashes, backups, and inverse edits. Checksummed append-only records note each completed write and fsync boundary. Recovery compares the manifest, valid records, active pointer, and target hashes. It ignores a truncated final record. A corrupt or ambiguous journal preserves the last working native and generated definitions, performs no destructive edit, and requires manual recovery with private conflict material. On every mutating command and shell integration status check, Alias Lens recovers or rolls back an incomplete transaction before doing new work.

Setup follows this order:

1. Detect or accept the target shell.
2. Discover the shell's real startup path without creating a higher-priority startup file.
3. Build a mutation plan and show it when the operation is a migration.
4. Create and fsync the transaction journal before the first file mutation.
5. Render a new immutable generation to a temporary path.
6. Validate the temporary file in a clean target-shell process that does not load user startup files.
7. Back up and fsync each file that will change.
8. Recheck expected hashes, path identities, and filesystem metadata.
9. Commit and fsync the immutable generated file without changing the active pointer.
10. Add and fsync the loader block while native definitions remain active.
11. Atomically update and fsync the active-generation pointer.
12. Apply only approved inverse edits that remove adopted native definitions, then fsync them. A crash before this step may leave a temporary duplicate but never a missing definition.
13. Record successful hashes and mark the journal committed only after every write is durable.

If a normal error occurs, setup keeps the previous generation and native definitions active and applies recorded inverse operations before releasing the lock. After a crash or power loss, the filesystem may contain transaction artifacts or both native and generated definitions. The required old-or-new invariant applies after mandatory journal recovery on the next Alias Lens invocation. Recovery must never leave an alias missing. Generated files that are not referenced by the active pointer are inert and may be collected only after recovery succeeds.

## Existing Bash and Zsh installations

The first catalog release must not migrate an existing installation automatically.

Existing aliases remain in `.bash_aliases` or `.zsh_aliases`. Alias Lens continues to read and manage them through the current legacy path until the user starts this flow:

```text
al catalog preview
al catalog import --from bash
al catalog adopt gs
al catalog enable
```

The commands have separate effects:

- `preview` performs no writes. It reports portable entries, native-only entries, duplicates, parse failures, and unsupported syntax.
- `import` copies recognized entries into an inactive catalog. It does not edit the native file or shell startup files.
- `adopt` records inactive removal intent after previewing the exact native definition. It performs no native alias-file, startup-file, or generated-file write. It records intent only when the parser identifies one complete definition and the source hash matches.
- `enable` renders and validates the catalog, installs the loader, activates the generated replacement, and only then applies approved removal intents. If activation or recovery fails, native definitions remain active. A temporary duplicate is acceptable; a missing definition is not.

If the parser cannot prove an entry's boundaries, Alias Lens leaves the entry in the native file and prints manual steps. A failed adoption does not block unrelated entries.

Rollback restores adopted native definitions before it deactivates the generated replacement:

```text
al catalog rollback
```

Rollback must work without network access and without parsing the current catalog. It restores removed definitions through recorded inverse edits before deactivating the generated replacement. Each inverse edit has an expected hash and exact range. If a file changed after enablement, rollback leaves it unchanged, saves private conflict material, and prints a manual recovery plan. Whole-file restoration is allowed only when the current hash matches a recorded transaction state.

## Sync and conflicts

Catalog activation is per shell: `al catalog enable --shell bash` does not activate or reconfigure Zsh. Once any shell uses catalog mode, `catalog.json` is one sync unit for all catalog-enabled shells. Every configured legacy shell keeps its native alias file as a separate legacy sync unit until that shell is adopted. Each unit has its own local hash, remote hash, enrolled repository path, and reconciliation state.

In a Bash-catalog and Zsh-legacy installation, autosync watches `catalog.json` for Bash and the enrolled Zsh alias file for Zsh. It reconciles and commits one unit at a time under the shared mutation lock, stages only that unit's enrolled repository path, and preserves unrelated staged and working-tree files. It never derives one unit from another. Enabling or removing one shell does not change another unit's source or state. Acceptance criteria must cover the repository paths, watch inputs, and conflict behavior for every mixed-mode combination.

Catalog reconciliation uses stable entry IDs. A name collision between different IDs is a conflict. A change to the same ID on two machines is also a conflict unless the changed fields do not overlap. Duplicate IDs, malformed IDs, duplicate names, unknown fields, and entries over documented size limits fail validation without rewriting the catalog.

If both sides changed, Alias Lens saves private local and remote copies and leaves the live catalog and generated files unchanged. Generated files update only after catalog reconciliation succeeds, target-shell validation passes, and the user approves any new or changed native implementation. Sync and every migration operation share the same mutation lock and transaction recovery path.

## Support requirements for a new shell

A shell is not supported when Alias Lens can only print its syntax. Every adapter needs tests for:

- Native aliases, functions, comments, quoting, multiline definitions, and malformed input.
- Deterministic rendering and parse-render-parse behavior for supported definitions.
- Parse-only validation that does not load user startup files or execute definitions.
- Startup-file precedence, setup, repair, removal, and rollback.
- Current-shell execution and prompt insertion.
- History parsing, timestamps, and direct alias usage counts.
- Optional key bindings and collision-free disablement.
- Generated-file permissions, symlinks, backups, and atomic replacement.
- Narrow and wide TUI behavior with that shell active.
- Real terminal tests on every claimed operating system.

Fish is the first candidate after Bash and Zsh catalog mode is stable. Fish aliases are functions, and Fish recommends abbreviations for shortcuts that should expand visibly at the prompt. PowerShell needs a separate adapter because its aliases map names to commands rather than command strings with arguments. Nushell needs native custom commands for pipelines and environment changes.

References:

- [Fish aliases](https://fishshell.com/docs/current/cmds/alias.html)
- [Fish abbreviations](https://fishshell.com/docs/current/interactive.html)
- [PowerShell aliases](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.utility/set-alias)
- [PowerShell profiles](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.core/about/about_profiles)
- [Nushell aliases](https://www.nushell.sh/book/aliases.html)
- [Nushell custom commands](https://www.nushell.sh/book/custom_commands)

## Rollout plan

The rollout favors reversibility over speed:

1. Write acceptance criteria for each implementation phase and map every criterion to an automated or real-terminal test. Review and approve them before writing implementation code.
2. Add catalog types and a versioned catalog package. Do not connect the catalog to shell startup.
3. Move Bash and Zsh parsing, rendering, validation, execution, history, and setup behind adapters. Preserve current behavior.
4. Generate Bash and Zsh files in shadow mode. Compare their parsed entries with the current native files but do not source them.
5. Release opt-in catalog preview and import. Keep the catalog inactive.
6. Release opt-in catalog enablement and rollback for clean test homes.
7. Test upgrades and rollback with real Bash and Zsh installations on Linux, WSL, and macOS.
8. Enable catalog mode by default only for new installations after at least one stable release cycle.
9. Keep existing installations in legacy mode until each user opts in.
10. Add Fish behind an experimental flag. Remove the flag only after the full support requirements pass.
11. Repeat the support process for PowerShell or Nushell. Do not reuse another shell's parser, writer, startup code, execution code, or key bindings.

No release may combine the catalog migration with an unrelated startup-file rewrite.

## Acceptance criteria gate

No catalog or adapter implementation code may be written until its phase has concrete acceptance criteria in a repository document linked from `TODO.md`. Every implementation pull request must cite the applicable criterion IDs. CI rejects implementation changes that introduce catalog or adapter behavior without those IDs and their mapped tests.

Each phase uses this review table:

| Field | Required evidence |
| --- | --- |
| Criterion ID | Stable identifier cited by implementation and tests |
| Environments | Exact shell versions and Linux, WSL, or macOS versions in scope |
| Initial state | Paths, content hashes, metadata, links, active definitions, and installed integrations |
| Operation | Exact command and inputs |
| Expected state | Content hashes, metadata, active shell behavior, and files that must remain untouched |
| Failure injection | Error or termination point and expected recoverable on-disk state |
| Recovery or rollback | Exact result after recovery, including conflicts caused by later user edits |
| Automated evidence | Test package and test name |
| Terminal evidence | Real-terminal procedure when automation cannot prove behavior |
| Approval | Reviewer, date, verdict, and unresolved exceptions recorded in the repository |

Broad statements such as "support Fish," "behave exactly as before," or "leave the complete old state" are not acceptance criteria. The table must name the observable filesystem state and shell behavior.

Before implementation begins, the applicable phase table must at least define criteria proving that:

- Existing Bash and Zsh command output, active definitions, startup-file hashes, alias-file hashes, integration status, key bindings, history behavior, and sync source match the recorded pre-change baseline.
- Preview and import cannot modify startup files, native alias files, generated files, or the active configuration.
- A stale preview, validation error, interrupted write, full disk, permission error, or concurrent mutation leaves the criterion's named old hashes, metadata, and active definitions intact after recovery.
- Alias Lens never deletes or rewrites text it cannot prove it owns.
- Setup, repair, removal, adoption, and rollback are idempotent.
- Rollback works offline when the catalog is missing or corrupt.
- No parser, validator, renderer, sync operation, or preview executes alias content.
- Tests use isolated temporary homes and cannot reach real user shell files.

The reviewer must approve the criteria and their test mapping before the corresponding implementation task can move to in progress. Criteria are split by rollout phase so approval of a pure catalog model does not authorize startup-file or migration work.

## Failure tests required before opt-in migration

Tests must stop or kill the process after each write and fsync boundary. They inspect the crash state, run mandatory journal recovery, and then require either the fully specified old state or fully specified new state. At no point may recovery deactivate the last working definition of an adopted alias.

The suite must cover:

- A startup file that changes after preview.
- A catalog that changes during rendering.
- A generated file that fails native syntax validation.
- A read-only startup file or directory.
- A startup file with an edited Alias Lens block.
- Duplicate or nested managed markers.
- A symlinked alias file, startup file, catalog, or generated file.
- A broken symlink.
- A disk-full or short-write failure.
- A truncated, checksummed-invalid, missing, or ambiguous transaction journal at every recovery state.
- A process termination before and after each atomic rename.
- An external editor replacing a target immediately before and after the final identity check, with the documented portable concurrency boundary reflected in the expected result.
- Two Alias Lens processes attempting migration at once.
- An autosync operation racing import, adoption, enablement, rollback, and a transaction longer than the former stale-lock interval.
- An unsupported native definition between two supported definitions.
- Native source containing leading or trailing commands, declaration redirections, and definition-time substitutions, with sentinel files proving no side effects.
- A name that is valid in one shell and invalid in another.
- A secret in a portable command or native implementation.
- Rollback after the catalog becomes unreadable.
- Rollback after unrelated user edits, definition edits, missing backups, corrupt state, and a partial earlier rollback.
- Removal while another shell adapter is configured.
- Bash and Zsh installed together, with setup and removal in either order and independent legacy sync sources.
- A parent-directory symlink, a link retargeted between preview and apply, an owner mismatch, a hard link, a FIFO, a broken link, and supported access-control or extended-attribute metadata.
- CRLF files, a missing final newline, and non-UTF-8 bytes in unrelated startup-file content.
- Portable arguments containing empty strings, whitespace, quotes, Unicode, leading dashes, and wildcard characters, with both argument-passing modes.
- Portable invocation exit codes, signals, native-versus-portable precedence, and platform exclusions.
- Duplicate IDs, duplicate names, malformed IDs, unknown fields, and oversized entries.
- Catalog sync committing only `catalog.json`, preserving unrelated staged files, and scanning native implementations without leaking source in diagnostics.
- Bash startup precedence on Linux, WSL, and macOS, plus Zsh with explicit and startup-derived `ZDOTDIR`.

Tests use temporary homes and repositories. They must never read or modify the developer's real shell files.

## Rejected designs

One shared shell file is unsafe. Fish, PowerShell, and Nushell do not share Bash syntax or startup behavior.

Automatic source translation is unsafe. Quoting, pipelines, argument passing, environment changes, and output types have different meanings across shells.

Rewriting a complete user startup file is unsafe. Alias Lens owns only its marked loader block.

Treating generated files as editable creates two sources of truth. Users edit the catalog or a native source entry, then Alias Lens renders the generated file.

Automatic migration on upgrade removes the user's chance to inspect unsupported definitions and collisions. Existing installations remain in legacy mode until the user opts in.
