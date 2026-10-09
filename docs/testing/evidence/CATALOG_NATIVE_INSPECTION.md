# Retained native inspection evidence

## Scope

The [acceptance contract](../../acceptance/CATALOG_NATIVE_INSPECTION.md) separates catalog import and approval from structural inspection of retained native functions. Ordinary command substitution and parameter expansion in unrelated helpers must not prevent installing reviewed aliases. Inspection must execute no user code.

## Review and verification

The medium implementation introduced a separate retained-native inventory and kept the existing importer and approval grammar. Independent high review and medium corrections resolved these defects:

| Finding | Resolution |
|---|---|
| A backslash-newline could form a heredoc operator and make heredoc content appear to be a native declaration or legacy loader. | Continuations that join tokens are rejected. Only immediate horizontal whitespace on both sides permits an unquoted continuation, preserving lexical state and original offsets. Hidden loader bytes remain untouched. |
| An unfinished control block or malformed command list could pass structural inspection. | Compound control syntax, reserved function headers, incomplete lists, and dangling redirections are rejected without executing a validator in the startup helper. Operator delimiters participate in reserved-token recognition. |
| Every matching loader candidate caused another prefix scan. | Candidate count is capped at 32 before prefix proofs. Independent synthetic checks with 100, 1,000, and 3,000 candidates return the fixed bounded refusal. |

The full suite also caught a changed error message for unsupported declarations that could mask handoff controls. The fixed source-free diagnostic preserves the existing control-mask wording and refusal behavior; the existing compiled control-mask tests pass.

Repeated independent high review found no remaining actionable findings. Opaque helpers retain no importable entry and cannot acquire approval or ownership. Collision and dependency checks include their names and bodies. The read-only startup path invokes no shell validator.

## Criterion mapping

| Criterion | Evidence |
|---|---|
| NI-001 | `TestCatalogNativeInventoryRetainsOpaqueFunctions`, `TestCatalogNativeInventoryQuotedBoundariesAndNestedSubstitution`, and `TestCatalogSeparatedContinuations` cover ordinary substitutions, parameters, and separated multiline arguments for both parsers. |
| NI-002 | `TestCatalogOpaqueNativeFunctionCannotAcquireOwnership` and `TestCatalogRetainedFunctionDependencies` cover refusal to enroll opaque helpers and ambiguous alias dependencies. |
| NI-003 / NI-003a | `TestCatalogNativeInventoryFailsClosed`, nesting checks, and `TestCatalogIntegrationCandidateBound` cover malformed input, fake boundaries, control masking, joining continuations, redirection targets, reserved tokens, and bounded work. |
| NI-004 / NI-004a | `TestCatalogLiteralShellInitIntegrationBoundaries` and caller review cover selected-shell literal routes, duplicates, hidden invocations, source offsets, exact ownership, and unchanged strict enrollment grammar. |
| NI-005 | `TestCompiledCatalogRetainedNativePTY` runs the compiled guided flow in actual Bash terminals at 52 and 140 columns. It installs alongside retained helpers with separated continuations, preserves trailing aliases, executes synthetic installed and retained definitions in a new shell, and leaves the inspection sentinel absent. |
| Offline rollback | The compiled PTY test makes the enrolled repository unavailable, then restores the exact original native and startup bytes, including the legacy loader. |

Focused parser and application tests pass. The existing `TestCatalogSurvivingControlMasksBlockBeforeWritesStartupPTY` passes its Bash cases. A private read-only preflight accepts the affected native file and verifies unchanged bytes; no source was executed, copied into fixtures, or approved by that check.

Final `make fmt check` passes on the host: all packages, vet, build, and whitespace checks. Verification used a disposable temporary directory and the existing Go cache with network module fetching disabled. The compiled Bash PTY test also passes independently with `AL_REQUIRE_PTY_SHELLS=1` at both widths.

Unavailable Zsh, macOS, and WSL runtime evidence remains a release gate.
