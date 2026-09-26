# TUI shortcut repeat handling

- Holding a page shortcut opens its page once; reported repeat events do not toggle it back to aliases.
- A deliberate second press after release still closes the current page on release-aware terminals, and a different page shortcut switches pages immediately.
- Terminals without key-release reporting do not flicker between pages during a held function-key shortcut.
- Ordinary typing, list movement, and the search cursor continue to work.
- A PTY test verifies a held shortcut leaves the selected page visible until another key is pressed.
