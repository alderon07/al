# Source installation latency

The user reported about two minutes for `make install`. Its previous dependency list was `install: test build`, so source installation included the full Go and PTY test suite. The correction changes that dependency to `build` and retains copying, mode `0755`, byte comparison and the notice for an already running Alias Lens process. `make check` retains the full test suite and other verification. Developer instructions use `make check && make install` so failed checks prevent installation.

Acceptance in `docs/acceptance/INSTALL_LATENCY.md` preceded implementation. Separate high review found no issues. Dry runs verified the dependency lists and `GO`, `OUTPUT`, `PREFIX` and `BINDIR` overrides. Temporary functional installs covered paths with spaces, byte equality and mode `0755`. A forced build failure stopped before creating the destination or copying the binary.

## Measured result

Both measurements used the same source tree, existing Go dependency/build caches, an offline proxy, and the same disposable home. The before run used a saved copy of the original Makefile. Outputs and install destinations were separate temporary paths. No real shell files or configured repository were used.

| Operation | Seconds | Exit |
| --- | ---: | ---: |
| Previous `make install`, including tests | 67.465 | 0 |
| Updated `make install`, build/copy/verify | 1.051 | 0 |

These are single warmed-cache observations on Linux. They identify the removed test-suite delay, not a cold-install guarantee. A first source build still downloads dependencies and compiles them; release archives avoid that build. No dependency or runtime code changed.

The before, after and full-check binaries had identical SHA-256 `8de6d817dca1259e1a3898030102c4ce7642b078f844a9cd70e0ad7db7f7b248`. The actual updated install copied matching bytes with mode `0755`.

## Verification

- `make fmt check` passed after the change. The full Go/PTY suite also passed during the timed original installation; repeated Go results in the later gate reused its test cache.
- Actual make-installed binary PTYs passed Bash and Zsh login and nonlogin startup. Each invoked only a synthetic alias and `al --version`, with expected output and zero exit status.
- The installed binary passed setup, repair and removal for both shells in private synthetic homes. Removal preserved the synthetic alias and its mode `0600`.
- Whitespace checks passed. Runtime code and release tag `v1.0.0-rc.4` remain unchanged; this correction is a source-installation workflow change.

Host: Linux x86-64, Go `1.27.1`, Bash `5.2.21`, disposable Zsh `5.9`. Native macOS/WSL manual release gates remain open. Timing logs, complete check output and synthetic PTY results were retained privately under the task's temporary directory.
