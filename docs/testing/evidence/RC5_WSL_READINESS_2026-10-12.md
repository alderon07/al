# RC5 WSL readiness, 2026-10-12

The selected candidate is `v1.0.0-rc.5`. Automated release checks pass. WSL kernel restart validation remains open, so this report does not declare complete WSL release readiness.

The prepared code is `c22c10edbb55b1a6de4c4fba6d00f04a845e7ce6` on `release/readiness-2026-10-12`. [PR #6](https://github.com/alderon07/al/pull/6) targets `dev`. Fetching `origin/dev` found no changes beyond the already integrated `31a33ec`. The isolated checkout preserves the concurrent Linux audit changes in the original workspace.

## Corrections

- `3c04ee2` accepts imported WSL PATH directories with trailing separators while retaining rejection of empty, relative, and dot-segment entries. The regression failed against upstream and passed with the fix.
- `d115eac` selects Go 1.27.2 and fixed `golang.org/x/text v0.41.0`, `golang.org/x/sys v0.44.0`, and the resulting `golang.org/x/sync v0.22.0`. Those dependency versions require Go 1.25.0. Installation documentation matches the new minimum and preferred toolchain.
- `9ec84d8` shares a Linux PTY fixture launcher across the shell and terminal tests. It opens the master with `O_NOCTTY` and close-on-exec, then obtains its slave with `TIOCGPTPEER`. Other Unix systems retain the existing launcher. Tests verify terminal ownership and dimensions, exit status, caller streams, preserved process attributes, and descriptor cleanup after a failed start.
- `c22c10e` removes `Ctrl+,` from the Windows default footer bindings. Native input opened Windows Terminal's settings instead of Alias Lens's footer. The Windows profile now advertises `o / F3`; Linux and macOS retain their existing settings bindings. Label and dispatch regressions failed before the correction and pass afterward.

## Environment and automated checks

The host is WSL 2.7.14.0 on Windows 10.0.26300.9457, Ubuntu 24.04.5 LTS, kernel 6.18.33.2-microsoft-standard-WSL2, x86_64. Windows Terminal is 1.24.12741.0; tmux is 3.4. Bash is 5.2.21. Both system Zsh 5.9 and Homebrew Zsh 5.9.2 are installed. The module automatically selects Go 1.27.2 from the installed Go 1.27.1.

[All six CI jobs pass for c22c10e](https://github.com/alderon07/al/actions/runs/38323178723): Ubuntu and macOS tests, both shadow matrices, source vulnerability scanning, and release-candidate packaging with race checks and archive scans. The earlier toolchain/dependency and PTY launcher commits also passed all six jobs.

| Check | Result |
| --- | --- |
| Full uncached unit and integration audit | 1,428 passing test/subtest events before adding the launcher tests; coverage 64.3%. See the earlier [WSL audit](WSL2_TEST_AUDIT_2026-10-12.md). |
| Launcher change: all required-shell PTYs, five repetitions | 835 passing test/subtest events, no failures or skips. |
| Launcher change: focused startup case under syscall tracing | 1,000 repetitions pass; no failed `TIOCSCTTY`. |
| Final shortcut change: all required-shell PTYs, three repetitions | 501 passing test/subtest events, no failures or skips. |
| `AL_REQUIRE_PTY_SHELLS=1 make fmt check` after each correction | Pass: formatting, full suite, vet, binary build, and whitespace checks. |
| Non-PTY race suite and separate launcher race tests | Pass locally; the final candidate CI race suite also passes. |
| Module verification and tidy diff | Pass, no module changes. |
| Three fuzz targets | Two 15-second runs with two workers pass for each target in the earlier audit. |
| Actionlint and GoReleaser configuration | Pass with pinned tools. |
| Catalog workflow | Bash/Zsh bootstrap, read-only preview, installed entries, rollback, and catalog PTYs pass. |
| Source and compiled archive vulnerability scans | No vulnerabilities found after the dependency correction. |

The old Go 1.24.2 release build exposed 40 reachable standard-library advisories. Go 1.27.2 cleared those findings. Binary scanning additionally found GO-2026-5970 in normalization code even when source reachability analysis did not report a call trace. The dependency update removes that finding, GO-2026-6629, and GO-2026-5024. Both CI snapshots and release builds now scan all four archived executables before publication. A real vulnerable archive failed the new gate; fixed archives passed.

The intermittent launch failure was reproduced at `TIOCSCTTY` before the shell executable ran. An original-launcher control failed ten times in 500 traced repetitions, and a matching diagnostic launcher failed five times in another 500. Descriptor-based launches passed the isolated 500-run experiment and the implemented 1,000-run check. A timing-altered baseline experiment also passed 2,000 repetitions. These results verify the new launcher on this host; they do not establish the exact kernel cause or guarantee that every possible launch fault is eliminated. No retry or required-shell skip masks an error.

## Archives and native terminal evidence

Pinned GoReleaser 2.18.0 and Syft 1.42.3 downloads were verified against published checksums. The clean c22c10e checkout produced four `1.0.0-rc.5-SNAPSHOT-c22c10e` archives, four SPDX SBOMs, and checksums. Contents, checksums, SBOM structure, build metadata, and all four archived binary scans pass. The native Linux archive passed isolated Bash/Zsh setup, doctor expectations, repair, completion installation/removal, and setup removal during preparation. Persistent homes now contain the c22c10e binary.

| Archive platform | SHA-256 |
| --- | --- |
| darwin/amd64 | `52343a1c9d5a74284e70d47d39d6505a9f6c72845cb3426e6d16303f17fbc2b7` |
| darwin/arm64 | `9636b076c9047246a6eb61861b6984a074f7b77394a10487f9c8876e4777b91d` |
| linux/amd64 | `353bbae181977636f988092d269c6d5cc2707d7fe64fe0e3a223e78aa30946eb` |
| linux/arm64 | `23e798fd51050b13ad9d9542a3b13bf8e33705300366797ec6c5f26eb820ac4c` |

The native c22c10e executable hash is `cf92fe6afe815ce707776626fc1b8b84bf693415842000576fe6127bde783263`.

Compiled tmux alias, search, and help flows pass at 48x18, 80x24, and 160x40. `TERM=dumb` produces plain search output. The Windows Terminal matrix uses dedicated windows and synthetic homes; `stty` confirms the requested dimensions. Window-only images show the selected alias, search field, footer, maker line, and a non-ASCII description with `NO_COLOR=1`.

Before the final shortcut correction, native Windows input passed F1 through F8, search, and page return for Bash and Zsh at all three sizes. Host `Ctrl+Shift+V` pasted `?` without opening help in command mode and appended it to search text in search mode. Clipboard contents were restored after the synthetic check. These host runs used the 9ec84d8 binary; the subsequent production change removes only the terminal-owned footer chord. The host-owned chord was separately reproduced before that correction. On the c22c10e binary, the 48x18 Bash run passed all function keys, search, host copy, and host paste. Its guide omitted `Ctrl+,`. Native Bash shell tests at 80x24 also passed Ctrl+G launch, Enter execution, exit status 41, Tab without execution, appended arguments, and nonempty-prompt cancellation. The complete Zsh shell handoff check remains open.

Windows automation diagnostics included a wrong distribution name, an incorrect UI Automation control type, premature prompt expectations, unsupported PowerShell type spelling, command-line length limits, and focus-guard failures. Early posted-message key tests were inconsistent and are not accepted as physical key evidence. The accepted native tests use `SendInput` only while the dedicated test window owns focus. Attempts to regain focus can interrupt desktop typing; foreground input automation was stopped pending a controlled host run. The ordinary `al` invocation prints a Tab selection; the editable prompt handoff must be checked through the Ctrl+G launcher.

## Restart preparation and open gates

Persistent synthetic Bash and Zsh homes, binaries, metadata, and logs are saved outside the repository under private `/var/tmp/alias-lens-release-readiness-20261012`. Pre-restart checks pass for login and nonlogin shells, binary resolution, the `al` function, completion, successful execution, exit status 41, Windows profile detection, expected doctor notices, and unchanged startup/alias hashes and modes. The post-check correctly rejects the current unchanged kernel ID.

A prepared Windows driver can shut down WSL, reopen Ubuntu-24.04, and run the saved post-check. It has passed its preparation mode. It has not performed shutdown. That operation stops this session and every running WSL process.

The pre-restart shadow verifier passes with these hashes:

```text
manifest 3272702548a24c6b8181d8fab21afb957487f02e0906c7c2df595b4dac5252e1
plain 69fa1a36e59413963dc7b89f48dd8023349de9ae20745221deda46626fb81d4e
json 7b49b8b3e23673e11aa349d8d84b62779dd65c65723ee05c71e7aa9f9c77b2ce
```

Still open: actual WSL shutdown/restart with matching post-restart hashes; complete final-binary Windows Terminal preferred-chord and width matrix, Zsh shell handoff, long-command, and live-resize checks; native architecture/platform release evidence outside this WSL host; and published archive attestations. Snapshot builds and macOS CI do not close the manual platform gates. No RC5 tag, release publication, or stable release claim has been made.

Raw logs and synthetic screenshots stay outside the repository. Earlier `/tmp` audit logs were copied into the private persistent directory before preparing shutdown.
