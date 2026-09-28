# Alias status marker acceptance criteria

## Scope

The alias browser shows favorite and local context marks in its wide list and narrow cards. No alias metadata or ranking behavior changes.

## Acceptance criteria

- `MARK-01`: In the default symbol appearance, a favorite uses the existing one-cell heart mark in both wide rows and narrow cards. The wide row no longer prints `FAV`.
- `MARK-02`: In the default symbol appearance, a local context mark uses a one-cell location mark in both layouts. The wide row no longer prints `HERE`. A narrow card pairs the mark with `LOCAL`; the selected wide detail continues to state whether the mark belongs to this project or folder.
- `MARK-03`: ASCII appearance uses `*` for favorite and `@` for local context. With markers disabled, the narrow card still prints `LOCAL` and the wide row prints `LOCAL` for context. The selected wide detail still spells out `Favorite` and the context scope.
- `MARK-04`: The favorite and context marks survive `NO_COLOR=1`, remain separate from health warnings, and do not shift category badges or exceed the list width at 120 and 132 columns.
- `MARK-05`: `al context add NAME` and the context shortcut remain the only ways to set the local mark. The mark changes suggestion order for the current repository or folder; it does not restrict where the alias can run.

## Test mapping

- `MARK-01` through `MARK-04`: marker-style and row-width assertions in the alias browser and icon tests, plus a compiled PTY check at wide and narrow widths.
- `MARK-05`: existing context ranking and toggle tests.

## Design decision

Core trait affected: Text beside semantic markers in dense alias rows.

User problem: `FAV` is visually heavy, and `HERE` does not explain that the alias is marked for the current project or folder.

Evidence from the current interface: The supplied screenshot shows `FAV HERE` beside the selected alias in the wide list. The user requested a heart and a relevant symbol.

Proposed change: Use the existing favorite heart and a location mark in wide rows. Keep an explicit `LOCAL` label in narrow cards and a project or folder explanation in the wide detail pane.

Traits kept unchanged: Theme color roles, alias names, category badges, selection border, issue text, keyboard footer, and marker appearance settings.

48x18 result: Narrow card with a location mark and `LOCAL` when that mark is present.

80x24 result: Narrow card with the same mark and label.

120x30 result: Heart and location marks in the compact row, with the selected context scope in the detail pane.

NO_COLOR result: The symbols or ASCII fallbacks remain visible.

Migration or compatibility effect: None. This changes rendering only.

Approval: Requested by the repository owner on 2026-09-27.
