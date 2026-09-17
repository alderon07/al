# Phase 3 baseline comparison

Date: 2026-09-17

Environment: WSL 2, Ubuntu 24.04.5 LTS

Baseline: `a5d6168ce3afc96ed6838dc00ca63a9eb0999dbe`

Candidate base commit: `1fa9a08bd5d3cd567d9087421fd747986471c1e0`, with the phase 3 working-tree changes

All commands used separate disposable homes. The comparison normalized only the binary path, disposable-home path, and generated export timestamp. Test aliases contained no private data.

## Setup matrix

The baseline and candidate binaries each ran the following operations twice for Bash and Zsh:

1. `setup SHELL`
2. `setup --repair SHELL`
3. `setup --remove SHELL`

All 24 processes exited with status 0. Standard output and standard error matched for every corresponding operation.

The final filesystem manifests differed only as follows:

- Candidate Bash and Zsh alias backups had mode `0600`; baseline backups retained the seeded permissive mode `0644`.
- Candidate Bash created a private backup for the existing login file before adding its loader. The baseline did not edit that Linux login file.

These are the approved private-backup and Linux/WSL login-loader fixes in `docs/acceptance/PHASE3_GAP_FIXES.md`.

## Command-output matrix

The following output, error, and exit-status cases matched after normalization:

- general help and unchanged command help
- `check`
- plain and JSON `search`
- JSON, YAML, and CSV alias export
- `stats all`
- `doctor`

Only these approved outputs differed:

- `help setup` describes the Linux and WSL Bash login loader.
- `shell-init bash` accepts the selected alias through Readline so native status, directory, and history semantics are preserved.
- `shell-init zsh` accepts the selected alias through ZLE for the same reason.

No unapproved command-output, error, status, or final-file difference was found.
