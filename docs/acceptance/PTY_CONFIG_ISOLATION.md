# Isolate PTY tests from the developer's config

## Acceptance criteria

- Every PTY resize test starts its helper with a temporary home. Opening footer settings reads only that test home.
- The Ctrl+, PTY cases open footer settings when the parent process has a malformed config in a separate temporary home.
- Tests do not read or change the developer's Alias Lens config or shell files.
- `make fmt check` passes.
