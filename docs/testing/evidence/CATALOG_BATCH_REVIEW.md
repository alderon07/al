# Catalog batch review evidence

## Scope

The [acceptance contract](../../acceptance/CATALOG_BATCH_REVIEW.md) requires one confirmation for a displayed batch of exact native approvals and eligible fallback enrollments, followed by final plan confirmation. Individual review remains available. Approval keys and application freshness checks retain their existing meanings.

## Review and verification

All fixtures use synthetic aliases and private temporary homes. No real user declarations belong in this evidence.

Independent high review found that failing the complete pending collector on one unsupported declaration prevented the terminal catalog view from loading entry status and saved semantic conflicts. A private tolerant inspection path now retains warnings, safe review items, entry status, and conflict recovery. Guided command review remains strict and excludes unsafe items from staged records. A separate high review verified the correction and found no remaining actionable issues.

The first full check also found a compiled bootstrap PTY still waiting for the old individual prompts. Its updated batch interaction passed cancellation, portable installation, and native installation checks.

## Criterion coverage

| Criteria | Evidence |
| --- | --- |
| CB-001, CB-002 | Shared captured batch records and complete display fields; compiled Bash installation with 63 native approvals and 63 fallback enrollments at 52 and 140 columns. Exactly one batch confirmation and one final application confirmation. |
| CB-003 | `TestCatalogBatchStagesCapturedItemsOrCancels`, `TestCatalogBatchOutputFailuresStageNothing`, and `TestCatalogBatchPTYManyEntriesCancelAndIndividual`. EOF, decline, failed or short output, individual selection, and final cancellation preserve the private home manifest. |
| CB-004 | `TestCatalogPendingReviewSkipsExactRecordsAndRejectsDrift` covers existing exact records, rollback provenance, changed ownership proof, canonical catalog changes, and native-file changes. `TestCatalogStatusKeepsEntriesAndConflictsWithUnsafeNative` covers tolerant inspection and strict collection. Existing application freshness and noninteractive installation gates remain in place. |
| CB-005 | `TestCatalogCompleteWrapPreservesAllContent`, `TestCatalogBatchAndPlanRequireCompleteRenderedCoverage`, `TestCatalogBatchEntryResetsStatusScroll`, and `TestCatalogFinalPlanRequiresCompleteDisplay`. Existing terminal drawer PTYs passed at narrow and wide widths. |
| CB-006 | `make fmt check` passed after correction. Compiled bootstrap PTYs passed cancellation and both portable and native installation. Independent compiled Bash checks preserved all 63 fallback definitions, saved the exact approval and ownership counts, verified the read-only generation loader, and used an installed alias in a new shell. |

## Local verification

Recorded on 2026-10-09. Tests ran on the host with private temporary homes because the filesystem sandbox remaps ownership and breaks trusted shell and filesystem checks.

- Full gate: `/tmp/al-batch-review-check-final.log`.
- Compiled 63-entry Bash PTYs: `/tmp/al-batch-review-compiled-pty.log`.
- Review cycle: medium implementation, high review, medium correction, and high review closure.

Native Zsh, macOS, and WSL evidence remains pending when those environments are unavailable.
