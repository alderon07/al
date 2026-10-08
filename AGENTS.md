# Working on Alias Lens

## General

- Code for reliability, maintainability, and operability.
- Go Best Practices https://go.dev/doc/effective_go
- Always code with the mindset that we'll expand the current feature in one way or another. So write modular/extensible code.
- This isn't being used by anyone but me rn. So, no need for unnecessary migrations.
- Avoid scope creep.

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

- `cmd/alias-lens/main.go` routes CLI commands. Command files own argument handling, plain output, credential prompts, embedded browser assets and linker version.
- `cmd/alias-lens/terminal.go` starts the terminal interface with the same application services used by CLI commands.
- `internal/app/` owns configuration, entry queries, catalog lifecycle, bootstrap, revisions, private data, sync and mutation sessions.
- `internal/app/alias_writer.go` performs validated, atomic alias-file edits under the shared mutation session.
- `internal/app/shell.go` selects shell adapters and applies startup edits under the application mutation session.
- `internal/app/{metadata,history,security}.go` handles entry metadata, local history analysis and secret detection without exposing secret values.
- `internal/app/{repository,autosync}.go` owns path-isolated synchronization, background reconciliation, status, retries and conflict copies.
- `internal/tui/` owns private Bubble Tea models and views for the alias browser, picker, statistics, settings, catalog review and conflicts.
- `internal/tui/github_picker.go` contains the provider-neutral remote repository picker. The filename remains for Git history.
- `internal/presentation/` owns theme palettes, appearance/footer schemas, icons and shared display formatting.
- `internal/shortcuts/` owns shortcut profiles, declarations, matching and shared validation.
- `internal/shell/` owns Bash and Zsh parsing, rendering, history syntax, integration, validation, handoff and read-only startup planning.
- `internal/entry/` owns shared entry data, metadata and display defaults.
- `internal/providers/` owns GitHub, Bitbucket Cloud, and GitLab repository discovery, immutable catalog reads, and provider transport policy.
- `internal/managedgit/` owns restricted Git execution, configuration audits, credential handoff, SSH policy, output bounds, and process cancellation.
- `internal/{catalog,catalogrender,catalogstore,plan,state,transaction}/` owns catalog formats, generation rendering and storage, plans, observations and durable transactions.
- `internal/{tea,usagelog,exportfile}/` contains the terminal compatibility adapter, private usage log, and export codecs shared by application packages.
- `cmd/alias-lens/web/` contains the optional local browser view.
- `docs/PACKAGE_ORGANIZATION.md` records package boundaries, acceptance criteria and verification evidence.
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
- Always write acceptance criteria before writing any code.
- Never write comments.

## Testing
- PTY testing is mandatory, not optional.
- Always verify a change works with evidence before reporting success.

## Verify a change

Run these commands from the repository root:

```bash
make fmt check
```

For terminal layout changes, run the compiled binary in a real terminal at narrow and wide widths. For provider changes, test missing credentials, invalid credentials, pagination, write-access filtering, and SSH fallback. Do not use a production token in a fixture.

Before a commit, inspect the staged file list and scan it for tokens, private keys, shell alias files, environment files, and local config.
