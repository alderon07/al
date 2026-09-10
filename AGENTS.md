# Working on Alias Lens

## Product rules

Alias Lens is a terminal-first Bash and Zsh alias manager. Keep the Bubble Tea interface as the default experience. The browser endpoint must remain optional.

The command name is `al`. The installed binary is `alias-lens`, which lets each shell integration define an `al` function without recursion.

Bash and Zsh are supported. `al setup` detects the current shell; explicit `al setup bash` and `al setup zsh` overrides remain available. Support Bash on Linux, WSL, and macOS without assuming that their startup files are identical. Keep the alias model and terminal interface usable by future shell adapters, but do not claim compatibility with another shell until its parser, writer, startup integration, execution behavior, and key bindings are implemented and tested.

Keep shell behavior behind `ShellAdapter`. Bash owns `.bash_aliases`, `.bash_history`, and Bash startup files; Zsh owns `.zsh_aliases`, `.zsh_history`, and `.zshrc`. Do not source one shell's alias file from another shell as a shortcut. Fish and PowerShell require native syntax or a shell-neutral storage design. Never modify another shell's startup files unless setup detects or the user explicitly selects that shell.

## Safety rules

- Treat shell alias files, shell history, provider tokens, backups, and local config as private user data.
- Auto-sync only the configured shell alias file and files the user enrolled with `al track`.
- Reject environment files, keys, and credential-shaped filenames from the tracked-file registry.
- Never add private user data to this repository, test fixtures, logs, screenshots, or error messages.
- Never execute an alias while parsing, searching, explaining, checking, or syncing it.
- Keep provider tokens out of `config.json`, command arguments, clone URLs, and Git remotes.
- Send API credentials only to the configured HTTPS host. Reject cross-host redirects.
- Run the secret scan before every remote push of the alias file.
- Preserve unrelated files and staged changes in a configured dotfiles repository. Alias Lens may commit only `alias_file`.
- Create a backup and a timestamped revision before changing the user's alias file.
- Pull with `--ff-only`. Do not overwrite a local alias when the remote repository defines a different command under the same name.
- Serialize automatic sync with one lock. Keep the last successful local and remote hashes.
- If both files changed, save private conflict copies and leave the live alias file unchanged.
- Create a missing alias file with mode `0600`. Prefer the configured repository copy when one exists.

## Code map

- `main.go` parses aliases and routes CLI commands.
- `tui.go` contains the Bubble Tea alias browser and picker.
- `alias_writer.go` performs validated, atomic alias-file edits.
- `shell.go` owns shell detection and the Bash and Zsh adapters.
- `metadata.go` handles shell functions, tags, collections, favorites, and platforms.
- `history.go` analyzes local shell history and creates suggestions.
- `security.go` detects likely secrets without returning their values.
- `revisions.go` stores and restores private alias revisions.
- `repository.go` handles alias-only Git synchronization and comparison.
- `autosync.go` owns background reconciliation, locking, status, offline retries, and conflict copies.
- `providers.go` implements GitHub, Bitbucket Cloud, and GitLab repository discovery.
- `github_picker.go` contains the provider-neutral remote repository picker. The filename remains for Git history.
- `config.go` owns app and provider configuration.
- `theme.go` contains verified dark Codex palettes, Alias Lens originals, selection order, and overrides.
- `web/` contains the optional local browser view.
- `TODO.md` tracks user-facing work and acceptance criteria.

## Implementation style

- Use the Go standard library unless a dependency materially improves the terminal interface.
- Keep provider behavior behind `RepoProvider`. Do not add provider checks to the picker.
- Keep shell-specific parsing, writing, startup-file changes, execution, and key bindings behind explicit commands such as `shell-init` and `setup`.
- Preserve Bash startup precedence. Never create a higher-priority login file that prevents an existing `.bash_login` or `.profile` from loading.
- Put shared behavior behind shell-independent functions before adding another shell. A future adapter must not add shell checks throughout the TUI, sync, or provider code.
- Use plain terminal text for non-interactive commands. Reserve Lip Gloss styling for interactive screens.
- Keep error messages actionable. Name the command that fixes the problem.
- Add tests for parsers, filesystem writes, Git path isolation, credential handling, and migrations.
- Use temporary directories in tests. Tests must never read or modify real shell alias files or the configured dotfiles repository.

## Verify a change

Run these commands from the repository root:

```bash
make fmt check
```

For terminal layout changes, run the compiled binary in a real terminal at narrow and wide widths. For provider changes, test missing credentials, invalid credentials, pagination, write-access filtering, and SSH fallback. Do not use a production token in a fixture.

Before a commit, inspect the staged file list and scan it for tokens, private keys, shell alias files, environment files, and local config.
