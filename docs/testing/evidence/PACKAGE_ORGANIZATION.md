# Package organization review evidence

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
