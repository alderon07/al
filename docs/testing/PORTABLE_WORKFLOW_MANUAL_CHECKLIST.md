# Test the portable workflow on WSL, macOS, and Linux

Use this checklist for the shortcut, completion, catalog, and local diff-viewer changes. Run the platform section on WSL, macOS, and native Linux. Run the common workflow section once on the computer you use most.

Do not save real aliases, tokens, home paths, or repository URLs in test evidence. Record only the versions, pass or fail results, and sanitized error messages.

## Record the build

Use the same commit on every computer.

```bash
git switch dev
git pull
git rev-parse HEAD
make install
alias-lens --version
```

Copy the result from `git rev-parse HEAD` into your evidence. Confirm that every computer reports the same commit before you compare results.

## Test these items on every platform

OS detection, shell startup, terminal key handling, and browser launch differ by platform. Test these items separately on WSL, macOS, and native Linux.

| Check | WSL | macOS | Linux |
| --- | --- | --- | --- |
| Installation succeeds | [ ] | [ ] | [ ] |
| `al doctor` passes | [ ] | [ ] | [ ] |
| Shortcut style matches the OS | [ ] | [ ] | [ ] |
| A saved shortcut style overrides detection | [ ] | [ ] | [ ] |
| Shell completion loads in a new shell | [ ] | [ ] | [ ] |
| `Ctrl+G` works at an empty prompt | [ ] | [ ] | [ ] |
| `Ctrl+G` does not replace typed text | [ ] | [ ] | [ ] |
| The TUI works at 48x18 and 160x40 | [ ] | [ ] | [ ] |
| The local diff viewer opens, when comparison data exists | [ ] | [ ] | [ ] |

### WSL

Run these commands inside WSL:

```bash
wsl.exe --version
uname -a
bash --version | head -1
al doctor
al shortcuts
al shortcuts test
al completion install bash
```

Confirm that `al shortcuts` selects the Windows style by default. Start a new Bash shell and confirm that `al ` followed by Tab shows suggestions.

Close every WSL window. From PowerShell, enter:

```powershell
wsl --shutdown
```

Open WSL again. Confirm that both commands still work:

```bash
command -v alias-lens
type al
al doctor
```

### macOS

Test the default Zsh installation:

```bash
sw_vers
zsh --version
al doctor
al shortcuts
al shortcuts test
al completion install zsh
```

Confirm that `al shortcuts` selects the macOS style by default. Start a new Zsh shell and test Tab suggestions.

If you support Bash users on macOS, test stock Bash 3.2 too. `Ctrl+G` keeps its normal cancel behavior there, so open Alias Lens by entering `al`.

### Native Linux

Test a Linux installation that is not WSL:

```bash
uname -a
bash --version | head -1
al doctor
al shortcuts
al shortcuts test
al completion install bash
```

Confirm that `al shortcuts` selects the Linux style by default. Start a new Bash shell and test Tab suggestions.

Test Zsh on either Linux or macOS:

```bash
zsh --version
al completion install zsh
```

## Test the common workflow once

Run this section on one platform after the platform checks pass.

### Check the wording

```bash
al status
al shortcuts test
al help status
al help catalog
al help completion
```

Confirm that each result explains what happened and what to enter next. Flag words that assume Git, shell, or configuration-file knowledge.

### Preview and import the catalog

Use `zsh` instead of `bash` when Zsh owns the aliases under test.

```bash
al catalog preview --from bash
al catalog import --from bash
al status
```

Confirm these results:

- Preview changes no files.
- Import keeps the native alias file unchanged.
- Import creates an inactive private catalog only when it finds a safe entry.
- A file with no safe entries does not create an empty catalog.
- The output says that the catalog is inactive.

### Check completion safety

```bash
al plan completion install bash
al completion install bash
```

Start a new shell. Confirm that completion suggests commands, flags, profiles, and entry names. Confirm that it does not print command bodies.

### Check the TUI

Open Alias Lens:

```bash
al
```

Check both a narrow terminal at 48x18 and a wide terminal at 160x40. Confirm these results:

- The footer and help fit without hiding the selected alias.
- The UI uses plain instructions such as "Enter" and "Use".
- The displayed shortcuts match the selected shortcut style.
- Enter chooses the selected item.
- Cancel closes the current view without changing aliases.

### Check the local diff viewer

`al catalog import` does not install or activate the catalog. Therefore, a new installation has no saved `installed` catalog to compare. This is expected until catalog activation is implemented.

If the test environment has an approved installed snapshot or a repository catalog, run one of these commands:

```bash
al catalog diff --from installed --shell bash --web
al catalog diff --from repository --shell bash --web
```

Check the viewer at narrow and wide browser widths. Use the keyboard for every control. Test light mode, dark mode, and reduced motion. Confirm that the first page hides command text until you choose to reveal it.

If no comparison source exists, run the automated viewer checks instead:

```bash
go test ./cmd/alias-lens -run 'TestCatalogDiffWeb' -count=1
```

Do not mark the manual browser row as passed from the automated test alone.

## Record the result

Create a dated file such as `docs/testing/evidence/PORTABLE_WORKFLOW_2026-09-19.md`. Record:

- The commit hash.
- The OS, shell, terminal, and browser versions.
- Each table result.
- Sanitized failure output.
- The terminal dimensions used for the TUI checks.
- Whether the browser check used an installed snapshot or a repository catalog.

Do not include alias names, command bodies, provider tokens, local paths, or repository URLs.
