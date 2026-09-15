# Alias Lens product roadmap

This roadmap tracks user problems. A checkbox closes only after the feature has tests and user documentation.

## Use an alias without retyping it

- [x] Add an alias picker that returns the selected command.
- [x] Add Bash and Zsh integrations that run the selected command in the current shell.
- [x] Add prompt-aware Bash and Zsh bindings that launch the alias picker.

## Suggest aliases from real usage

- [x] Count commands in local Bash or Zsh history without sending history anywhere.
- [x] Suggest repeated long commands that do not already have aliases.
- [x] Use local command frequency to improve the default alias ranking.

## Stop unsafe pushes

- [x] Detect common credentials and private keys in alias commands.
- [x] Block `al sync --push` when the alias file contains a likely secret.
- [x] Let the user inspect findings without printing the secret value.

## Synchronize more than one machine

- [x] Show the difference between the local alias file and the tracked copy.
- [x] Pull fast-forward repository updates without overwriting local aliases.
- [x] Report alias-level conflicts with both command values.
- [x] Reconcile local and remote changes in a locked background worker.
- [x] Retry offline operations without overwriting either file.
- [x] Save private conflict copies outside the live alias file.
- [x] Auto-sync explicitly enrolled configuration files without staging unrelated files.
- [x] Reject credential-shaped files before enrollment.

## Start without an alias file

- [x] Restore the active shell alias file from the configured repository when available.
- [x] Create a private empty shell alias file when no copy exists.
- [x] Install portable shell integration without a machine-specific path.

## Recover old aliases

- [x] Save timestamped revisions in addition to the latest backup.
- [x] List revisions from the terminal.
- [x] Restore the latest or a selected revision.

## Support commands that aliases cannot express

- [x] Discover common Bash and Zsh functions alongside aliases.
- [x] Label functions in search results and explain their commands.

## Handle different machines and contexts

- [x] Parse tags, collections, platforms, and favorites from structured comments.
- [x] Warn when an entry does not support the current platform.
- [x] Include metadata in search and default ranking.

## Diagnose setup problems

- [x] Add `al doctor` checks for installation, shell loading, Git, repository access, providers, and SSH.
- [x] Add a first-run setup command that installs the shell integration safely.

## Keep documentation accurate

- [x] Document every new command and metadata field.
- [x] Run tests, static checks, a secret scan, and a live terminal check.

## Make the terminal interface easier to learn

- [x] Add a searchable keyboard guide opened with `?`.
- [x] Confirm risky aliases before execution and explain why they were flagged.
- [x] Make alias descriptions editable from the TUI.
- [x] Browse and restore private revisions with `Ctrl+Z`.
- [x] Distinguish missing, broken, and risky aliases in the wide header.
- [x] Show a one-time keyboard tour after setup.

## Finish the alias workflow

- [x] Explain that the category badge is inferred from the command by default.
- [x] Make the description, category, and tags editable in the add and edit form.
- [x] Preserve platform and favorite metadata when the TUI edits an alias.
- [x] Add `al search [--json] [QUERY]` for non-interactive search.
- [x] Record picker launches without storing the command text.
- [x] Add `al stats` views for all time, today, the last 7 days, and the last 12 months.
- [x] Count aliases typed directly when the active shell history contains them.
- [x] Add a themed interactive stats dashboard with plain output for scripts.
- [x] Export aliases and stats as JSON, YAML, or CSV.
- [x] Show the expanded command in the terminal after a picker launch.
- [x] Make `Ctrl+G` launch the picker from an empty Bash or Zsh prompt.
- [x] Preserve `Ctrl+G` cancel behavior when the prompt contains text.
- [x] Let users disable the binding with `ALIAS_LENS_NOBIND=1` and bind `_alias_lens_launch` themselves.
- [x] Move the executable package and embedded browser assets under `cmd/alias-lens/`.
- [x] Add `al check` for definitions, metadata, duplicate names, native shell syntax, and warnings.
- [x] Run the syntax checker as part of `al doctor`.
- [x] Extract export encoding and private file writes into an internal package.

## Reduce the next alias pain points

- [ ] Let `Tab` return the selected alias to the prompt for editing instead of running it.
- [ ] Add directory and Git-workspace scopes for suggestions and search ranking.
- [ ] Add optional completion hooks for success rate, duration, and last-used stats.
- [ ] Add configurable history exclusions before collecting more execution context.
- [ ] Add an import preview that finds duplicate names and commands before changing the alias file.
- [ ] Benchmark startup, large alias files, and search before considering a language rewrite.
- [ ] Design end-to-end encryption before adding any hosted sync service.
