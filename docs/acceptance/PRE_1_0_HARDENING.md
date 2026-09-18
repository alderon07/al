# Pre-1.0 hardening acceptance criteria

These criteria cover the remaining safety, privacy, terminal, release, and workflow work before 1.0. Automated checks must pass before a box is checked. Hardware and operating-system checks need dated evidence under `docs/testing/evidence/`.

## Data safety and sync

- Alias and tracked-file replacements create a private backup before the live path changes.
- Writes use a temporary file in the destination directory, sync file contents, rename atomically, and sync the directory.
- A failed write leaves the old file or the complete new file at the live path.
- Git commands preserve unrelated staged files and commit only the configured path.
- Pull, push, clone, provider, and syntax-check processes have deadlines and accept cancellation.
- The automatic worker handles termination signals, records a stopped state, and removes its lock.
- Conflict recovery never changes the live alias file and tells the user to run `al diff`.
- Automatic sync never activates remote alias-file bytes. A remote-only alias change leaves the live file untouched and tells the user to review it with `al diff` and apply safe alias additions with `al sync --pull`.
- Alias Lens refuses to track its own `config.json`, and it validates every persisted tracked-file entry before the background worker uses it.
- Repository paths reject absolute paths and traversal. Supported Unix writers resolve every component relative to open directory descriptors with no-follow semantics, so a symlink inserted after validation cannot redirect the write.
- Credential-oriented filenames, including `.netrc`, `.npmrc`, and files ending in `.env`, cannot enter the tracked-file registry. Secret scanning recognizes their common assignment formats before any push.
- Automatic sync never installs remote tracked-file bytes into a live file. It saves private conflict copies for explicit review, including when the local file is missing.
- Alias contents are scanned before every local synchronization commit, and every outgoing alias revision is scanned before a push so removing a secret in a later commit cannot expose the earlier revision.

## Optional web interface

- Each `al --web` process creates a cryptographically random session token and prints a loopback URL containing it.
- The alias API accepts only authenticated `GET` requests from the expected loopback Host and Origin.
- Static web assets remain available on the loopback listener, but they cannot read alias data without the session token.
- API responses disable caching and browser content sniffing. Invalid requests do not load or disclose aliases.

## Privacy commands

- `al data paths` lists every Alias Lens config, data, state, backup, revision, conflict, repository, and shell-integration path without creating missing paths.
- `al data clear-usage` removes only the local usage log and succeeds when it is already absent.
- `al data clear-revisions` removes only private alias revisions and succeeds when they are already absent.
- Help and privacy documentation state the contents, expected permissions, and removal method for every local file.

## Terminal behavior

- `NO_COLOR` removes ANSI color styling without changing content or keyboard behavior.
- `TERM=dumb` prints a plain, non-interactive alias list and does not enter the alternate screen.
- Terminals smaller than 48 columns or 18 rows show a short size requirement instead of a clipped interface.
- Every action remains keyboard accessible.
- Truncation, wrapping, and padding use terminal cell width for wide and combining Unicode characters.
- Built-in themes meet the documented text and control contrast thresholds, or an automated test identifies the failing preset.

## Selection and import

- Enter keeps its current execute or select behavior.
- Tab returns the selected alias to an integration-launched prompt without executing it, so the user can add arguments or edit it.
- `al import FILE` reports malformed syntax, duplicate names, duplicate commands, conflicts, and aliases it would add without writing.
- `al import FILE --apply` refuses any syntax error or name conflict and writes all accepted aliases in one backed-up atomic replacement.
- Import accepts only regular files no larger than the documented input limit and never blocks on FIFOs, devices, or sockets.
- Import previews render command text as inert terminal text. C0, DEL, CSI, OSC, and other control bytes are escaped before output.

## Parser and rendering resource limits

- Shadow parsing enforces a line-count limit in addition to its byte and definition limits.
- Function boundary detection and secret-to-result mapping complete in work proportional to the accepted input size.
- Terminal truncation scans each input once, handles zero-width Unicode text, and caps work before rendering untrusted descriptions or commands.

## Release security

- The repository publishes a private vulnerability-reporting route in `SECURITY.md`.
- CI runs `govulncheck` and dependency updates are automated.
- Every release includes archive SBOMs, SHA-256 checksums, and GitHub build-provenance attestations.
- The release checklist verifies archive contents, checksums, attestations, and executable startup on Linux and macOS.

## Real terminal matrix

- The same release candidate passes clean install and upgrade checks on Ubuntu Bash, WSL Ubuntu after shutdown/restart, macOS Zsh, and macOS Bash.
- Login and non-login shells pass, plus narrow, medium, wide, and tmux sessions.
- Evidence records the exact release candidate, operating-system version, shell version, terminal dimensions, and result.
