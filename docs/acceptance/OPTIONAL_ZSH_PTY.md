# Optional Zsh PTY tests

- Catalog startup PTY tests run their Bash cases when Zsh is unavailable.
- Zsh PTY subtests skip with a clear reason only when the Zsh executable is missing.
- Other executable lookup failures remain test failures.
- With `AL_REQUIRE_PTY_SHELLS=1`, a missing Zsh executable fails the catalog startup PTY subtest.
- With Zsh installed, the same Zsh PTY assertions run unchanged.
- `make fmt check` passes in an environment without Zsh, and the Bash PTY cases execute.
