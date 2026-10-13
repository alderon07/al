# Linux Mint release verification

Verification covers the worktree based on `31a33ec4434d01e74f0b6116f3fbd0432fd05d7a`, on branch `fix/linux-release-readiness`. It does not certify a published tag. The existing staged optional-Zsh test changes and performance-table edit were preserved.

## Environment

| Item | Value |
| --- | --- |
| Distribution | Linux Mint 22.3 Zena, Ubuntu Noble base |
| Kernel | Linux 7.0.0-34-generic |
| Architecture | x86-64 |
| Release Go toolchain | 1.27.2 |
| Source compatibility minimum | 1.26.0 |
| Bash | 5.2.21 |
| Zsh | 5.9, root-owned `/usr/bin/zsh`, mode `0755` |
| Terminal | `xterm-256color` PTYs and tmux 3.8 |
| Release tools | GoReleaser 2.18.0, Syft 1.42.3, actionlint 1.7.11, govulncheck 1.9.0 |

Zsh was initially absent. The host owner installed system Zsh before required-shell verification. Homebrew Zsh cannot replace the root-owned system validator required by compiled bootstrap tests.

## Corrections

The catalog startup PTY lookup now respects `AL_REQUIRE_PTY_SHELLS=1`. With a controlled PATH containing Bash and Go but no Zsh, ordinary development checks skip Zsh and strict checks fail. Installed-Zsh assertions remain unchanged.

The old Go 1.24.2 build selected from `go.mod` produced 40 called standard-library vulnerability findings. The `toolchain go1.27.2` directive now selects the patched release compiler through the existing `setup-go` workflows. The [official Go download feed](https://go.dev/dl/?mode=json) lists this compiler as stable.

Dependency fixes remove [GO-2026-5970](https://pkg.go.dev/vuln/GO-2026-5970), [GO-2026-6629](https://pkg.go.dev/vuln/GO-2026-6629), and [GO-2026-5024](https://pkg.go.dev/vuln/GO-2026-5024). Source scanning initially found no called dependency vulnerability, but scans of both stripped and unstripped binaries flagged `x/text`. `x/text` is now 0.41.0. The renderer's dependency graph selects `x/sys` 0.47.0 and `x/sync` 0.22.0.

The renderer correction is a separate runtime fix. Bubble Tea 2.0.1 could skip a pending clear when its view had already rendered. `TestSettledViewClearScreenPTY` reproduced the defect with unchanged content and no later model updates. It failed with no repaint after the clear request. Bubble Tea 2.1.0 [tracks a pending erase through the next flush](https://github.com/charmbracelet/bubbletea/blob/v2.1.0/cursed_renderer.go), and the regression passes. This update requires Go 1.26 and its accompanying renderer dependencies. Installation instructions now state that minimum and recommend a supported compiler with current security patches.

PTY search tests now share a helper that waits for each character's visible cursor update. A two-character query can arrive as separate differential redraws, so its full text need not appear contiguously in the raw stream. Wide navigation waits for the complete cleared frame and footer before checking for stale content or scroll-region operations. All previous final-frame assertions remain in place.

The settled-clear, wide-navigation, single-letter search, and keyboard-guide query tests passed 50 repetitions each after these changes, covering 200 real PTY runs in 48.294 seconds.

The follow-up browser verification reproduced two clipboard writes from one copy-button click. Both the button and its parent card handled the same bubbling event. Removing the redundant button handler preserves card and button copying through one handler. Live verification against the rebuilt Linux archive confirms one write each for the button, card, and Enter key. The synthetic execution sentinel remains absent.

## Verification commands

The final uncached suite passes 1,425 tests and subtests across 17 tested packages, including 166 PTY cases. `TestShadowMacOSMatrix` is the only skipped test. `internal/exportfile` has no independent test files; its application export paths are covered by the application and command suites. Required `make fmt check`, the full non-PTY race suite, and the full suite on the Go 1.26.0 compatibility minimum also pass after the renderer and dependency changes. Minimum-version testing checks compatibility only; release artifacts use patched Go 1.27.2.

The final run uses private temporary directories, synthetic aliases, local repositories, and synthetic credentials. Heavy jobs run in sequence, with `GOMAXPROCS=8` and `GOFLAGS=-p=4` to bound compilation load.

```bash
AL_REQUIRE_PTY_SHELLS=1 make fmt check
AL_REQUIRE_PTY_SHELLS=1 go test -count=1 -json ./...
AL_REQUIRE_PTY_SHELLS=1 go test -race -count=1 -skip PTY ./...
GOTOOLCHAIN=go1.26.0 AL_REQUIRE_PTY_SHELLS=1 go test -count=1 ./...
AL_REQUIRE_PTY_SHELLS=1 scripts/verify-catalog-workflow.sh
go mod verify
go mod tidy -diff
go list -m -u -mod=readonly all
govulncheck -show verbose ./...
actionlint .github/workflows/*.yml
goreleaser check
goreleaser release --snapshot --clean --skip=publish
scripts/verify-release-artifacts.sh dist
govulncheck -mode=binary alias-lens
```

All three fuzz targets pass separate 15-second runs with two workers. The benchmark smoke matrix passes all 85 workloads, including the 10,000-entry Bash and Zsh startup membership checks. It uses `BENCHTIME=100ms` and `BENCH_COUNT=1` to check execution and membership, rather than establish a new performance baseline.

Strict catalog verification, module integrity and tidiness, workflow validation, and GoReleaser validation pass. Source and both stripped and unstripped Linux x86-64 binary scans report zero vulnerability findings. The snapshot contains four archives, four SPDX JSON SBOMs, and `checksums.txt`; archive contents and all eight listed checksums pass verification. Build metadata confirms Go 1.27.2 in all four binaries. Native archive setup, repair, removal, login and nonlogin shell PTYs, and the compiled tmux checks pass on the final dependencies.

The complete final verification script exits zero. Final staged and unstaged whitespace checks also pass. Sanitized summaries are recorded here; detailed temporary logs remain outside the repository under `/tmp/al-mint-release-evidence/`.

## Terminal and archive coverage

The compiled browser runs at 48x18, 80x24, and 160x40 inside an isolated tmux server. Checks cover the selected alias, search filtering, keyboard help, statistics, footer settings, themes, sync, health, revisions, and return navigation. The browser keeps its search field, action footer, and maker line visible. The wide detail pane appears at 160 columns. `NO_COLOR=1` remains readable, and `TERM=dumb search` produces plain output without ANSI escapes.

The full PTY suite covers direct terminals, resize, wide redraw, configured shortcuts, repeat events, favorite writes, diff and revision previews, catalog review and cancellation, actual shell startup, native masking, arguments, exit status, signal forwarding, prompt handoff, and `Ctrl+G` behavior.

The Linux x86-64 snapshot archive is extracted and tested independently. Bash and Zsh each run login and nonlogin PTYs. Setup, repair, removal, synthetic alias execution, and version output use that extracted executable. Removal preserves the alias bytes, mode `0600`, config, and unrelated shell startup files. Doctor correctly reports unconfigured repository, autosync, and provider credentials as missing; that expected exit status is not counted as a fully configured doctor pass.

The optional browser bundle passes `npm ci --ignore-scripts`, `npm run build`, and `npm audit --omit=dev` in a disposable copy. The rebuilt bundle matches the embedded bytes exactly, and the audit reports zero vulnerabilities. The compiled `--web` endpoint also passes live browser search and reload checks with synthetic aliases and no console errors. The test service is stopped and its temporary session data removed afterward. Go integration tests cover the authenticated loopback API and read-only diff requests.

## Fresh verification after the follow-up request

The full matrix was rerun with new logs and disposable homes. It passed `make fmt check`, the uncached full suite, the full non-PTY race suite, the full Go 1.26.0 suite, strict catalog verification, module checks, workflow linting, source vulnerability scanning, packaging, native archive checks, tmux checks, all three fuzz targets, and all 85 benchmark smoke workloads. The uncached suite again reports 1,425 passing tests and subtests, 166 PTY cases, 17 tested packages, and only the macOS matrix skip. The four affected PTY cases passed another 50 repetitions each, taking 47.517 seconds for 200 runs.

The browser copy defect was corrected after that full matrix completed. The final asset then passed JavaScript syntax validation, another complete `make fmt check`, an uncached command-package suite, command-package race checks, and command-package Go 1.26.0 checks. Source and unstripped build vulnerability scans passed again. The release snapshot was rebuilt, and all final archives, checksums, SBOMs, native Linux archive flows, tmux widths, and an isolated `make install` passed.

All four final packaged binaries pass static vulnerability scanning and report the expected OS, architecture, and Go 1.27.2 compiler. All four SBOMs parse as SPDX JSON with unique identifiers and valid relationship references; each lists 34 packages. Cross-built binary checks do not establish native ARM64 or macOS execution.

Live tests against the final extracted Linux archive verify search, actual arrow-key and Escape events, single-write copying, and reload after adding a synthetic alias. The served `app.js` bytes match the final source. API checks return `401` for missing or invalid authentication, `403` for the wrong Host or Origin, `405` for a write request, and `200` for an authenticated read. No alias executes and no console error appears. Clipboard instrumentation, the temporary session credential, and the test service are removed afterward.

The initial 597 tracked-file hashes stayed unchanged through the fresh full matrix. The browser asset is the sole tracked-file change afterward and has the additional verification above. The original staged patch stayed byte-for-byte unchanged throughout verification. Fresh detailed logs are outside the repository under `/tmp/al-mint-reverify-t0n6cx5g/`.

The owner subsequently requested a commit and push of the Linux feature branch. The commit includes the optional-Zsh helper and acceptance criteria needed by the strict-shell correction. The unrelated staged performance-table edit remains outside the commit.

## Earlier failed or inconclusive runs

- Missing Zsh initially prevented strict evidence. Required runs began after installation.
- The controlled missing-Zsh strict test intentionally exits nonzero. It confirms the release gate rather than a product failure.
- A Go 1.24.2 PTY run timed out waiting for one startup fixture during overlapping heavy jobs. That case passed ten isolated repetitions. A later old-toolchain run overlapped the minimum-version dependency change and is invalid evidence for the final worktree.
- Overlapping compilation, race tests, packaging, and PTYs drove the host load above 60. A Zsh prompt timed out, and the raw search and wide redraw assertions failed. Final evidence uses sequential heavy jobs; the reproduced renderer defect and the incremental-output assumptions were fixed.
- A temporary Python terminal emulator produced an inconsistent differential screen. Its screen assertions are excluded. The compiled tmux captures and repository PTYs provide the terminal evidence.
- The first archive harness found an existing executable through PATH. It was corrected to place the extracted archive first and disable credential programs. Only the corrected isolated runs count as archive evidence.
- The fresh browser-bundle harness expected an optional `diff.js.LEGAL.txt` sidecar that neither the build nor the embedded assets contains. Checking the actual generated asset inventory instead passes with matching bytes. A browser navigation helper also used an unavailable `URL` constructor in the tool runtime; corrected navigation succeeded. These harness failures are excluded from product verification results.

## Remaining Linux release gates

- Native Linux ARM64 execution is unavailable on this x86-64 host. The ARM64 binary, archive, checksum, and SBOM are cross-built and inspected only.
- Combine the reviewed Linux changes with the intended candidate work and rerun required CI on that exact clean commit. This record covers the tested source before its Linux feature-branch commit, including the subsequent browser correction and its verification.
- Verify checksums and attestations for the final published Linux assets and install the exact new public tag. The existing `v1.0.0-rc.4` public Go installation passed independently; it does not certify the new candidate.

WSL verification belongs to the separate WSL thread. This record makes no new WSL or macOS runtime claim.
