# Performance baseline acceptance

Written before benchmark implementation on 2026-10-08. This pass measures existing behavior and adds no production optimizations or dependencies.

| Criterion | Required evidence |
| --- | --- |
| PB-001 Synthetic workloads | Generate deterministic alias and catalog datasets with 100, 1,000 and 10,000 entries. Use only disposable homes, repositories and history. Never read the user's configuration, history or credentials. Shell startup may use smaller explicit sizes if installed-artifact limits prevent the largest case; report the reason. |
| PB-002 Large input | Benchmark actual Bash/Zsh parsing and catalog decoding/resolution. Report bytes, allocations and input sizes. Preparation and file generation stay outside the timed section. Validate expected entry counts and successful results. |
| PB-003 Search | Benchmark current application search with prefix, semantic, typo and no-match queries, plus default suggestion ranking. Inputs include metadata, favorites and usage. Check results outside timing so a failed or empty fast path cannot masquerade as success. |
| PB-004 Picker loading | Measure application entry loading from private native files and an installed catalog. Separately measure the current picker model's initial view at narrow and wide widths, including ranking/rendering cost. State which startup steps are excluded; do not label an in-memory render as complete launch latency. |
| PB-005 Shell startup | Measure real fresh interactive Bash and Zsh processes through a PTY with empty, native integration and verified catalog startup fixtures. Include process/PTY overhead and distinguish login from nonlogin routes where supported. Verify integration and installed membership without invoking entries. Build, setup and activation stay outside timing. Bound subprocess execution, drain output and reap processes on failure. Missing shells must fail the documented full evidence run; unsupported platforms may skip explicit shell cases. |
| PB-006 Repeatable runner | Provide one documented command for allocation and latency measurements with configurable benchmark duration, repetition and output location. Record Go, OS/architecture, CPU, shell versions, source commit and dirty state. Do not capture environment dumps or private paths. Preserve failures through output capture and clean temporary artifacts. No timing thresholds in ordinary CI or tests. |
| PB-007 Evidence and review | Medium implementation, separate high review, medium corrections and high re-review. Run every benchmark case, record repeated Linux results and limitations, and run `make fmt check` and actual terminal PTYs. Native macOS/WSL performance measurements remain pending until recorded on those systems. |

## Scope and interpretation

These are warm filesystem-cache measurements on a named development machine. Fresh subprocesses do not imply a cold OS cache. Go allocation reports exclude allocations in child processes. Report sample variation and the largest-input hotspots without claiming a speedup or choosing a language rewrite. Profiles may be collected using the standard Go benchmark flags with synthetic workloads.

All persistent fixtures remain under private temporary roots. Benchmark-only helpers stay in their owning test packages. Existing shell adapters, application services, catalog codecs and private terminal models are the measured code paths. No product API is added for benchmarks.
