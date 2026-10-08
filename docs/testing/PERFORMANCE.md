# Measure Alias Lens performance

Run benchmarks from the repository root with Go, Git, Bash and Zsh available:

```bash
make benchmark
```

The runner uses synthetic data, private temporary homes and fresh shell processes. It records the source commit, dirty state, Go version, OS/architecture, CPU and shell versions. It saves a private report under `/tmp` and prints its location. Temporary fixture files are removed after the run. The report contains benchmark names and measurements, not entry bodies, history, credentials or environment dumps.

## Repeat or select measurements

```bash
BENCHTIME=1s BENCH_COUNT=5 BENCH_OUTPUT=/tmp/al-performance-baseline.log make benchmark
BENCH_PATTERN=Search BENCHTIME=500ms BENCH_COUNT=5 make benchmark
```

`BENCH_OUTPUT` must name a new file; the runner refuses to overwrite an existing report. `BENCH_PATTERN` is a Go benchmark regular expression. A selection that produces no measurements fails. The default is all benchmarks in the catalog, shell, app, TUI and command packages. The default duration is 200 ms per case, repeated three times. Go may take longer to collect enough iterations, and fixture setup adds untimed work.

The full evidence command requires both supported shells. A missing shell fails rather than silently producing a partial baseline. Unsupported platform cases may explicitly skip shell PTYs. Run the evidence command on Linux, macOS or WSL with native supported shells. Local builds of Zsh may supply runtime measurements; they do not replace the separate trusted system-shell release gates.

## What each benchmark measures

| Benchmark | Timed work |
| --- | --- |
| `CatalogDecode` / `CatalogResolve` | Catalog codec/validation or resolution against an already prepared portable catalog. |
| `ShellParse` | Bash/Zsh alias definitions, function files and structural shadow import. |
| `EntrySearch` / `EntrySuggestions` | Current application search or ranking over prepared entries with metadata, favorites and usage. |
| `EntryLoad` | Application entry reads, parsing, synthetic history counts, health checks, and installed-catalog verification where selected. |
| `PickerInitialView` | Current model's first-view ranking and rendering at 60 or 140 columns, with an explicit true-color renderer and synthetic entries. |
| `ShellStartup` | A fresh interactive shell through a PTY, prompt readiness, definition checks and process teardown, for login and nonlogin routes. |

Picker view cases exclude settings/theme/context file reads, entry loading, usage refresh, watcher startup and Bubble Tea terminal initialization/painting. Native and installed-catalog entry reads are measured separately. Installed entry-loading fixtures use the real generation builder and reader; startup fixtures use the application setup and activation flows.

The shell baseline has no Alias Lens integration or entries. Every Zsh case uses the same real `compinit -i` initialization and excludes duplicate global completion initialization with a fixture-only flag. This is a controlled interactive startup baseline, not `zsh -f`. Untimed validation warms the completion dump and filesystem caches. Native cases use aliases; catalog cases use portable commands installed as functions. Compare each with its shell/route baseline rather than treating aliases and portable functions as identical payloads.

## Read the results

Go reports average nanoseconds per operation, bytes allocated per operation and allocation counts. Parser and catalog decode cases also report input throughput. `input-B` describes an in-memory input representation; `fixture-B` describes prepared entry/history/artifact bytes, not total disk traffic. Picker throughput describes rendered view bytes, not input-file scanning. Input sizes appear in the benchmark names. Shell startup includes process creation, PTY setup, loading, verification and teardown. Go's allocation counts exclude memory allocated in child processes. Shell `input-bytes` is the native alias file or declared catalog JSON size, not the complete installed generation footprint. Each timed shell iteration checks integration and the last entry, exits and drains the process, and checks an execution sentinel; full membership validation stays outside timing.

Filesystem benchmarks run against a warm OS cache. The runner does not flush caches. A fresh shell is a new process, not a cold-machine measurement. Compare repeated samples on the same machine, toolchain, power settings and workloads. Avoid unrelated CPU or disk activity during a baseline run.

Picker measurements distinguish application entry reads from in-memory first-view construction. Neither alone is the complete time until a user sees an interactive picker. There are no timing assertions or release thresholds in the normal test suite.

## Collect a profile

Use the owning package and select one benchmark when collecting a profile:

```bash
go test ./internal/app -run '^$' -bench Search -benchtime 2s -benchmem \
    -cpuprofile /tmp/al-search.cpu -memprofile /tmp/al-search.heap -o /tmp/al-app-bench.test
go tool pprof /tmp/al-app-bench.test /tmp/al-search.cpu
```

These profiles measure the Go process. Profiling shell startup requires separate child-process tooling. Keep profiles outside the repository. Use the synthetic fixtures throughout.

Acceptance criteria are in `docs/acceptance/PERFORMANCE_BASELINE.md`. Dated measurements and review resolutions belong in `docs/testing/evidence/PERFORMANCE_BASELINE.md`.
