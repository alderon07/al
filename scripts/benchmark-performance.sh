#!/usr/bin/env bash
set -euo pipefail
umask 077

repository_root=$(cd "$(dirname "$0")/.." && pwd)
cd "$repository_root"
benchmark_go=${GO:-go}
benchmark_time=${BENCHTIME:-200ms}
benchmark_count=${BENCH_COUNT:-3}
benchmark_pattern=${BENCH_PATTERN:-.}
if [[ ! "$benchmark_count" =~ ^[1-9][0-9]*$ ]]; then
    printf 'BENCH_COUNT must be a positive integer.\n' >&2
    exit 1
fi
for program in "$benchmark_go" git bash zsh; do
    command -v "$program" >/dev/null || {
        printf 'Performance verification requires %s.\n' "$program" >&2
        exit 1
    }
done
benchmark_root=$(mktemp -d /tmp/al-bench.XXXXXX)
benchmark_root=$(cd "$benchmark_root" && pwd -P)
trap 'rm -rf "$benchmark_root"' EXIT
benchmark_output=${BENCH_OUTPUT:-$(mktemp /tmp/al-performance.XXXXXX)}
if [[ -n "${BENCH_OUTPUT:-}" ]]; then
    set -C
    exec 3> "$benchmark_output"
    set +C
else
    exec 3> "$benchmark_output"
fi
{
    printf 'Alias Lens performance baseline\n'
    printf 'UTC: %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    printf 'Commit: %s\n' "$(git rev-parse HEAD)"
    if [[ -z "$(git status --porcelain)" ]]; then
        printf 'Worktree: clean\n'
    else
        printf 'Worktree: dirty\n'
    fi
    "$benchmark_go" version
    printf 'Platform: %s/%s\n' "$("$benchmark_go" env GOOS)" "$("$benchmark_go" env GOARCH)"
    printf 'Kernel: %s %s\n' "$(uname -s)" "$(uname -r)"
    if [[ -r /proc/cpuinfo ]]; then
        awk -F ': ' '/^model name/ {print "CPU: " $2; exit}' /proc/cpuinfo
    elif command -v sysctl >/dev/null; then
        printf 'CPU: %s\n' "$(sysctl -n machdep.cpu.brand_string)"
    fi
    bash --version | sed -n '1p'
    zsh --version
    printf 'Benchtime: %s; repetitions: %s\n' "$benchmark_time" "$benchmark_count"
    printf 'Fresh shell processes; filesystem cache is not flushed. Go allocations exclude child processes.\n'
    TMPDIR="$benchmark_root" AL_REQUIRE_PTY_SHELLS=1 "$benchmark_go" test \
        ./internal/catalog ./internal/shell ./internal/app ./internal/tui ./cmd/alias-lens \
        -run '^$' -bench "$benchmark_pattern" -benchmem -benchtime "$benchmark_time" \
        -count "$benchmark_count" -timeout 30m
} 2>&1 | tee /dev/fd/3
if ! awk '/^Benchmark[^[:space:]]+[[:space:]]+[0-9]+[[:space:]]/ {found=1} END {exit !found}' "$benchmark_output"; then
    printf 'No benchmark measurements were collected; check BENCH_PATTERN.\n' >&2
    exit 1
fi
printf 'Saved performance report: %s\n' "$benchmark_output"
