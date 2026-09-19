# TUI global navigation acceptance criteria

## Behavior

- `?` opens help from the alias list, stats, themes, revisions, sync status, and the health-filtered alias list.
- `Ctrl+S`, `Ctrl+T`, `Ctrl+Z`, `Ctrl+F`, and `Ctrl+H` open stats, themes, revisions, sync status, and alias health from any of those pages.
- Pressing the shortcut for the current page returns to the alias list.
- Pressing a different page shortcut closes the current page before opening the requested page.
- Leaving the theme picker through a page shortcut restores the theme that was active before the preview.
- Page shortcuts do not interrupt the add or edit form, the first-run tour, or a delete, run, or revision-restore confirmation.
- Select mode keeps its existing page restrictions. Help remains available.
- Secondary-page footers advertise the shared page shortcuts at terminal widths that can display them.

## Test mapping

- A table-driven model test covers every source page and destination shortcut.
- Focused tests cover shortcut toggling, theme restoration, select-mode restrictions, and protected forms and confirmations.
- Existing page-specific tests continue to cover each page's local controls.
