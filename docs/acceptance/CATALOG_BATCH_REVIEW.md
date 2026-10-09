# Catalog batch review

## Acceptance criteria

- CB-001: Interactive catalog enable, review, and local or remote init display their pending exact native declarations and eligible fallback ownership records together. One explicit batch confirmation stages these exact records. The existing final save or application confirmation remains required. Review saves approvals; enable and init also review ownership. Focused approve and adopt retain individual prompts.
- CB-002: The batch lists the number of native approvals and fallback enrollments. It displays names, IDs, implementation hashes, exact declarations, fallback source ranges and hashes, exact fallback bytes, and candidate entries. Previously matching approvals and ownership records need no repeated confirmation.
- CB-003: Users can choose individual review before staging the batch. Focused approve and adopt commands remain available. Declining the batch, EOF, interrupt, or failed or incomplete output stages no decisions and saves no records.
- CB-004: Batch confirmation grants no approval to undisplayed entries or unrelated name collisions. Structural validation, source identity, exact ownership matching, and application freshness checks still apply. Noninteractive --apply cannot grant new approvals or ownership.
- CB-005: The terminal interface offers batch and individual review from the same captured records. Full review content wraps within the viewport. Batch approval is unavailable until the end of the review has been reached; resize must not bypass this requirement. Escape cancels staged decisions.
- CB-006: Tests use synthetic catalogs and private temporary homes. Actual Bash PTYs exercise many-entry batch installation, individual selection, cancellation, and final confirmation at narrow and wide widths. Unit tests cover failed output, freshness changes, installed records, and terminal scrolling and resizing. Zsh runtime evidence remains required when its executable is available.

## Review gate

Medium implementation, separate high review, medium corrections, and repeated high review must resolve actionable findings. Run make fmt check and record terminal evidence before reporting completion.
