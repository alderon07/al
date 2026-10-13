# Linux Mint release verification

- Record the Linux Mint version, kernel, architecture, Go toolchain, shell versions, base commit, and pre-existing worktree changes.
- Run uncached tests across all packages, including the Bash and Zsh PTYs, integration tests, filesystem safety, provider transport, and Git isolation tests.
- Require both supported shells for release evidence. Missing shells must fail the strict evidence gate rather than count as passing skipped tests.
- Run `make fmt check`, race tests, module verification, module tidiness, dependency update review, vulnerability checks, workflow validation, and the pinned release snapshot checks.
- Build release artifacts and run CI vulnerability scans with a supported, patched Go toolchain. Keep the minimum source compatibility version separate and verify it explicitly.
- Remove the reported `golang.org/x/text` and `golang.org/x/sys` dependency findings with compatible fixed versions. If those fixes require a newer minimum Go version, update the installation instructions and verify that minimum.
- Exercise the compiled binary at 48x18, 80x24, and 160x40 in a PTY, including search, keyboard help, page navigation, prompt handoff, execution status, and monochrome output. Repeat narrow and wide checks in tmux.
- Use private temporary homes, synthetic aliases, local test repositories, and synthetic credentials throughout verification.
- Fix reproduced release failures within their existing package boundaries and verify each fix before rerunning the affected release checks.
- A settled terminal view repaints after `ClearScreen`, including when its content is unchanged. A focused PTY regression must fail before the renderer fix and pass afterward.
- PTY search typing waits for each character's cursor update. Wide-browser checks inspect the complete cleared frame after asynchronous repaint, including its footer.
- The optional browser copies a command exactly once per copy-button click, card click, or Enter key. Search, selection, and reload do not execute aliases.
- Record passing checks, failures, skipped checks, and remaining native platform or publication gates in dated release evidence. Linux Mint results cannot close macOS, WSL restart, or native ARM64 gates.
- Preserve unrelated staged work. Commit and push the Linux feature branch when the release owner requests it. Leave tags and publication to the release owner.
