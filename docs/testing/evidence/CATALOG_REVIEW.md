# Catalog implementation evidence

Implementation authorized on 2026-10-03. Baseline `make check` passed before code changes. A disposable Zsh 5.9 runtime was built under `/tmp` without installing system packages. Linux Bash and Zsh startup PTYs ran during implementation. Native CI verification and the remaining manual platform gates are recorded below.

## Review cadence

Record medium implementation, high review findings, medium corrections, high re-review, and verification for each bounded stage. Evidence uses only synthetic commands and disposable homes/repositories.

## Gates

- Acceptance contracts written before implementation.
- Storage and transaction foundation: automated acceptance and high review passed.
- Shell activation and offline rollback: automated acceptance and Bash/Zsh PTYs passed.
- Catalog editing, execution, and TUI: automated acceptance and narrow/wide PTYs passed.
- Semantic sync: automated acceptance, actual Git isolation, and uncertain push reconciliation passed.
- Local bootstrap: compiled Bash and helper Bash/Zsh guided apply/cancellation passed.
- Remote bootstrap: synthetic provider, actual filtered Git, bounds, cancellation, promotion, and recovery evidence passed.
- Complete end-to-end behavior: automated acceptance and strict system-shell verifier passed on Linux and macOS CI.
- Trusted system Zsh bootstrap/verifier evidence: passed on Linux and macOS CI.
- Native macOS automated runtime evidence: passed. Manual macOS terminal, restart and installation checks and native WSL release checks remain pending.

## Native CI verification, 2026-10-08

[CI run 37821864225](https://github.com/alderon07/al/actions/runs/37821864225) passed all six jobs for `aa83bcc873154222074ac830ed8ab9c0e5e68324`. Linux and macOS full checks and the disposable catalog workflow verifier passed with required Bash/Zsh PTYs and trusted system Zsh. Both native shadow matrices, vulnerability scanning and release-candidate checks also passed.

The package refactor, CI fixture corrections and atomic readonly-conflict preflight review are recorded in `docs/testing/evidence/PACKAGE_ORGANIZATION.md`. This run closes the strict verifier gate reported in the earlier local checkpoints. The manual checks in `docs/testing/RELEASE_MACOS.md` and `docs/testing/RELEASE_WSL.md` remain required before release. Earlier dated sections below retain the intermediate findings and their verification context.

## Storage and renderer review, 2026-10-03

A medium implementation added strict private records and renderer v2. A separate high review found case-insensitive JSON field acceptance, insufficient private-file observation, incomplete list bounds and generation vectors, and function declaration behavior under a masking alias. Medium corrections added exact recursive field checks, descriptor-based private reads with post-read identity checks, bounded lists, deterministic vectors, and the `function NAME {` declaration form. The shell loader must guard each replacement before removing a masking alias.

Independent verification: `GOCACHE=/tmp/al-review-gocache go test ./internal/catalogstore ./internal/catalogrender` passed. The medium implementer split private reads by platform and verified Windows and Darwin test compilation. High re-review accepted the corrected storage contracts. Storage integration and the full stage gate remain pending.

## Transaction review, 2026-10-03

A medium implementation added exact multi-target transactions and dependency-ordered recovery. Its focused tests include process death after forward rename, another process death after inverse rename, and repeated recovery. High review requested stricter journal field and progress validation, length-framed promoted-tree hashes, preview binding of promotion source identity, resulting recovery identities, and explicit directory-creation behavior. Medium corrections added those behaviors. A second high review found that a recreated empty directory could be removed after recovery; a recorded-identity check and reproduction test now prevent that removal.

Independent verification: `GOCACHE=/tmp/al-go-cache go test ./internal/catalogstore ./internal/catalogrender ./internal/transaction ./internal/plan` passed. Shared managed-writer coverage and the complete stage gate remain pending.

## Shell installation review, 2026-10-03

High review requested safe takeover review for differing native definitions and portable entries; positive plan action sequences; semantic startup-route labels; refusal of unreachable and conditional startup routes; snapshot-consistent execution of pending deletions; immutable-artifact replacement refusal; rollback of originally missing files and corrupt pointers; and startup decline guidance. It also required complete shell integration during bootstrap and actual startup PTYs. Medium corrections and integration evidence are in progress.

Subsequent high review found native drift prevented renewed ownership review, preserved user source blocks loaded the overlay at the end of the startup file, focused approvals hashed a filtered catalog instead of the full source, separate buffered readers could lose final confirmation input, readonly helper conflicts could invoke a stale helper, and profile changes were absent from status fingerprints. Medium corrections added immutable native snapshots, refreshed exact ownership with retained baseline, insertion offsets, full-source freshness, bounded one-byte review reads, guarded helper creation, and deterministic machine-resolution fingerprints. Regression tests passed for those scenarios, including actual Bash and Zsh startup, masking aliases, readonly functions, empty generations, and Bash login-file precedence.

High review also required bounded and proven ZDOTDIR discovery, exact installed completion membership, typed catalog import/description updates, semantic conflict choices with final confirmation, shared locking of legacy writers, and read-only inspection of new workflow journals. Those integration checks continue before the complete lifecycle gate closes.

## Semantic sync review, 2026-10-03

The first medium substage implemented semantic pull from an enrolled repository, private conflict copies, stale-plan checks, and legacy fallback exclusions. High review requested strict installed-record observation, refusal of dangerous local Git configuration and lazy fetching, bounded separate conflict payloads, and remote-revision proof. The implementer added the strict reads, restricted network entry point, configuration checks, and separate payloads. Remote push, provider preview, and bootstrap evidence remain pending.

Later review found worktree configuration could bypass transport auditing, actual Git credential prompts included a trailing space missing from the controlled askpass patterns, stderr overflow could race a successful process exit, staging was on the home filesystem instead of the destination filesystem, and a one-entry index represented deletions rather than a sparse checkout. Medium corrections are being verified with synthetic credentials and disposable repositories. Review requires a complete remote-pull → local-edit → path-only-push flow, durable uncertain push reconciliation, and mixed-mode dispatch before acceptance.

## Intermediate suite evidence

`GOCACHE=/tmp/al-go-cache PATH=/tmp/al-zsh-runtime/build/bin:$PATH go test ./...` passed after the first lifecycle and sync integration checkpoint. This is intermediate evidence; later shared-writer and bootstrap corrections require the final `make fmt check` and disposable workflow verifier.

## Final review corrections, 2026-10-03

High review found that native leaf symlink retargeting could leave a previously resolved generation input valid. Generation manifests now bind the logical source path, exact link text, and leaf identity as well as resolved native bytes. Plans pin both leaf and referent identities. Retargeting refuses application and startup loading while preserving both native files.

Installed diff originally used the complete source snapshot, including entries omitted during installation. It now uses verified manifest membership. Tests prove an omitted entry remains pending, is absent from completion, and appears as an addition in the next installed comparison. CLI help and completions share the command registry.

Native aliases or functions masking loader control words could change startup handoff behavior. Conservative preflight and read-only package verification now refuse these definitions with migration guidance. Actual Bash and Zsh startup PTYs prove application stops before writes, no synthetic sentinel runs, and native fallbacks still work. Intentional entry-name and `al` masking retains the guarded replacement behavior.

The next review extended this finding to combined and continued alias definitions outside the literal import grammar. These forms require conservative ambiguity refusal before installation; single-line head matching alone is insufficient.

Compatibility review then required recognizing the exact integration emitted by the existing setup command. Guided activation must upgrade its unsafe early alias removal through the displayed native target, preserve unrelated native definitions, and retain the pre-catalog integration in the offline rollback baseline.

Medium corrections added adapter-owned raw and guarded integration forms, proven boundary matching, explicit native relocation, rebased ownership ranges, and one pinned guarded startup integration. Bash and Zsh PTYs passed for both existing forms, repeated enablement, new-shell dispatch, and byte-exact offline rollback. Nested, modified, and duplicate forms remain refused.

Plan-copy review required naming retained/refreshed/renamed/deleted/excluded fallbacks and explaining that rollback restores the enrollment baseline, potentially bringing back later renamed or deleted aliases. Focused assertions now cover those effects. They also caught an invalid synthetic condition probe; command and function exclusions now use a valid portable probe independently of candidate availability.

Executable-resolution review found that an execute bit alone did not prove current-user access. Go now checks each explicit absolute candidate through `exec.LookPath`, after validating captured PATH components. A non-root permission test proves an inaccessible new program is omitted and an inaccessible installed replacement blocks the operation.

Autosync originally required a legacy repository, used an age-based worker lock, and selected the invoking shell's native file. Corrections add catalog-only enrollment, independent mixed units, configured-shell selection, persistent OS worker deduplication, exact tracked-file replacement, and refusal of stale fallback tracking. A process contention matrix verifies ten managed command routes leave disposable state unchanged under a held mutation lock.

Remote push review required pinning the actual source commit, destination ref, parent base, and catalog bytes before every retry. Durable intents now enforce those bindings and rescan the outgoing enrolled history. The actual Git and synthetic HTTPS API test covers semantic remote pull, local editing, path-only push, failed transport after remote advancement, and failed transport before advancement. Both outcomes reconcile without force-push and preserve unrelated staged and dirty files.

Filtered Git fetch records promisor settings under the explicit transport URL. Transport auditing permits only the expected `promisor=true` and `partialclonefilter=blob:none` keys for the pinned URL; rewrites, command overrides, includes, and credentials remain refused. Controlled SSH tests pin the agent socket and known-host bytes/identity, reject changed inputs before starting Git, and prove API credentials are absent from SSH handoff.

Remote staging tests exercise negotiated filtering, ignored-filter refusal, changed revision/blob refusal, inert hooks/filters/LFS/submodule metadata, process-group cancellation, destination collision, sparse index metadata, destination-filesystem staging, and journaled promotion. Smaller injected deadline, tree, and packet bounds exercise limits without large fixtures. Production defaults remain five minutes, 256 MiB of clone material, and 8 MiB of packet evidence.

Cleanup review also required repeated recovery after stage removal but before parent removal. Parent cleanup must continue when the stage is already absent, accept already removed parents, and refuse a recreated parent with a different identity.

The cleanup correction and read-only status regression passed independent review. Recovery and status now share one canonical stage decoder with parent identities; stale synthetic fixtures must use empty required arrays rather than null.

The first final combined check exposed a timing-dependent packet-bound failure. A clone completing before the monitor's next poll used the production byte bound for final validation instead of the selected test bound. Medium correction uses the selected bound for the final private trace read; the complete staging matrix passed five consecutive runs afterward. Production limits are unchanged.

The next combined `make fmt check` passed. Race verification then found that closing a monitor stop channel did not join the goroutine before clearing its trace-path field. Medium correction captures immutable monitor inputs and joins both clone and metadata-fetch monitors before reconfiguration or cleanup. Focused race evidence passed twice, and the final full race suite passed afterward.

## Concrete coverage map

| Criteria | Test evidence |
| --- | --- |
| CL-001, SW-004, SW-014 | `TestWorkflowForwardBoundaryRecovery`, `TestWorkflowProcessDeathTwice`, `TestWorkflowDirectoryPromotionRecovery`, workflow metadata/identity refusal tests, and `TestMutationSubprocessContentionLeavesManagedFilesUnchanged`. |
| CL-002, CL-003 | `TestCatalogActualStartupPTY`, `TestCatalogSurvivingControlMasksBlockBeforeWritesStartupPTY`, `TestCatalogRenewedOwnershipAfterNativeDrift`, `TestCatalogEnrollmentPreservesUserLeafSymlinks`, and `TestCatalogRollbackPreservesOriginalAbsenceAndInvalidPointer`. |
| CL-004, SW-005 | `TestCatalogEditorPreservesIdentityFieldsAndInstalledMembership`, `TestCatalogInstalledDiffUsesExactIncludedMembership`, `TestCatalogPendingPickerCannotRun`, and `TestCatalogDefaultBinaryPTYNarrowWideReviewCancellation`. |
| CL-005, SW-008 through SW-010, SW-013 | Semantic pull/conflict tests, `TestCatalogConflictDrawerStagesFieldsAndAppliesCatalogOnly`, `TestRemotePullEditPushUncertainReconciliation`, path-only commit tests, and catalog-only/mixed autosync tests. |
| CB-001, SW-011 local | Local init preview/freshness/configuration tests, `TestCompiledInitPTYCancellationAndPortableApply`, and `TestHelperInitGuidedBashAndZshPTY`. |
| CB-002, SW-011 provider | Immutable GitHub/GitLab/Bitbucket preview tests, missing credentials/access tests, pinned blob/locator refusal, controlled credential prompts, and controlled SSH transport tests. |
| CB-003, SW-011 remote | `TestRemoteStageActualFilteredGitAndPromotion`, staging limit tests, stage-intent replacement/recovery refusal, and workflow promotion crash recovery. |
| CL-006, SW-012, SW-015 | Shared help/completion tests, read-only status/inventory tests, privacy/data-path tests, complete package suite, and the strict disposable verifier in CI. |

Native macOS and WSL checks are still release gates. The local disposable Zsh runtime provides actual shell behavior evidence; it does not satisfy the separate trusted system-validator/bootstrap release gate.

## Final verification, 2026-10-03

All implementation modules were frozen before these final commands. Documentation was updated afterward to record the results.

| Check | Result |
| --- | --- |
| `GOCACHE=/tmp/al-go-cache PATH=/tmp/al-zsh-runtime/build/bin:$PATH make fmt check` | Passed. Command package tests took 70.212 seconds; formatting, all package tests, vet, production build, and whitespace checks succeeded. |
| `GOCACHE=/tmp/al-go-cache PATH=/tmp/al-zsh-runtime/build/bin:$PATH go test -race -count=1 -skip PTY ./...` | Passed. Command package tests took 57.049 seconds. PTYs ran separately in the full suite. |
| Normal compiled Bash init cancellation, portable apply, native review/apply; helper Bash/Zsh init; controlled SSH socket/known-host transport | Passed outside the sandbox UID mapping, using disposable data only, in 9.144 seconds. |
| Darwin amd64 and Windows amd64 production builds | Passed. Compilation is not native runtime evidence. |
| `bash -n scripts/verify-catalog-workflow.sh` | Passed. |
| Strict disposable workflow verifier | Bash preview/init/installed membership/rollback completed. Zsh installation stopped at isolated validation because no trusted `/bin/zsh` or `/usr/bin/zsh` is available. Full verifier remains a strict CI/release gate. |
| Changed/new file credential-pattern scan | 160 files inspected; zero token/private-key pattern matches. Only synthetic user data appears in tests. |
| Existing user AGENTS.md edit and dependencies | The `Avoid scope creep` edit remains intact. No dependency upgrades were made. |

The compiled normal Zsh bootstrap test explicitly fails rather than skips when `AL_REQUIRE_PTY_SHELLS=1`. CI installs system Zsh on Ubuntu and runs the strict verifier on Linux and macOS. The native release checklists remain unchecked until their candidate-specific runs are recorded. No release-readiness claim is made from cross-builds or the disposable Zsh fixture.
