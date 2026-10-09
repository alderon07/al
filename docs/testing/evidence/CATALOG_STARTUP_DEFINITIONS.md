# Catalog startup placement evidence

## Scope

The [acceptance contract](../../acceptance/CATALOG_STARTUP_DEFINITIONS.md) covers declaration names, source ordering, original coordinates, and explicit placement after other static loads. Automatic inspection stays conservative. Explicit relocation requires a displayed application plan and preserves the original offline rollback bytes.

## Criterion mapping

| Criteria | Evidence |
|---|---|
| SD-001, SD-002, SD-003 | `TestCatalogStartupLiteralDefinitionsAndCoordinates`; compiled retained-native startup PTYs at 52 and 140 columns. |
| SD-004 | Existing startup ambiguity tests; quoted owned-loader and guard regressions; known control masks; `TestCatalogImmediateOpaqueEvalNeedsExplicitEnrollment`, `TestCatalogUnsupportedLateFunctionHeadersRefused` and `TestCatalogUnprovenBraceContextRejectedBeforeEnrollment`. |
| SD-005 | `TestCompiledCatalogRetainedNativePTY` and `TestCompiledCatalogExplicitStartupPTY`, including new-shell dispatch, retained helpers, fallback and byte-exact offline rollback. |
| SD-006 | Complete private read-only application-plan preflight with proposed decisions only in memory; aggregate result and unchanged native/startup/config/state manifest. |
| SD-007, SD-008 | Explicit source relocation and managed-segment tests; compiled opaque-alias sentinels, prior option restoration, verified re-enable and offline rollback. |
| SD-009 | `TestCatalogExplicitScalarOpaqueRoutes` and `TestCatalogScalarTargetsAndInertArguments`; compiled startup fixture and private preflight observe the complete application plan without sourcing private code. |

## Findings and corrections

| Finding | Correction |
|---|---|
| Any matching token in an alias value was treated as a declaration name. | Read the literal declaration header and ignore quoted values, arguments and comments. |
| Earlier native-covered defaults were reported as late overrides. | Bind permission to reviewed enrolled native aliases and compare the final source position. Native function fallbacks receive no such permission. |
| Removing the interactive guard shifted insertion coordinates. | Mask analysis text without removing newline or byte positions. |
| Explicit enrollment still refused ordinary other startup loads. | Relocate one exact proven native block through the displayed plan; leave other loads user-owned and uninspected. |
| Opaque aliases could rewrite native definitions or integration. | Disable alias expansion through standalone escaped builtin statements around the complete native/catalog/integration block; restore its previous state. |
| The first guard assembly restored alias expansion before integration. | Assemble integration before applying the complete guard. |
| Quoted loader markers or source guards could be treated as executable routes. | Require visible top-level context before editing or identifying managed routes. Refresh must use the same context proof. |
| BASHOPTS would require Bash 4.1. | Capture the option through the escaped shopt builtin with normalized success instead. [The Bash 4.1 release notes](https://lists.gnu.org/r/bug-bash/2009-12/msg00139.html) identify BASHOPTS as new in that version. |
| Common escaped-dot tool loads still blocked the complete plan. | Accept narrowly guarded loads with preceding literal scalar hints under explicit enrollment; disclose opaque code and variable effects as trusted. |
| A later opaque eval could override automatic placement. | Require explicit placement for immediate opaque eval; keep deferred function-body code deferred. |
| Visible scalar mutations could retain a stale directory hint. | Invalidate literal assignments, command-prefixed/list assignments, append assignments and mutator targets; ambiguous targets invalidate hints conservatively. |
| Refresh could choose an inert quoted marker even after authenticating the full startup hash. | Share visible top-level managed-marker selection with verified refresh and native-route extraction. |
| Repeated managed blocks and ordinary statements could trigger excess prefix scans or filesystem calls. | Refuse duplicate/capped marker candidates early, cache source bytes and check source verbs before symlink resolution. |
| Mutation scanning counted inert arguments and HOME substrings as variable writes. | Check active command and assignment positions and literal mutator targets; preserve ordinary arguments, unrelated variable names and HOME reads. |
| Inline deferred function bodies were checked as immediate declarations. | Prove the function boundary before inspecting immediate command positions, while still checking its actual declaration name. |
| An unsupported spaced function header could pass as an ordinary command and override a catalog name later. | Refuse recognizable unproven declaration headers instead of widening the retained native grammar. |
| Marker scope counted literal argument braces as function delimiters, and a second-command function declaration bypassed the header refusal. | Refuse unproven unquoted brace contexts before initial enrollment; preserve quoted braces and proven deferred bodies. Command-list declarations receive the same refusal. |

## Review result

Medium implementation, independent high review and repeated medium corrections closed the findings above. The final independent review reported no unresolved actionable findings. Its synthetic reproductions covered quoted and deferred marker context, real and inert variable mutations, opaque eval, unsupported late declarations and bounded scanning. The reviewer read product code and synthetic fixtures only.

## Verification

- Both-shell focused startup tests passed after the final source freeze. Zsh parser coverage used synthetic bytes, not a Zsh runtime.
- Compiled Bash PTYs passed at 52 and 140 columns for guided default and explicit installation, alias-header/body/control sentinels, restored alias options and table, initially disabled options on helper success and failure, missing-generation fallbacks, re-enable without repeated path selection, prior setup upgrade and repeat application, and byte-exact offline rollback. Opaque source bytes also remained unchanged.
- The final complete private application-plan preflight passed with 13 proposed actions and unchanged native/startup/config/state bytes and modes. Proposed approvals and ownership decisions stayed in memory. No private source was executed, written into fixtures, or emitted; no decisions were persisted or applied. Its generic temporary workspace helper was removed afterward.
- The final `make fmt check` passed: formatting, all package tests (including compiled PTYs), vet, product build and whitespace checks. Sanitized output is in `/tmp/al-catalog-startup-check-verified.log`. The app suite completed in 107.695 seconds; the shell suite completed in 1.457 seconds. The three unrelated staged user changes remained intact.

Native Zsh, macOS Bash 3.2 and WSL runtime evidence remain release gates. Synthetic parser coverage does not replace those checks.
