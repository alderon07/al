# Use platform-friendly shortcut profiles

## Acceptance criteria

- The Windows profile follows Windows conventions for new, edit, refresh, save, undo, and delete.
- The Linux profile follows GNOME conventions for new, settings, refresh, save, undo, help, and delete.
- The macOS profile uses Command shortcuts for new, settings, refresh, save, undo, and help. It does not repurpose a standard Command shortcut for an unrelated action.
- App-specific pages use `F2` through `F8`. Each page keeps one unmodified function-key binding that works in a legacy terminal.
- Every action has a terminal-safe binding. A profile remains usable when the user selects it on a different operating system or when the terminal does not send Command or modified punctuation keys.
- Enhanced terminal reports preserve the shifted printable character, so `Ctrl+?` opens help and `Ctrl+,` opens settings when the terminal reports those modifiers.
- `Delete` removes the selected alias only when the search field is empty. `Backspace` and `Delete` continue to edit nonempty search text.
- Forms, confirmations, and text fields handle input before global shortcuts.
- Pasted text and unassigned modified keys do not invoke an action or enter text.
- The keyboard guide and footer labels come from the same bindings that handle key events.
- Table tests cover each profile, every fallback, duplicate bindings, and Bubble Tea translation for `F1` through `F8`.

## Shortcut matrix

| Action | Windows | Linux | macOS | Portable binding |
| --- | --- | --- | --- | --- |
| Help | `F1` or `?` | `Ctrl+?`, `F1`, or `?` | `Cmd+?`, `F1`, or `?` | `F1` or `?` |
| Stats | `F2` | `F2` | `Cmd+2` or `F2` | `F2` |
| Settings | `Ctrl+,` or `F3` | `Ctrl+,` or `F3` | `Cmd+,` or `F3` | `F3` |
| Themes | `F4` | `F4` | `Cmd+4` or `F4` | `F4` |
| Refresh | `F5` or `Ctrl+R` | `Ctrl+R` or `F5` | `Cmd+R`, `F5`, or `Ctrl+R` | `F5` or `Ctrl+R` |
| Sync status | `F6` | `F6` | `Cmd+6` or `F6` | `F6` |
| Health | `F7` | `F7` | `Cmd+7` or `F7` | `F7` |
| Revisions | `Ctrl+Z` or `F8` | `Ctrl+Z` or `F8` | `Cmd+Z` or `F8` | `F8` |
| Add | `Ctrl+N` | `Ctrl+N` | `Cmd+N` or `Ctrl+N` | `Ctrl+N` |
| Edit | `Ctrl+E` | `Ctrl+E` | `Cmd+Shift+E` or `Ctrl+E` | `Ctrl+E` |
| Delete | `Delete` or `Ctrl+D` | `Delete` or `Ctrl+D` | `Cmd+Backspace` or `Delete` | `Delete` |
| Save a form | `Ctrl+S` | `Ctrl+S` | `Cmd+S` or `Ctrl+S` | `Enter` on the save action |

Alias Lens runs the TUI in raw terminal mode. On Unix, raw mode disables `IXON` and `ISIG`, so `Ctrl+S` does not pause output and `Ctrl+Z` does not suspend the process while the TUI is open.
