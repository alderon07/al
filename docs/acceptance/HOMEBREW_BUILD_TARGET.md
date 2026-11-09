# Homebrew build target

Acceptance criteria, written before implementation:

- The Homebrew formula builds `./cmd/alias-lens` from the repository root.
- The formula retains Homebrew's build flags, the `alias-lens` binary name, and the release version linker flag.
- The formula's Bash initialization test matches the emitted `Alias Lens Bash integration` header. The disposable-prefix build exposed the existing stale `Alias Lens shell integration` assertion before this correction.
- Verify Ruby syntax and execute the formula's build invocation against this checkout in a disposable prefix. Verify the installed binary with the formula's version and shell-init assertions.
- Run `make fmt check` with Bash and Zsh PTY coverage required.
- Record the supplied macOS validation for `03b1279` separately from local packaging verification. Published-release attestation, tap installation and upgrade, Terminal.app, Intel execution, and manual Linux and WSL validation remain open.

Local verification:

- Homebrew Ruby syntax validation passed for the rendered formula.
- Homebrew's `Formulary.from_contents` loaded a temporary formula with synthetic version `1.0.0-rc.99`. Its real `install` method built this checkout from the repository root into a disposable prefix using Homebrew's `std_go_args`.
- The formula's test block passed against that binary. The harness supplied `shell_output` and `assert_match` methods without invoking the tap test runner.
- `AL_REQUIRE_PTY_SHELLS=1 make fmt check` passed, including Bash and Zsh PTYs. The subsequent formula assertion correction passed the disposable-prefix verification again.
- A separate review found no blocking correctness issues. Full tap install, `brew test`, audit, and upgrade remain release-time checks.
