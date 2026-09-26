# Native terminal diff viewer acceptance criteria

## Scope

Alias Lens displays a read-only file diff in its Bubble Tea interface. A selected private revision can be compared with the live alias file before restore. The sync screen can compare the live alias file with its configured repository copy. `al diff` keeps its plain output for scripts, and `al diff --tui` opens the same viewer directly.

## Acceptance criteria

- Opening a diff reads only the two selected local files. It never executes an alias or starts sync.
- The viewer shows additions, deletions, and unchanged context, including comments, metadata, ordering, and shell syntax. It also reports alias-level additions, removals, and changed commands.
- The comparison direction is explicit. For a revision, the left or old side is the current file and the right or new side is the selected revision, so the displayed changes describe a restore.
- The viewer uses a unified layout in narrow terminals and offers split layout where both columns fit. Long lines are clipped visibly and can be inspected by horizontal scrolling.
- Arrow keys, Page Up/Down, Home/End, and next/previous change navigation work without changing a file. Escape returns to the prior screen.
- A revision restore requires a separate confirmation after preview. The existing backup and private revision behavior remains in place.
- Files are read with the existing alias-file size limit. Error messages do not print private command contents. No diff contents are logged, stored in config, or sent to a browser.
- Noninteractive `al diff` output remains compatible. The native viewer can run without Node.js, a browser, or a network connection.

## Required evidence

- Tests for changed commands, comments-only changes, equal files, navigation, restore confirmation, and read-only behavior.
- `make fmt check` from the repository root.
- PTY checks at narrow and wide terminal widths, including a real keypress through the preview and return flow.

## Recorded evidence

- `GOCACHE=/tmp/alias-lens-diff-go-cache make fmt check` passed on 2026-09-26.
- PTY tests passed at 48×18, 80×24, and 120×36. The revision PTY test previewed with Enter, confirmed with `r` and `y`, and verified the saved file contents.
