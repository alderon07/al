# RC5 Linux and WSL coordination

The user selected `v1.0.0-rc.5`. This thread owns WSL readiness. The Lenovo thread owns Linux readiness. Both tracks need the same candidate commit and archive hashes before publication.

## WSL status

Latest prepared code: `c22c10edbb55b1a6de4c4fba6d00f04a845e7ce6`. All six jobs passed in https://github.com/alderon07/al/actions/runs/38323178723. The WSL feature branch includes the PATH fix, patched toolchain/dependencies, archive scanning, the shared Linux PTY launcher, and removal of Windows Terminal's `Ctrl+,` from the Windows default. PR #6 remains the review entry point.

The launcher passed five complete PTY repetitions (835 events) and 1,000 focused traced startup repetitions. The shortcut correction passed three complete PTY repetitions (501 events) and `make fmt check`. Four c22c10e snapshot archives and SBOMs pass contents, checksums, and vulnerability scans. Bash/Zsh Windows Terminal F1-F8 and search passed at all three sizes before the final chord removal. Persistent c22c10e installations pass pre-restart checks; shutdown and final host checks remain open. See `docs/testing/evidence/RC5_WSL_READINESS_2026-10-12.md` on the feature branch for candidate hashes and the gate status.

Preparation history:

- Committed WSL PATH resolution and audit evidence in `3c04ee2` on local `dev`.
- Full suite, three PTY repetitions, red/green regression verification, race tests, fuzzing, catalog workflow, and `make fmt check` passed. Coverage is 64.3%.
- Ten repeated catalog startup PTYs did not reproduce the earlier isolated Homebrew Zsh process-start failure.
- Release preparation now uses branch `release/readiness-2026-10-12` in `/var/tmp/alias-lens-release-readiness-20261012/checkout`, based on `3c04ee2`.
- The release workflow's Go 1.24.2 selection exposes 40 reachable standard-library advisories. Go 1.27.2 clears those findings.
- Binary scanning additionally detected GO-2026-5970 in `golang.org/x/text` despite no source-level call trace. The preparation branch selects `golang.org/x/text v0.41.0`, `golang.org/x/sys v0.44.0`, and the resulting `golang.org/x/sync v0.22.0`, with `go 1.25.0` and `toolchain go1.27.2`.
- Shared security correction is committed as `d115eacae6a803254080113fc7c1b871fc597217` on `release/readiness-2026-10-12`. Both source and compiled-binary scans now report no vulnerabilities, including module advisories.
- Updated dependencies pass the full suite, race checks, and `make fmt check`. An untraced three-run PTY retry passed all 492 events; the first repetition hit an isolated Bash launch EPERM. Broad syscall tracing caused startup timeouts and did not identify that launch failure.
- Draft PR https://github.com/alderon07/al/pull/6 is linked to this thread. All six CI jobs passed at d115eac, run https://github.com/alderon07/al/actions/runs/38318663812.
- All four RC5 snapshot archives and SBOMs built with pinned GoReleaser/Syft, passed checksums/content checks, and passed binary vulnerability scanning. The native Linux archive passed isolated Bash/Zsh setup, repair, completion, and removal.
- Focused syscall tracing reproduced the isolated launch error at TIOCSCTTY. Its cause is still being isolated; ordinary required-shell runs and the untraced retry pass.
- WSL terminal and restart checks are continuing. Preparation data and the candidate checkout moved to private /var/tmp storage so they survive WSL shutdown.

## Ownership and handoff

- The WSL thread preserves the Lenovo work on `internal/managedgit/runner_unix_test.go` and `docs/acceptance/LINUX_RELEASE_AUDIT_2026_10.md`.
- Lenovo can record its status and commit hashes below or share a committed patch for inclusion in the candidate branch.
- WSL owns the shared toolchain/dependency correction. Linux reruns its acceptance checks after taking that commit.
- Neither track creates or moves the `v1.0.0-rc.5` tag until both report their candidate results.
- WSL restart and Windows Terminal evidence remain separate gates. Unix PTYs do not establish host keyboard or clipboard behavior.
- Raw logs and synthetic homes stay outside the repository. This file contains only sanitized status and commit references.

## Lenovo status

Awaiting the Linux thread's status and commit references.
