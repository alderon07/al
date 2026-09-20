# Keep the TUI footer visible after terminal resizes

Status: approved by the user's bug-fix request on 2026-09-19.

## Problem

The footer can disappear after the terminal window changes size. The regression
started when footer spacing and bottom-row alignment changed. Static render
tests do not reproduce the terminal renderer's resize and clipping behavior.

## Acceptance criteria

### RF-001 keeps footer rows inside the viewport

Every full-screen TUI view accounts for its outer padding when it places the
control footer, global navigation, and maker line. The rendered frame occupies
the current terminal width and height without relying on terminal clipping.

### RF-002 redraws the footer after live resizes

After each supported `WindowSizeMsg`, the active view shows its control footer
and configured maker line. This holds when the terminal grows, shrinks, and
grows again. Resize handling preserves the active page and selection.

### RF-003 covers the real terminal path

An automated PTY test starts the compiled Bubble Tea program in an alternate
screen, changes the PTY window size while it is running, and inspects the
resulting screen after each change. The footer must remain visible at narrow
and wide supported sizes.

### RF-004 keeps one content column across pages

The main alias browser and every page opened inside it use the same horizontal
padding and maximum content width. Switching to stats, keyboard help, footer
settings, themes, revisions, sync status, health, or a confirmation must not
make the header, controls, global navigation, divider, or maker line grow or
shrink on the same terminal.

The standalone stats dashboard may use the full terminal width because it has
no parent browser column to match.

## Test mapping

- Render tests assert exact viewport dimensions and footer placement for a
  grow, shrink, and grow sequence.
- The PTY resize test exercises `SIGWINCH`, Bubble Tea's renderer, and the
  alternate-screen output rather than calling `View` alone.
- Wide-terminal render tests compare the visible content bounds of every main
  TUI page.
- A PTY test switches between aliases, stats, and keyboard help at one terminal
  size and checks the shared divider width after each switch.
- `make fmt check` passes.
