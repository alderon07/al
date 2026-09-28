# Sync lock and private file fixes

- A running sync worker keeps its lock current throughout reconciliation and sleep, including intervals longer than two minutes. A second worker cannot acquire that lock.
- A worker removes the lock on exit only if it still owns the file at that path. A stale worker cannot remove a replacement lock.
- Sync writes the repository alias copy with mode `0600`, including when replacing a more permissive copy.
- Editing an existing empty alias file saves an empty private backup and a timestamped revision before replacing it. Creating a previously missing file does not invent a prior revision.
- Focused filesystem and lock tests pass, the PTY tests pass, and `make fmt check` passes.
