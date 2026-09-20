# Use focused terminal art and an editable footer

Status: approved for implementation by the user on 2026-09-19.

## Problem

The built-in 4×2 bitmap icons appear as unrelated blocks in large terminal fonts because Alias Lens compresses each icon into one row and repeats it throughout the interface. Claude Code instead gives its mascot several rows and uses it only where the layout has room. The footer settings page also marks a field as active but shows no text cursor. Typing only appends to an invisible position, so the form does not behave like an editor.

## Acceptance criteria

### TF-001 reserves pixel art for spacious layouts

The full Alias Lens mark uses a recognizable three-row silhouette. It appears only in the tour and empty state when the terminal is wide and tall enough. Compact headers use the configured brand text. Dense alias rows use `$`, `!`, and `*` markers instead of bitmap icons.

Page headings use text labels without decorative bitmap fragments. Named footer icons use their single-character symbols. The footer renderer keeps custom bitmap support only for an existing saved pattern.

### TF-002 keeps the product interface fixed

`ALIAS LENS` is the fixed product name and is not editable. Brand art and interface markers are not user-facing settings. A legacy custom brand value is ignored.

New footer settings use `Made by Naqi`, no icon, and centered alignment. Alignment accepts `left`, `center`, or `right`. Existing valid footer messages and icons remain unchanged and receive centered alignment.

### TF-003 makes every footer field editable

The footer page edits `Message`, `Icon`, `Alignment`, `Tone`, and `Rule`. Icons, alignment, tone, and rule are choices changed with Left and Right. Tone accepts `quiet`, `accent`, or `bright`. Rule accepts `none`, `thin`, or `dots`. The active message field shows a cursor. Typing and pasting insert text at the cursor. Left, Right, Home, and End move the cursor. Backspace removes the item before the cursor. Delete removes the item at the cursor. `Ctrl+U` clears the message.

Tab, Shift+Tab, Up, and Down change fields without losing any cursor position. Enter moves to the next field. Save and Cancel are explicit action rows. The displayed help names editing, field movement, clearing, saving, and canceling.

Editing uses terminal grapheme clusters so one Backspace removes one visible emoji or combined character. Valid settings update the preview. Escape restores the saved footer.

### TF-004 keeps the editor usable at supported sizes

At 48×18 and 160×40, the active field, its cursor, the validation result, and the save and cancel instructions remain visible. Long values scroll around the cursor instead of hiding it.

### TF-005 makes choices and form actions explicit

Choice fields show every available value and mark the current value. Left and Right change the selected value. Space selects the next value. The icon picker offers `none` and the built-in named icons; it does not offer or advertise custom bitmap syntax.

Save and Cancel are focusable rows after the settings fields. Enter activates either action. This gives terminals that reserve `Ctrl+S` a visible save path. An invalid field keeps the editor open, moves focus to that field, and shows its error beside the field.

Existing valid custom emoji and bitmap footer icons remain readable and renderable for configuration compatibility. Choosing a named icon in the footer editor replaces that legacy value.

The heart uses Unicode text presentation. When the message puts a space after a heart icon, Alias Lens uses a non-breaking terminal cell for that separator. The gap remains visible in both the live preview and the saved footer, including terminals that treat the heart as a wide glyph.

The `accent` tone uses the active theme's primary accent color. Changing or previewing a theme updates the footer preview and the saved footer without changing footer settings.

### TF-006 presents the footer as a focused workbench

At wide sizes, the page separates footer controls from a bordered live preview. Only the focused row receives an accent marker; inactive rows remain quiet. Save and Cancel share a compact action bar.

At narrow sizes, the page shows one focused setting or action at a time, its position in the seven-row flow, a compact preview, and the relevant controls. The layout does not imitate a full-width text-entry form or depend on clipped explanatory copy.

## Automated evidence

- Dense interactive views contain their text labels without built-in bitmap fragments.
- Full, compact, text, and hidden brand styles have render tests.
- Footer insertion, movement, deletion, clearing, field switching, save, and cancel have model tests.
- Combined emoji deletion removes one visible grapheme.
- Narrow and wide render tests keep the active cursor and instructions visible.
- Choice and action-row tests cover visible options, Enter-to-save, Enter-to-cancel, Space-to-select, and invalid-field focus.
- `make fmt check` passes.
