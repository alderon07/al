# Working on Alias Lens

## Product rules

Alias Lens is a terminal-first Bash alias manager. Keep the Bubble Tea interface as the default experience. The browser endpoint must remain optional.

The command name is `al`. The installed binary is `alias-lens`, which lets the Bash integration define an `al` function without recursion.

## Safety rules

- Treat `~/.bash_aliases`, shell history, provider tokens, backups, and local config as private user data.
- Never add private user data to this repository, test fixtures, logs, screenshots, or error messages.
- Never execute an alias while parsing, searching, explaining, checking, or syncing it.
- Keep provider tokens out of `config.json`, command arguments, clone URLs, and Git remotes.
- Send API credentials only to the configured HTTPS host. Reject cross-host redirects.
- Run the secret scan before every remote push of the alias file.
- Preserve unrelated files and staged changes in a configured dotfiles repository. Alias Lens may commit only `alias_file`.
- Create a backup and a timestamped revision before changing the user's alias file.
- Pull with `--ff-only`. Do not overwrite a local alias when the remote repository defines a different command under the same name.

## Code map

- `main.go` parses aliases and routes CLI commands.
- `tui.go` contains the Bubble Tea alias browser and picker.
- `alias_writer.go` performs validated, atomic alias-file edits.
- `metadata.go` handles Bash functions, tags, collections, favorites, and platforms.
- `history.go` analyzes local Bash history and creates suggestions.
- `security.go` detects likely secrets without returning their values.
- `revisions.go` stores and restores private alias revisions.
- `repository.go` handles alias-only Git synchronization and comparison.
- `providers.go` implements GitHub, Bitbucket Cloud, and GitLab repository discovery.
- `github_picker.go` contains the provider-neutral remote repository picker. The filename remains for Git history.
- `config.go` owns app and provider configuration.
- `theme.go` contains verified theme palettes and overrides.
- `web/` contains the optional local browser view.
- `TODO.md` tracks user-facing work and acceptance criteria.

## Implementation style

- Use the Go standard library unless a dependency materially improves the terminal interface.
- Keep provider behavior behind `RepoProvider`. Do not add provider checks to the picker.
- Keep shell-specific behavior behind explicit commands such as `shell-init` and `setup`.
- Use plain terminal text for non-interactive commands. Reserve Lip Gloss styling for interactive screens.
- Keep error messages actionable. Name the command that fixes the problem.
- Add tests for parsers, filesystem writes, Git path isolation, credential handling, and migrations.
- Use temporary directories in tests. Tests must never read or modify the real `.bash_aliases` or configured dotfiles repository.

## Verify a change

Run these commands from the repository root:

```bash
gofmt -w *.go
go test ./...
go vet ./...
go build -buildvcs=false -o /tmp/alias-lens-release .
git diff --check
```

For terminal layout changes, run the compiled binary in a real terminal at narrow and wide widths. For provider changes, test missing credentials, invalid credentials, pagination, write-access filtering, and SSH fallback. Do not use a production token in a fixture.

Before a commit, inspect the staged file list and scan it for tokens, private keys, `.bash_aliases`, environment files, and local config.
