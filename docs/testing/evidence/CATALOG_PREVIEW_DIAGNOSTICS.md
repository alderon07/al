# Catalog preview diagnostics evidence

Verified on October 8, 2026, against the reviewed working tree on `dev`.

## Behavior and coverage

| Criterion | Evidence |
| --- | --- |
| Specific reasons, source coordinates, function scanner stop lines and grouped-range explanations | `TestPreviewDiagnosticReasonsAndShadowContract`, including synthetic Bash and Zsh cases |
| Quoted literals, comments and heredocs retain their existing interpretation | `TestPreviewRespectsQuotedAndCommentBoundaries` |
| Read-only inspection, no source execution, and equivalent-only partial import | `TestCatalogPreviewDetailsReadOnlyAndPartialImport`, with unchanged recursive file manifests and native bytes |
| Source text and credential-shaped names stay private in plain and JSON output, including omitted diagnostics | `TestPreviewDiagnosticsContainNoSourceText`, `TestCatalogPreviewDiagnosticBudgetAndPrivacy`, `TestCatalogPreviewHidesSecretNamesBeforeDiagnosticTruncation` |
| Diagnostic budget, global notices, output failures and short writes | `TestCatalogPreviewDiagnosticBudgetAndPrivacy`, `TestCatalogPreviewDisplaysGlobalDiagnostics`, `TestCatalogPreviewDetectsOutputFailures` |
| Safe entry names remain visible with coordinates and escaped terminal controls | `TestCatalogPreviewPreservesAcceptedNamesAndCoordinates` |
| Reserved markers cannot borrow a reason from another source range | `TestPreviewReservedMarkerDoesNotInspectFollowingRange` |
| Existing shadow output, parser acceptance, ranges and stable identity remain unchanged | Existing shadow plain/JSON golden tests and the preview parser-contract comparisons |

Preview enrichment runs before the existing 100-diagnostic limit. Secret-blocked names are cleared before truncation. Preview output retains the 16 MiB limit and uses one checked write, including its final guidance.

## Review resolutions

A medium implementer and separate high reviewer repeated implementation, review and correction until the high review had no remaining findings.

| Finding | Correction |
| --- | --- |
| A reserved marker could use a diagnostic from the following range | Enforce range boundaries and give the marker an explicit fixed reason |
| Unsupported function headers and names were labeled as shell configuration | Add preview-only declaration-header explanations without changing parser acceptance |
| Comments ending in parentheses were mistaken for function headers | Respect comment, quote and escape boundaries and require one name token |
| Empty array assignments were mistaken for function names | Exclude assignment-shaped candidates from declaration classification |
| A credential-shaped alias name could leak from a secret-blocked result | Clear names on secret-blocked preview results before diagnostic truncation; verify plain and JSON output |

The secret-name reproduction passed independently after correction. Both-shell regression tests cover 107 blocked ranges, including credential-shaped names and secrets in attached comments and command bodies.

## Verification

The final reviewed code passed `make fmt check`, including all Go packages, vet, build and whitespace checks. It used an owned disposable `TMPDIR`, a writable temporary Go cache, and the host filesystem. Initial sandbox runs failed unrelated filesystem trust tests because the sandbox exposes synthetic ownership metadata. The same tests passed on the host; no trust policy was weakened.

```bash
task_tmp=$(mktemp -d /tmp/al-preview-verified.XXXXXX)
TMPDIR="$task_tmp" GOCACHE=/tmp/al-preview-go-cache GOPROXY=off make fmt check
GOCACHE=/tmp/al-preview-go-cache GOPROXY=off go build -o /tmp/al-preview-pty ./cmd/alias-lens
python3 /tmp/al-catalog-preview-pty.py
```

The final compiled binary passed real PTY checks at widths 40 and 140. Each run used a synthetic alias file in a disposable home and returned the expected status 1 with one equivalent and two unsupported ranges, no blocked entries, and no validator failure. The safe name and coordinates, quoting reason, function rejection line, grouped-range explanation and next steps were visible. Source command text and terminal controls were absent. The home file manifest stayed unchanged.

Local verification artifacts:

- `/tmp/al-catalog-preview-check-verified.log`
- `/tmp/al-catalog-preview-pty.py`
- `/tmp/al-catalog-preview-pty-verified.log`

Bash runtime verification passed. Zsh parser and application fixtures passed; Zsh was unavailable for a runtime PTY check. This evidence does not complete the outstanding platform release gates. Unrelated staged changes were preserved.
