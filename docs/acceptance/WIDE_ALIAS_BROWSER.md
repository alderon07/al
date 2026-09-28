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
- `WIDE-10`: At a given wide viewport width, category badges begin in one list column for short, long, selected, and Unicode alias names. Long names truncate in the list, remain complete in the selected detail pane, and do not push status markers or the detail pane outside the viewport.
- `WIDE-11`: The overview below search shows the existing favorite, category, and attention counts at wide and narrow widths.
- `WIDE-12`: With room in the selected-alias card, a Usage section shows all-time, today, and last-seven-day alias uses from the active shell history, the latest dated run or an honest unknown/never state, and the separate exact-command history match count. It updates with selection.
- `WIDE-13`: The selected-alias card stops at 72 cells on roomy terminals. It uses the available detail width when that is smaller, wraps long text inside the card, and leaves the rest of the pane in the page background.
- `WIDE-14`: The two-cell gap between the selected alias name and its category badge uses the card panel background in true-color output. The detail card still renders without color.
- `WIDE-15`: Missing or unreadable history never looks like measured zero usage. Reloading aliases or stats refreshes the selected usage. Short wide terminals keep the selected command and footer visible even when the Usage section does not fit.
- `WIDE-16`: The alias browser, stats view, and theme picker leave the full-screen canvas at the terminal's native background. The selected-alias card alone may use the theme panel color within its 72-cell bound.
- `WIDE-17`: At 16-color depth or with `NO_COLOR=1`, neither view requests a terminal-wide RGB background or paints a dark panel that could map to an unrelated ANSI color. Text, borders, badges, and selected-alias details remain legible.
- `WIDE-18`: At 256-color depth, bounded dark panel colors use the closest terminal RGB color, not a saturated hue selected by the color library. Nested text style resets do not expose the terminal background inside the selected-alias card. The app does not change the terminal's default background color.
- `WIDE-19`: Main-page headers stay in the same visible terminal column across color profiles. Alignment checks ignore ANSI styling bytes.

## Test mapping

- `WIDE-01` through `WIDE-04`: structured renderer assertions in `tui_alias_panes_test.go`.
- `WIDE-05`: control-text and cell-width cases in `tui_alias_panes_test.go`.
- `WIDE-06` and `WIDE-07`: viewport and resize tests in `tui_alias_panes_test.go`.
- `WIDE-08`: `make fmt check` plus recorded PTY evidence for the compiled binary.
- `WIDE-09`: Exact-width and overflow ANSI regressions in `accessibility_test.go`, plus a color PTY check of the compiled wide browser.
- `WIDE-10`: Category column and long-name assertions in `tui_alias_panes_test.go`, plus a wide PTY inspection with aliases of different name lengths.
- `WIDE-11` and `WIDE-12`: Overview and selected-usage assertions in `tui_alias_panes_test.go`.
- `WIDE-13` and `WIDE-14`: Card-width and styled-gap assertions in `tui_alias_panes_test.go`, plus wide and narrow PTY checks.
- `WIDE-15`: Missing-history, refresh, and short-viewport assertions in `tui_alias_panes_test.go`, plus a compiled PTY check.
- `WIDE-16` and `WIDE-17`: Color-profile assertions in `tui_theme_canvas_test.go` and `stats_test.go`, plus compiled PTY checks at narrow and wide widths.
- `WIDE-18`: ANSI-256 color and nested-style assertions in `tui_theme_canvas_test.go`, plus compiled Oscurange PTY checks at 256-color and true-color depth.
- `WIDE-19`: Header-column assertions in `tui_footer_test.go` across true-color and 256-color rendering.

## Design decision

Core trait affected: Card density in the wide alias browser.

User problem: The current wide view uses a full-row highlight and full-height divider that do not match the standard alias cards. Category and state information collapse into punctuation, and the mostly empty detail pane lacks a clear boundary.

Evidence from the current interface: The supplied 1127x1244 screenshot shows 46 visually uniform rows, a gray selected bar, category names without badges, state punctuation without labels, and a divider continuing through a large empty area.

Proposed change: Keep one-line rows, add themed left borders and category badges, render the selected details inside a content-height card, and show selected-alias usage below its details while keeping the familiar overview below search.

Traits kept unchanged: Dark working field, Phosphor accent, cyan commands, command-first content, keyboard footer, human footer, keyboard behavior, and narrow alias cards.

48x18 result: Existing single-column card view.

80x24 result: Existing single-column card view.

120x30 result: Compact list and bounded selected-alias card.

NO_COLOR result: Pointer, borders, badge text, and state labels preserve meaning.

Migration or compatibility effect: None. This changes rendering only.

Approval: Requested by the repository owner on 2026-09-27.
