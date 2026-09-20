# Security remediation acceptance criteria

These criteria cover the validated findings from Codex Security scan
`796e9f83-fdc1-4a1a-a05e-317ca637fdac`.

## Terminal output

- Repository diff, the Bubble Tea list and confirmation views, and plain search
  output render alias-controlled fields without emitting raw terminal control or
  bidirectional-formatting characters.
- Structured JSON output and explicit alias execution preserve the original
  command value.
- A real PTY test covers representative CSI and OSC payloads.

## Repository resource limits

- Automatic reconciliation, manual diff, and manual pull reject alias or
  tracked-file inputs above the documented limits (8 MiB for alias files and
  32 MiB for explicitly enrolled tracked files) before hashing,
  parsing, conflict copying, or rewriting the live file.
- Inputs at the limit and normal alias imports continue to work.
- Manual pull applies all accepted remote-only aliases with one validated atomic
  alias-file update.

## Provider resource limits

- Direct HTTP responses and provider CLI API output have byte limits.
- Provider pagination has page and total-result limits, and next-link pagination
  rejects cycles.
- Normal GitHub, GitLab, and Bitbucket discovery remains compatible.

## Private files and directories

- Alias Lens creates and verifies every managed repository directory as an
  owner-only real directory with mode `0700` before cloning.
- A pre-existing permissive managed root is tightened when safely owned; unsafe
  path types fail closed.
- Startup-file backups are atomically replaced with mode `0600`, even when a
  permissive backup already exists.

## Git publication

- Before either manual or automatic push, Alias Lens examines every outgoing
  commit and changed path relative to the exact upstream.
- A push is rejected if any outgoing path is outside the configured alias file
  and explicitly enrolled tracked-file allowlist.
- Secret scanning covers every allowed outgoing blob, while ordinary allowed-only
  histories continue to push.

## Release integrity

- Every third-party GitHub Action is pinned to a reviewed full commit SHA.
- GoReleaser and Syft resolve to exact reviewed versions.
- Build and test work runs with read-only repository permissions; publication and
  attestation permissions exist only in the release job that needs them.

## Verification

- Focused regression tests prove each original trigger no longer reaches its
  vulnerable sink and prove a legitimate control still works.
- `make fmt check` passes.
- The compiled binary is exercised in a real terminal at narrow and wide widths.
