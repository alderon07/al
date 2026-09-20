# Make installed updates visible

Status: approved for implementation by the user on 2026-09-19.

## Problem

Linux and macOS can keep a running Alias Lens process alive after `make install` replaces its executable. The open TUI continues to show the old interface even though new shells start the new binary. The installer currently gives no result that helps the user tell these states apart.

## Acceptance criteria

### IU-001 verifies the installed file

`make install` compares the built file with the installed file after the copy. The command fails if the bytes differ. A successful command prints the installed path.

### IU-002 warns about open Alias Lens screens

When `pgrep` reports a running `alias-lens` process, `make install` tells the user to close and reopen existing Alias Lens screens. The check does not stop or signal a process. Installation still works when `pgrep` is unavailable.

### IU-003 detects replacement while the TUI is open

The TUI records the identity of its executable at startup. On a later UI tick, it compares that identity with the file at the installed path. If the path is missing or identifies a different file, every page in the alias TUI shows this message until the process exits:

```text
Alias Lens was updated. Close this screen, then enter al again.
```

The TUI does not exit automatically. It does not discard a form, selection, or unsaved input.

### IU-004 keeps failure behavior quiet

If Alias Lens cannot find or inspect its executable at startup, the replacement check stays disabled. A permission or transient read error after startup does not interrupt the TUI. The check reads file metadata only.

## Automated evidence

- A temporary executable path reports unchanged before replacement and changed after an atomic replacement.
- A model tick makes the update notice visible after replacement.
- An unchanged executable does not show the notice.
- `make fmt check` passes.
