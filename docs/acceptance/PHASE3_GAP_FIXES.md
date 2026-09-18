# Phase 3 shell gap acceptance criteria

Status: approved for implementation on 2026-09-17.

## Scope

These criteria fix two failures found during the WSL phase 3 manual check. They are intentional exceptions to the behavior at baseline commit `a5d6168`.

The change does not alter alias syntax, provider behavior, synchronization, or the terminal interface.

## Picker execution preserves command status

### Initial state

Bash and Zsh PTYs use isolated homes. Each alias file defines a successful command, a command that returns a nonzero status, and a command that changes the current directory.

### Operation

Load the matching output from `alias-lens shell-init`. Press `Ctrl+G` on an empty prompt and select each alias. Repeat after entering text at the prompt.

### Expected state

- Readline or ZLE accepts the selected alias name as a normal command line.
- The terminal prints `$ NAME` once before command output.
- `echo $?` prints the selected command's status.
- A directory-changing alias changes the current shell process.
- Native shell history contains the selected alias once.
- Private usage history contains one event for the selection.
- A definition-load failure executes no old definition with the same name.
- On a nonempty prompt, `Ctrl+G` clears the line and executes no alias.
- `ALIAS_LENS_NOBIND=1` preserves an existing `Ctrl+G` binding.

### Automated evidence

`TestBashPTYExecution` and `TestZshPTYExecution` cover nonzero status, current-directory changes, history, cancellation, and definition-load failure. Existing integration tests cover binding opt-out.

## Linux and WSL Bash login shells load the integration

### Initial state

Setup uses an isolated home on Linux or WSL. Test homes cover each existing login file, a login file that already loads `.bashrc`, and a home with no login file. The Alias Lens executable is in a user-owned directory.

### Operation

Run `al setup bash` twice. Start both an interactive non-login shell and an interactive login shell. Run `al setup --remove bash` twice.

### Expected state

- Setup keeps `.bashrc` as the non-login integration file.
- Setup uses the first existing login file in this order: `.bash_profile`, `.bash_login`, `.profile`.
- Setup creates `.bash_profile` only when no login file exists.
- Setup never creates a higher-priority login file that shadows an existing file.
- Setup adds the generated login loader only when the selected login file does not already load `.bashrc` or `.bash_aliases`.
- Both shell modes resolve `alias-lens` and the `al` function.
- Repeated setup leaves one generated block and preserves unrelated content and file modes.
- Removal deletes only generated Alias Lens blocks. It keeps user aliases, user startup commands, configuration, revisions, and repositories.
- `al doctor` reports a broken login loader and names `al setup bash` as the repair command.

### WSL evidence

From PowerShell, start the configured distribution after `wsl --shutdown`. Run a login shell against the disposable home and record `command -v alias-lens`, `type al`, `al doctor`, and the exit status.

## Alias backups remain private

### Initial state

An alias file has a permissive mode. A stale `.alias-lens.bak` file may also have a permissive mode.

### Operation

Change the alias file through `writeAliasFile`.

### Expected state

- The backup contains the complete pre-change alias file.
- The backup mode is `0600` for both a new backup and an existing backup.
- The active alias file keeps the mode requested by the caller.

### Automated evidence

`TestAddAliasPlacesRelatedCommandsTogetherAndCreatesPrivateBackup` checks backup content and mode.

## Baseline comparison

The current binary may differ from `a5d6168` only in these results:

- Linux and WSL Bash setup, repair, removal, startup status, and created login files.
- Bash 4+ and Zsh `Ctrl+G` execution internals, Bash 3.2 fallback behavior, and the corrected command status.
- Alias backup modes when a missing or existing backup is more permissive than `0600`.

All other phase 3 baseline output, files, modes, exit statuses, shell behavior, and command results must match.
