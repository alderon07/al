# Wide alias browser acceptance criteria

## Scope

This change updates the populated alias browser at terminal widths of 120 cells or more. Narrow widths keep the existing single-column alias cards.

## Acceptance criteria

- `WIDE-01`: At 120 columns or more, the alias list and selected-alias detail pane use the active theme's accent, secondary, panel, border, text, muted, and category roles. The renderer adds no hard-coded colors.
- `WIDE-02`: The selected compact row uses the same thick left accent border and visible pointer as the standard alias cards. Inactive rows retain a border-colored left edge.
- `WIDE-03`: Compact rows show the alias name and a dark-on-category badge. Favorite, local-context, and health states have text or symbol markers that remain present with `NO_COLOR=1`.
- `WIDE-04`: The detail pane is a bounded card with the selected alias name, kind, category, local context, complete wrapped command, and any description, tags, platforms, or health issues available on the alias.
- `WIDE-05`: The alias list and detail card remain aligned without clipping for ASCII, CJK, combining, emoji, long names, and long commands. User-controlled terminal bytes render as inert text.
- `WIDE-06`: At 120x18, the selected alias and its command remain visible with the footer and maker credit. At 132x36, the list shows a visible range summary and the complete detail content when it fits.
- `WIDE-07`: Resizing below 120 columns returns to the existing single-column cards without changing the selected alias or search query.
- `WIDE-08`: The Phosphor true-color view, ANSI-256 view, and `NO_COLOR=1` view preserve labels, focus, and warnings. A real PTY check covers wide and narrow widths.
- `WIDE-09`: A styled row that exactly fills its column keeps its complete ANSI sequences and text. A row that exceeds its column truncates by visible terminal cells without leaking foreground or background color into the detail pane or later rows.

## Test mapping

- `WIDE-01` through `WIDE-04`: structured renderer assertions in `tui_alias_panes_test.go`.
- `WIDE-05`: control-text and cell-width cases in `tui_alias_panes_test.go`.
- `WIDE-06` and `WIDE-07`: viewport and resize tests in `tui_alias_panes_test.go`.
- `WIDE-08`: `make fmt check` plus recorded PTY evidence for the compiled binary.
- `WIDE-09`: Exact-width and overflow ANSI regressions in `accessibility_test.go`, plus a color PTY check of the compiled wide browser.

## Design decision

Core trait affected: Card density in the wide alias browser.

User problem: The current wide view uses a full-row highlight and full-height divider that do not match the standard alias cards. Category and state information collapse into punctuation, and the mostly empty detail pane lacks a clear boundary.

Evidence from the current interface: The supplied 1127x1244 screenshot shows 46 visually uniform rows, a gray selected bar, category names without badges, state punctuation without labels, and a divider continuing through a large empty area.

Proposed change: Keep one-line rows, add themed left borders and category badges, and render the selected details inside a content-height card.

Traits kept unchanged: Dark working field, Phosphor accent, cyan commands, command-first content, keyboard footer, human footer, keyboard behavior, and narrow alias cards.

48x18 result: Existing single-column card view.

80x24 result: Existing single-column card view.

120x30 result: Compact list and bounded selected-alias card.

NO_COLOR result: Pointer, borders, badge text, and state labels preserve meaning.

Migration or compatibility effect: None. This changes rendering only.

Approval: Requested by the repository owner on 2026-09-27.
