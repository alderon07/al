# Catalog alias dependency inspection

## Acceptance criteria

- AD-001: Dependency inspection distinguishes an alias-eligible command word from flags, ordinary arguments, paths, assignment values, redirection targets, quoted or escaped literals, and comments. A flag containing an entry name must not block installation. No user code is executed.
- AD-002: Unquoted calls to another alias remain blocked when their expansion would depend on native or generated installation order. Pipelines, command lists, assignment prefixes, command substitutions, and surviving native function bodies cannot hide those calls. Unsupported grammar and bounded-input failures produce an actionable refusal rather than an unsafe guess.
- AD-003: Alias replacement self-reference follows the selected shell's proven expansion semantics. A function calling its own name retains ordinary function semantics and must not receive an alias replacement exemption. Trailing-blank alias expansion and shell-specific forms remain conservative unless actual shell evidence proves the supported case.
- AD-004: Generated declarations, retained helpers, fallback refresh, and read-only generation loading share the corrected dependency policy. Portable template control checks remain intact. Strict declaration validation, approvals, exact ownership, immutable generations, and whole-operation failure behavior are preserved.
- AD-005: Both parser paths receive synthetic positive and negative regression cases. Actual compiled Bash PTYs at narrow and wide widths exercise a batch with names repeated in flags and literals, verify installed dispatch in a new shell, preserve native fallback bytes, and restore the offline baseline. Real private source is never copied to fixtures, executed by diagnostics, or approved by tests.

## Review gate

Medium implementation, independent high review, medium corrections, and repeated high review must close actionable findings. Run make fmt check. Track unavailable Zsh and platform runtime evidence explicitly.

## Reference semantics

The [Bash alias manual](https://www.gnu.org/software/bash/manual/html_node/Aliases.html) describes command-word expansion, suppression of recursive replacement, and trailing-blank behavior. The [Zsh alias grammar](https://zsh.sourceforge.io/Doc/Release/Shell-Grammar.html#Aliasing) distinguishes command-position aliases from global aliases and documents quoting and trailing-space behavior. Global and dynamic native alias declarations remain outside the enrolled grammar.
