# Shortcut editor

- The keyboard guide opens a shortcut editor from the TUI without requiring a command line.
- The editor lists every configurable TUI action and the Bash/Zsh launcher, with current keys and changed bindings marked.
- A user can filter actions, select one, press a replacement key, or restore its default. Invalid or conflicting keys leave the saved config unchanged and show an actionable error.
- A saved TUI binding works immediately. A changed shell launcher shows a reminder to reload shell integration.
- The editor remains usable at narrow and wide terminal sizes, and PTY tests cover the editing flow.
