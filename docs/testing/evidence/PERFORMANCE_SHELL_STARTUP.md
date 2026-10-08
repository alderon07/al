# Catalog shell startup optimization

The runtime change follows `docs/acceptance/PERFORMANCE_SHELL_STARTUP.md` and the parent performance acceptance criteria. Fixtures and measurements use synthetic declarations and disposable homes only.

## Diagnosis and correction

Separate parsing and parsing-plus-loading measurements reproduced the large Zsh delay before any entry body ran. The original generator embeds one conditional eval and alias-removal statement per function in a single shell function. Parsing the 10,000-entry wrapper alone consumed nearly all of the combined parsing and loading time. A constant-size loop prototype reduced that cost but still spent substantial time parsing its many literal arguments.

The correction groups the unchanged entry handoff statements into quoted batches of at most 100 entries for Zsh catalogs larger than 100 entries. The outer wrapper parses each batch as one literal; `builtin eval` then parses each smaller batch. Each function declaration still succeeds before its alias is removed. Readonly table rejection runs before every batch, and the wrapper propagates failed status. Bash and small Zsh handoff bytes remain unchanged. The batch commands use only existing protected shell control names and introduce no local variables, arrays, option changes or subprocesses. Immutable generation verification and structural declaration validation remain ahead of runtime generation.

The genuine PTY regression uses 205 mixed aliases and quoted multiline function bodies across both batch boundaries. It checks existing alias replacement, direct declaration equivalence, readonly function and alias tables, IFS/options, quoted arguments, status 23 and absence of a function-execution sentinel. Existing argument/status/signal, readonly-helper and verified-loader regressions cover the shared paths.

## Measurement conditions

The repeated isolated samples launch disposable Zsh 5.9 with `-f -c`, a private empty HOME/ZDOTDIR, a minimal environment and generated portable function definitions. They exclude Go generation and the application verifier and include process creation. Both original and batched handoffs define the same 100, 1,000 or 10,000 synthetic functions. Each workload runs three times with warm filesystem caches. A sentinel check rejects body execution after each sample. These noninteractive diagnosis samples are distinct from the complete interactive startup PTYs below.

Native macOS and WSL performance are not measured by these Linux samples. The disposable Linux Bash 3.2 runtime verifies compatible shell syntax but does not replace native Apple Bash release evidence.

## Isolated normal-host Zsh results

| Entries | Original parse median ms | Batched parse median ms | Original parse/load median ms | Batched parse/load median ms |
| ---: | ---: | ---: | ---: | ---: |
| 100 | 6.584 | 8.275 | 9.580 | 11.462 |
| 1,000 | 133.460 | 14.317 | 131.181 | 62.212 |
| 10,000 | 15,302.445 | 85.209 | 14,191.127 | 1,062.866 |

At 10,000 entries, original parse samples were 15,302.445, 15,715.505 and 14,685.560 ms; original parse/load samples were 14,191.127, 14,224.155 and 14,030.835 ms. Batched parse samples were 98.151, 83.607 and 85.209 ms; batched parse/load samples were 1,152.458, 1,062.866 and 1,044.374 ms. Combined parsing/loading improved by 92.5% in this direct comparison. The 100-entry path uses the same original source in both variants; its small variation reflects process/host timing.

## Complete interactive startup PTYs

The corrected startup benchmark ran with `-benchtime=200ms -count=3` and required both actual shells. It verified full membership outside timing for every fixture, then timed fresh login and nonlogin PTYs, checked installed declarations and rejected entry execution. The raw after report is [PERFORMANCE_SHELL_STARTUP_2026-10-08.txt](PERFORMANCE_SHELL_STARTUP_2026-10-08.txt).

Final evidence review found that the original untimed membership gate counted matching names and checked the last entry, which could accept a missing middle entry replaced by an unexpected name with the same count. PS-006 was written before correcting that gate. It now checks every expected name with builtin alias/function lookups, rejects extra names through the count check and rejects aliases masking a catalog function. The fixture generation and timed predicate are unchanged. Both shells passed valid controls and missing first/middle/last and wrong-kind negative PTYs for native aliases and catalog functions, including the same-count replacement with the last entry present. The original two negative PTY cases also remain and pass; no candidate executed in any of the 22 cases.

The corrected startup fixture SHA-256 is `ef6db253dc220bd535cedb97489bd97e593e50549d243239c201ce459ee2301f`. The original fixture hash after normalizing module imports was `39b0421fd43f9c517fa22ed237b843e6be5e4ff91c0f5a88a4a693e7356f2039`; the fixture now differs in its untimed membership gate and negative tests. The full 36-row report was repeated after this correction and supersedes the earlier after report. Production codec optimizations from the parent performance change also affect these complete startup samples, so the direct shell measurements above isolate the handoff correction.

| Shell | Entries | Route | Baseline median ms | After median ms | After range ms |
| --- | ---: | --- | ---: | ---: | --- |
| bash | 100 | nonlogin | 40.967 | 30.423 | 29.328 to 32.479 |
| bash | 100 | login | 61.589 | 75.775 | 60.828 to 95.073 |
| bash | 1,000 | nonlogin | 113.926 | 124.237 | 118.935 to 200.030 |
| bash | 1,000 | login | 149.403 | 158.863 | 157.268 to 242.730 |
| bash | 10,000 | nonlogin | 691.805 | 939.540 | 930.798 to 1003.835 |
| bash | 10,000 | login | 746.535 | 959.264 | 918.082 to 1207.532 |
| zsh | 100 | nonlogin | 42.692 | 59.794 | 55.137 to 61.469 |
| zsh | 100 | login | 46.832 | 57.062 | 55.662 to 95.287 |
| zsh | 1,000 | nonlogin | 146.874 | 134.773 | 125.195 to 183.415 |
| zsh | 1,000 | login | 160.318 | 129.043 | 127.091 to 141.604 |
| zsh | 10,000 | nonlogin | 7950.606 | 1439.911 | 1400.822 to 1570.480 |
| zsh | 10,000 | login | 7911.669 | 1393.074 | 1376.957 to 1444.316 |

The 10,000-entry Zsh startup median fell by 81.9% for nonlogin and 82.4% for login, exceeding PS-002. This repeated run emitted all 36 requested rows and passed every assertion. Go startup allocations remain about 44 KiB per operation and exclude shell/helper allocations.

Bash handoff generation is unchanged; its measured 10,000-entry startup was slower than the earlier recorded baseline. The direct original Zsh parser samples were also slower than the earlier complete-startup baseline, so host conditions were not identical across sessions. CPU frequency and unrelated host activity were not controlled. The same-workload isolated before/after comparison establishes the parser improvement, while the full PTYs establish real installed startup behavior. No Bash acceleration is claimed.

## Review and verification

The focused runtime handoff tests passed with genuine Bash 5.2 and Zsh 5.9 PTYs, including the new batch boundary/options/readonly matrix. Separate high review inspected the runtime generator, protected command dependencies, loader consumers, readonly handling and caller option preservation and reported no findings. The parent change runs the complete `make fmt check` gate and records its result in the combined performance evidence.

The focused runtime matrix also passed with the disposable Linux Bash 3.2 executable, including readonly preflight and the existing argument/status/signal PTY test. The final strengthened IFS/option comparison and no-execution sentinel passed again with Zsh in that run.

Separate high review of the membership correction found no remaining code issues. Final report and summary arithmetic were refreshed from the repeated corrected run.

The corrected complete-membership and negative startup PTYs also passed with disposable Linux Bash 3.2 and Zsh 5.9, exercising all 22 controls/rejections without candidate execution.
