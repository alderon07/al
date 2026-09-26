# TUI shortcut footer placement

- On screens with the dotted signature rule, the last visible shortcut row sits directly above the rule when the terminal has spare height.
- Multirow shortcut blocks keep their internal blank row and order.
- Content stays at the top of the screen; dense content does not push shortcuts or the signature outside the terminal.
- The footer remains legible at the minimum 48 × 18 layout and at wide terminal sizes.
- Clearing the maker message removes its row; shortcuts use the reclaimed row and stay on the last content row.
- The footer editor keeps its own preview layout.
- PTY checks exercise narrow and wide terminal sizes.
