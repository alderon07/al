# Catalog shell startup performance

Acceptance criteria written before the runtime handoff change on 2026-10-08.

- PS-001: Establish the Zsh bottleneck with separate parsing and execution measurements of synthetic generated runtime handoffs. Bound the generated wrapper's executable statement count per parsing batch without changing verified generation loading.
- PS-002: Reduce the median 10,000-entry Zsh catalog startup time by at least 70% against the recorded Linux baseline. Repeat genuine interactive PTY startup at 100, 1,000 and 10,000 entries for Bash and Zsh, login and nonlogin, checking every installed name and the no-execution sentinel.
- PS-003: Preserve successful definition before removal of its existing alias, readonly preflight before any entry replacement, caller shell options, exact fresh-shell definitions, argument/status/signal behavior and Bash 3.2 syntax. Batch boundaries must support mixed aliases and functions and quoted multiline bodies without executing a candidate during verification.
- PS-004: Keep Bash generation unchanged. Keep shell dependencies behind the existing adapter and protect newly parsed batches against alias expansion of control commands. Use no private homes, credentials or shell content in measurements or fixtures.
- PS-005: Run focused shell and installed-loader regressions and the repository gate. Record comparable before/after samples and platform limitations; native macOS and WSL performance remain unmeasured.

- PS-006: The untimed complete membership gate checks every expected synthetic entry in the required alias/function table and rejects extra entries. Genuine Bash and Zsh negative PTYs must reject missing first, middle or last entries, including a missing middle entry replaced by an unexpected name with unchanged count and last entry present, and reject an entry present only with the wrong declaration kind. A valid complete fixture must pass. These assertions stay outside the timed phase. Evidence maps to `TestStartupBenchmarkRejectsMissingDeclarationsPTY`, `TestStartupBenchmarkCompleteMembershipPTY` and the repeated 36-row `BenchmarkShellStartup` catalog run.
