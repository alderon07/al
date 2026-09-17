# Finish the phase 3 shell checks

Use this checklist on disposable home directories. None of these commands should read or change your real alias or startup files. Store the restart-test home under `~/.cache`, because some WSL installations clear `/tmp` during shutdown.

## What the three checks mean

- A PTY is the terminal device that an interactive shell sees. A PTY test sends real keystrokes to Bash or Zsh and checks the prompt, cursor, key bindings, output, exit status, history, and current directory.
- The baseline matrix runs the old and new Alias Lens binaries with the same inputs. Their output and file changes must match. This catches behavior changes hidden by the shell-adapter refactor.
- WSL evidence proves that the feature works inside real WSL 2. A Linux test with a WSL environment variable does not test Windows Terminal, WSL startup behavior, or the Windows-to-WSL boundary.

## Prepare a safe test build

- [x] Open the Alias Lens repository on `dev`.
- [x] Confirm that the working tree contains no changes that you might lose. The current changes are the intentional phase 3 work under test.
- [x] Build the current binary.

```bash
git switch dev
git status --short
mkdir -p /tmp/al-phase3/current-bin
go build -buildvcs=false -o /tmp/al-phase3/current-bin/alias-lens ./cmd/alias-lens
```

- [x] Create disposable Bash and Zsh homes.

```bash
mkdir -p /tmp/al-phase3/bash-home /tmp/al-phase3/zsh-home
```

Delete `/tmp/al-phase3` after the tests. Do not substitute your real home directory in any command below.

## Check the PTY behavior

- [x] Test `Ctrl+G` in Bash on an empty prompt.
- [x] Select an alias and press Enter.
- [x] Confirm that the terminal shows the accepted alias once before the command output.
- [x] Confirm that the command runs in the current shell.
- [x] Test `Ctrl+G` after you type text at the prompt. Confirm that Alias Lens does not run the text.
- [x] Repeat the checks in Zsh.

Example Bash test:

```bash
printf "alias ok='echo PTY_OK'\n" > /tmp/al-phase3/bash-home/.bash_aliases
env -i \
  HOME=/tmp/al-phase3/bash-home \
  PATH=/tmp/al-phase3/current-bin:/usr/bin:/bin \
  SHELL=/bin/bash \
  TERM="$TERM" \
  ALIAS_LENS_SHELL=bash \
  bash --noprofile --norc
```

At the clean Bash prompt, run these commands:

```bash
eval "$(alias-lens shell-init bash)"
```

Press `Ctrl+G`, select `ok`, and press Enter. You should see output like this:

```text
$ ok
PTY_OK
```

The prompt should return without an error. This is a PTY test because Bash, Readline, and Alias Lens are interacting through a real terminal. A unit test that calls a Go function directly cannot prove this behavior.

Record the shell version, exact keys, visible output, `echo $?`, `pwd`, and any unexpected behavior.

## Compare the baseline and current builds

- [x] Build baseline commit `a5d6168` in a temporary worktree.
- [x] Run each binary with its own disposable home.
- [x] Compare setup, repair, removal, command output, and created files.
- [x] Test both Bash and Zsh.
- [x] Confirm that differences are limited to the approved gap fixes and normalized temporary data.

Evidence: [phase 3 baseline comparison](evidence/PHASE3_BASELINE_2026-09-17.md).

Build the baseline:

```bash
git worktree add --detach /tmp/alias-lens-baseline a5d6168
mkdir -p /tmp/al-phase3/baseline-bin
go -C /tmp/alias-lens-baseline build -buildvcs=false -o /tmp/al-phase3/baseline-bin/alias-lens ./cmd/alias-lens
```

Example setup comparison:

```bash
mkdir -p /tmp/al-phase3/old-home /tmp/al-phase3/new-home

env -i HOME=/tmp/al-phase3/old-home PATH=/usr/bin:/bin SHELL=/bin/bash \
  /tmp/al-phase3/baseline-bin/alias-lens setup bash > /tmp/al-phase3/old.out 2> /tmp/al-phase3/old.err

env -i HOME=/tmp/al-phase3/new-home PATH=/usr/bin:/bin SHELL=/bin/bash \
  /tmp/al-phase3/current-bin/alias-lens setup bash > /tmp/al-phase3/new.out 2> /tmp/al-phase3/new.err

diff -u /tmp/al-phase3/old.out /tmp/al-phase3/new.out
diff -u /tmp/al-phase3/old.err /tmp/al-phase3/new.err
```

Both `diff` commands should print nothing. Compare the two temporary homes next. The logical file contents, modes, and file names should match. Ignore only the different temporary root paths and timestamps.

Record any difference before you change the code. A difference can be a real regression or an intentional behavior change that needs separate acceptance criteria.

## Record WSL 2 evidence

- [x] Run this section inside Ubuntu on WSL 2, not a normal Linux installation.
- [x] Record the Windows, WSL, Linux, Bash, Zsh, Go, Git, and Alias Lens versions.
- [x] Run the Go tests.
- [x] Repeat the Bash and Zsh PTY examples inside WSL.
- [x] Confirm that a new WSL session still finds `alias-lens` and `al`.
- [x] Save the sanitized transcript in `docs/testing/evidence/` without aliases, tokens, home paths, or other private data.

Evidence: [WSL 2 terminal record](evidence/PHASE3_WSL2_2026-09-17.md).

Start with these commands:

```bash
wsl.exe --version
uname -a
bash --version
go version
git rev-parse HEAD
GOCACHE=/tmp/alias-lens-phase3-cache go test ./...
```

Example restart test:

1. Build Alias Lens at `/tmp/al-phase3/current-bin/alias-lens` inside WSL.
2. Install the binary in a persistent disposable home and run setup:

   ```bash
   phase3_home="$HOME/.cache/alias-lens-phase3-wsl-home"
   mkdir -p "$phase3_home/.local/bin"
   install -m 0755 /tmp/al-phase3/current-bin/alias-lens \
     "$phase3_home/.local/bin/alias-lens"
   install -m 0600 /dev/null "$phase3_home/.hushlogin"
   env -i HOME="$phase3_home" \
     PATH="$phase3_home/.local/bin:/usr/bin:/bin" \
     SHELL=/bin/bash \
     "$phase3_home/.local/bin/alias-lens" setup bash
   ```

3. Close the WSL terminal window.
4. From PowerShell, run `wsl --shutdown`.
5. From PowerShell, start a login shell against the disposable home:

   ```powershell
   wsl -d Ubuntu -- env HOME=<LINUX_HOME>/.cache/alias-lens-phase3-wsl-home PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin SHELL=/bin/bash bash -lic 'command -v alias-lens; type al; al doctor; echo DOCTOR_STATUS=$?'
   ```

Replace `<LINUX_HOME>` with the Linux home path. If your distribution is not named `Ubuntu`, replace that name with the output from `wsl -l -q`.

Both `alias-lens` and the `al` function should resolve after the restart. `al doctor` should not report a missing executable or broken Bash integration.

For the evidence record, include:

- The command output from the version checks.
- The exact keys used in the PTY test.
- The visible `$ ok` and `PTY_OK` lines.
- The exit status from `echo $?`.
- The output from `pwd`.
- The result after `wsl --shutdown`.
- A note that the test used a disposable home.

## Mark the gate complete

- [x] Add automated PTY coverage for Bash and Zsh.
- [x] Commit the baseline comparison results and test fixtures.
- [x] Commit the sanitized WSL 2 evidence record.
- [x] Run `make fmt check`.
- [x] Check the three remaining phase 3 boxes in `TODO.md` only after their evidence exists.

All three checks passed. Phase 4 can proceed.
