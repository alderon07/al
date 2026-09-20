# Pre-1.0 release-candidate matrix

Record one dated evidence file under `docs/testing/evidence/` for each release candidate. Include the commit, tag or archive checksum, operating-system version, shell version, terminal application, dimensions, tmux version when used, and every failed command.

## Install and upgrade matrix

Run clean install, upgrade from the previous published version, `al doctor`, `al setup --repair`, and `al setup --remove` in each environment. After removal, diff every startup and alias file against its pre-install copy.

| Environment | Login | Non-login | Restart boundary | Narrow 48x18 | Medium 80x24 | Wide 160x40 | tmux |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Ubuntu Bash | required | required | new user session | required | required | required | required |
| WSL Ubuntu Bash | required | required | `wsl.exe --shutdown` | required | required | required | required |
| macOS Zsh | required | required | log out and in | required | required | required | required |
| macOS Bash | required | required | new Terminal login shell | required | required | required | required |

Verify Ctrl+G with an empty and non-empty prompt. Enter must execute the selected alias. Tab must return it without execution and allow arguments to be appended. Check `NO_COLOR=1 al`, `TERM=dumb al`, non-ASCII descriptions, and every help-listed shortcut.

## Upgrade and corruption cases

For each case, copy the alias file, its mode, the config, the sync state, and the repository status before and after the action.

- Reject a versionless configuration and confirm the file remains byte-for-byte unchanged.
- Start with malformed JSON and confirm the command fails without changing the file.
- Start with a truncated alias definition and run `al check` and `al import` preview.
- Kill the process during alias replacement. The live path must contain the old or complete new bytes, never a prefix.
- Kill the worker during pull, commit, and push. Restart it and confirm the lock recovers.
- Drop the network during pull and push. Confirm an offline status and a successful later retry.
- Edit in the TUI while automatic sync runs. Confirm serialization or private conflict copies.
- Stage unrelated repository files before sync. Confirm their index entries and contents do not change.
- Create a remote conflict, inspect with `al diff`, resolve it, and confirm the next cycle reaches `synced`.

Do not use production provider tokens or real alias files for this matrix.
