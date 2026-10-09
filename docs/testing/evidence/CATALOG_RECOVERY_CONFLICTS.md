# Catalog recovery conflict evidence

## Initial observation

Read-only inspection found one interrupted version-2 workflow with one regular private sync-status target. The original backup matched its recorded digest. Current target metadata and parent identity matched; only diagnostic fields differed, while local and remote sync hashes were unchanged. Repeated attempts had appended recovery-start records without inverse progress. No private source, values, paths, or record identities are reproduced here.

## Review

A fresh high-effort reviewer approved the narrow retention design subject to strict authority, metadata, backup, replay, and terminal cleanup checks. A separate high-effort implementer delivered the fix; repeated fresh review closed four findings:

| Finding | Resolution |
| --- | --- |
| Filesystem and journal-write errors could expose private paths. | Recovery failures now use fixed categories and target indexes, without printing wrapped filesystem errors. |
| Retention backup checks could unnecessarily block ordinary baseline recovery. | Baseline and planned-content recovery take the existing strict rollback path before retention proof. |
| A replacement between retained replay snapshots could become a new accepted identity. | Every replay snapshot must match the original durable retained identity, including terminal cleanup. |
| Go accepts malformed timestamp spellings. | The retention codec requires canonical timestamp encoding and rejects malformed hour, fraction, and offset examples. |

The final fresh review had no actionable findings. Independent temporary synthetic checks also covered invalid retained proof, mixed inverse progress, unsafe mode, and private-error redaction. Those checks used disposable data and made no repository or real-state changes.

## Coverage

| Criteria | Evidence |
| --- | --- |
| RC-001, RC-005 | TestWorkflowRetentionRefusesUnprovenChanges, TestWorkflowRetentionRefusesMultipleTargets, TestWorkflowRetentionRefusesReplacedParent, independent redaction checks |
| RC-002, RC-003 | TestRecoverySyncStateStrictCodec, TestSyncDiagnosticRecoveryConflictAuthority, TestSyncDiagnosticRecoveryRefusesAuthorityAndOtherTargets |
| RC-004 | TestWorkflowRetainedIdentityReplayAndTerminalCleanup, TestWorkflowRetainedProcessDeath, TestSyncDiagnosticRecoveryPreservesCurrentAndOriginalBackup |
| RC-005 ordinary rollback | TestWorkflowPolicyPreservesOrdinaryBaselineWithoutBackup, TestSyncDiagnosticPolicyPreservesOrdinaryBaselineWithoutBackup, existing ordered and repeated recovery tests |
| RC-006 | TestWorkflowBlockedRetryPreservesRepeatedStarts, blocked application and compiled PTY retries |
| RC-007 | TestCompiledCatalogRecoveryPTY: compiled CLI in Bash PTYs at 52 and 140 columns, retained and authority-change cases, repeated calls, unchanged target identity and backup, no private output |
| RC-008 | Fresh high review, final full gate, exact read-only preflight, locked repair, private before-and-after verification, installation byte comparison |

## Verification

- Focused transaction, application, strict-codec, crash, and compiled PTY checks passed.
- Final make fmt check passed with disposable homes and real host ownership. The final run included application tests in 106.409 seconds, transaction tests in 8.800 seconds, and CLI tests in 22.941 seconds, followed by vet, build, and diff checks.
- The first full run hit an existing TUI PTY assertion that expected adjacent search letters in a single output fragment. Incremental terminal redraws split the letters. The final full rerun passed that test and the complete suite. No unrelated TUI code was changed.
- A reviewed temporary read-only helper reused the exact journal, filesystem, backup, identity, and application-policy checks. It also confirmed no other pending legacy journals or catalog staging work. Its repository sources were removed before execution; it reported only eligibility.
- Actual recovery used the reviewed CLI and shared mutation lock. Verification confirmed successful journal cleanup and unchanged status-file identity, original backup, aliases, startup files, and all other observed private configuration and state files. No private content or identity was copied into evidence.
- make install completed and compared the installed executable against the tested build. Existing Alias Lens processes need reopening to use it.
- The three pre-existing staged files remain untouched. No commit or push was made.

## Compatibility and remaining evidence

Existing version-2 journals remain readable. The new retained-progress stage is private; an older executable rejects it safely if interrupted recovery leaves it on disk. Recovery never treats retained bytes as a restored baseline. Public catalog, status, and plan formats are unchanged.

This verification establishes Linux Bash behavior. Zsh, macOS, and WSL release evidence remains separately tracked.
