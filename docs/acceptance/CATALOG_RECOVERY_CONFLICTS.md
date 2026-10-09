# Catalog recovery conflicts

## Acceptance criteria

- RC-001: Generic recovery preserves unfamiliar target bytes and reports a fixed, actionable failure category and target index without exposing paths, payloads, hashes, or wrapped filesystem errors. Existing rollback identity, metadata, parent, link, and backup checks remain strict.
- RC-002: Application recovery may retain diagnostic drift only for one existing regular private target at the exact primary sync-state path. The target cannot be a removal, directory, promotion, or symlink; no forward target completion or inverse progress may precede retention. Expected and current metadata, parent identity, and the original private backup must pass validation.
- RC-003: Retention strictly decodes bounded current and original sync records. Reject unknown, duplicate, case-variant, null, mistyped, or missing required fields, trailing documents, unsupported statuses, malformed timestamps, and malformed nonempty hashes. Retain only when local and remote hashes and the conflict authority bit are unchanged. Private messages never appear in diagnostics.
- RC-004: Retention preserves current bytes, mode, and inode and retains the original backup. Persist its exact observed identity in a distinct recovery progress stage before finalization. After a crash, revalidate the identity, backup, parent, and semantic predicate, including when terminal recovered progress exists. Never roll a retained target back or silently accept subsequent drift.
- RC-005: Other targets, multiple-target workflows, authority changes, corrupt or absent backups, unsafe metadata, parent replacement, symlinks, and hard links remain blocked without changing target or backup bytes. No force or broad skip mechanism is added.
- RC-006: Repeated attempts blocked before inverse progress do not grow identical trailing recovery-start records. Existing journals containing repeated starts remain readable. Multi-target retry and previously recorded inverse identity checks retain their existing guarantees.
- RC-007: Synthetic filesystem, strict-codec, authority, crash, and replay tests cover these boundaries. Compiled Bash PTYs exercise catalog recover at narrow and wide widths and prove retained, blocked, and repeated outcomes. Tests never read real user state.
- RC-008: High-effort implementation and a fresh independent high-effort review close actionable findings before make fmt check and private recovery. A read-only real-state eligibility check precedes any repair; repair uses the reviewed mutation lock and policy, preserves target bytes and backups, and records only sanitized verification evidence.

## Scope and review gate

This change resolves diagnostic sync-status drift after an interrupted transaction. It does not authorize semantic merging of catalog, native, startup, repository, approval, ownership, or installed-state data. Platform runtime evidence remains a release gate.
