# State workflow implementation approval

Status: approved for staged implementation by the user on 2026-09-19. The user will perform the remaining native terminal checks later. The phase 4 WSL shutdown-and-reopen result remains a release gate and is not marked complete by this approval.

## Approved stages

The implementation can proceed in the dependency order defined by `docs/STATE_WORKFLOW_SPEC.md`:

1. Pure state resolution, observational status, and bounded operation plans.
2. Shared locking, journal recovery, private snapshots, native approvals, and transactional writes.
3. Explicit catalog preview, import, adoption, enablement, and offline rollback.
4. Semantic catalog comparison and three-way combination with conflict preservation.
5. Catalog data format 2 conditions and local machine profiles.
6. Shared typed CLI and TUI status, plan, review, native approval, and conflict views.
7. One-command catalog bootstrap with bounded provider reads and isolated repository setup.
8. Bash and Zsh completion generation, installation, and removal.
9. The optional local diff viewer under `docs/acceptance/LOCAL_DIFF_VIEWER.md`.

Alias packs remain deferred for one stable release cycle as required by the specification.

## Conditions

- Each stage cites the applicable SW acceptance criteria in its tests or implementation notes.
- Existing installations stay in legacy mode until the user explicitly enables catalog mode.
- Read-only commands do not call helpers that migrate configuration, create alias files, recover transactions, or update timestamps.
- New write paths do not reuse the removable stale-file sync lock as their safety boundary.
- A stage can merge before manual platform evidence is complete, but it cannot be called release-ready until its required Ubuntu, WSL, macOS, Bash, Zsh, and browser evidence is recorded.
- A partial stage stays visibly marked incomplete in `TODO.md` and user-facing help must not promise unavailable behavior.
