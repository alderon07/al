# Check a release on WSL 2

Run this checklist on the exact release commit in WSL 2. Use temporary home directories and synthetic aliases. Do not use a real alias file, provider token, or dotfiles repository.

Record the commit, `wsl.exe --version`, Windows version, distribution, kernel, architecture, Go version, shell versions, Windows Terminal version, dimensions, and every failed command in a dated file under `docs/testing/evidence/`.

## Record and check the environment

- [ ] Confirm that `git status --short` is empty and record `git rev-parse HEAD`.
- [ ] Record `wsl.exe --version`, `uname -a`, the distribution release, `go version`, `bash --version | head -1`, and `zsh --version`.
- [ ] Run `make fmt check`.
- [ ] Run `govulncheck ./...`.
- [ ] Run `./scripts/verify-shadow-wsl.sh` and record `PASS shadow-wsl` with its sanitized hashes.

## Check setup across a WSL restart

Run Bash and Zsh in separate temporary homes.

- [ ] Run clean `setup`, `doctor`, `setup --repair`, and `setup --remove` flows for Bash.
- [ ] Run clean `setup`, `doctor`, `setup --repair`, and `setup --remove` flows for Zsh.
- [ ] Confirm that setup changes only the selected shell's startup and alias files.
- [ ] Confirm that `al shortcuts` selects the Windows profile by default.
- [ ] Save another shortcut profile, confirm that it overrides detection, then run `al shortcuts auto`.
- [ ] Install Bash and Zsh completion, start new shells, and confirm that command, flag, profile, and entry suggestions work without exposing command bodies.
- [ ] Close every WSL window, run `wsl.exe --shutdown` from PowerShell, and reopen the distribution with the same test home.
- [ ] Confirm that `command -v alias-lens`, `type al`, completion, and `al doctor` still work.
- [ ] Run `./scripts/verify-shadow-wsl.sh` again. Confirm that its result matches the pre-shutdown run.

## Check Windows Terminal behavior

Run the compiled binary at 48x18, 80x24, and 160x40.

- [ ] Confirm that the selected alias, search field, footer, and maker line remain visible.
- [ ] Confirm that every displayed Windows shortcut and its function-key fallback works.
- [ ] Confirm that `Enter` runs the selected alias and preserves its exit status.
- [ ] Confirm that `Tab` returns the alias to the prompt without running it and allows arguments to be appended.
- [ ] Confirm that `Ctrl+G` opens Alias Lens at an empty Bash 4+ or Zsh prompt and keeps normal cancel behavior when the prompt contains text.
- [ ] Confirm that copy and paste remain terminal operations and pasted text never invokes a shortcut.
- [ ] Check `NO_COLOR=1`, non-ASCII descriptions, long commands, and resize behavior.
- [ ] Repeat the narrow and wide checks inside tmux when tmux is part of the supported WSL setup.

## Check Linux archive use from WSL

- [ ] Verify `checksums.txt` for the matching Linux archive.
- [ ] Extract the archive and run `alias-lens --version`.
- [ ] Run setup, doctor, repair, and removal from the archive in a temporary home.
- [ ] Verify the published archive provenance with `gh attestation verify ARCHIVE --repo alderon07/al`.
