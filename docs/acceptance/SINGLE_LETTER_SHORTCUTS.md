# Single letter shortcuts

- The shell launcher stays on `Ctrl+G` by default.
- On the alias screen, the TUI starts in command mode. A single letter opens each major page or runs its listed action. Existing function keys remain available.
- `/` enters alias search. Printable letters type into search and do not trigger actions. `Esc` returns to command mode while retaining the query, and `/` can reopen search.
- Text forms and the keyboard guide filter accept letters as text. Their non-text controls continue to work.
- Stats and diff use their own single letter actions before any same-letter global page action.
- The keyboard guide and shortcut editor show the current keys. Users can set custom single letters for command-mode actions; conflicting actions in the same context are rejected.
- PTY tests cover command/search behavior and narrow and wide terminal views.
