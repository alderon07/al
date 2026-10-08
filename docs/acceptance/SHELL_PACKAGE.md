# Shared entry and shell package acceptance

## Scope

Stage 3 extracts shared entry data and shell behavior from the command package. Application services and the terminal UI remain later stages. Save the existing command sources as the review baseline before extraction.

## Criteria

- SP-001: `internal/entry` owns the shared entry and metadata types and pure metadata/default helpers. Preserve all entry JSON fields, omission rules, metadata ordering, descriptions, categories and platform eligibility. Application configuration and persistent metadata edits stay in the command package.
- SP-002: `internal/shell` owns Bash and Zsh parsing, declaration rendering, integration bytes, history syntax, execution/prompt/binding specifications and shell handoff. Preserve existing declaration bytes, alias masking behavior, readonly preflight, shell options and protected names. It imports neither the command package nor terminal UI.
- SP-003: Startup planning reads and returns ordered edits without writing files, recovering journals or acquiring locks. The application applies edits under its existing mutation session, with fresh inputs, identity checks, backups, permissions and no-op suppression. Preserve Bash login precedence and separate legacy versus catalog ZDOTDIR rules. Setup changes only the selected shell's files.
- SP-004: Catalog structural parsing, source ranges, origin hashes, declaration/dependency validation and narrowly supported startup grammar move together. Preserve public shadow report JSON, private field exclusion, golden outputs, renderer IDs, record versions and generation identity. Application ownership/approval/storage orchestration stays outside the shell package.
- SP-005: Preserve separate validator policies. Legacy syntax validation retains its existing shell lookup and environment behavior. Catalog validation retains fixed trusted executable paths, private temporary environment, parse-only execution, timeout, output bounds and process group cancellation. Neither parser nor inspection executes entry code.
- SP-006: Tests exercise the extracted package API directly as well as application call sites. Verify metadata and declaration compatibility, read-only planning, startup precedence, symlinks/freshness, structural refusal and validator boundaries. Existing compiled binary and actual Bash/Zsh PTYs must pass, including masking, readonly conflicts, arguments, exit status, signals and cancellation where covered by current runtime contracts.
- SP-007: Run `make fmt check`, affected race tests and Darwin/Windows production compilation. Record the evidence and repeated independent high review. Unavailable native macOS, WSL and trusted system Zsh checks remain release gates. No dependency upgrades or application/TUI extraction in this stage.

## SP-008 Apple Bash handoff compatibility

The native Apple Bash 3.2 runtime must accept installed integration and `shell-entry` handoff when `al` is both a readonly function and a masking alias. Loading must preserve the readonly function and alias without syntax errors or executing an attempted replacement body. The PTY loads exact production bytes from private fixture files rather than pasting a large multiline program through terminal input. Any correction needs an actual Bash 3.2 reproduction, supported modern Bash/Zsh regression evidence and separate high review of declaration safety, shell options, golden outputs and generation/approval contracts. Do not skip this compatibility case or change the shell selected by macOS tests.

## SP-009 atomic readonly preflight

Runtime catalog handoff checks every selected name for readonly function conflicts before defining any entry or removing any masking alias. Bash 3.2 and modern Bash must reject ordinary and exported readonly functions, including alias-entry collisions, without partially applying the generation. Use supported shell builtins to inspect attributes without executing function bodies; preserve shell options, prior definitions and aliases. Regressions reproduce first-entry mutation followed by a readonly conflict and verify that both entries remain unchanged after refusal. This bounded runtime-wrapper correction preserves immutable stored declarations, renderer IDs, generation identities, approval keys and public JSON contracts. Separate high review and actual Bash 3.2/modern Bash/Zsh evidence are required.

## Review workflow

Implement the entry substep before adapter extraction. A medium-effort implementer provides the bounded diff and verification; a separate high-effort reviewer compares it with the saved baseline and these criteria. Correct each finding with medium effort and repeat high review. Record concrete failure scenarios, corrections and remaining platform evidence in `docs/testing/evidence/PACKAGE_ORGANIZATION.md`.

## Existing regression evidence to preserve

| Criteria | Existing command tests |
| --- | --- |
| SP-001 | `TestFunctionsAndMetadataAreDiscoverable`, `TestMetadataEditPreservesUnchangedFields`, description tests and `TestPlatformNameUsesCatalogIdentifiers`. |
| SP-002 | `TestShellAdapterContractBash`, `TestShellAdapterContractZsh`, integration goldens, history fixtures, `TestBashPTYExecution` and `TestZshPTYExecution`. |
| SP-003 | Linux/WSL and macOS setup tests, Zsh idempotence, startup backup safety, enrollment symlinks, actual startup PTYs and existing setup upgrade/offline rollback PTYs. |
| SP-004 | Shadow origin/hash vectors, declaration and heredoc matrices, report goldens/private-field exclusion, protected names, ambiguous dependencies and startup route proof tests. |
| SP-005 | `TestShadowNeverExecutesBashContent`, `TestShadowNeverExecutesZshContent`, isolated environment/output bounds/process group tests and legacy check tests. |
| SP-006 | Catalog guarded handoff, readonly helper, surviving controls, shell argument/status/signal tests and guided Bash/Zsh bootstrap cancellation PTYs. |

The extracted packages also need direct tests for their public boundaries. Passing the command tests alone does not verify that callers supply all required inputs or that planners remain read-only.
