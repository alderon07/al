# WSL 2 test audit, 2026-10-12

The automated WSL checks pass after fixing portable executable resolution for imported Windows PATH directories. This report covers the working tree based on `31a33ec4434d01e74f0b6116f3fbd0432fd05d7a`, plus the changes in [the acceptance criteria](../../acceptance/WSL_TEST_AUDIT_2026_10.md).

The checkout was initially clean at `4bc122a737330d03bb0b88b21d7ed3641374a76a`. `git pull --ff-only` updated `dev` to `origin/dev` at `31a33ec`. Validation restarted after the pull.

## Environment

| Component | Version |
| --- | --- |
| Distribution | Ubuntu 24.04.5 LTS, x86_64 |
| WSL | 2.7.14.0 |
| Running kernel | 6.18.33.2-microsoft-standard-WSL2 |
| Windows | 10.0.26300.9457 |
| Windows Terminal installed | 1.24.12741.0 |
| Default Go | go1.27.1 linux/amd64 |
| Validation Go | go1.27.2 linux/amd64 |
| Bash | 5.2.21 |
| Homebrew Zsh | 5.9.2 |
| System Zsh installed for trusted-validator tests | 5.9 |
| tmux installed | 3.4 |

Final validation commands use `GOTOOLCHAIN=go1.27.2`. The default Go installation remains unchanged. System Zsh was installed through Ubuntu's package manager because the compiled bootstrap requires a root-owned validator in `/bin` or `/usr/bin`.

## Failures and fixes

The first uncached suite on the updated checkout reported 13 failing leaf cases, including Bash and Zsh portable initialization, catalog startup, existing-setup upgrade, and explicit startup. The catalog workflow script also failed with `catalog entry is pending`.

WSL imports absolute Windows PATH directories with trailing separators. `resolveCatalogExecutable` rejected the entire PATH when any directory differed from `filepath.Clean`. Rendering consequently omitted portable entries, even when their executable existed in `/usr/bin`.

`TestCatalogExecutableResolutionAcceptsWSLPATH` failed before the production fix. It now covers trailing separators, the root directory, and Windows-style directory names with spaces before and after the executable's directory. The rejection test also covers empty entries, relative directories, repeated internal separators, and dot segments. A second failing assertion caught `/./` being accepted by an intermediate root-directory exception. The final implementation accepts a root directory only when trimming trailing separators leaves an empty absolute path.

The regression command failed before each fix and passes afterward:

```bash
GOTOOLCHAIN=go1.27.2 go test ./internal/app -run '^TestCatalogExecutableResolution' -count=1
```

`TestCompiledZshInitGuidedPTY` initially failed because only Homebrew Zsh was installed. Installing system Zsh satisfied the existing validator policy without changing application trust rules.

The initial vulnerability scan exited with status 3 and identified ten reachable Go standard-library vulnerabilities in Go 1.27.1. All ten report Go 1.27.2 as the fixed version. The scan under Go 1.27.2 reports zero reachable vulnerabilities. It still reports one vulnerability in imported packages and two in required modules whose vulnerable symbols the application does not call.

An early race run crossed the repository update and failed with missing files from the old package layout. Fresh race runs against the updated tree passed. The supplemental CLI smoke harness initially assumed `doctor` would exit successfully in an unconfigured home, then incorrectly expected four notices. The corrected check expects status 1 and exactly three notices for the unconfigured repository, disabled automatic sync, and absent GitHub connection.

A coverage run alongside several other suites failed once while starting Homebrew Zsh in `TestCatalogActualStartupPTY/zsh`, with `fork/exec: operation not permitted`. The uncached full suite, three PTY repetitions, and an earlier coverage run all passed that case. The standalone coverage rerun passed all packages. The cause of the isolated process-start failure remains unconfirmed.

## Automated results

Every command below completed successfully. `AL_REQUIRE_PTY_SHELLS=1` makes missing shells fatal in tests that implement that gate.

| Command, with `GOTOOLCHAIN=go1.27.2` | Result |
| --- | --- |
| `AL_REQUIRE_PTY_SHELLS=1 go test -json -count=1 ./...` | 17 tested packages pass. 1,428 passing test and subtest events. The macOS-only shadow matrix is the sole skipped test. |
| `AL_REQUIRE_PTY_SHELLS=1 go test -json -count=3 -run PTY ./...` | 492 passing test and subtest events across three repetitions, including helper entry points. No test failures or skips. |
| `go test -race -count=1 -skip PTY ./...` | All tested packages pass. PTYs run separately without race instrumentation. |
| `AL_REQUIRE_PTY_SHELLS=1 go test -coverprofile=/tmp/alias-lens-wsl-audit-20261012/coverage-retry.out -count=1 ./...` | All tested packages pass on the standalone rerun. Statement coverage is 64.3%. |
| `AL_REQUIRE_PTY_SHELLS=1 make fmt check` | Formatting, tests, vet, binary build, and whitespace checks pass. |
| `go mod verify` | All modules verified. |
| `go mod tidy -diff` | No module changes. |
| `./scripts/verify-shadow-wsl.sh` | Fixture hashes match and the synthetic alias manifest remains unchanged. |
| `./scripts/verify-catalog-workflow.sh` | Bash and Zsh bootstrap, read-only preview, installed entries, rollback, and catalog PTYs pass. |
| `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` | Zero reachable vulnerabilities. Scanner module version is v1.9.0. |

The full suite includes unit and integration coverage for parsing, private filesystem writes, startup precedence, completion, provider transport and credentials, Git path isolation, synchronization conflicts, catalog lifecycle, recovery, and shell execution. Fuzz seed cases run in the full suite. Each fuzz target also ran for 15 seconds with two workers:

| Command, with `GOTOOLCHAIN=go1.27.2` | Result |
| --- | --- |
| `go test ./internal/app -run '^$' -fuzz '^FuzzShadowImporter$' -fuzztime=15s -parallel=2` | Pass, 22,975 executions. |
| `go test ./internal/shell -run '^$' -fuzz '^FuzzCatalogAliasDependencyInspection$' -fuzztime=15s -parallel=2` | Pass, 188,255 executions. |
| `go test ./internal/catalog -run '^$' -fuzz '^FuzzDecode$' -fuzztime=15s -parallel=2` | Pass, 36,915 executions. |

PTY coverage includes shell execution, argument forwarding, exit status, signals, readonly declarations, masking, initialization cancellation, native review, rollback, recovery, catalog synchronization, shortcut editing, search modes, favorites, diff previews, and resizing. The resize test exercises 48x18, 80x24, 100x30, 120x36, 160x40, and 200x50. Other tests exercise additional widths, including compiled-binary catalog flows.

The supplemental compiled CLI smoke check used separate temporary Bash and Zsh homes. Setup, repair, completion installation and removal, saved shortcut overrides, automatic Windows shortcut selection, and setup removal passed. Fresh interactive shells resolved the binary, defined `al`, and registered completion. Alias files had mode `0600`, and setup did not create the other shell's startup or alias file. The corrected harness uses failing synthetic `ssh` and `gh` executables to keep optional provider probes isolated.

The WSL shadow fixture reported:

```text
PASS shadow-wsl
manifest 3272702548a24c6b8181d8fab21afb957487f02e0906c7c2df595b4dac5252e1
plain 69fa1a36e59413963dc7b89f48dd8023349de9ae20745221deda46626fb81d4e
json 7b49b8b3e23673e11aa349d8d84b62779dd65c65723ee05c71e7aa9f9c77b2ce
```

Raw test output and coverage stay outside the repository in `/tmp/alias-lens-wsl-audit-20261012`. This report contains only sanitized environment details and test summaries.

## Repeat verification

A second audit on 2026-10-12 began with the same five audit files modified or untracked. `git pull --ff-only` reported `Already up to date` at `31a33ec`. The patched Go toolchain and both installed Zsh runtimes remained available.

The earlier Homebrew Zsh process-start error did not reproduce in ten consecutive `TestCatalogActualStartupPTY` runs. Each repetition includes both Bash login routes, interactive Bash, and interactive Zsh. All 50 test and subtest events passed. The original unexplained failure remains recorded above.

The red/green regression check used `git archive HEAD` in a temporary checkout. It copied the current regression tests onto the upstream implementation and required exit status 1 with the specific WSL PATH failure. Copying the fixed `catalog_adapter.go` into that checkout then produced exit status 0. The live checkout remained unchanged during this check.

| Command, with `GOTOOLCHAIN=go1.27.2` | Repeat result |
| --- | --- |
| `AL_REQUIRE_PTY_SHELLS=1 go test ./internal/app -run '^TestCatalogActualStartupPTY$' -count=10` | Pass, 50 passing test and subtest events. |
| `AL_REQUIRE_PTY_SHELLS=1 go test -json -coverprofile=/tmp/alias-lens-wsl-audit-20261012-repeat/coverage.out -count=1 ./...` | Pass, 1,428 passing test and subtest events. Only the macOS shadow matrix skips. Statement coverage remains 64.3%. |
| `AL_REQUIRE_PTY_SHELLS=1 go test -json -count=3 -run PTY ./...` | Pass, 492 passing test and subtest events. No test failures or skips. |
| `go test -race -count=1 -skip PTY ./...` | Pass. |
| `go test ./internal/app -run '^TestCatalogExecutableResolution' -count=1` | Expected failure on upstream, pass with the fix. |
| `go mod verify` and `go mod tidy -diff` | Pass, no module changes. |
| `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` | Pass, zero reachable vulnerabilities. The same uncalled dependency advisories remain. |
| `./scripts/verify-catalog-workflow.sh` | Pass, Bash and Zsh bootstrap, preview, installed entries, rollback, and catalog PTYs. |
| `AL_REQUIRE_PTY_SHELLS=1 make fmt check` | Pass, formatting, tests, vet, binary build, and whitespace checks. |
| `./scripts/verify-shadow-wsl.sh` | Pass, all three hashes match the earlier run. |

All three fuzz targets passed another 15-second run with two workers. `FuzzShadowImporter` executed 70,913 inputs, `FuzzCatalogAliasDependencyInspection` executed 80,495, and `FuzzDecode` executed 328,147.

Repeat logs stay outside the repository in `/tmp/alias-lens-wsl-audit-20261012-repeat`.

Concurrent Linux audit edits appeared during this run. The WSL commit includes only `TODO.md`, `catalog_adapter.go`, its regression test file, the WSL acceptance criteria, and this report.

## Remaining release checks

This audit does not complete [the WSL release checklist](../RELEASE_WSL.md). WSL shutdown and restart persistence were not exercised because shutdown terminates this session. Windows Terminal keyboard delivery, clipboard and paste behavior, visual inspection, and tmux interaction were not manually exercised. Installed version numbers do not establish those behaviors.

Published archive checksums, archive setup, and release attestations were not checked. This evidence describes the updated development working tree, not an exact published release commit.
