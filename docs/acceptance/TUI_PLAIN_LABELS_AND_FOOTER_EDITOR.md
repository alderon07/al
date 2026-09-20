# Use focused terminal art and editable appearance settings

Status: approved for implementation by the user on 2026-09-19.

## Problem

The built-in 4×2 bitmap icons appear as unrelated blocks in large terminal fonts because Alias Lens compresses each icon into one row and repeats it throughout the interface. Claude Code instead gives its mascot several rows and uses it only where the layout has room. The footer settings page also marks a field as active but shows no text cursor. Typing only appends to an invisible position, so the form does not behave like an editor.

## Acceptance criteria

### TF-001 reserves pixel art for spacious layouts

The full Alias Lens mark uses a recognizable three-row silhouette. It appears only in the tour and empty state when the terminal is wide and tall enough. Compact headers use the configured brand text. Dense alias rows use `$`, `!`, and `*` markers instead of bitmap icons.

Page headings use text labels without decorative bitmap fragments. The footer renderer keeps explicit custom bitmap support for existing saved settings. Alias Lens renders a bitmap there only when the saved footer selects a bitmap name or pattern.

### TF-002 makes the visual style selectable

Appearance settings contain a custom brand label, a brand-art style, and a marker style. Brand art accepts `full`, `compact`, `text`, or `none`. Full uses the three-row mark where space allows. Compact uses a one-row mark. Text shows only the brand label. None hides the brand label and mark. Markers accept `symbols`, `ascii`, or `none`. Existing configurations receive full brand art, the `ALIAS LENS` label, and symbol markers.

New footer settings use `Made by Naqi`, no icon, and centered alignment. Alignment accepts `left`, `center`, or `right`. Existing valid footer messages and icons remain unchanged and receive centered alignment.

### TF-003 makes every appearance field editable

The appearance page edits `Brand`, `Brand art`, `Markers`, `Message`, `Icon`, and `Alignment`. Brand art, markers, and alignment are choices changed with Left and Right. The active text field shows a cursor. Typing and pasting insert text at the cursor. Left, Right, Home, and End move the cursor. Backspace removes the item before the cursor. Delete removes the item at the cursor. `Ctrl+U` clears the active text field.

Tab, Shift+Tab, Up, and Down change fields without losing any cursor position. Enter moves to the next field; Enter on `Alignment` saves. The displayed help names editing, field movement, clearing, saving, and canceling.

Editing uses terminal grapheme clusters so one Backspace removes one visible emoji or combined character. Valid settings update the preview. Escape restores the saved appearance and footer.

### TF-004 keeps the editor usable at supported sizes

At 48×18 and 160×40, the active field, its cursor, the validation result, and the save and cancel instructions remain visible. Long values scroll around the cursor instead of hiding it.

## Automated evidence

- Dense interactive views contain their text labels without built-in bitmap fragments.
- Full, compact, text, and hidden brand styles have render tests.
- Footer insertion, movement, deletion, clearing, field switching, save, and cancel have model tests.
- Combined emoji deletion removes one visible grapheme.
- Narrow and wide render tests keep the active cursor and instructions visible.
- `make fmt check` passes.
