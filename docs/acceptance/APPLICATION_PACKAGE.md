# Application package acceptance

## Scope

Stage 4 moves configuration, mutation coordination, catalog lifecycle, bootstrap, sync, entry loading and private-data operations into `internal/app`. Command routing and terminal models call concrete services. Shells, providers, Git execution, storage, plans and transactions retain their existing packages.

## Criteria

- AP-001: Application services own persistent changes and recovery. CLI and TUI use the same operations. Nested operations pass the existing mutation session and never acquire another lock. Confirmation does not hold the mutation lock.
- AP-002: Services expose typed inputs and results for settings, entries, catalog review, plans, revisions, tracked files and synchronization. Private implementation helpers remain private. Application code imports neither command routing nor terminal models, Bubble Tea or Lip Gloss. It contains no command router, interactive screen launcher or compatibility facade exposing the legacy helper collection.
- AP-003: Application dependencies are explicit at the service boundary. Terminal prompts and rendering belong to callers. Shared appearance and shortcut schemas may live in a separate typed package when both configuration validation and terminal rendering require them.
- AP-004: Read-only status, plan, search, completion and loader paths retain their filesystem, provider, subprocess and execution restrictions. Preserve observed configuration bytes, plan inputs, revision hashes and identities across service results. Freshness, approval cancellation, immutable generation verification and installed membership remain unchanged.
- AP-005: All writers retain private modes, symlink and identity checks, backups, revision recording, transaction ordering and crash recovery. Sync retains path-only commits, outgoing secret scans, conflict preservation and uncertain-push reconciliation.
- AP-006: Command names, exit codes, public JSON, private record versions, generation identities and shell declaration bytes remain unchanged. Embedded browser assets and linker-injected version behavior remain available from the executable.
- AP-007: Existing unit tests move with their implementation where practical. Compiled-binary tests explicitly build the executable package. Synthetic helper processes use disposable homes and retain their production entry-point dispatch. CI and release-verifier selectors include the new owning packages and continue selecting their expected tests.
- AP-008: Each bounded implementation gets independent high review and corrections. Verification includes affected tests, real Bash/Zsh PTYs, race checks, Darwin and Windows builds and `make fmt check`. Native macOS and WSL release evidence remains open.

## Test mapping

Use the existing mutation contention and repeated recovery tests for AP-001 and AP-005; configuration merge tests for AP-002 and AP-003; status observational and catalog loader tests for AP-004; help, completion, generation vectors and shell integration golden tests for AP-006; compiled init, startup and narrow/wide terminal tests for AP-007. Record commands and review resolutions in `docs/testing/evidence/PACKAGE_ORGANIZATION.md` before closing AP-008.
