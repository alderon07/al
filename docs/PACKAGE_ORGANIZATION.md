# Package organization

## Current pass

Stages 1 and 2 completed the selected Git/provider extraction and shared helper promotion. Stage 3 completed shared entry and shell adapter extraction. Stage 4 completed application operations under `docs/acceptance/APPLICATION_PACKAGE.md`, with explicit runtime dependencies and private mutation sessions. Stage 5 completed terminal models and views under `docs/acceptance/TERMINAL_PACKAGE.md`. All five stages passed independent high review and full checks. The implementation and verification are recorded in `docs/testing/evidence/PACKAGE_ORGANIZATION.md`.

## Intended dependency direction

Command routing and terminal UI call application services. Application services coordinate shell adapters, providers, managed Git, catalog storage, plans, and transactions. Lower-level packages never import command routing or terminal UI.

Existing `internal/catalog`, `internal/catalogrender`, `internal/catalogstore`, `internal/plan`, `internal/state`, and `internal/transaction` remain separate.

## Extraction order

| Stage | Package | Work and reason | Gate |
| --- | --- | --- | --- |
| 1 | `internal/managedgit` | Move restricted execution, repository transport audits, credentials, SSH policy, process groups, and output bounds. Replace catalog preview dependencies with transport inputs. Keep application timeout and mutation policy at callers. | PO-001 through PO-007; focused transport and bootstrap tests, race checks, PTYs, formatting/static checks and platform compilation. |
| 2 | `internal/providers` | Move discovery and immutable reads behind provider interfaces. Supply settings, credential access and HTTP transports explicitly. Keep terminal sign-in prompts and config persistence at callers. | PP-001 through PP-007; missing/invalid credentials, pagination, capability/write filtering, redirect refusal and SSH fallback. |
| 3 | `internal/entry` and `internal/shell` | Move the entry data shared by parsers and views, then pure adapter behavior and startup planning. Keep actual writes under the application mutation session. | SP-001 through SP-007; Bash/Zsh declarations, startup routes, masking aliases, readonly functions and argument/status/signal PTYs. |
| 4 | `internal/app` | Move lifecycle, bootstrap, sync, configuration and mutation orchestration behind concrete services with explicit dependencies. CLI and TUI use the same services. | Freshness, approval cancellation, transaction recovery, sync isolation and public JSON contract evidence. |
| 5 | `internal/tui` | Move models/views and review screens once application services are available. Keep shell execution policy and persistent writes in application services. | Narrow/wide PTYs, resize, cancellation, installed membership and confirmation evidence. |

## Constraints

Every stage has acceptance criteria before code, medium-effort implementation, separate high-effort review, corrections and repeated review. Avoid competing writers on the same files.

Moving a Go file into a directory creates a new package boundary. Expose only the operations callers need. Do not retain the full command package behind compatibility aliases or introduce generic utility packages for unrelated behavior.

The existing `tea`, `usagelog`, and `exportfile` packages now live at root-level `internal`. Their old command-scoped paths prevented `internal/app` and `internal/tui` from importing them under Go internal visibility rules.

Shell adapters use shared entry types and return read-only startup plans. One application adapter applies those plans under the existing mutation session. Providers receive copied settings and runtime dependencies. Application code owns config persistence, catalog decoding, secret scanning and transport selection; command code owns credential prompts. The TUI uses typed application operations for persistence and observations. Its private models and views live in `internal/tui`, whose public API contains five launch operations and two option types.

The shell package preserves distinct legacy and catalog validator trust policies, separate legacy versus catalog ZDOTDIR handling and Bash login precedence.

Keep platform build tags, shell declaration bytes, generation identities, record formats, JSON contracts and command behavior unchanged. Preserve compiled-binary tests and embedded browser assets when moving command/application files. The existing platform release gates remain required.

## Evidence

Stage 4 and stage 5 were authorized on 2026-10-07. Their contracts are in `docs/acceptance/APPLICATION_PACKAGE.md` and `docs/acceptance/TERMINAL_PACKAGE.md`. Both require typed application services, private implementation helpers, no application dependency on terminal models or rendering, and unchanged configuration and plan freshness. Stages 4 and 5 passed their gates on 2026-10-08. Linux and native macOS automated checks, required Bash/Zsh PTYs and the trusted system Zsh verifier passed for `aa83bcc` in [CI run 37821864225](https://github.com/alderon07/al/actions/runs/37821864225). Manual macOS terminal, restart and installation checks and native WSL release checks remain required.

Acceptance criteria: `docs/acceptance/PACKAGE_ORGANIZATION.md`.

The first extraction review and verification results are recorded in `docs/testing/evidence/PACKAGE_ORGANIZATION.md`.
