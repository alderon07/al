# Phase 2 catalog-model acceptance criteria

Status: approved on 2026-09-17. Phase 2 implementation may proceed within this scope.

## Scope

Phase 2 defines a pure, versioned Go model for a shell-neutral alias catalog. It covers decoding, structural validation, semantic validation, normalization, deterministic encoding, and comparison. It does not read or write files, inspect the environment, invoke a shell, generate IDs, render shell text, migrate aliases, activate catalog mode, or sync data.

The implementation belongs in `internal/catalog`. Its public operations accept bytes or typed values and return typed values, bytes, or diagnostics. Phase 2 validates an existing ID. A later command may generate an ID through explicitly injected entropy.

## Entry gate

Implementation cannot start until an independent reviewer approves every criterion in this document. The approval commit is the comparison base for the phase 2 diff.

## Version 1 schema

The root object contains exactly these fields:

| Field | Type | Rule |
| --- | --- | --- |
| `schema_version` | integer | Required and exactly `1`. |
| `entries` | array | Required. At most 10,000 entries. |

An entry contains only these fields, in this semantic order:

| Field | Type | Rule |
| --- | --- | --- |
| `id` | string | Required. Exactly 32 lowercase hexadecimal characters. |
| `name` | string | Required. Command names match `^[A-Za-z_][A-Za-z0-9_.-]*$`; function names match `^[A-Za-z_][A-Za-z0-9_]*$`. |
| `kind` | string | Required. `command` or `function`. |
| `description` | string | Optional. UTF-8, no NUL, at most 1,024 bytes. |
| `category` | string | Optional. UTF-8, no NUL or newline, at most 64 bytes. |
| `tags` | array of strings | Optional. At most 64 input values; each is nonempty UTF-8 without NUL or newline and at most 64 bytes. |
| `platforms` | array of strings | Optional. At most four input values from `linux`, `macos`, `wsl`, and `windows`. |
| `favorite` | boolean | Optional. |
| `portable` | object | Optional and valid only for `command`. |
| `native` | object keyed by shell | Optional. Only `bash` and `zsh` are accepted in phase 2. |

An entry must have `portable` or at least one `native` implementation. Explicit `null` is invalid at every level. Unknown fields are invalid.

`portable` contains exactly:

| Field | Type | Rule |
| --- | --- | --- |
| `program` | string | Required. Matches `^[A-Za-z0-9_][A-Za-z0-9_.+-]*$`, is at most 255 bytes, and is not denied below. |
| `args` | array of strings | Required, including when empty. At most 256 input values; each is valid UTF-8 without NUL and at most 65,536 bytes. |
| `pass_arguments` | boolean | Required. |

The program grammar excludes whitespace, `/`, `\`, `=`, `$`, backticks, glob characters, and a leading dash. The version 1 denylist is case-sensitive and contains:

```text
alias bg break builtin cd command compdef continue declare dirs disown enable
eval exec exit export false fc fg functions getopts hash jobs kill let local
logout popd pushd read readonly return set shift source suspend times trap true
typeset ulimit umask unalias unset wait whence where which . :
```

The denylist is a usability filter, not a complete shell-safety boundary. Phase 2 validates only the token and this versioned list. Before a portable entry can render or run, each adapter must resolve `program` to an external executable without shell lookup and reject any alias, function, builtin, reserved word, hashed command, or executable path supplied by catalog data. A catalog can therefore be structurally valid but unavailable on one target shell.

Each `native` value is an object with exactly one field selected by entry kind:

- A `command` uses `alias_value`, a UTF-8 string without NUL and at most 65,536 bytes.
- A `function` uses `function_body`, a UTF-8 string without NUL and at most 262,144 bytes.

The value is structure, not a generic `source` field. The adapter supplies the entry name and declaration syntax. Phase 2 rejects an empty native value, a generic `source` field, and a field that does not match the entry kind. It does not inspect shell grammar inside an alias value or function body. Phase 4 adapter import and rendering own rejection of unsafe declarations, redirections, expansions, and top-level commands.

Duplicate IDs and duplicate names are invalid. Name comparison is exact and case-sensitive.

## Normalization and canonical encoding

Validation observes the original input before normalization. Count limits apply before duplicate removal. After successful validation:

- Entries sort bytewise by `name`, then by `id`.
- Tags deduplicate by exact value and sort bytewise, case-sensitive.
- Platforms deduplicate and sort in this fixed order: `linux`, `macos`, `wsl`, `windows`.
- An absent optional slice or map remains absent. An explicitly empty `tags`, `platforms`, or `native` value normalizes to absent, except that an empty `native` still fails the implementation-presence rule when no portable implementation exists.
- Empty optional strings and `favorite: false` normalize to absent.
- Required empty arrays and booleans inside `portable` remain present.

Canonical JSON uses UTF-8, two-space indentation, no HTML escaping, and exactly one final line feed. It emits no trailing spaces. Object fields use this order:

1. Root: `schema_version`, `entries`.
2. Entry: `id`, `name`, `kind`, `description`, `category`, `tags`, `platforms`, `favorite`, `portable`, `native`.
3. Portable: `program`, `args`, `pass_arguments`.
4. Native shell keys: bytewise lexical order.
5. Native implementation: `alias_value` or `function_body`.

Optional zero fields are omitted. The encoder rejects a typed value that cannot satisfy the schema instead of emitting invalid JSON.

Raw input larger than 8 MiB is rejected before JSON parsing. Canonical output larger than 8 MiB is rejected before bytes are returned.

## Diagnostics

A diagnostic contains `code`, `entry_index` when applicable, `field`, and a redacted message. It never contains a portable argument, alias value, function body, or secret-like field value.

Diagnostics sort by original entry index, then this field order: root, schema version, entries, id, name, kind, description, category, tags, platforms, favorite, portable, native. Within one field they sort by diagnostic code. Root diagnostics precede entry diagnostics. At most 100 ordinary diagnostics are returned. When more exist, the final record is `diagnostics_truncated` with the exact omitted count.

## Criteria

### SA2-001 is pure and side-effect free

| Field | Required evidence |
| --- | --- |
| Operation | Decode, validate, normalize, encode, and compare fixtures repeatedly and concurrently. |
| Expected state | No filesystem, environment, process, clock, random, network, global-state, or logging access occurs. Equal inputs return equal results. |
| Enforcement | Package imports are limited to `bytes`, `encoding/json`, `errors`, `fmt`, `io`, `regexp`, `sort`, `strings`, and `unicode/utf8`. `crypto/sha256` and `encoding/binary` are allowed only if comparison fixtures require a pure hash. Imports of `os`, `os/exec`, `net`, `syscall`, `unsafe`, Cgo, and external modules fail review. No side-effecting package initializer is allowed. |
| Automated evidence | `TestCatalogOperationsAreDeterministic`, concurrent race tests, and `TestCatalogImportAllowlist` |
| Approval | Sol/high approved on 2026-09-17. |

### SA2-002 rejects invalid document structure

| Field | Required evidence |
| --- | --- |
| Fixtures | Missing fields, `null`, unknown fields, duplicate JSON keys, trailing JSON values, malformed UTF-8, unsupported versions, wrong types, and depth or size boundary cases. |
| Expected state | Decoding fails closed and returns ordered, redacted diagnostics. It returns no partially valid catalog. Version `0`, a missing version, and a future version never trigger migration or rewriting. |
| Automated evidence | `TestDecodeCatalogStructureMatrix`, `TestDecodeCatalogRejectsDuplicateKeys`, and fuzz tests |
| Approval | Sol/high approved on 2026-09-17. |

### SA2-003 enforces cross-field entry rules

| Field | Required evidence |
| --- | --- |
| Fixtures | Both kinds, both name grammars, portable-only commands, native-only commands and functions, mixed implementations, empty implementations, a generic `source` field, wrong native field for kind, duplicate names, and duplicate IDs. |
| Expected state | Only schema-valid combinations pass. Portable functions, generic native source fields, and kind-field mismatches fail. Phase 2 does not attempt to identify declarations or nested shell syntax inside implementation strings. Duplicate comparisons are exact and case-sensitive. |
| Automated evidence | `TestValidateEntryCrossFieldMatrix` and `TestValidateEntryNameGrammar` |
| Approval | Sol/high approved on 2026-09-17. |

### SA2-004 constrains portable execution data

| Field | Required evidence |
| --- | --- |
| Fixtures | Every program grammar boundary, every denylisted word, safe lookalikes, empty arguments, Unicode arguments, literal shell metacharacters in arguments, and all size limits. |
| Expected state | The program is a bare token accepted by the documented grammar and denylist. Arguments remain inert data and round trip byte-for-byte. No phase 2 operation interprets shell syntax or claims that the program resolves externally. Adapter criteria must prove external-executable-only resolution before rendering or execution. |
| Automated evidence | `TestPortableProgramGrammar`, `TestPortableProgramDenylist`, and `TestPortableArgumentsRemainLiteral` |
| Approval | Sol/high approved on 2026-09-17. |

### SA2-005 constrains native implementation shape

| Field | Required evidence |
| --- | --- |
| Fixtures | Bash and Zsh objects with the correct and incorrect field for each kind, unknown shell keys, empty values, NUL, malformed UTF-8, and limits. |
| Expected state | The model stores only `alias_value` or `function_body`. It cannot store a complete declaration under a generic source field. It makes no safety or syntax claim beyond structural validation. |
| Automated evidence | `TestNativeImplementationShapeMatrix` |
| Approval | Sol/high approved on 2026-09-17. |

### SA2-006 normalizes without losing meaning

| Field | Required evidence |
| --- | --- |
| Operation | Normalize permutations, repeated metadata, optional empty values, and already normalized catalogs. |
| Expected state | Normalization follows the rules above, is idempotent, and does not change IDs, names, implementation data, argument order, or entry meaning. |
| Automated evidence | `TestNormalizeCatalogGolden`, `TestNormalizeCatalogIdempotent`, and property tests over permutations |
| Approval | Sol/high approved on 2026-09-17. |

### SA2-007 emits one canonical byte representation

| Field | Required evidence |
| --- | --- |
| Fixtures | A committed `testdata/catalog-v1` corpus covers every field, escaping, Unicode, empty required values, map order, list order, and final-newline behavior. |
| Expected state | Every semantically equivalent permutation produces the same golden bytes. Field order, indentation, escaping, omission, and final line feed follow this document exactly. Decode, normalize, encode, and decode is stable. |
| Automated evidence | `TestCanonicalCatalogGolden`, `TestCanonicalCatalogPermutations`, and `TestCanonicalCatalogRoundTrip` |
| Approval | Sol/high approved on 2026-09-17. |

### SA2-008 returns bounded deterministic diagnostics

| Field | Required evidence |
| --- | --- |
| Operation | Validate documents with reordered failures and more than 100 failures. |
| Expected state | Ordering follows the diagnostics section. Output contains at most 100 ordinary records plus one truncation record with the exact omitted count. Messages contain no implementation body or argument value. |
| Automated evidence | `TestCatalogDiagnosticOrder`, `TestCatalogDiagnosticLimit`, and canary redaction tests |
| Approval | Sol/high approved on 2026-09-17. |

### SA2-009 enforces resource limits at the boundary

| Field | Required evidence |
| --- | --- |
| Operation | Test every count and byte limit at the limit and one unit over. Feed oversized, deeply nested, and adversarial JSON under a bounded test timeout. |
| Expected state | Raw input is checked before parsing. Encoded output is checked before return. The decoder does not allocate from an attacker-controlled declared size and does not return partial state. |
| Automated evidence | `TestCatalogLimitBoundaries`, fuzz tests, and bounded benchmark fixtures |
| Approval | Sol/high approved on 2026-09-17. |

### SA2-010 compares catalogs by stable identity

| Field | Required evidence |
| --- | --- |
| Fixtures | Renames with stable IDs, same names with different IDs, field-by-field changes, order-only changes, invalid duplicate identities, and native changes. |
| Expected state | Comparison pairs entries by ID, reports changed fields deterministically, treats a rename as a field change, and refuses invalid inputs. It never pairs by array position. |
| Automated evidence | `TestCompareCatalogIdentityMatrix` and `TestCompareCatalogFieldMatrix` |
| Approval | Sol/high approved on 2026-09-17. |

### SA2-011 keeps implementation scope narrow

| Field | Required evidence |
| --- | --- |
| Operation | Review `git diff <criteria-commit>...HEAD`, package imports, `go.mod`, and `go.sum`; run `make fmt check`. |
| Expected state | The diff adds the pure model, tests, golden data, and the minimal Makefile change needed to format Go recursively. It does not add file persistence, ID generation, shell adapters, rendering, execution, migration, activation, sync, or dependencies. `make fmt` and `make fmt-check` cover every Go file beneath `cmd` and `internal`. |
| Automated evidence | Full phase 2 suite, `make fmt check`, `git diff --check`, and dependency-file hashes |
| Approval | Sol/high approved on 2026-09-17. |

## Approval record

Sol/high completed three read-only reviews. The first review rejected the draft because the schema, native structure, canonical encoding, program grammar, purity boundary, and resource limits were incomplete. The second review required the model and adapter responsibilities to be separated and portable resolution to be external-executable-only. The review on 2026-09-17 approved SA2-001 through SA2-011 with no unresolved P0 or P1 findings.
