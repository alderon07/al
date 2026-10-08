# Package organization acceptance

## Scope and order

Extract packages incrementally in this order: managed Git, providers, shared entry and shell behavior, application orchestration, then terminal UI. Each extraction preserves behavior and gets its own review and verification gate. Keep the existing catalog, renderer, storage, plan, state, and transaction boundaries.

The first extraction moves restricted Git execution and transport policy into `internal/managedgit`. Application code supplies catalog-independent transport inputs. The package must not import `cmd/alias-lens`, application configuration, catalog lifecycle, or terminal UI.

## Managed Git criteria

- PO-001: `internal/managedgit` owns local and network execution, process cancellation, output limits, repository transport audits, HTTPS credential handoff, and controlled SSH policy. No production runner implementation remains in the command package.
- PO-002: Explicit transport inputs replace the dependency on catalog preview records. Credentials remain bound to the pinned HTTPS authority. SSH uses owned stable known-host bytes, an owned agent socket, and trusted system SSH. Configuration, hooks, filters, environment, redirects, and credential restrictions remain unchanged.
- PO-003: Existing bootstrap and sync callers use the extracted API. Application timeout policy and mutation sessions remain in application code. Extraction adds no lock acquisition, recovery, configuration writes, or approval flow.
- PO-004: Runner tests reside with the package where practical. Bootstrap, catalog sync, uncertain push, and SSH integration tests still verify their command-level behavior. Package tests use disposable synthetic repositories and credentials.
- PO-005: Preserve platform build tags and Unix process-group termination. Linux tests, race checks, Bash/Zsh PTYs, Darwin and Windows compilation, and `make fmt check` provide verification evidence. Unavailable native platform release evidence remains open.
- PO-006: Keep CLI commands, public JSON contracts, private record versions, shell declarations, catalog generation identity, and embedded browser assets unchanged. No dependency updates or unrelated file cleanup.

- PO-007: Promote the existing command-scoped `tea`, `usagelog`, and `exportfile` packages to root-level `internal` packages with byte-for-byte implementation and test preservation. Update import paths only. Future application and TUI packages can then import them under Go internal visibility rules. Preserve historical evidence documents.

## Review gate

A medium-effort implementer extracts the runner and supplies focused evidence. A separate high-effort reviewer checks the moved implementation, changed call sites, security constraints, and tests against PO-001 through PO-006. Correct findings and repeat review before recording the extraction as complete.

Later extractions require their own acceptance criteria and dependency review before moving code. Do not create import cycles, expose all helpers, or split the transaction session across package-owned locks.
