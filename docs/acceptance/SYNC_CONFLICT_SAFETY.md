# Sync conflict safety

## Problem

Plain `al sync` copies the active alias file over the repository copy. When automatic sync has already recorded a conflict, this can commit the removal of aliases that exist only in the repository copy.

## Acceptance criteria

- Plain `al sync` fails while sync status is `conflict`.
- The failure says that the repository was not changed and names `al diff`, `al sync --pull`, and `al sync --push` as the next choices.
- A rejected sync leaves the repository alias file, `HEAD`, the index, and the active alias file unchanged.
- `al sync --pull` remains available to import remote-only aliases.
- `al sync --push` remains the explicit choice to replace the repository copy with the active alias file.
- The conflict refusal is readable in a real terminal at a narrow width.
- `al diff` identifies the active and repository files without exposing alias contents beyond the requested diff.
- A repository-only diff recommends `al sync --pull` before any push and explains that repository-only functions still require manual review.
- A local-only diff recommends `al sync --push` and explains that it publishes the active file.
- A diff with non-conflicting additions on both sides recommends `al sync --pull`, another `al diff`, and `al sync --push` only after remaining repository-only entries are copied or intentionally discarded.
- A changed definition says that Alias Lens cannot choose between commands and requires manual resolution before `al sync --push`.
- Whole-file differences recommend explicit `al sync --push` instead of plain `al sync`.
