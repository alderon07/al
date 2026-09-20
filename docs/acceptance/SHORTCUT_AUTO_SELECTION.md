# Restore automatic shortcut selection

## Acceptance criteria

- `al shortcuts auto` removes a saved shortcut profile instead of saving the current operating system's profile.
- After the reset, Alias Lens chooses Windows on Windows and WSL, macOS on macOS, and Linux on other Linux systems.
- Later configuration writes keep the shortcut profile unset, so moving the configuration to another computer does not preserve the old computer's shortcut style.
- `al shortcuts` identifies whether the active style is automatic or saved and tells users how to restore automatic selection when a saved choice is active.
- Command help, shell completion, and the README list `auto` with the explicit profile choices.
- Tests use a temporary home and never read or change the user's real Alias Lens configuration.
