# Keep the wide alias browser clean during navigation

## Acceptance criteria

- Moving through aliases at 128x33 leaves exactly one selected row and a detail card for that alias. Old rows, commands, and usage text do not remain on screen.
- The fix works with the alternate screen and color output used by macOS terminals, including terminals that mishandle scroll-region updates.
- Narrow and wide navigation keep their current keyboard behavior and selection.
- A PTY regression uses synthetic aliases and inspects redraw behavior after repeated selection changes.
- `make fmt check` passes.
