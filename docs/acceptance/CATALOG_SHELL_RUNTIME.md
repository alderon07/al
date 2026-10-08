# Catalog shell runtime acceptance

Approved implementation boundary for CL-002 and CL-003. Runtime validation accepts a deliberately bounded grammar and rejects ambiguous input before running a parse-only validator.

## Structural declarations

A native function implementation must form exactly one complete function declaration when wrapped with its catalog name. The shadow parser must prove a single whole source range with the exact same name, kind, and body. No closing-brace escape, trailing declaration, top-level command, declaration redirection, or unsupported lexical construct is accepted. An alias implementation renders exactly one literal alias declaration. Alias Lens control names, shell reserved words, and shell primitives cannot become catalog declarations.

Surviving native aliases that mask loader or handoff control words, and native functions named builtin or command, block installation before writes. The diagnostic names only the unsafe definition and gives migration guidance. Intentional entry-name aliases, the al shell integration, and current-shell readonly helper conflicts retain their existing guarded handling. Portable templates are checked for dependencies on their unquoted control tokens. Unsupported native units containing alias definitions, control dispatch, or dynamic expansion refuse installation when their effects on those controls cannot be proven. Combined declarations, multiple assignments, and backslash continuations do not bypass that refusal.

Every catalog declaration and surviving native declaration is checked for ambiguous alias expansion dependencies. A reference to a native alias from a function or another alias blocks automatic replacement. Validation never executes user code. The isolated Bash or Zsh process uses parse-only mode, a disposable home, no startup files, bounded output, and a deadline.

## Startup routes

A startup file may contain one static standalone source statement for the selected native file, or the Ubuntu static file-existence guard containing that exact statement. Exact Alias Lens legacy blocks are owned and can be replaced. Duplicate sources, dynamic sources, sources hidden in unsupported control structures, later definitions that override catalog names, early returns, and ambiguous insertion points block installation and report the need for explicit placement.

Bash installation respects the existing .bash_profile, .bash_login, .profile precedence and proves the interactive and login routes without creating a higher-priority file. Zsh uses inherited ZDOTDIR or a proven static ZDOTDIR assignment in .zshenv. Dynamic discovery requires explicit enrollment through the shell adapter.

## Runtime fallback and handoff

Initial adoption retains exact native source bytes. Later enablement updates or removes only the exact enrolled ranges, and preserves the original rollback bytes. Rename, deletion, and intentional condition exclusion update those ranges. Unexpected source changes require fresh review. If an installed replacement becomes pending or unavailable, enablement blocks the complete replacement unless that entry was explicitly deleted or excluded by a condition.

Startup loads the native file once, then captures complete verified helper output before evaluating it. The helper verifies a strict active pointer, generation package, exact included IDs, structural declarations, and expected native-input hash using only immutable artifacts. Missing, corrupt, or drifted artifacts produce no runnable output. Mutable catalog, config, recovery, validators, writes, and network are absent from this helper path.

The shell checks actual readonly alias/function state before handoff. An existing alias is removed only after verification and only for an included declaration. Native fallbacks remain callable when helper verification fails. Portable declarations invoke a pinned absolute external executable.

## Required evidence

Temporary-home tests cover injected top-level commands, redirections, protected names, alias dependencies, duplicate and dynamic source statements, Bash login precedence, inherited/static/dynamic ZDOTDIR, native-input drift, corrupt pointer/package, pending installed replacement, retained fallback edits and rollback. PTY tests start new Bash and Zsh shells at startup and transaction boundaries and verify fallbacks plus pinned executable dispatch. No test reads real user shell files.

## Offline rollback baseline

Rollback records include the original native-file existence and each original startup-file existence. Empty original files are restored as empty files. Originally absent files are removed, so a new .bash_profile cannot mask a later .profile. A missing or corrupt active pointer does not prevent offline rollback. The installed record and byte-exact private baseline authorize restoration, and any currently present pointer is pinned as a transaction target before removal.

Immutable generation IDs cannot be reused to repair changed artifact bytes. An existing generation or snapshot with mismatched bytes blocks activation. Runtime decline emits a sanitized next command while leaving native fallbacks callable.

## Reviewed symlink enrollment

- A user-owned native or startup leaf symlink can be enrolled when its parent and referent pass the shared user-file observation policy. The review binds the link target, leaf identity, referent identity, and bytes. Applying replaces the referent and preserves the leaf symlink.
- Generations record the resolved native referent. Runtime verification reads that pinned path, while startup proof accepts the original static source only when it resolves to that same referent.
- Retargeting a reviewed link, changing its referent, adding hard links, or replacing a parent after preview refuses application. Recovery cannot overwrite a changed link or parent.

## Existing setup integration upgrade

Catalog activation recognizes the adapter's exact current and pre-upgrade integration bytes only after a proven top-level prefix. Modified, nested, or duplicate integration blocks are refused. The reviewed native target removes the exact owned integration, and startup installs one guarded integration with a pinned executable. Adoption ranges are adjusted only by that exact removal. The original native and startup bytes remain the offline rollback baseline. Fresh Bash and Zsh PTYs verify an existing setup upgrades without eager `unalias al`, dispatches through the installed executable, and rolls back to the original bytes.

## Lifecycle plan explanations

Activation plans describe retained, refreshed, renamed, deleted, and condition-disabled enrolled fallback names, and any exact owned integration relocation. Each private-state action states its role. Rollback plans and final confirmation explicitly restore the enrollment baseline and warn that aliases renamed or deleted after enrollment can return. Focused plan-text assertions cover these effects.

Executable resolution checks whether the current user can execute each explicit absolute candidate from the captured PATH. An inaccessible new program is omitted, and an inaccessible replacement of an installed entry blocks activation. Inspection never runs the candidate.
