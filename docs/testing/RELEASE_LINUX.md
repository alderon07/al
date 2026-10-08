# Check a release on Linux

Run this checklist on the exact release commit. Use temporary home directories and synthetic aliases. Do not use a real alias file, provider token, or dotfiles repository.

Record the commit, distribution, kernel, architecture, Go version, Bash version, Zsh version, terminal, tmux version when used, and every failed command in a dated file under `docs/testing/evidence/`.

## Run automated checks

- [ ] Confirm that `git status --short` is empty and record `git rev-parse HEAD`.
- [ ] Run `make fmt check`.
- [ ] Run `go test -race -count=1 -skip PTY ./...`. `make check` runs the PTY tests without race instrumentation.
- [ ] Run `go mod verify` and `go mod tidy -diff`.
- [ ] Run `go list -m -u -mod=readonly all` and review available direct and transitive updates.
- [ ] Run `govulncheck ./...`. Review imported-package and module findings even when no called symbol is vulnerable.
- [ ] Run `actionlint .github/workflows/*.yml`.
- [ ] Run `goreleaser check` with the GoReleaser version pinned in `.github/workflows/release.yml`.
- [ ] Install the pinned Syft version, then run `goreleaser release --snapshot --clean --skip=publish`.
- [ ] Confirm that `dist/` contains four archives, four SPDX JSON SBOMs, and `checksums.txt`.
- [ ] Run `(cd dist && sha256sum -c checksums.txt)`.
- [ ] Inspect each archive. Confirm that it contains `alias-lens`, `LICENSE`, `NOTICE`, `README.md`, `THIRD_PARTY_NOTICES.md`, and `THIRD_PARTY_LICENSES.txt`.

## Check Bash and Zsh

Run each shell check in a new temporary home.

- [ ] Test Bash in login and non-login sessions.
- [ ] Test Zsh in login and non-login sessions.
- [ ] Run clean `setup`, `doctor`, `setup --repair`, and `setup --remove` flows for each shell.
- [ ] Confirm that setup changes only the selected shell's startup and alias files.
- [ ] Confirm that removal leaves aliases, config, revisions, and repositories in place.
- [ ] Confirm that a missing alias file is created with mode `0600`.
- [ ] Confirm that setup preserves the startup-file precedence already present in the temporary home.
- [ ] Start a new shell and confirm that `alias-lens`, `al`, and installed completion remain available.

## Check terminal behavior

Run the compiled binary in a real terminal at 48x18, 80x24, and 160x40.

- [ ] Confirm that the selected alias, search field, footer, and maker line remain visible.
- [ ] Confirm that the wide detail pane appears only when it fits and never clips the footer.
- [ ] Confirm that `NO_COLOR=1 al` remains readable and does not depend on color.
- [ ] Confirm that `TERM=dumb al search QUERY` prints plain text.
- [ ] Confirm that `/` accepts search text and `?` opens the keyboard guide.
- [ ] Confirm that every help-listed page shortcut opens the expected page and returns to aliases when pressed again.
- [ ] Confirm that `Enter` runs the selected alias and preserves its exit status.
- [ ] Confirm that `Tab` returns the alias to the prompt without running it and allows arguments to be appended.
- [ ] Confirm that `Ctrl+G` opens Alias Lens at an empty Bash 4+ or Zsh prompt and keeps normal cancel behavior when the prompt contains text.
- [ ] Repeat the narrow and wide checks inside tmux.

## Check safety and recovery

- [ ] Run `al check`, `al scan`, `al status`, and `al doctor` against synthetic data. Confirm that none executes an alias.
- [ ] Test an import preview with duplicate names, duplicate commands, invalid syntax, and a safe entry. Apply only after reviewing the preview.
- [ ] Test a local-only sync change, a repository-only change, a changed definition, and non-conflicting additions on both sides.
- [ ] Confirm that automatic sync stages only the alias file and explicitly tracked files.
- [ ] Confirm that environment files, keys, and credential-shaped filenames cannot be tracked.
- [ ] Confirm that a likely secret blocks a push without printing the secret value.
- [ ] Interrupt an alias write and a sync operation. Confirm that recovery keeps an old or complete new file, never a partial file.
- [ ] Confirm that conflict copies, revisions, backups, config, state, and cloned repositories are private.

## Check the Linux archives

- [ ] Extract the `linux_amd64` archive on an x86-64 Linux host and run `alias-lens --version`.
- [ ] Extract the `linux_arm64` archive on an ARM64 Linux host and run `alias-lens --version`.
- [ ] Run setup and removal from each native archive in a temporary home.
- [ ] Confirm that the archive checksum and provenance match the published release before announcing it.

## Check catalog installation and bootstrap

Use disposable homes and repositories for every case. Required Bash and Zsh PTY shells must be present; a missing shell is a failed evidence gate.

- [ ] Run `scripts/verify-catalog-workflow.sh` and record its sanitized result.
- [ ] Review native code and existing-name ownership in `al init`; cancel at each prompt and confirm that no decisions or files were saved.
- [ ] Enable adopted entries, start new login and nonlogin shells, and verify retained fallbacks when the helper, pointer, or generation is unavailable.
- [ ] Change the native file and confirm that startup declines the overlay with renewed-review guidance.
- [ ] Verify masking aliases, readonly functions, argument forwarding, exit status, signals, and preserved shell options.
- [ ] Edit, rename, delete, and exclude catalog entries; verify pending labels and exact installed completion membership before and after enablement.
- [ ] Roll back offline and confirm that the original enrollment baseline returns, including later renamed or deleted entries.
- [ ] Pull independent catalog changes without activation; verify semantic conflicts and preservation of unrelated staged and worktree files.
- [ ] Exercise restricted remote bootstrap with synthetic credentials and malicious hooks, filters, submodules, unsupported filtering, limits, and cancellation.
- [ ] Crash during forward application and recovery, then recover twice and verify dependency order and private artifacts.
