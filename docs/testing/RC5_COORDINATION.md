# RC5 Linux and WSL coordination

The user selected `v1.0.0-rc.5`. This thread owns WSL readiness. The Lenovo thread owns Linux readiness. Both tracks need the same candidate commit and archive hashes before publication.

## WSL status

The tested combined code candidate is `2cbd387c469dfae0899bf30ad420568b4c827d97` on `release/readiness-2026-10-12`. It merges Linux commit `f8a7da3` and preserves the WSL PATH fix, patched Go 1.27.2 toolchain and archive scans, descriptor-based PTY launcher, and Windows Terminal shortcut correction. Bubble Tea 2.1.0 now requires Go 1.26. All six jobs pass in https://github.com/alderon07/al/actions/runs/38325473094. PR #6 remains the review entry point.

The combined tree passes `make fmt check`, 1,434 uncached tests/subtests, 507 required-shell PTY events over three repetitions, 50 clear-screen regression repetitions, race checks, Homebrew/system Zsh catalog workflows, module verification, source and four archive vulnerability scans, native archive lifecycle, compiled tmux widths, and pre-restart installation/shadow checks.

Controlled Windows Terminal checks pass for Bash and Zsh at 48x18, 80x24, and 160x40, including F1-F8, search, host copy/paste, preferred bindings, shell launcher/execution/Tab handoff, long commands, and live resizing. The exact snapshot executable and archive hashes are recorded in `docs/testing/evidence/RC5_WSL_READINESS_2026-10-12.md`.

WSL shutdown/restart verification passed with preserved installations and identical shadow hashes. The combined code candidate is ready for WSL review. Linux and publication gates remain separately recorded.

## Ownership and handoff

- The WSL thread preserves the Lenovo work on `internal/managedgit/runner_unix_test.go` and `docs/acceptance/LINUX_RELEASE_AUDIT_2026_10.md`.
- Lenovo can record its status and commit hashes below or share a committed patch for inclusion in the candidate branch.
- WSL owns the shared toolchain/dependency correction. Linux reruns its acceptance checks after taking that commit.
- Neither track creates or moves the `v1.0.0-rc.5` tag until both report their candidate results.
- WSL restart and Windows Terminal evidence remain separate gates. Unix PTYs do not establish host keyboard or clipboard behavior.
- Raw logs and synthetic homes stay outside the repository. This file contains only sanitized status and commit references.

## Lenovo status

The Linux thread pushed `f8a7da3` on `fix/linux-release-readiness`. Its sanitized Mint evidence is in `docs/testing/evidence/RELEASE_LINUX_MINT_2026-10-12.md`. WSL reproduced the settled-view renderer failure on its earlier candidate and merged the Linux branch as `2cbd387c469dfae0899bf30ad420568b4c827d97`, preserving both parents. The combined candidate selects Bubble Tea 2.1.0, Go 1.26 minimum, preferred Go 1.27.2, and the matching dependency graph. It also includes the Linux browser copy correction and stronger PTY assertions.

The combined WSL tree passes `make fmt check`, 1,434 uncached tests/subtests, 507 required-shell PTY events across three repetitions, 50 clear-screen regression repetitions, race checks, catalog workflow, source and four archive vulnerability scans, module verification, native archive lifecycle, pre-restart setup, shadow hashes, and compiled tmux widths. Native Windows Terminal and shutdown checks are in progress. Both tracks can review PR #6 directly; no user message relay is needed.
