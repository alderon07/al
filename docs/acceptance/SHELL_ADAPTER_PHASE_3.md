# Phase 3 shell adapter acceptance criteria

Status: approved on 2026-09-16. Phase 3 implementation may proceed under these criteria.

## Scope and baseline

Phase 3 moves existing Bash and Zsh behavior behind `ShellAdapter`. It does not add the catalog, a migration, generated alias files, a configuration schema, a dependency, a command, or support for another shell.

Commit `a5d6168ce3afc96ed6838dc00ca63a9eb0999dbe` defines the behavior baseline. These fixtures were captured from a binary built at that commit:

| Fixture | SHA-256 |
| --- | --- |
| `cmd/alias-lens/testdata/phase3/bash-integration.golden` | `55565c3d456886a57493fb4afbf62cf98d88cad72c051bf766c5f5be57b4d355` |
| `cmd/alias-lens/testdata/phase3/zsh-integration.golden` | `88f3e897c2e1c16ab0d793c4143cb062ab3e77226ef88fab4f97fac9743b4bb7` |

The phase 3 implementation must not update either fixture. A fixture change needs a separate behavior-change proposal and review.

For setup, repair, removal, command output, sync sources, and TUI behavior, the test harness builds baseline commit `a5d6168` and the candidate commit in separate temporary worktrees. CI checks out full history so the baseline object is available without network access during tests. Both binaries receive matching isolated homes and inputs. Comparisons may normalize only the two temporary absolute roots and timestamps that the test injected itself. They must not normalize filenames, modes, exit statuses, shell names, commands, counts, layout text, or output order.

## Environment matrix

CI must pin the runner versions before the refactor changes shell behavior. Each job records `go version`, the shell version, and the operating-system version in its log.

| ID | Environment | Required shell evidence |
| --- | --- | --- |
| `U24-B52` | Ubuntu 24.04 x86-64, Go 1.24 latest patch | GNU Bash 5.2 and Zsh 5.9 |
| `M14-B32-Z59` | macOS 14, Go 1.24 latest patch | Apple Bash 3.2 and Zsh 5.9 |
| `W11-U24-B52` | Windows 11 with WSL 2 and Ubuntu 24.04 | GNU Bash 5.2, `wsl.exe --version`, and `uname -a` |

GitHub Actions supplies `U24-B52` and `M14-B32-Z59`. A maintainer records `W11-U24-B52` terminal evidence before merge. If a runner resolves a different patch version, record the exact version and review the difference before accepting the result.

All automated tests named below are in package `alias-lens/cmd/alias-lens`. Tests set `HOME`, `XDG_CONFIG_HOME`, `XDG_STATE_HOME`, `ZDOTDIR`, `HISTFILE`, `ALIAS_LENS_HISTORY_FILE`, `ALIAS_LENS_SHELL`, `SHELL`, and the working directory to isolated temporary paths. Tests must not read a developer's shell files.

## Evidence rules

Each filesystem test records a recursive manifest before and after the operation. The manifest contains relative path, file type, SHA-256, mode, owner, group, device, inode, link count, symbolic-link target, and modification time. It excludes access time because a read can update it.

Each terminal record includes the environment ID, version commands, setup commands, exact keystrokes, expected visible text, expected exit status, `pwd`, history contents, and the before-and-after manifest. PTY tests require a real pseudo-terminal. String searches do not count as PTY evidence.

## Criteria

### SA3-001 preserves active runtime shell selection

| Field | Required evidence |
| --- | --- |
| Initial state | A temporary version 1 `config.json` selects Bash, Zsh, an unsupported value, or no value. Separate cases use missing, unreadable, malformed, and newer-version files. |
| Operation | Call `activeShellAdapter` with each combination of `ALIAS_LENS_SHELL` and saved configuration. |
| Expected state | A valid `ALIAS_LENS_SHELL` wins. With no valid integration value, a valid saved shell wins. The baseline falls back to Bash when loading or interpreting the saved shell fails. `SHELL` does not affect runtime selection. No alias, history, or startup file is opened. The filesystem manifest does not change. |
| Failure result | The fallback behavior is a documented baseline limitation, not approval for new fallback paths. A later safety change must use a separate criterion. |
| Automated evidence | `TestActiveShellAdapterSelectionTable` and `TestActiveShellSelectionDoesNotOpenShellFiles` |
| Terminal evidence | None |
| Approval | Sol/high approved on 2026-09-16. |

### SA3-002 preserves setup shell selection

| Field | Required evidence |
| --- | --- |
| Initial state | Use a table that covers explicit `bash`, explicit `zsh`, empty and whitespace-only arguments, valid and invalid `ALIAS_LENS_SHELL`, `SHELL` basenames for Bash, Zsh, and Fish, and each saved configuration state from SA3-001. |
| Operation | Call `requestedShellAdapter` for every row. |
| Expected state | A valid explicit argument wins. Without one, any non-empty `ALIAS_LENS_SHELL` is parsed and an unsupported value returns `unsupported shell`. Without that value, the basename of non-empty `SHELL` is parsed and an unsupported value returns `unsupported shell`. Otherwise SA3-001 decides. The test records the exact returned adapter or exact error substring for every row. No shell file is opened or changed. |
| Failure result | Unreadable or malformed configuration follows SA3-001 only when both environment selectors are empty. |
| Automated evidence | `TestRequestedShellAdapterSelectionTable` and `TestSetupSelectionDoesNotOpenShellFiles` |
| Terminal evidence | In `U24-B52`, run `env -i HOME="$TMP_HOME" PATH=/usr/bin:/bin SHELL=/usr/bin/fish "$ALIAS_LENS_BIN" setup`. Expect standard error to contain `unsupported shell "fish"`, expect a zero process status under the current CLI error policy, and expect no shell file in `$TMP_HOME`. |
| Approval | Sol/high approved on 2026-09-16. |

### SA3-003 establishes the adapter boundary

| Field | Required evidence |
| --- | --- |
| Initial state | Both adapter types compile against the expanded `ShellAdapter` interface. |
| Operation | Review the phase 3 diff against baseline commit `a5d6168`. Run the adapter contract tests. |
| Expected state | The interface owns shell name validation, alias and function parsing, definition rendering, history-line parsing, dated history parsing, integration text, current-shell invocation description, prompt insertion description, key binding description, startup discovery, startup configuration, repair, removal, and status. Bash and Zsh implement every method. Shared TUI, sync, provider, repository, and writer code contains no new `bash` or `zsh` branch. Global Bash-shaped `shellEntryDefinition` and string-based history shell switches no longer decide shell behavior. |
| Failure result | A missing method, a type switch outside adapter selection, or a new shell-name branch outside adapter files fails review. |
| Automated evidence | Compile-time declarations for both adapters, `TestShellAdapterContractBash`, and `TestShellAdapterContractZsh` |
| Terminal evidence | None |
| Approval | Sol/high approved on 2026-09-16. |

### SA3-004 preserves parsing, names, and rendering

| Field | Required evidence |
| --- | --- |
| Initial state | Fixtures include plain aliases, single and double quotes, dollar signs, semicolons, comments, metadata comments, multiline functions, malformed declarations, and sentinel commands. The name matrix includes `ll`, `g.s`, `1x`, `-x`, `_x`, `é`, an empty name, whitespace, and a newline. |
| Operation | Parse, render, and parse again through each adapter. Run `alias-lens shell-entry NAME` against a temporary alias file. |
| Expected state | Phase 3 preserves the baseline writer contract `^[A-Za-z0-9_.-]+$` for aliases and `^[A-Za-z_][A-Za-z0-9_]*$` for functions. Shell-specific validation reports unsafe native names but does not silently change an existing name. Type `function` renders a function. Every other current type renders an alias because that is baseline behavior. Exact rendered bytes match table fixtures. No parse or render operation executes content or creates a sentinel. |
| Failure result | A proposed rejection of a baseline alias name or unknown type is a separate behavior change. Invalid function names return `invalid function name`. Names outside the alias regex return `invalid alias name`. |
| Automated evidence | `TestAdapterNameMatrix`, `TestAdapterAliasParseFixtures`, `TestAdapterFunctionParseFixtures`, `TestAdapterRenderRoundTrip`, `TestUnknownEntryTypeBaseline`, and `TestAdapterParsingDoesNotExecuteContent` |
| Terminal evidence | None |
| Approval | Sol/high approved on 2026-09-16. |

### SA3-005 preserves history behavior

| Field | Required evidence |
| --- | --- |
| Initial state | Bash fixtures contain valid timestamp comments, invalid numeric comments, consecutive timestamps, untimestamped commands, a missing final newline, and a line longer than 1 MiB. Zsh fixtures contain valid extended history, malformed metadata, metadata with no semicolon, and commands that contain semicolons. |
| Operation | Read command counts and dated usage events through each adapter. Repeat with missing and unreadable files. |
| Expected state | Bash timestamp comments apply to the next non-comment command and then reset. Zsh count parsing preserves the baseline rule: any line beginning `: ` with a semicolon loses the prefix through its first semicolon. Dated Zsh parsing sets a time only when the first metadata field parses as Unix seconds. Every fixture declares exact counts and event timestamps. A missing file returns empty results. An unreadable file returns an error and no result. A token over 1 MiB returns the scanner error; tests do not use partial results. History bytes and the manifest do not change. |
| Failure result | Strict Zsh metadata validation is a separate behavior change. |
| Automated evidence | `TestBashHistoryAdapterFixture`, `TestZshHistoryAdapterFixture`, `TestHistoryAdapterMissingFile`, `TestHistoryAdapterReadError`, `TestHistoryAdapterScannerLimit`, and existing stats-period tests |
| Terminal evidence | Set an isolated `HISTFILE`, run `ll` once, flush history, and run `al stats today` in Bash and Zsh. The record names the exact `ll` row, count `1`, history line, and timestamp. |
| Approval | Sol/high approved on 2026-09-16. |

### SA3-006 preserves integration bytes

| Field | Required evidence |
| --- | --- |
| Initial state | Use the two committed golden fixtures and verify their recorded SHA-256 values. |
| Operation | Run `alias-lens shell-init bash` and `alias-lens shell-init zsh`. |
| Expected state | Standard output matches the corresponding fixture byte for byte. Standard error is empty and the exit status is zero. An unsupported shell returns an actionable error. Missing or extra CLI arguments print the baseline usage text. This criterion proves emitted bytes only. |
| Failure result | Any fixture update blocks the refactor and needs separate review. |
| Automated evidence | `TestBashShellIntegrationGolden`, `TestZshShellIntegrationGolden`, and `TestShellIntegrationArgumentErrors` |
| Terminal evidence | Run `sha256sum` on each command output in `U24-B52` and compare it with the table above. |
| Approval | Sol/high approved on 2026-09-16. |

### SA3-007 preserves current-shell execution

| Field | Required evidence |
| --- | --- |
| Initial state | A PTY starts a clean shell with an isolated home and shims for `alias-lens`, autosync, and private usage recording. The alias file defines `ok`, `bad`, and a directory-changing function. |
| Operation | Select each entry through the TUI execution path. Test cancellation, `shell-entry` failure, malformed definition output, success, exit status 37, and a current-directory change. |
| Expected state | The PTY shows exactly one `$ NAME` line before command output. The shell loads the current definition before execution. Success records one native-history entry and one private usage event. `echo $?` prints the command status. The function changes `pwd` in the current shell. Cancellation runs nothing. A definition-load failure does not execute an older definition with the same name. |
| Failure result | Standard error contains the captured baseline error for each failing shim. Alias and startup files keep their hashes and modes. |
| Automated evidence | `TestBashPTYExecution`, `TestZshPTYExecution`, `TestPTYExecutionCancellation`, `TestPTYDefinitionFailureDoesNotRunStaleEntry`, and `TestPTYExecutionPreservesStatusAndDirectory` |
| Terminal evidence | Repeat the success, status 37, cancellation, and directory-change cases in `U24-B52`, `M14-B32-Z59`, and `W11-U24-B52`. Record visible lines, `echo $?`, `pwd`, native history, and the private event count. |
| Approval | Sol/high approved on 2026-09-16. |

### SA3-008 preserves autosync gating during shell execution

| Field | Required evidence |
| --- | --- |
| Initial state | A process shim logs `alias-lens watch --ensure`. Separate temporary configurations set autosync enabled and disabled. |
| Operation | Source the integration in a PTY and run one command. |
| Expected state | The integration invokes `watch --ensure` once in both cases. The command itself exits without starting a watcher when autosync is disabled and starts only the isolated shim when enabled. No production watcher survives the test. |
| Failure result | A watcher call outside the isolated process group fails the test. |
| Automated evidence | `TestBashPTYAutosyncGate` and `TestZshPTYAutosyncGate` |
| Terminal evidence | None |
| Approval | Sol/high approved on 2026-09-16. |

### SA3-009 preserves key bindings

| Field | Required evidence |
| --- | --- |
| Initial state | A PTY starts Bash 3.2, Bash 4+, or Zsh. A preexisting `Ctrl+G` binding writes `USER_BINDING`. Repeat with and without `ALIAS_LENS_NOBIND=1`, and with `alias-lens` removed from `PATH` after startup. |
| Operation | Press `Ctrl+G` on an empty prompt and after typing `keep-me`. |
| Expected state | With Alias Lens binding enabled, an empty Bash 4+ or Zsh prompt opens the picker. Bash 4+ clears a non-empty Readline buffer and does not open the picker. Zsh sends `send-break`, redraws a clean prompt, and does not run the typed buffer. Bash 3.2 keeps Readline cancellation and opens Alias Lens through `al`. Reloading removes only the exact legacy Alias Lens macro. With `ALIAS_LENS_NOBIND=1`, `USER_BINDING` appears and the existing binding remains. If the executable disappears, the shell returns to an interactive prompt with nonzero command status and no file change. |
| Failure result | Running without an interactive line editor emits the baseline shell error and does not edit shell files. |
| Automated evidence | `TestBashPTYBinding`, `TestZshPTYBinding`, `TestBashPTYBindingDisabledPreservesUserBinding`, and `TestZshPTYBindingDisabledPreservesUserBinding` |
| Terminal evidence | Repeat the enabled, non-empty, disabled, and missing-executable cases in every environment that supplies that shell. Record the exact buffer and prompt state. |
| Approval | Sol/high approved on 2026-09-16. |

### SA3-010 preserves paths and startup behavior

| Field | Required evidence |
| --- | --- |
| Initial state | Disposable homes cover missing files, `.bash_profile`, `.bash_login`, `.profile`, `.bashrc`, `.zshrc`, inherited absolute `ZDOTDIR`, relative `ZDOTDIR`, an unwritable `ZDOTDIR` parent, custom `HISTFILE`, and `ALIAS_LENS_HISTORY_FILE`. |
| Operation | Discover alias, history, and startup paths. Start real interactive login and non-login shells where the product claims support. |
| Expected state | Bash uses `.bash_aliases` and `.bash_history`. Zsh uses `.zsh_aliases` and `.zsh_history`. A nonblank `ALIAS_LENS_HISTORY_FILE` overrides the adapter default. Direct path discovery does not read `HISTFILE`. Bash and Zsh integration pass their `HISTFILE` value through `ALIAS_LENS_HISTORY_FILE`; an empty `HISTFILE` uses the adapter default. The refactor preserves the current relative `ZDOTDIR` base, which is the process working directory. Setup creates a missing writable `ZDOTDIR` parent with mode `0700` and writes `.zshrc`. On macOS, Linux, and WSL, Bash uses the first existing file in this order: `.bash_profile`, `.bash_login`, `.profile`; if none exists, it selects `.bash_profile`. `.bashrc` loads aliases in interactive non-login shells. The selected login file loads `.bashrc` in login shells. |
| Failure result | An unwritable `ZDOTDIR` parent returns the baseline write error and creates no `.zshrc`. Discovery itself performs no write. |
| Automated evidence | `TestAdapterPathMatrix`, `TestBashLoginPathPrecedence`, `TestZdotdirPathMatrix`, and `TestHistoryPathOverrides` |
| Terminal evidence | Use disposable homes to start Bash and Zsh in every claimed mode. Record which startup file the shell reads. The WSL record must come from WSL, not a Linux test with `platform=linux`. |
| Approval | Sol/high approved on 2026-09-16. |

### SA3-011 preserves setup, repair, and removal

| Field | Required evidence |
| --- | --- |
| Initial state | Matching baseline and candidate homes contain missing files, user settings, duplicate generated blocks, edited integration lines, custom modes, a configured repository copy, tour state, and each Bash login-file case. Record config bytes, alias and startup manifests, backups, revisions, repository files, and output. |
| Operation | Run setup, repair, and removal twice for Bash and Zsh. Run Bash then Zsh, Zsh then Bash, and removal in both orders. Inject failure after configuration save, alias-file creation, alias-file integration write, and startup-file write. |
| Expected state | Successful output and logical file content match the baseline binary. Repeated setup leaves one integration block and unchanged logical configuration content. It creates no extra backup or revision beyond baseline behavior. The test records that baseline `saveConfig` can replace `config.json` on each run, which changes its inode and modification time. Removal keeps user aliases and unrelated startup text. Setup for the second shell does not change the first shell's alias or startup files, but it preserves the current version 1 behavior of changing `config.shell` and the default `config.alias_file` to the most recently set-up shell. Removal does not rewrite that configuration. Repository restoration, tour scheduling, backup creation, revision creation, and file modes match the baseline. |
| Failure result | Phase 3 records and preserves the current write order: configuration can change before a later alias or startup write fails. Every injected case names the exact changed and unchanged hashes. Transactional setup is separate work and must land before catalog migration. |
| Automated evidence | New tests: `TestSetupGoldenMatrix`, `TestSetupFailureStateMatrix`, `TestSetupRepositoryRestore`, `TestSetupCrossShellSequence`, `TestRepairGoldenMatrix`, and `TestRemovalGoldenMatrix`. Existing named tests: `TestBashLoaderSetupIsIdempotent`, `TestLinuxAndWSLSetupUseBashrc`, `TestMacBashSetupCoversLoginShells`, `TestMacBashSetupPreservesProfileThatLoadsBashrc`, `TestMacBashSetupUsesExistingLoginFilePrecedence`, `TestCommentedStartupReferenceDoesNotBlockSetup`, `TestSetupRemoveKeepsAliasesAndUnrelatedBashSettings`, `TestSetupRemoveRespectsZdotdir`, `TestSetupRepairNormalizesDuplicateGeneratedBlocks`, `TestMacBashSetupNormalizesDuplicateLoginLoaders`, `TestZshSetupPersistsUserBinaryDirectory`, `TestZshStartupSetupIsIdempotent`, `TestZshSetupRespectsZdotdir`, and `TestBashSetupKeepsUserBinaryOnPathAfterRestart` |
| Terminal evidence | Run setup, repair, and removal in disposable homes in `U24-B52`, `M14-B32-Z59`, and `W11-U24-B52`. Record standard output, standard error, exit status, and both manifests. |
| Approval | Sol/high approved on 2026-09-16. |

### SA3-012 keeps pure adapter methods read-only

| Field | Required evidence |
| --- | --- |
| Initial state | An isolated directory tree contains regular files, links, malformed definitions, unreadable history, broken startup references, and sentinel commands. Record the recursive manifest. |
| Operation | Separately call adapter selection, name validation, byte parsing, definition rendering, history-line parsing, dated-history parsing, integration generation, path discovery, syntax validation, and status inspection. Pure methods receive bytes or an explicit discovery context. They do not call high-level `loadAliases` or configuration migration. |
| Expected state | Each operation leaves the recursive manifest unchanged, excluding access time. No sentinel exists. Parse-only shell validation starts the shell without user startup files and executes no definition. An aggregate test repeats the full pure set. |
| Failure result | A method that needs to create or migrate a file is not pure and must return a mutation plan instead. |
| Automated evidence | One `TestPureAdapter...DoesNotWriteOrExecute` test for each named operation and `TestPureAdapterOperationsAggregateManifest` |
| Terminal evidence | None |
| Approval | Sol/high approved on 2026-09-16. |

### SA3-013 preserves command output and sync sources

| Field | Required evidence |
| --- | --- |
| Initial state | Matching baseline and candidate homes cover Bash, Zsh, a custom `alias_file`, a configured repository, autosync state, command help, `doctor`, `stats`, `search`, export JSON, and TUI sizes 60 by 20 and 140 by 45. |
| Operation | Run the same commands through the baseline and candidate binaries. Ask autosync for its watched source and staged repository path. |
| Expected state | Noninteractive output matches after the permitted temporary-root and injected-timestamp normalization. Stable TUI output matches at both widths under the same terminal profile. Configuration bytes, repository target, autosync watched path, and staged path do not change. Bash and Zsh continue to use only the active version 1 `alias_file`. |
| Failure result | Any intended output or sync change moves to a separate reviewed change. |
| Automated evidence | `TestPhase3CommandOutputFixtures`, `TestPhase3TUILayoutFixtures`, `TestPhase3SyncSourceMatrix`, and existing usage, export, configuration, and version tests |
| Terminal evidence | Run `al`, `al doctor`, `al stats`, and `al search` in disposable Bash and Zsh homes at both named widths. Record normalized output and exit status. |
| Approval | Sol/high approved on 2026-09-16. |

### SA3-014 keeps the refactor isolated

| Field | Required evidence |
| --- | --- |
| Initial state | Baseline commit `a5d6168` and the approved criteria commit define the comparison points. |
| Operation | Review `git diff <criteria-commit>...HEAD`, then run `make fmt check`. |
| Expected state | The diff contains no catalog storage, catalog command, migration, generated alias file, configuration version change, dependency change, or support claim for another shell. `go.mod` and `go.sum` match the criteria commit. Every behavior change is rejected or moved to a separate proposal. |
| Failure result | A scope violation blocks merge even when tests pass. |
| Automated evidence | `make fmt check`, `git diff --check`, direct hashes for `go.mod` and `go.sum`, and the complete phase 3 test suite |
| Terminal evidence | None |
| Approval | Sol/high approved on 2026-09-16. |

## Known baseline limits

Phase 3 preserves several behaviors because changing them during the adapter refactor would hide regressions:

- Runtime selection falls back to Bash when configuration loading fails.
- Zsh history parsing accepts any `: ` prefix that contains a semicolon.
- Unknown entry types render as aliases.
- Relative `ZDOTDIR` resolves against the process working directory.
- Setup can save configuration before a later shell-file write fails.
- Version 1 configuration stores one active shell and one legacy sync source.

Each limit needs separate acceptance criteria before a later change. Catalog migration cannot ship until setup writes become transactional and per-shell installed state replaces the single-shell version 1 state.

## Approval record

Sol/high reviewed the first draft on 2026-09-16 and returned `approve after fixes`. That review approved no criterion. This revision addresses all twelve findings and adds immutable integration fixtures.

The second review approved SA3-001 through SA3-005, SA3-007 through SA3-009, SA3-012, and SA3-014. It left SA3-006, SA3-010, SA3-011, and SA3-013 pending. The current revision corrects the fixture bytes, `ZDOTDIR` behavior, repeated-setup metadata expectations, and missing baseline comparison method.

The final Sol/high review approved SA3-001 through SA3-014 and found no unresolved P0 or P1 blockers. Implementation may proceed only within this approved scope.
