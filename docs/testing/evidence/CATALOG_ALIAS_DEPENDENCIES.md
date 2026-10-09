# Catalog alias dependency evidence

## Scope

The [acceptance contract](../../acceptance/CATALOG_ALIAS_DEPENDENCIES.md) replaces substring matching with bounded shell-word inspection. A matching flag or literal must not become a dependency. Actual unquoted alias calls remain guarded.

## Review and verification

Medium implementation replaced the regular-expression token scan with a bounded command-word lexer. The existing native boundary scanner first proves supported syntax. The lexer then checks whole unquoted command-position words and recursively inspects command substitutions, including substitutions inside arguments, assignments, parameter operands, and redirection targets.

Independent high review and its separate synthetic reproduction matrix found no remaining actionable findings. The review verified these constraints:

- Assignment and redirection prefixes preserve the next command position. Pipes and command lists start a new command position.
- Quoted or escaped words and ordinary arguments cannot create substring dependencies.
- Bash's initial alias-replacement self exemption is cleared for nested substitutions and later commands. Functions never receive the exemption. Zsh self replacements remain blocked pending runtime evidence.
- Trailing-blank aliases are rejected because they change the eligibility of a following word.
- Retained alias replacement bodies and helper bodies receive the same checks as generated entries.
- Planning and read-only immutable generation loading call the same pure policy. Startup verification launches no syntax-validator subprocess.

## Criterion mapping

| Criterion | Evidence |
|---|---|
| AD-001 | `TestCatalogAliasDependencyWords` covers flags, arguments, paths, quote concatenation, escapes, comments, assignments, and redirection targets. |
| AD-002 | The same test covers true calls after prefixes, pipes, lists, and inside nested substitutions. `TestCatalogAliasDependencyBounds` covers unsupported grammar and nesting limits. |
| AD-003 | Both parser paths test function self-call refusal and nested/later self-call refusal. Compiled Bash PTYs prove initial self replacement against a synthetic external executable. Zsh keeps conservative refusal. |
| AD-004 | `TestCatalogImmutableLoaderChecksAliasDependenciesWithoutValidator` verifies safe literal acceptance, real dependency refusal, and zero syntax-validator calls. Existing portable-template, ownership, and retained-helper tests remain in the focused gate. |
| AD-005 | `TestCompiledCatalogRetainedNativePTY` runs the compiled guided flow at 52 and 140 columns. It installs three synthetic aliases with repeated names in flags/literals, retains multiline helpers and trailing aliases, verifies fresh-shell dispatch, and restores exact native/startup bytes with the repository unavailable. The inspection sentinel stays absent. |

Focused tests and actual compiled Bash PTYs pass. A ten-second run of `FuzzCatalogAliasDependencyInspection` completed 24,055 executions without failure using one worker and synthetic inputs. A private read-only preflight checked the complete affected native entry set against the shared policy, verified unchanged bytes, executed no native code, and saved no approvals. No private source entered fixtures or evidence.

Final make fmt check passes on the host: all packages, vet, build, and whitespace checks. Verification used a disposable temporary directory, the existing Go cache, and disabled network module fetching. Zsh, macOS, and WSL runtime evidence remains explicitly tracked as a release gate.
