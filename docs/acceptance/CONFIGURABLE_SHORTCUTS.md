# Configure keyboard shortcuts

## Acceptance criteria

- `al shortcuts` lists every configurable action and its effective key. `al shortcuts set ACTION KEY` saves one override; `al shortcuts reset ACTION` restores that action's profile default; `al shortcuts reset-all` restores all defaults.
- The shell launcher is configurable for Bash and Zsh through `al shortcuts set launcher KEY`. Existing shell sessions pick up the new binding after reloading their integration. `ALIAS_LENS_NOBIND=1` still disables the launcher.
- TUI page, alias, selection, navigation, confirmation, sync comparison, stats, and diff actions can be rebound. Standard text entry remains possible, including search and form fields.
- Context-specific actions use dotted names such as `stats.next-view` and `diff.next-change`. The same default key may mean different things in different views; conflicts within a view are rejected.
- A saved override replaces the default binding for its action. Other profile defaults remain available. Help and footer labels show effective keys.
- Invalid, ambiguous, and conflicting assignments fail without changing config. A user can restore defaults from the CLI even if a TUI key becomes unusable in their terminal.
- Pasted input never invokes a shortcut. A shortcut assigned to a printable key does not steal text while a text field is active.
- Configuration is stored in the existing private app config. Tests use temporary homes and do not access real alias or history files.
- `make fmt check` and PTY tests cover shell bindings and at least one custom TUI shortcut.
