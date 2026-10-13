# RC5 WSL readiness, 2026-10-12

The selected candidate is `v1.0.0-rc.5`. Automated and native Windows Terminal checks pass for the combined candidate. WSL shutdown/restart verification passed. All required checks for the prepared WSL candidate pass.

The tested code is `2cbd387c469dfae0899bf30ad420568b4c827d97` on `release/readiness-2026-10-12`. [PR #6](https://github.com/alderon07/al/pull/6) targets `dev`. The merge combines the WSL preparation and Linux commit `f8a7da31a99586f7e74883b017648b4bf7ca2f58`, retaining both parents. Fetching `origin/dev` found no newer changes beyond `31a33ec`. Concurrent Linux edits in the original workspace remain preserved.

## Corrections and regression evidence

- `3c04ee2` fixes WSL PATH directories with trailing separators while retaining rejection of empty, relative, and dot-segment entries. The regression failed before the correction and passed afterward.
- `d115eac` selects patched Go 1.27.2 and fixed normalization/system dependencies. Source and compiled archive scans clear the previously detected advisories. Both candidate CI and release publication require scans of all four archived executables.
- `9ec84d8` shares a Linux PTY fixture launcher that opens its master with `O_NOCTTY` and close-on-exec and obtains the descriptor-linked slave with `TIOCGPTPEER`. Other Unix systems retain their existing launcher. Meaningful tests cover terminal ownership, dimensions, exit status, caller streams, existing process attributes, and descriptor cleanup after a failed start.
- `c22c10e` removes Windows Terminal's settings chord `Ctrl+,` from the Windows default. Physical host input reproduced the conflict. Windows now advertises `o / F3`; Linux and macOS retain their conventions. Label and dispatch regressions fail before the correction and pass afterward.
- `2cbd387` combines the Linux Bubble Tea 2.1.0 renderer correction, its Go 1.26 minimum and dependency graph, the browser copy correction, and stronger PTY assertions with the WSL fixes. The settled-view clear regression failed on the earlier WSL candidate: `PTY redraw did not contain "PTY_CLEAR_END": ""`. It passes 50 repetitions on the combined tree. The new regression uses the descriptor-based launcher. The preferred release toolchain remains Go 1.27.2.

The earlier PTY launch experiment reproduced `TIOCSCTTY` failure before shell execution: ten failures in 500 original-launcher traced repetitions and five in 500 diagnostic repetitions. Descriptor-linked launches passed 500 experimental repetitions and 1,000 implemented repetitions. A timing-altered baseline also passed 2,000 repetitions. These results verify the launcher on this host without establishing an exact kernel cause. Required-shell failures are neither retried nor skipped.

## Environment and automated results

WSL is 2.7.14.0 on Windows 10.0.26300.9457. The distribution is Ubuntu 24.04.5 LTS, kernel 6.18.33.2-microsoft-standard-WSL2, x86_64. Windows Terminal is 1.24.12741.0; tmux is 3.4; Bash is 5.2.21. System Zsh 5.9 and Homebrew Zsh 5.9.2 are installed. Builds select Go 1.27.2.

[All six CI jobs pass on the combined commit](https://github.com/alderon07/al/actions/runs/38325473094): Ubuntu/macOS tests, both shadow matrices, source vulnerability scanning, and candidate packaging with race checks and archive scans.

| Check on the combined candidate | Result |
| --- | --- |
| `AL_REQUIRE_PTY_SHELLS=1 make fmt check` | Pass: formatting, full suite, vet, build, whitespace checks |
| Uncached full suite | 1,434 passing test/subtest events across 18 tested packages; only the macOS shadow matrix is skipped on WSL |
| All required-shell PTYs, three repetitions | 507 passing events, no failures or skips |
| Settled-view clear regression | Expected red result on the preceding candidate; 50 green repetitions on the combined tree |
| Non-PTY race suite and separate launcher race tests | Pass |
| Catalog workflow with Homebrew and system Zsh | Both pass; system-shell run puts `/usr/bin:/bin` before Homebrew in PATH |
| Module verification and tidy | Pass; no module changes after tidy |
| Browser script syntax | Pass; Linux's live copy evidence is recorded in its separate report |
| Source and all four compiled archive vulnerability scans | No vulnerabilities found |
| Native Linux archive | Pass: matching executable, version, isolated Bash/Zsh setup, doctor expectations, repair, completion, saved profile override, automatic Windows profile, and removal |
| Compiled tmux | Alias/search/help at 48x18, 80x24, 160x40 and plain `TERM=dumb` search pass |
| Pre-restart persistent installation | Login/nonlogin startup, binary/function/completion, alias exit status, Windows profile, doctor expectations, unchanged startup/alias hashes and modes pass |
| Actual WSL shutdown and persistence | Pass: changed kernel ID; Bash/Zsh startup, binary, completion, alias modes, and shadow hashes preserved |

The prior [WSL audit](WSL2_TEST_AUDIT_2026-10-12.md) records PATH TDD evidence, coverage of 64.3%, and two 15-second runs for each of three fuzz targets. The [Linux report](RELEASE_LINUX_MINT_2026-10-12.md) records its native terminal/browser evidence, minimum-Go compatibility checks, fuzzing, and benchmark smoke results. Those prior results retain their original commit scope.

## Combined snapshot artifacts

Pinned GoReleaser 2.18.0 and Syft 1.42.3 downloads were checked against published checksums. A clean combined checkout produced four `1.0.0-rc.5-SNAPSHOT-2cbd387` archives, four SPDX SBOMs, and checksums. Archive contents, checksums, SBOM structure, build metadata, and binary vulnerability scans pass. Native lifecycle and host checks use that snapshot's Linux/amd64 executable.

| Platform | Archive SHA-256 |
| --- | --- |
| darwin/amd64 | `e265a77a149ee55c3c2cef06c4c71379ccc35f40659347d66371e3844a5a88ae` |
| darwin/arm64 | `08887bf233681fd408c99b1fffbbb94b4ed0a3c072a5a8995ffc448e069cee5a` |
| linux/amd64 | `f44af5a02a654b0f547e8e8bfa2b28defdbf426db3a2b96b739452b2de2747a9` |
| linux/arm64 | `1894d96b99c2825fea38d7144c55ac3f8447275274d8e7f08921fd9d96a44e4f` |

The native executable SHA-256 is `ad0cc8b3956b58f23fbc5ddb1bb1bbf1fc36d7af24e1d230592d91668d107a10`. Evidence-only commits after `2cbd387` do not change its runtime source. These hashes identify the tested snapshots; tagged release artifacts require their own checksums and provenance verification.

## Controlled Windows Terminal evidence

The user authorized taking terminal focus and restarting WSL. Each native run creates a dedicated Windows Terminal window and uses an isolated synthetic Bash or Zsh home. The launcher starts with a restricted environment. Physical key delivery uses `SendInput` only when the owned window and its `TermControl` own focus. Clipboard checks restore the previous clipboard without recording its contents. Captures cover only the owned window.

- Both shells pass at 48x18, 80x24, and 160x40. `stty` confirms the dimensions. The selected alias, search, footer, maker line, and non-ASCII description remain visible with `NO_COLOR=1`. The narrow and wide captures were inspected.
- F1 through F8, search, page return, host copy, and host paste pass at all six shell/size combinations. Pasted `?` does not invoke help and appends to the active search. The guide omits Windows Terminal's `Ctrl+,`.
- Both shells pass Ctrl+G launch at an empty prompt, Enter execution, exit status 41, Tab without execution, appended arguments, and cancellation of a nonempty prompt. Editable handoff is tested through the shell's Ctrl+G launcher.
- Both shells pass preferred letter bindings and control fallbacks at 80x24: page access, add/cancel, edit/cancel, context marks, favorites, refresh, revisions, delete/cancel/confirm, and Ctrl+S save of a synthetic alias. Delete fallback is tested with an unfiltered list.
- Both shells pass the `l` catalog entry point and `p`/Ctrl+G commit behavior without a configured repository. The commit operation reports the setup command and leaves the fixture isolated.
- Both shells pass long-command live resizing from 80x24 to 48x18, then 160x40, then back to 80x24. Kernel PTY dimensions are checked after every resize. The complete alias frame and maker line remain visible. Bash window captures and Zsh terminal text were inspected; a direct visible-pixel check confirmed the complete Zsh redraw.

The controlled harness initially expected Delete to act on a filtered list and overcorrected native window dimensions. `PrintWindow` omitted part of some Zsh resize captures even after the terminal text settled. A direct visible-pixel check confirmed that the actual view was complete; it also picked up the transparent terminal background, so those direct captures were deleted from both artifact locations. They are not retained evidence. The corrected harness tests an unfiltered Delete fallback, dampens dimension adjustments, and verifies the complete resized terminal text. Incomplete `PrintWindow` images remain diagnostic evidence only. Earlier automation diagnostics and pre-combination host results remain in the private logs.

## Restart preparation and publication gates

Persistent synthetic homes, binaries, metadata, logs, and the independent Windows task are saved under private `/var/tmp/alias-lens-release-readiness-20261012`. The Windows driver shuts down WSL, reopens Ubuntu-24.04, verifies a changed kernel boot ID, checks both installations, and compares every shadow output line. A guarded finalizer records, scans, commits, and pushes the restart result only after verification succeeds and the prepared branch is unchanged.

The pre-restart shadow baseline is:

```text
PASS shadow-wsl
manifest 3272702548a24c6b8181d8fab21afb957487f02e0906c7c2df595b4dac5252e1
plain 69fa1a36e59413963dc7b89f48dd8023349de9ae20745221deda46626fb81d4e
json 7b49b8b3e23673e11aa349d8d84b62779dd65c65723ee05c71e7aa9f9c77b2ce
```

Native architecture/platform manual evidence outside this WSL host and published archive attestations remain release-publication gates. Snapshots and macOS CI do not close those gates. Both platform tracks must agree on the tagged candidate and artifact hashes before publication.

Raw logs and synthetic screenshots remain outside the repository. Earlier `/tmp` audit logs were copied to private persistent storage before shutdown preparation.

## Verified restart

The independent Windows task ran `wsl.exe --shutdown`, reopened Ubuntu-24.04, and completed the saved post-check. The previous kernel boot ID was `3573e4d5-56fe-4567-b4e7-8c0fea6e8f8c`; the restarted kernel boot ID is `e099404c-deae-4b10-8501-8e22b8b8b7d1`. Both persistent homes retain the candidate executable SHA-256, startup and alias bytes, permissions, binary resolution, `al` function, completion, exit status 41, and expected doctor results. The post-restart shadow result matches every pre-restart line and all three recorded hashes.

The task committed and pushed this result only after those checks passed. Raw logs remain in the private persistent readiness directory.
