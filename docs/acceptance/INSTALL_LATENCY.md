# Source installation latency

## Acceptance criteria

- `make install` builds Alias Lens, copies it to `BINDIR`, and verifies that the installed binary matches `OUTPUT` without running the Go test suite or PTY tests.
- `make check` continues to run formatting checks, the full Go test suite including PTY tests, static analysis, the build, and whitespace checks.
- Installation preserves `GO`, `OUTPUT`, `PREFIX`, and `BINDIR` overrides, executable mode `0755`, and the notice for an already running Alias Lens process.
- Make help and the README describe installation accurately and tell developers to run `make check` before `make install` after code changes.
- The README explains that the first source build can still take time to download dependencies and compile them, and points to release archives for installation without a Go build.
- Verify the dependency change with a dry run, install into a temporary directory, compare the binary and its mode, and run `make fmt check` and a PTY smoke test before reporting success.
