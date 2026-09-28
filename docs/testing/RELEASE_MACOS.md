# Check a release on macOS

Run this checklist on the exact release commit. Use temporary home directories and synthetic aliases. Do not use a real alias file, provider token, or dotfiles repository.

Record the commit, macOS version, architecture, Go version, Apple Bash version, Zsh version, terminal, dimensions, Homebrew version, and every failed command in a dated file under `docs/testing/evidence/`.

## Run automated checks

- [ ] Confirm that `git status --short` is empty and record `git rev-parse HEAD`.
- [ ] Record `sw_vers`, `uname -a`, `go version`, `bash --version | head -1`, and `zsh --version`.
- [ ] Run `make fmt check`.
- [ ] Run `govulncheck ./...`.
- [ ] Confirm that the `test` and `shadow-macos` GitHub Actions jobs pass for this commit.

## Check Zsh and stock Bash

Run each shell check in a new temporary home.

- [ ] Run clean `setup`, `doctor`, `setup --repair`, and `setup --remove` flows for the default Zsh.
- [ ] Confirm that `Ctrl+G` opens Alias Lens at an empty Zsh prompt and keeps normal cancel behavior when the prompt contains text.
- [ ] Confirm that `Tab` returns the selected alias to the Zsh prompt without running it.
- [ ] Test stock Bash 3.2 in login and non-login sessions.
- [ ] Confirm that stock Bash keeps its normal `Ctrl+G` behavior and that entering `al` opens Alias Lens.
- [ ] Confirm that setup preserves `.bash_profile`, `.bash_login`, and `.profile` precedence. It must not create a higher-priority login file that hides an existing file.
- [ ] Confirm that setup changes only the selected shell's startup and alias files.
- [ ] Install Bash and Zsh completion, start new shells, and confirm that suggestions work without exposing command bodies.

## Check terminal behavior

Run the compiled binary at 48x18, 80x24, and 160x40 in Terminal.app. Repeat the important key checks in the terminal that you normally use.

- [ ] Confirm that the selected alias, search field, footer, and maker line remain visible.
- [ ] Confirm that `al shortcuts` selects the macOS profile by default.
- [ ] Confirm that Command shortcuts work when the terminal sends them and that every function-key fallback works.
- [ ] Confirm that copy and paste remain terminal operations and pasted text never invokes a shortcut.
- [ ] Confirm that `Enter` runs the selected alias and preserves its exit status.
- [ ] Confirm that `Tab` returns the alias to the prompt without running it and allows arguments to be appended.
- [ ] Check `NO_COLOR=1`, non-ASCII descriptions, long commands, and resize behavior.
- [ ] Repeat the narrow and wide checks inside tmux when tmux is part of the supported macOS setup.

## Check macOS archives and Homebrew

- [ ] On Apple silicon, verify and extract the `darwin_arm64` archive, then run `alias-lens --version`, setup, doctor, and removal in a temporary home.
- [ ] On Intel macOS, verify and extract the `darwin_amd64` archive, then run `alias-lens --version`, setup, doctor, and removal in a temporary home.
- [ ] Verify published archive provenance with `gh attestation verify ARCHIVE --repo alderon07/al`.
- [ ] Update the personal-tap formula with the immutable release source URL and SHA-256 checksum.
- [ ] Run `brew install --build-from-source ./Formula/alias-lens.rb`.
- [ ] Run `brew test ./Formula/alias-lens.rb` and `brew audit --strict ./Formula/alias-lens.rb`.
- [ ] Run `brew uninstall alias-lens`, install from the tap, and confirm that `brew upgrade` recognizes the release.
