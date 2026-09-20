# Repository checkout path acceptance criteria

## Goal

Alias Lens stores provider-managed clones in a directory hierarchy that matches the remote repository name. A GitHub repository named `alderon07/dotfiles` uses `repos/github/alderon07/dotfiles`, not `repos/github/alderon07--dotfiles`.

## Acceptance criteria

- New provider-managed clones use `repos/<provider>/<namespace>/<repository>`.
- Repository names with more than one namespace component keep the full hierarchy.
- Empty components, absolute names, and `.` or `..` components are rejected before Alias Lens creates a destination.
- Existing clones and configured repository paths are not moved or rewritten.
- Automated tests cover destination construction and unsafe remote names.
- A PTY check confirms the sync screen displays the hierarchical repository path at narrow and wide terminal widths.
