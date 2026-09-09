# Alias Lens product roadmap

This roadmap tracks user problems. A checkbox closes only after the feature has tests and user documentation.

## Use an alias without retyping it

- [x] Add an alias picker that returns the selected command.
- [x] Add Bash integration that runs the selected command in the current shell.
- [x] Add a Readline binding that inserts the selected alias at the cursor.

## Suggest aliases from real usage

- [x] Count commands in local Bash history without sending history anywhere.
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

## Recover old aliases

- [x] Save timestamped revisions in addition to the latest backup.
- [x] List revisions from the terminal.
- [x] Restore the latest or a selected revision.

## Support commands that aliases cannot express

- [x] Discover Bash functions alongside aliases.
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
