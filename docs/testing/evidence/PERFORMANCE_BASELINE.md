# Performance baseline evidence

## Scope, 2026-10-08

The user authorized benchmarks after the catalog workflow and package refactor. Acceptance criteria in `docs/acceptance/PERFORMANCE_BASELINE.md` preceded implementation. This pass adds benchmark fixtures, a runner and documentation. It changes no production algorithms, package APIs or dependencies.

## Runner review

Medium implementation added `make benchmark` and `scripts/benchmark-performance.sh`. Separate high review requested a failure when a benchmark selection produces no measurements. The correction checks actual Go measurement rows after capture.

Independent high re-review approved the runner. Temporary fake-Go harness probes verified that exit status 71 survives output capture, an empty successful run fails with selection guidance, reports use mode `0600`, canonical private temporary roots are cleaned, and an existing output file is refused without changing its bytes. These probes test the harness; they are not performance measurements. Shell syntax validation passed.

## Workload review and corrections

Medium implementation added five benchmark-only files beside their owning packages. Separate high review and medium corrections resolved misleading throughput for typed catalog resolution, a generation source hash that did not match the canonical catalog, unstable automatic picker color detection, and startup membership checks that could discard an earlier failed predicate. The final fixtures bind canonical source bytes, use an explicit true-color renderer, group all startup predicates, and detect entry execution through a private sentinel. Startup subprocess cancellation kills the PTY process group; output is bounded, drained and the shell is reaped. No product code changed.

Medium validation passed all core cases and all 28 shell startup cases at one iteration, including 10,000 entries without limit skips. Separate high validation passed every core case and the four largest catalog startup cases for Bash/Zsh and both routes. Negative real-PTY cases reject missing integration or a missing last entry even when the total entry count still matches. Focused timed race checks passed for catalog decoding/resolution, alias parsing, installed catalog entry loading and picker rendering. Darwin ARM64 command test compilation passed.

The final `make fmt check` passed after source freeze, including command 23.024 seconds, app 59.866 seconds, shell 1.395 seconds and TUI 13.452 seconds, plus all other packages, vet, build and whitespace checks. Actual Bash/Zsh and terminal PTYs ran in that suite. The local disposable Zsh runtime does not replace trusted system-shell release verification.

## Measurement conditions

Measurements use synthetic inputs and disposable homes. They include warm filesystem caches and exclude fixture preparation. Shell measurements include process creation, PTY startup, loading, verification and teardown. Go allocation counts exclude child processes. In-memory picker views and application entry reads are separate measurements, not complete interactive launch latency.

Native macOS and WSL performance baselines remain pending. Automated functional release evidence for those platforms is tracked separately.

## Repeated Linux baseline

`make benchmark` passed with `BENCHTIME=200ms` and `BENCH_COUNT=3`. All 85 cases emitted three measurements, for 255 rows. Coverage is six catalog, 18 parser, 27 application, six picker and 28 shell startup cases. No largest-input or shell cases were skipped. The complete sanitized Go report is [PERFORMANCE_2026-10-08.txt](PERFORMANCE_2026-10-08.txt).

Conditions recorded by the runner:

- Go 1.27.1, Linux amd64, kernel 7.0.0-34-generic.
- Intel Core i7-13705H; Go benchmark GOMAXPROCS suffix 20; each case uses a serial loop.
- Bash 5.2.21 and disposable Zsh 5.9 with genuine packaged completion functions.
- Start time 2026-10-08T18:31:09Z; source base `e90ec6f607eda746275f600f3ce5d7701aba535b`, with the benchmark additions uncommitted at measurement time.
- Benchmark source files, runner and Makefile were frozen before measurement and their hashes remained unchanged afterwards. Their sorted path-to-SHA256 JSON map has digest `bb775d95faad979727c6ca2ab80f79991417a1989fad2e61e6d11e10e8f6365f`.
- No other agent benchmark or repository-check run competed with the repeated run. Filesystem caches were warm; CPU frequency and other host activity were not controlled.

### Largest input results

These are medians of three per-run averages, with the minimum and maximum of those averages. Bytes are total Go allocations per operation, not peak resident memory. Each row uses 10,000 entries. Picker cases use 60 or 140 columns. Shell checks include startup, assertions, sentinel inspection and teardown; their allocation values exclude the shell and helper processes.

| Operation | Median ms/op | Sample range ms/op | Go allocated MiB/op |
| --- | ---: | ---: | ---: |
| Catalog decode | 90.821 | 87.182 to 93.111 | 44.052 |
| Catalog resolution | 11.466 | 10.585 to 14.004 | 5.950 |
| Bash alias parsing | 1.870 | 1.865 to 1.924 | 0.000 |
| Bash function parsing | 15.137 | 13.690 to 15.355 | 12.005 |
| Bash structural import | 2242.572 | 2240.518 to 2244.718 | 20.451 |
| Zsh structural import | 2240.928 | 2240.483 to 2241.293 | 20.451 |
| Prefix search | 6.305 | 5.748 to 7.507 | 10.197 |
| Semantic search | 14.079 | 11.964 to 14.587 | 12.427 |
| Typo search | 22.813 | 19.424 to 24.631 | 11.193 |
| No-match search | 29.896 | 27.393 to 31.727 | 19.989 |
| Suggestions | 12.241 | 11.863 to 14.427 | 10.367 |
| Bash native entry loading | 56.223 | 53.725 to 63.355 | 43.614 |
| Bash installed catalog loading | 392.921 | 384.397 to 398.413 | 308.870 |
| Zsh native entry loading | 55.185 | 53.818 to 64.398 | 43.538 |
| Zsh installed catalog loading | 400.775 | 390.452 to 424.011 | 308.790 |
| Picker first view, 60 columns | 17.835 | 17.436 to 19.296 | 12.902 |
| Picker first view, 140 columns | 17.475 | 16.213 to 17.752 | 13.253 |
| Bash native startup, nonlogin | 49.479 | 47.437 to 52.523 | 0.042 |
| Bash catalog startup, nonlogin | 691.805 | 664.209 to 704.964 | 0.042 |
| Bash catalog startup, login | 746.535 | 745.874 to 750.028 | 0.042 |
| Zsh native startup, nonlogin | 69.202 | 68.020 to 70.572 | 0.042 |
| Zsh catalog startup, nonlogin | 7950.606 | 7931.913 to 7957.012 | 0.042 |
| Zsh catalog startup, login | 7911.669 | 7888.572 to 7920.522 | 0.042 |

Controlled empty-shell medians were 5.983 ms for Bash nonlogin, 36.989 ms for Bash login, 19.970 ms for Zsh nonlogin and 19.995 ms for Zsh login. The Zsh baseline includes the shared real completion initialization; native aliases and catalog portable functions have different payloads. These values are comparisons within this fixture, not universal shell startup times.

## Findings for the next optimization pass

1. Structural import is the clearest measured target. Bash median time grew from 0.570 ms at 100 entries to 24.008 ms at 1,000 and 2,242.572 ms at 10,000. A separate one-iteration CPU profile of the 10,000-entry synthetic Bash case placed 98.89% of samples in SHA-256 block processing. It includes both untimed validation and the timed operation. Source inspection confirms `newShadowResult` calls `shadowOrigin`, which hashes the entire source for every entry. Compute that digest once per import while preserving origin IDs and golden output in a separately reviewed optimization.
2. Zsh catalog startup needs a separate shell/helper breakdown. The measured 10,000-entry result is about 7.9 seconds, versus about 0.7 seconds for Bash with the same portable catalog class. This benchmark identifies the delay but does not establish which Zsh operation causes it. Keep the verified-byte loader and readonly/alias handoff contracts intact during investigation.
3. Installed catalog entry loading allocates about 309 MiB per operation at 10,000 entries and takes about 0.4 seconds. Profile its decoding and validation before introducing caches; cache invalidation must preserve native drift checks, installed membership and approval freshness.
4. Typo/no-match search has about 310,000 allocations per operation at 10,000 entries. Profile edit-distance and normalization work before adding buffer reuse. Preserve ranking and avoid sharing mutable buffers across calls.

No optimization or performance threshold is part of this change. Native macOS/WSL measurements, cold-cache behavior, complete picker readiness and child-process memory remain unmeasured.

## Final review

Separate high review verified all 85 cases and 255 samples, the raw report's integrity, every summary calculation, unchanged source hashes and the CPU profile finding. Staged path inspection and the private-data scan found no private files or credential patterns. Review approved PB-007 with no unresolved findings; the required repository gate and actual PTYs passed. PB-001 through PB-007 are complete for this Linux baseline. The platform and measurement limits above remain open.
