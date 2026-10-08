# Package organization review evidence

## Application and terminal extraction, 2026-10-07

The user authorized completion of stages 4 and 5 after the reviewed catalog and earlier package changes were pushed. Acceptance criteria were written in `docs/acceptance/APPLICATION_PACKAGE.md` and `docs/acceptance/TERMINAL_PACKAGE.md` before source edits.

An independent high architecture review approved the dependency direction `cmd -> tui -> app -> existing lower packages`. It required separating shared appearance and shortcut schemas, preserving observed configuration bytes and plan/revision identities, keeping command routing and terminal prompts outside app, and moving revision restore and repository clone policy into application operations. Windows stubs must split between app and TUI. Compiled-binary PTY tests must explicitly build the executable after relocation.

A baseline matrix recorded output, exit codes and filesystem observations for 27 CLI invocations using the committed compiled binary and a disposable home with synthetic entries. It includes the explicitly empty tracked repository path found during settings review. All invocations left the recursive filesystem manifest unchanged. The final candidate comparison passed all 27 invocations without changing the disposable filesystem. Both stage gates are recorded below.

### Shared settings prerequisites

Medium implementation moved theme palettes, appearance/footer schemas and icon validation into `internal/presentation`. High review found no blocking differences in palette values, JSON fields, defaults or validation. The replacement width function preserves the existing Lip Gloss terminal-cell calculation. Focused theme, appearance, footer and icon tests passed. Pure tests moved beside the extracted code.

A separate medium implementation moved shortcut profiles, declarations, validation, matching and translation into `internal/shortcuts`. High source review compared 30 moved functions and the complete registries against the baseline. Literal labels, action scopes, default order and OS/WSL detection were preserved. Public descriptors return copies and profile overrides remain immutable. Focused tests passed in the command and shortcut packages; the shortcut race check passed. Actual shortcut editor, search-mode and configured Bash/Zsh launcher PTYs passed in 1.245 seconds.

Temporary command adapters remain during application extraction and must be removed as their callers move. These prerequisites do not close the application or terminal stage gates.

### Settings service review

The first application tranche moved configuration schemas, observation, defaults, validation, tracking and the three-way merge into `SettingsService`. Existing command mutation sessions compose with it through a temporary settings-only writer. High review verified that observed bytes, fresh reload under the lock and saves within an existing transaction retain their earlier behavior.

Review reproduced a regression with compiled before/after binaries: an explicitly empty tracked repository destination was incorrectly treated as omitted and created settings. Medium correction added a typed optional destination that distinguishes omission from an empty argument. Independent re-review and the no-mutation regression passed. Broader configuration, mutation, observation and profile tests passed in 1.680 seconds. The complete application gate remains open until the temporary dependencies and helper adapters become app-private and tests move to their implementation.

### Query service review

Medium implementation moved search, suggestion ranking and edit-distance matching from terminal code into the application query service. High review compared all nine moved functions with the original implementation. The explicit scorer retains favorite priority, context ordering, stable ties, dangerous-command exclusions, Unicode normalization and empty-context behavior. Only operations used by callers are public; normalization and edit-distance helpers remain private. Three pure tests moved beside the implementation. Independent focused tests passed in the app and command packages, including context ranking and pending-entry selection.

The compiled prerequisite checkpoint passed all 27 baseline CLI comparisons, including output and exit codes. Its disposable home retained the complete initial filesystem manifest. `go mod verify` also passed. These are intermediate checks; the full backend and terminal moves still require their final gates.

### Write-operation precursor review

Typed setup requests/results, description-update results, reviewed revision restore and repository clone/configure were prepared before moving the private backend. High review found no remaining bounded blockers. Focused tests passed in 22.912 seconds, including actual Bash/Zsh setup and revision PTYs. Nine additional compiled CLI comparisons preserved exit codes, stdout, stderr and alias bytes for setup, repair/removal, startup failure, description changes and restore outcomes. Setup preserves partial-failure notices in order and prompts after releasing the mutation lock. Reviewed restoration checks the selected revision and hashes inside one mutation session; catalog restoration reuses that session.

### Runtime and test ownership review

The backend extraction compiles in a disposable source tree before replacing the working source. Application dependencies cover home, working directory, environment, executable, time, provider transport, validators, watcher startup and operation notices. The compiled runtime checkpoint passed all 27 original CLI comparisons without changing the disposable home's recursive filesystem manifest.

High review reproduced inconsistent runtime boundaries with two synthetic homes. Settings, locks, catalog records and revisions correctly used the service home, but watcher startup inherited the process home and ignored an injected watcher callback. Provider token lookup also bypassed the injected environment. Medium corrections passed independent re-review for home isolation, read-only behavior, watcher callbacks, child environment and synthetic credential lookup. Zsh environment and working-directory isolation now uses a narrow shell adapter runtime constructor that preserves existing defaults. Independent tests cover absolute and relative legacy ZDOTDIR, the catalog's narrower discovery rule and unchanged process-home files.

Test relocation initially separated some subprocess tests from their helper entry points. Medium correction keeps each test and its helper in the same package. The inventory retains all 534 original command-package test/helper names with no duplicates. Expanded actual PTY and subprocess verification passed in 4.569 seconds, covering catalog narrow/wide cancellation, mutation and watch contention, shortcuts, editor/search modes, diff and revision previews, favorites, wide redraw and startup inspection. Nine further CLI PTY checks passed with disposable homes for import, context changes, private export, cleanup and autosync settings. Focused import/export/catalog and context race checks passed.

The full app suite subsequently found an environment-only bootstrap helper dispatch missed by the initial inventory. Guided bootstrap tests and the compiled Zsh test now remain beside command `TestMain`; no app test dispatches its own executable. The frozen full suite passed after this correction.

### Stage 4 final gate, 2026-10-08

Independent high review approved AP-001 through AP-007 with no unresolved findings. It verified application ownership, private sessions and settings writers, explicit runtime dependencies, read-only repository comparison, immutable generation verification, installed membership, unchanged field tags and literals, path-isolated sync and uncertain-push handling. Command routing, credential prompts, embedded browser assets and linker version remain in the executable package. No app code imports terminal models or rendering. Shared shortcut validation may depend on the lower terminal protocol adapter; app exposes no key events or terminal types.

- Required root `make fmt check` passed: command package 27.731 seconds, app 61.242 seconds, shell 0.412 seconds, all packages, vet, executable build and whitespace checks.
- Application-stage race checks passed: app 47.129 seconds, shell 1.067 seconds, presentation 1.041 seconds and shortcuts 1.047 seconds. Actual PTYs run in the full suite and bounded checks above.
- Linux, Windows amd64 and Darwin amd64 production builds passed.
- The final app binary passed all 27 baseline output and exit-code comparisons; the complete disposable-home filesystem manifest remained unchanged.
- All 534 original test/helper names remain present, with no duplicates; private core tests moved beside implementation and compiled tests target the executable package.

These results close AP-008 and stage 4. Terminal extraction and its final gate remain in progress. Native macOS, WSL and trusted system Zsh evidence remain separate release gates.


### Stage 5 review and final gate, 2026-10-08

Medium implementation moved private terminal models and views into `internal/tui`. The public surface contains five operations (`Browser`, `Picker`, `Stats`, `RepositoryPicker` and `CatalogConflict`) and two option types. Each launcher receives the same explicit application services used by command routing. Models, diff helpers and asynchronous review/clone work retain that service instance. Terminal code contains no persistent writers, Git transport, shell execution policy or fallback service factory.

Independent high review checked TP-001 through TP-006 against the pre-terminal snapshot. It compared 232 normalized production function bodies exactly and manually checked the remaining ten differences: removal of the temporary shortcut JSON adapter and explicit service parameters for metadata/query helpers. Review retained keyboard behavior, layouts, installed versus pending labels, confirmation, cancellation and asynchronous messages. A combined inventory preserved all 277 command test/helper names remaining after stage 4 and added one regression for an injected application home; 188 tests moved beside their terminal implementation. The complete 534-name command baseline also has no missing or duplicate test/helper names after both stages.

Race-instrumented shortcut PTYs exposed a test capture boundary: waiting for the screen title could return before its footer rendered. Medium correction waits for the existing `enter change` footer on the initial frame and each resized frame before checking layout. This prevents output from an older incomplete frame from satisfying the next capture. Separate high re-review approved the fixture change; three repeated normal runs passed in 0.726 seconds and three race runs passed in 2.163 seconds. Product rendering and timing are unchanged.

Final root verification:

- A fresh full `make fmt check` passed: command package 19.127 seconds, app 64.071 seconds, transaction 6.599 seconds and TUI 13.140 seconds, with all other packages, vet, executable build and whitespace checks passing. The full suite includes actual Bash/Zsh shell behavior and compiled narrow/wide terminal PTYs.
- Repository-wide `go test -race -count=1 -skip PTY ./...` passed, including command 13.641 seconds, app 49.566 seconds, transaction 7.181 seconds and TUI 9.082 seconds. Actual PTYs ran separately in the full suite; independent focused terminal PTYs passed in 8.606 seconds and targeted injected-home/shortcut race checks passed in 3.085 seconds.
- Production compilation passed for Linux amd64/arm64, Darwin amd64/arm64 and Windows amd64. Cross-compilation supplies build evidence only.
- The final binary passed all 27 saved CLI output and exit-code comparisons, with no filesystem mutations in the synthetic home. A linker-injected `main.version` build reported `alias-lens package-boundary-test` and created no home files. Embedded browser assets remain in the executable package.
- `go mod verify` passed. No dependency or public/private format versions changed.
- Final production ownership is 50 command files / 4,675 lines, 99 app files / 13,269 lines, 26 TUI files / 6,125 lines, seven presentation files / 697 lines and three shortcut files / 807 lines.

CI shadow selectors now include command, application and shell owners. The catalog verifier includes command, application and terminal owners and accepts suffixes after `PTY`, so the moved narrow/wide cancellation tests are selected. Independent high review approved the selector corrections. Both shadow selectors passed on Linux (the native macOS matrix retains its platform skip). With `PATH=/tmp/al-zsh-runtime/build/bin:$PATH`, `AL_REQUIRE_PTY_SHELLS=1 go test ./cmd/alias-lens ./internal/app ./internal/tui -run '^(TestCatalog.*PTY|Test.*Init.*PTY)' -skip '^TestCompiledZshInitGuidedPTY$' -count=1` passed with only the unavailable fixed-path Zsh bootstrap test explicitly skipped: command 6.096 seconds, app 17.893 seconds and TUI 1.556 seconds. The final repeated `make fmt check` also passed after the PTY fixture correction, including TUI 11.206 seconds. The full catalog verifier’s fixed-path Zsh bootstrap test needs trusted system Zsh. The local disposable Zsh 5.9 runtime supports actual shell PTYs but does not meet that production validator requirement; the policy remains unchanged. Native macOS and WSL evidence and the full verifier with trusted system Zsh remain release gates.

Stage 5 source review has no unresolved findings. The evidence above closes TP-007 for this package refactor; it does not declare platform release readiness. Changes remain uncommitted and unpushed at this verification checkpoint.


### CI follow-up, 2026-10-08

The reviewed refactor was committed and pushed as `0f825560c68056ebce6bec08bf4bd01a56fecfb1`. Before that commit, `make fmt check` passed again (command 17.068 seconds, app 55.998 seconds, TUI 11.989 seconds), staged paths were inspected and the private-data scan found no new credential patterns or private data files. All five copied fixtures matched the earlier committed fixtures.

[CI run 37818859900](https://github.com/alderon07/al/actions/runs/37818859900) passed both native shadow jobs, the vulnerability scan and the release-candidate job (race tests, workflow syntax, snapshot archives, SBOMs and artifact verification). The Linux test job reproduced the preceding catalog commit's Zsh completion-security prompt. The macOS job was cancelled by matrix failure, but its logs also showed real failures caused by temporary paths traversing `/var`, an oversized Unix socket pathname and a corrupted multiline PTY paste.

Acceptance was amended before each bounded correction: CB-004 for completion isolation, AP-007 for a private canonical temporary root and SP-008 for exact fixture-file loading in the native Bash PTY. Medium corrections preserve production startup code and validator policy. The Zsh fixture initializes genuine completions with insecure directories excluded and verifies both Alias Lens command mappings. Its synthetic global-startup scenario reproduces the original prompt, excludes the unsafe function and proves its marker is never written. The CI root resolves to a short physical directory with mode `0700`; independent probes verified Go and `mktemp` use it, an actual Unix socket binds and a representative Darwin path is 87 bytes. The readonly integration PTY sources exact mode-`0600` fixture bytes, then checks both the masking alias and original readonly function.

Separate high review approved those fixture corrections. Independent real Bash/Zsh and synthetic completion PTYs passed in 6.725 seconds. GNU Bash 3.2 accepted the unchanged integration; ten strengthened readonly PTY repetitions passed in 1.105 seconds and independent high repetition passed in 0.343 seconds. Modern Bash/Zsh integration and handoff checks also passed. The earlier macOS syntax failure is not evidence that intact generated integration is incompatible.

Further high review reproduced a separate existing runtime-handoff defect: `declare -p -f NAME` cannot inspect the function on Bash 3.2, and matching only `-fr`/`-rf` misses exported readonly functions on modern Bash. A later readonly conflict could therefore leave the first generated entry replaced; alias-entry handoff on Bash 3.2 could also mask the readonly function. SP-009 froze the required atomic preflight before the runtime-wrapper correction. The corrected wrapper captures `builtin declare -Fr` without names, which supplies declaration metadata without function bodies or attribute mutation. It compares all selected names exactly before any declaration or alias removal. The regression matrix caught and corrected a `nocasematch` false positive in the first candidate; final comparisons preserve case sensitivity, custom IFS, shell options, exported attributes and prior definitions. Only populated Bash runtime-wrapper vectors changed; stored declarations, generation identities, renderer IDs, approvals, JSON versions, Zsh vectors and empty Bash vectors are unchanged.

Repeated independent high review approved SP-009 with no unresolved findings. The original two-function and alias reproductions now refuse atomically on Bash 3.2; exported readonly functions are also refused on modern Bash. Valid handoffs still apply. Independent GNU Bash 3.2 nine-case PTYs and wrapper vectors passed three repetitions in 2.924 seconds; targeted modern-shell race tests passed in 2.013 seconds. Medium affected app/shell race checks and Darwin/Windows builds also passed.

The complete corrected `make fmt check` passed with genuine disposable Zsh completion functions: command 22.265 seconds, app 60.254 seconds, shell 1.384 seconds and TUI 12.952 seconds, plus all other packages, vet, build and whitespace checks. The corrected binary also passed all 27 saved CLI output and exit-code comparisons without changing the disposable home.

### Native CI verification, 2026-10-08

The corrections were committed and pushed as `aa83bcc873154222074ac830ed8ab9c0e5e68324`. [CI run 37821864225](https://github.com/alderon07/al/actions/runs/37821864225) completed successfully for that exact commit. All six jobs passed:

- Linux and macOS full checks and `scripts/verify-catalog-workflow.sh` passed with required Bash/Zsh PTYs and trusted system Zsh. This closes the strict verifier gate left open in the earlier local checkpoints.
- Both native shadow matrices passed.
- The vulnerability scan passed.
- Release-candidate race tests, workflow syntax, snapshot archives, SBOMs and artifact verification passed.

The native macOS suite supplies runtime evidence for Apple Bash and system Zsh, including the corrected readonly-conflict preflight. Manual macOS terminal, restart and installation checks in `docs/testing/RELEASE_MACOS.md`, and native WSL checks in `docs/testing/RELEASE_WSL.md`, remain release gates. Automated CI success does not complete those checklists.

## Baseline and scope

The catalog workflow implementation and the user's `AGENTS.md` changes were already uncommitted when this refactor began. Reviews compare the organization changes with disposable pre-extraction source snapshots, not with the older Git HEAD.

Acceptance: `docs/acceptance/PACKAGE_ORGANIZATION.md`. Dependency plan: `docs/PACKAGE_ORGANIZATION.md`.

## Managed Git and shared helpers

A medium-effort implementer moved restricted Git behavior to `internal/managedgit`. A separate high-effort reviewer compared normalized production code with its baseline, inspected explicit transport inputs, and checked authentication, process termination, limits, configuration audits, file identity and trusted executable checks.

The package accepts a catalog-independent transport containing pinned URL, transport mode, reviewed known-host hash and identity, explicit user home and agent socket. The application keeps timeout policy, lifecycle decisions and mutation sessions. The runner imports no application, catalog, command or UI packages.

The existing terminal compatibility adapter, usage log and export codec moved from `cmd/alias-lens/internal` to root `internal`. All five implementation/test files were byte-identical to the captured originals. The 41 caller changes were import substitutions and formatting only. This removes the Go internal visibility obstacle for future application and terminal UI packages.

### Review findings and corrections

| Finding | Correction | Review evidence |
| --- | --- | --- |
| Hook suppression test installed a post-checkout hook but only ran rev-parse. | Run an actual checkout. | Reviewer removed the hooksPath override in an isolated copy; the corrected test failed because the sentinel was created. |
| Transport audit tests could pass because DNS failed even without an audit. | Call AuditNetworkConfig directly, check policy errors, and verify clean configuration acceptance. | Reviewer replaced the audit with unconditional success in an isolated copy; both unsafe-config tests failed. |

Repeated high review found no remaining managed Git/helper extraction findings. The original move preserved production behavior. Root review subsequently found an invalid exported-input edge: a relative SSH home could loop forever while traversing parents. The package now rejects relative home/parent paths before traversal and returns no reviewed identity for them. A bounded subprocess regression also verifies absolute content, hash and identity acceptance. Separate high review and normal-host SSH tests passed.

### Verification

- Expanded runner/bootstrap/sync tests passed: runner 0.083 seconds, command package 9.612 seconds.
- Runner race check passed.
- Shared helper package tests passed.
- Initial full `make fmt check` passed: command package 72.665 seconds, all packages, vet, compiled binary, and whitespace checks.
- Corrected-stage full `make fmt check` passed: command package 71.158 seconds, all packages, vet, compiled binary, and whitespace checks.
- After corrections, a normal-host strict runner check passed, including actual owned-socket controlled SSH handoff, inherited config/hook suppression, and unsafe network/worktree audits. Tests used only synthetic credentials and disposable repositories/socket paths.

## Provider extraction

The same medium-effort implementer moved provider behavior to `internal/providers`. A separate high-effort reviewer inspected the bounded changes against the provider snapshot. Root reviewed application call sites and exported input boundaries.

The package owns repository discovery, locator parsing, pagination, immutable object proofs and ancestry, and accepts copied settings plus per-registry HTTP/credential/environment/command dependencies. Concrete providers remain private. Application code retains sign-in prompts, config persistence, reviewed SSH selection, catalog schema decoding and whole-catalog secret scanning before returning usable bytes.

### Review findings and corrections

| Finding | Correction | Evidence |
| --- | --- | --- |
| Ancestry wrapper used default settings, which would reject an enrolled enterprise host. | Pass the observed configuration from remote snapshot discovery into ancestry. | Enterprise host/port ancestry and per-registry isolation tests. |
| Injected credentials did not consistently reach Bitbucket discovery/clone and GitLab API discovery. | Use callback-first credential access without ambient environment overriding injected credentials. | Credential-only and synthetic ambient-token tests; prompted Bitbucket token survives in memory and never enters config. |
| Newly exported preview/ancestry could receive an unconfigured locator. | Validate selected provider, authority and safe identities before credential lookup. | Foreign authority, port and userinfo regressions assert zero lookup/HTTP calls. |
| Bitbucket clone accepted forged URLs and initially probed SSH before rejecting them. | Validate selected HTTPS/SSH repository URLs and repository parts before any probe, credential or Git operation. Preserve canonical SSH and HTTPS-only auto routes. | Independent public API probe rejects a foreign auto clone with zero commands and zero credential lookups; traversal and supported-route regressions. |
| Legacy HTTP redirect/pagination compared hostname without port. | Require exact HTTPS authority and refuse redirect userinfo. | Port, userinfo and scheme rejection tests. |

Repeated high review found no unresolved extraction findings. Input/authority corrections are recorded separately from the code moves.

### Criterion mapping

| Criteria | Evidence |
| --- | --- |
| PO-001/PO-003/PO-006 | Package dependency/source comparison, bootstrap and uncertain push integration, unchanged command/state/shadow contracts in full suite. |
| PO-002/PO-004 | Runner credential, hooks, transport audits, unsafe known-host sources, controlled SSH and relative-home regression tests. |
| PO-005 | Full checks, race suite, actual shell PTYs and Darwin/Windows compilation. |
| PO-007 | Byte comparison of all five moved helper files and import-only caller review. |
| PP-001/PP-003 | Immutable provider proofs, discovery/access filtering, bounds, paging, redirects, and public authority tests. |
| PP-002/PP-005 | Enterprise/runtime isolation and connection tests with synthetic prompted/injected credentials; config bytes exclude the token. |
| PP-004 | `TestProviderPreviewKeepsAppCatalogValidation` rejects unsupported schema and likely secrets before returning bytes. |
| PP-006/PP-007 | Moved provider tests, command integration tests, focused race tests, full gates and PTYs; no dependency or record version changes. |

### Overall verification

- Final production `make fmt check` passed on the normal host: command package 104.590 seconds, all packages, vet, compiled binary and whitespace checks.
- `go test -race -count=1 -skip PTY ./...` passed on the normal host: command package 90.142 seconds, managed Git 2.279 seconds and providers 1.360 seconds. Actual PTYs run separately and in the full suite.
- Darwin amd64 and Windows amd64 production builds passed. These are compilation evidence, not native platform runtime evidence.

- After the final test-only addition, `make fmt check` passed again: command package 78.984 seconds, provider tests 0.126 seconds, all packages, vet, compiled binary and whitespace checks.
- `TestGitHubListPaginationAndWriteFilteringUsingRuntime` passed with actual harmless subprocesses reading disposable JSON pages. It verifies two-page discovery, configured hostname/auth arguments, exact writable/nonarchived membership and preserved repository metadata. Root high review passed the test. The complete provider race suite then passed in 4.284 seconds, covering this final test-only change.
- Verbose normal-host PTY verification passed in 9.357 seconds: compiled Bash bootstrap cancellation/portable application/native review, real Bash and Zsh helper workflows for the same cases, and semantic catalog pull. The disposable Zsh runtime exercises actual shell behavior without changing the production trusted-validator policy.

The command directory now has 89 production files and 25,353 production lines, compared with 91 files and 26,654 lines at the beginning of this pass. Shared helper implementation/test files also moved out of the command subtree. Both extracted production packages import only the Go standard library. `go.mod` and `go.sum` were unchanged by this pass. The user's `Avoid scope creep` instruction remains in `AGENTS.md`; its code map now points to the extracted packages.

These results complete the originally selected Git/provider/helper pass. The user subsequently asked to continue with the shell stage. Application services and full terminal UI extraction remain future stages. No commit or remote push was performed.

## Shared entries and shell extraction

Acceptance was written in `docs/acceptance/SHELL_PACKAGE.md` before code. Reviews use the current command-source snapshot at `/tmp/al-shell-organization-before`; the earlier catalog and package changes were already uncommitted.

A medium-effort implementer extracted shared entry data, then shell adapters and read-only startup planning. A separate high-effort reviewer checked the acceptance boundaries, including startup mutation sessions, the two validator trust policies, source-range and hash contracts, and private shadow report fields. Public validator APIs reject unsupported shell names before executable resolution.

### Review findings

| Finding | Required correction | Status |
| --- | --- | --- |
| Automated identifier renaming also changed the lowercase category JSON tag and metadata literals. | Restore the exact literals and assert JSON/metadata bytes at the package boundary. Compare moved function literals with the baseline. | Corrected. Separate high review confirmed all entry field names/types/tags/order and zero string-literal mismatches across 13 moved functions. Direct JSON omission/full serialization, metadata copy ownership/normalization and platform tests passed in 0.015 seconds. |
| Per-shell command wrappers repeated every adapter method. A future adapter would require the same duplication. | Embed the package adapter in one application adapter and retain only mutation methods there. | Corrected and passed repeated high review. |
| The moved legacy syntax command helper exposed arbitrary shell arguments. | Keep the helper private; expose the adapter's fixed parse-only check. | Corrected and passed repeated high review. |
| Newly exported declaration/startup helpers accepted unsupported shell names; the loader diagnostic interpolated the shell into shell code. | Validate the supported shell before parsing/building code, or keep the builder behind the selected adapter. | Corrected. Direct tests reject traversal, unsupported declarations/startup and an injected loader shell name before resolving or emitting code. |
| A new malformed-source guard returned an empty source range. Native control validation could then miss a masked control in a multi-assignment line after an invalid UTF-8 comment. | Fail closed on invalid source in native control validation and preserve meaningful rejected-source ranges. Add malformed-source/control-mask regressions. | Corrected. Independent probes rejected invalid UTF-8 and oversized-line masks. Direct tests cover invalid UTF-8/NUL and complete refused-source ranges. |
| Parser provenance constructors were exported only to support command-package test fixtures. | Keep construction and hashing private, move vector tests to the package and use typed fixtures for application tests. | Corrected. Original known-answer vectors remain in package tests; repeated high review confirmed no exposed construction helpers. |
| Catalog runtime handoff assembly remained in the command loader. | Move pure Bash/Zsh handoff code to the shell package; keep immutable generation verification and stdout routing in the application. | Corrected. Independent comparison with the saved original builder matched eight Bash/Zsh empty/alias/quoted-function/mixed cases byte-for-byte; unsupported input returns no code and an error. |

Repeated high review passed with no unresolved findings. The independent source audit found 101 copied function literal sequences unchanged and 89 normalized function bodies identical directly. Other differences are type/field/package renames, explicit shell/source guards and per-call validator dependencies. All integration and loader constants remain unchanged. The loader's verification call chain remains structural-only; it launches no shell validator.

One deliberate startup error-path change remains. The planner discovers all required files before application, so an unreadable later login file leaves an earlier `.bashrc` untouched. Previously, setup could change the earlier file before failing. Successful edits retain their original ordering, modes, backups and shared transaction identity/freshness/symlink checks. Newly exposed structural APIs also reject unsupported source encoding and bounds before scanning controls.

### Verification

- Direct entry tests and command metadata/description/platform compatibility tests passed in 0.016 and 1.182 seconds.
- Focused shell/shadow/setup/history/metadata/integration/check command tests passed in 49.991 seconds with the disposable Zsh runtime.
- Existing Bash/Zsh execution PTYs passed in 0.878 seconds.
- After the final private-helper and runtime-handoff corrections, focused command tests passed in 16.806 seconds and the shell race suite passed in 1.544 seconds. Direct Bash and Zsh PTYs verified a masking alias, spaced argument forwarding, exit status 23 and foreground Ctrl+C status 130.
- Independent final focused tests passed in 0.024 seconds for shell and 0.574 seconds for the command package, including known-answer vectors, validator launch/concurrency limits and actual guarded/readonly PTYs.
- Initial full `make fmt check` passed on the normal host: command package 127.756 seconds, shell 0.470 seconds, all packages, vet, compiled binary and whitespace checks.
- Final corrected `make fmt check` passed on the normal host: command package 133.400 seconds, shell 0.455 seconds, all packages, vet, compiled binary and whitespace checks.
- Final affected-package `go test -race -count=1 -skip PTY` passed on the normal host: command package 132.711 seconds, entry 1.068 seconds and shell 1.124 seconds. Actual shell PTYs passed separately and in the full checks.
- Final corrected Darwin amd64 and Windows amd64 production builds passed. Native platform runtime evidence remains separate.

### Criterion mapping

| Criteria | Evidence |
| --- | --- |
| SP-001 | Exact entry JSON/metadata tests, copied field and literal audit, platform and description compatibility tests. |
| SP-002 | Adapter parsing/rendering/history tests, unchanged integration constants, handoff golden and independent eight-case comparison, actual argument/status/masking/signal PTYs. |
| SP-003 | Recursive planning filesystem manifests, fresh ZDOTDIR reads, startup precedence tests and existing startup/symlink/backup/transaction PTYs. Application writer source remains unchanged. |
| SP-004 | Original hash vectors now in package tests, shadow/report goldens and private-field exclusion, structural/parser/control/startup tests, unchanged format/dependency files. |
| SP-005 | Unsupported shell rejection before resolver, existing never-exec/environment/output/process-group tests and independent structural-only loader call-chain review. |
| SP-006/SP-007 | Direct package tests, repeated high reviews and corrections, full checks, race gates, actual Bash/Zsh PTYs and platform builds. Native platform release gates remain open. |

Stage 3 is complete. The command directory contains 89 production files and 23,040 production lines, down from 25,353 lines before this stage. The shell package imports shared entry, catalog and catalog storage packages; it imports no application, command or terminal UI code. Dependency files and format versions remain unchanged. Application service and full terminal UI extraction remain later stages. No commit or remote push was performed.

## Release evidence

The sandbox maps system files to a different owner, so trusted system SSH evidence is run on the normal host. The local disposable Zsh runtime supports real PTYs; trusted system Zsh remains unavailable for the existing strict release verifier. Native macOS and WSL restart evidence remain release gates. Cross-compilation does not satisfy those gates.
