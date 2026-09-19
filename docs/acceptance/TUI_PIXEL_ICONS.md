# TUI pixel icon acceptance criteria

## Purpose

Add a small pixel-art icon set that makes interactive Alias Lens screens easier to scan. Icons support labels. They never replace labels or keyboard hints.

## Rendering

- [x] Build every icon from terminal block characters and render it in one terminal row.
- [x] Give every icon the same cell width so headings and controls stay aligned.
- [x] Keep icon rendering theme-aware and free of hard-coded ANSI escape sequences.
- [x] Keep all icons out of plain, non-interactive command output.

## Placement

- [x] Give the main page, help, stats, themes, revisions, sync, health, forms, confirmations, and repository picker a fitting icon.
- [x] Use small semantic icons for alias favorites, functions, commands, warnings, and tracked-file destinations where they improve recognition.
- [x] Do not add icons to dense keyboard footers or every line of body copy.

## Accessibility and layout

- [x] Keep a readable text label next to every icon.
- [x] Preserve the existing minimum terminal size and narrow-layout behavior.
- [x] Test the exact terminal-cell width of every icon.
- [x] Test representative views for their text labels and pixel icons.

## Verification

- [x] Run `make fmt check`.
- [x] Inspect the compiled TUI at narrow and wide terminal widths.

## Maker credit

- [x] Show `Made with ♥ by Naqi` below the controls on every interactive TUI screen.
- [x] Render the heart with the theme warning color and keep the surrounding credit muted.
- [x] Keep the credit centered within the available content width.
- [x] Keep the credit out of plain, non-interactive output.
- [x] Test the credit text, heart, width, and representative TUI screens.

## Custom maker credit

- [x] Store the footer message and icon choice in `config.json` with backward-compatible defaults.
- [x] Add `al config footer-message MESSAGE`, `al config footer-icon ICON`, and `al config footer-reset`.
- [x] Use `{icon}` as the optional icon position in the message.
- [x] Accept documented built-in icons, `none`, and custom 4×2 `#` and `.` bitmaps.
- [x] Accept one Unicode grapheme through `emoji:VALUE`, including variation selectors and skin-tone sequences.
- [x] Reject control characters, multiline messages, oversized messages, and malformed icon values.
- [x] Apply the configured footer to every interactive TUI without reading the config on each render.
- [x] Document the commands and custom bitmap format.
- [x] Test defaults, configuration writes, validation, rendering, and reset behavior.

## Footer settings page

- [x] Open footer settings with `F3` from every non-modal TUI page.
- [x] Edit the footer message and icon with keyboard-only controls.
- [x] Preview valid changes in the pinned footer before saving.
- [x] Save through the existing private, atomic config writer.
- [x] Restore the saved footer when the user cancels or switches pages.
- [x] Show validation errors without closing the settings page.
- [x] Add the page to the keyboard guide and global navigation hint.
- [x] Test page navigation, editing, save, cancel, and invalid input.
