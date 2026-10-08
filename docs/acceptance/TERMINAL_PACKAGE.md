# Terminal package acceptance

## Scope

Stage 5 moves Bubble Tea models, views, picker, settings, statistics, revisions, catalog review and repository selection into `internal/tui` after application services are available. The terminal interface remains the default experience.

## Criteria

- TP-001: Terminal models call the application services for configuration, alias edits, context, revisions, catalog lifecycle, bootstrap and sync. The terminal package does not implement persistent writers, recovery, Git transport or shell execution policy.
- TP-002: Command routing starts the terminal interface through a small typed API. Terminal code imports neither `cmd/alias-lens` nor executable assets. Application code does not import the terminal package.
- TP-003: Preserve keyboard semantics, scoped shortcuts, search input, selection output, execution confirmation and installed versus pending entry behavior. A pending candidate cannot execute under an installed label.
- TP-004: Preserve narrow and wide layouts, theme and appearance settings, alternate-screen behavior, focus handling, resize and redraw. Shared configuration validation must agree with the terminal settings choices.
- TP-005: Review decisions and semantic conflict choices remain staged until final confirmation. Cancellation persists nothing. Slow application work retains asynchronous messages and stale-plan checks.
- TP-006: Terminal tests reside beside the models and use synthetic data. Actual executable tests still build `cmd/alias-lens`, including when their working directory changes. No test reads or writes real settings, aliases, history or repositories.
- TP-007: Separate high review resolves all acceptance findings. Required evidence includes narrow/wide compiled-binary PTYs, resize, redraw, cancellation, alias masking, argument/status/signal behavior, full race tests, platform compilation and `make fmt check`.

## Test mapping

Existing TUI catalog, revisions, favorites, settings, shortcut and selection tests cover TP-001, TP-003 and TP-005. Existing footer, panes, wide redraw, resize and appearance PTYs cover TP-004 and TP-006. Catalog pending picker tests cover the execution gate. Dependency inspection proves TP-002. Record final commands and review resolutions in `docs/testing/evidence/PACKAGE_ORGANIZATION.md`; unavailable native platform checks remain release gates.
