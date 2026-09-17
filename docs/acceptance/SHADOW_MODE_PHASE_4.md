# Phase 4 shadow-mode acceptance criteria

Status: approved on 2026-09-17. Implementation remains blocked by the entry gate.

## Scope

Phase 4 adds a read-only inspection command:

```text
al catalog shadow [--shell bash|zsh] [--json]
```

It reads one selected shell alias file, imports definitions into an in-memory candidate catalog, renders native text, validates the rendered syntax without execution, parses it again, and compares supported structure. It never enables the catalog or writes user, application, repository, shell, history, usage, backup, revision, or sync state.

Shadow mode reports structural equivalence only. It does not claim runtime or behavioral equivalence for arbitrary shell code.

## Entry gate

Phase 4 code cannot start until all of these are true:

- Every phase 2 criterion is approved and passing.
- Phase 3 criteria SA3-001 through SA3-014 pass.
- The phase 3 WSL 2 record is committed.
- An independent reviewer approves every criterion in this document.

## Read-only selection path

Shadow mode must not call the ordinary configuration loader or alias loader because those paths can migrate configuration or create a missing alias file.

An explicit `--shell` bypasses configuration, selects that adapter directly, and uses its default alias filename beneath the current home. Without the flag, a dedicated read-only decoder applies this complete matrix:

| Configuration state | Result |
| --- | --- |
| Config directory absent | Select Bash and `~/.bash_aliases`. |
| `config.json` absent | Select Bash and `~/.bash_aliases`. |
| Version field absent | Decode as legacy version 0 in memory; an empty shell becomes Bash, `bash` selects Bash, `zsh` selects Zsh, and any other value exits `2` with `unsupported configured shell`. |
| Version `0` | Same in-memory result as a versionless file. |
| Version `1` | An empty shell becomes Bash, `bash` selects Bash, `zsh` selects Zsh, and any other value exits `2` with `unsupported configured shell`. |
| Negative version | Exit `2` with `invalid configuration version`. |
| Malformed JSON or wrong field type | Exit `2` with `invalid configuration`. |
| `null` version or shell | Exit `2` with `invalid configuration`. |
| Duplicate `version` or `shell` key | Exit `2` with `duplicate configuration field`. |
| Version greater than `1` | Exit `2` with `configuration version unsupported`. |
| Unreadable file or directory | Exit `2` with `configuration unreadable`. |

The version 1 `alias_file` field is a repository sync path in the current product and is ignored for local shadow selection. No row migrates, creates, or saves configuration.

The read-only decoder limits `config.json` to 1 MiB. It uses the same nonblocking, close-on-exec, open-once, and descriptor `fstat` rules as the alias source. A regular file or leaf symlink to a regular file is allowed. A broken link, FIFO, socket, directory, device, oversized file, or path replaced with a non-regular file exits `2` without reading content or blocking.

After selection, shadow mode obtains the selected adapter's alias path and opens it read-only. It does not call `loadAliases`. A missing file is an operating error and remains missing.

## Input boundaries and identity

All byte ranges are zero-based, half-open `[start_byte,end_byte)`. Line numbers are one-based and inclusive. Offsets refer to original bytes, so CRLF counts as two bytes. A missing final line feed is part of the final line.

Leading contiguous metadata comments belong to a definition only when the current adapter recognizes their exact format. An ordinary comment is its own unsupported or ignored range as documented by the parser. A trailing comment belongs to a definition only when the native grammar makes it part of that declaration.

A bounded unsupported construct may end at a parser-proven boundary, allowing a following supported definition to remain independent. An unclosed quote, function, command substitution, here-document, conditional, or other ambiguous construct consumes through end of file. Text inside that range is not recovered as a neighboring definition.

Before candidate construction, every proven definition and unsupported range receives an origin key:

```text
sha256(frame(shell) || frame(source_sha256) || frame(start_byte) || frame(end_byte))
```

`frame(x)` is an unsigned 64-bit big-endian byte length followed by the bytes of `x`. `source_sha256` means the raw 32 digest bytes, not its hexadecimal text. Offsets use their unsigned 64-bit big-endian representation as the framed payload. For shell `bash`, a 32-byte all-zero source digest, start `0`, and end `10`, the origin key is `0e4470091b064fefd0a1574d8aa97801eab2016fe14c3fc7fe88ba518b360bfb`. Candidate IDs are the first 16 digest bytes, encoded as 32 lowercase hexadecimal characters. Origin keys and IDs are internal, provisional, deterministic, and never persisted or reported.

The renderer emits `# al-shadow-origin: <origin-key>` before the owned description and metadata comments for each rendered definition. The reparser uses that marker for identity and removes it from the user-facing model. A missing, duplicate, malformed, or misplaced marker makes the result `invalid`. Comparison never pairs entries by list position or name. Every member of an exact-name collision set is `duplicate`, and the entire set is excluded from candidate rendering.

## Supported source grammar and metadata

Shadow mode accepts these source forms only:

- An alias is one logical line containing `alias`, horizontal whitespace, one static name, `=`, and exactly one single-quoted shell word. The word may use the standard close-quote, escaped-quote, reopen-quote sequence to contain a literal apostrophe. Only horizontal whitespace and an ordinary trailing comment may follow it.
- A function uses a static valid function name in `name() { ... }` or `function name { ... }` form. A quote-aware and nesting-aware scanner must prove the matching outer brace. `function_body` is the exact original byte slice between those outer braces, including leading and trailing whitespace, comments, quoting, line endings, and a missing final line feed inside the body. The body may contain syntax that executes only when the function is invoked, including nested functions. No prefix command, suffix command, declaration redirection, here-document, or top-level operator may share the declaration range.

Unquoted or double-quoted alias values, multiple alias operands, alias options, dynamic names, definition-time parameter, command, process, or arithmetic expansion, declaration redirection, and adjacent top-level commands are `unsupported`. Sentinel tests prove that none execute. Malformed forms are `invalid` only when the adapter can prove the intended definition range; otherwise the ambiguous range is `unsupported` through EOF.

A contiguous owned comment block has no blank line before its definition. It contains zero or more nonempty description lines of the form `# TEXT`, followed by zero or one metadata line. The last description line becomes `description` and must satisfy the phase 2 description limit. A metadata line has exact prefix `# al: ` followed by space-separated, unique `key=value` fields in this order: `tags`, `platforms`, `favorite`, `category`.

The importable metadata subset is exact:

- Tags and category values match `^[a-z0-9][a-z0-9_-]{0,63}$` and preserve their lowercase bytes.
- Tags are a comma-separated list with no empty or duplicate element.
- Platforms are a comma-separated list with no empty or duplicate element. Each value is exactly `linux`, `macos`, `wsl`, or `windows` and appears in that canonical order.
- Favorite is exactly `true`. Absence means false.
- No metadata value has whitespace, quoting, backslash escaping, or percent encoding.

Unknown, repeated, empty, out-of-order, uppercase, escaped, or otherwise unrepresentable metadata makes the complete owned block and definition unsupported. A phase 2 string outside this importable subset is valid catalog data but cannot round trip through phase 4 native comments. Section separators and every standalone ordinary comment are ignored ranges and create no result unless raw secret scanning blocks their range. A blank line breaks ownership.

For an alias, `alias_value` is the exact decoded literal byte sequence represented by the accepted single-quoted word. The renderer may use a canonical single-quote escape sequence, but reparsing must recover identical value bytes. Rendering a function copies `function_body` byte-for-byte between adapter-supplied outer declaration delimiters.

Rendering order is origin marker, optional description line, optional metadata line, then declaration. Metadata values use phase 2 canonical order. Comparison covers exactly `name`, `kind`, selected-shell `alias_value` or `function_body`, `description`, `category`, `tags`, `platforms`, and `favorite`. Only outer declaration spelling, canonical alias quoting, origin-marker formatting, and documented metadata formatting are insignificant. Whitespace, comments, quoting, or line endings inside a function body and every decoded alias-value byte are significant. Comparison does not include provisional ID, origin marker, inferred category, ignored comments, or unselected-shell implementations.

## Result and report schema

Each source unit has one status:

| Status | Meaning |
| --- | --- |
| `invalid` | Source, candidate, marker identity, or rendered native syntax is invalid. |
| `blocked` | A secret, missing validator, or safety limit prevents inspection. |
| `duplicate` | More than one proven definition uses the same exact case-sensitive name. |
| `different` | A supported field changed during the round trip. |
| `unsupported` | The adapter cannot prove or represent the complete source range. |
| `equivalent` | Every supported field survived import, render, validation, and reparse. |

JSON output has this exact version 1 root shape and field order:

```json
{
  "schema_version": 1,
  "shell": "bash",
  "summary": {
    "equivalent": 0,
    "unsupported": 0,
    "different": 0,
    "duplicate": 0,
    "invalid": 0,
    "blocked": 0
  },
  "diagnostics": [],
  "results": []
}
```

Each result uses this field order: `unit`, `name`, `kind`, `status`, `start_byte`, `end_byte`, `start_line`, `end_line`, `different_fields`, `diagnostics`. `unit` is a zero-based integer assigned by source range order. Optional `name` and `kind` appear only when proven safe. `different_fields` is present only for `different` and uses the comparison-field order above. `diagnostics` is always an array of redacted objects with `code` and `message`. The root `diagnostics` array contains only whole-report notices such as diagnostic truncation. Reports never expose source digests, origin keys, provisional IDs, or generation hashes.

Results sort by this severity order: invalid, blocked, duplicate, different, unsupported, equivalent; then by start byte and unit. Plain output follows the same order. Normal output does not contain the absolute source path. A whole-command operating error writes no JSON, writes one actionable redacted error to standard error, and exits `2`. JSON is fully buffered and validated before one write to standard output.

Exit status is `0` only when every result is equivalent, `1` for any other result status, and `2` for a whole-command read, selection, or reporting failure.

## Limits and process isolation

| Resource | Limit |
| --- | --- |
| Configuration file | 1 MiB |
| Source file | 8 MiB |
| One physical line | 1 MiB |
| Proven definitions | 10,000 |
| Total result ranges | 20,000 |
| Ordinary diagnostics | 100 |
| Validator standard error | 64 KiB |
| Validator standard output | 64 KiB |
| Encoded report | 16 MiB |
| One native validator process | 5 seconds |
| All native validation | 30 seconds |
| Validator process launches | 256 |
| Concurrent validators | 1 |

The source path may be a regular file or a leaf symlink that resolves to a regular file. Shadow mode opens the path once with the platform's nonblocking and close-on-exec flags, then verifies the descriptor with `fstat`. A broken link, FIFO, socket, directory, block device, or character device fails before content is read. A path replacement between preliminary inspection and open cannot bypass descriptor validation. The reader obtains size-bounded bytes from that descriptor and never reopens by path.

Native validation checks Bash candidates `/bin/bash` then `/usr/bin/bash`, and Zsh candidates `/bin/zsh` then `/usr/bin/zsh`. It resolves every symlink and verifies that the target and every traversed directory are owned by root and are not writable by group or other. It rejects a candidate from a user-writable directory, including `/usr/local/bin`, a Homebrew prefix, the project, or the current `PATH`. The resolved target must be a regular executable file. Validation invokes that absolute path with `exec.CommandContext` and a new process group. Timeout or cancellation terminates the process group and reaps it. Rendered bytes go to standard input. Exact argument vectors are `[resolvedBash, "--noprofile", "--norc", "-n"]` and `[resolvedZsh, "-f", "-n"]`.

The adapter runs one validator process at a time and launches at most 256 processes for the command. It first validates the complete rendered body. If that succeeds, every definition advances to reparse. If it fails, the adapter recursively bisects contiguous definition ranges and validates each half until each failure is assigned to one definition or a launch or time limit is reached. Passed ranges remain unaffected. An isolated syntax failure is `invalid`. A timeout, cancellation, nonempty validator standard output, output-limit failure, or diagnostic that cannot be assigned before a limit makes only the unresolved batch `blocked`. Each process still has a five-second deadline, and all validation has a 30-second deadline. Pending batches become blocked without another launch when either global cap is reached.

The environment is an allowlist containing only `PATH=/usr/bin:/bin`, `HOME=<isolated-empty-directory>`, `ZDOTDIR=<same-isolated-directory>`, `BASH_ENV=/dev/null`, `ENV=/dev/null`, and `LC_ALL=C`. Validator standard output and standard error are captured independently up to 64 KiB plus one detection byte and are never forwarded. Any nonempty standard output blocks the current validation batch and its bytes are discarded. Standard error is used only to classify a redacted syntax result, then discarded. The private temporary home contains no source content, is removed on normal completion, and may leave only an empty directory after a crash. The implementation documents that `zsh -f` skips user startup files but the installed system zsh may still apply its compiled system configuration boundary. No user-controlled startup file is loaded.

The generation hash is SHA-256 over this length-framed sequence:

1. Renderer ID, `bash/v1` or `zsh/v1`.
2. Platform, `linux`, `macos`, or `wsl` in phase 4.
3. Native-approval input hash as 32 raw digest bytes, the SHA-256 of an empty byte string in shadow mode.
4. Rendered definition-body bytes, including origin markers but excluding any generated-file header.

Each item uses the same unsigned 64-bit big-endian length framing as origin keys. Origin markers are part of the definition-body bytes, so provisional IDs affect the hash deterministically.

This framed formula is also the architecture-wide generated-file hash contract. For renderer `bash/v1`, platform `linux`, the raw SHA-256 digest of empty bytes as the approval hash, and body bytes `# body\n`, the generation hash is `06fc5ce2f0aaa98290cf5ceecb582c406e0b4249606891779927bcf8f86bc205`. Phase 4 does not report the hash.

## Limit outcomes

| Limit failure | Outcome |
| --- | --- |
| Configuration over 1 MiB | Whole-command error, exit `2`, no JSON. |
| Source over 8 MiB | Whole-command error, exit `2`, no JSON. |
| Physical line over 1 MiB | Whole-command error, exit `2`, no JSON. |
| More than 10,000 proven definitions | Whole-command error, exit `2`, no JSON. |
| More than 20,000 total ranges | Whole-command error, exit `2`, no JSON. |
| More than 100 ordered diagnostics | Keep the first 100; add one root `diagnostics_truncated` record with the exact omitted count. Result statuses still determine exit `0` or `1`. |
| Validator standard error over 64 KiB | Truncate captured bytes, discard their text from the report, mark the current unresolved batch `blocked`, exit `1`. |
| Validator standard output is nonempty or over 64 KiB | Capture only through the detection bound, discard the bytes, mark the current unresolved batch `blocked`, exit `1`. |
| Per-process timeout, overall timeout, or cancellation | Mark only the current unresolved batch `blocked`, terminate and reap the group, exit `1`. |
| Validator launch cap reached | Mark every remaining unresolved batch `blocked`, launch no more processes, exit `1`. |
| Encoded report over 16 MiB | Whole-command error, exit `2`, no plain or JSON report bytes. |

Diagnostics have one global semantic order: root diagnostics by code, followed by results in report order and each result's diagnostics by code. Keep the first 100 ordinary diagnostics from that sequence. If any are omitted, append one root `diagnostics_truncated` notice containing the exact omitted count. Root notices appear in the root array even though their semantic position is after the retained ordinary sequence.

## Secret handling

Secret scanning runs on raw source bytes before parsing and before any source-derived diagnostic is formatted. Findings are mapped to proven definition ranges. A finding outside a proven definition creates a redacted blocked range. A blocked range is never rendered.

Reports may contain a safe entry name, finding kind, and line number. They never contain implementation bodies, source lines, arguments, secret values, or validator text that includes rendered content. Tests place unique canaries in valid definitions, malformed syntax, unsupported ranges, oversized input, metadata, validator standard error, cancellation errors, and panic recovery output.

## Criteria

### SA4-001 keeps every path read-only

| Field | Required evidence |
| --- | --- |
| Operation | Run plain and JSON shadow mode across success and failure cases with manifests of the isolated home, config, state, repository, Git index, alias, startup, backup, revision, history, and usage paths. |
| Expected state | User, application, shell, and repository manifests do not change. The validator may create only its private empty temporary home, which is outside those roots and is removed on normal completion. Missing alias files remain missing. Config version `0`, versionless, malformed, future, unreadable, and absent-directory cases never migrate or save. |
| Automated evidence | `TestShadowReadOnlyManifest`, `TestShadowReadOnlyConfigMatrix`, and `TestShadowMissingAliasDoesNotCreate` |
| Approval | Sol/high approved on 2026-09-17. |

### SA4-002 never executes inspected content

| Field | Required evidence |
| --- | --- |
| Fixtures | Commands, substitutions, traps, redirections, startup hooks, and function bodies that would create sentinel files if executed. |
| Expected state | No sentinel exists. Parsing and rendering are inert. Validators receive standard input and the exact isolated argv and environment above. |
| Automated evidence | `TestShadowNeverExecutesBashContent`, `TestShadowNeverExecutesZshContent`, and `TestShadowValidatorIsolation` |
| Approval | Sol/high approved on 2026-09-17. |

### SA4-003 selects only the requested shell

| Field | Required evidence |
| --- | --- |
| Operation | Exercise explicit flags and every read-only config state with different Bash and Zsh files. |
| Expected state | An explicit flag bypasses config. Implicit selection reads config without mutation. Only the selected alias file is opened. Unsupported shells make no support claim. |
| Automated evidence | `TestShadowShellSelectionMatrix` and open-call recording doubles |
| Approval | Sol/high approved on 2026-09-17. |

### SA4-004 proves source ranges conservatively

| Field | Required evidence |
| --- | --- |
| Fixtures | Every accepted and rejected source form above, metadata order, marker placement, CRLF, missing final line feed, escaped lines, here-documents, incomplete constructs, comments, duplicates, redirections, expansions, and adjacent commands. |
| Expected state | Grammar, ranges, exact function-body slices, decoded alias-value bytes, ignored comments, metadata ownership, and compared fields follow this document. Bounded unsupported text preserves proven neighbors. Ambiguous incomplete text consumes through EOF. Sentinel content never executes. |
| Automated evidence | Bash and Zsh golden grammar and range fixtures, `TestShadowDefinitionTimeExpansionSentinels`, and `TestShadowAmbiguousRangeConsumesEOF` |
| Approval | Sol/high approved on 2026-09-17. |

### SA4-005 builds only valid candidate entries

| Field | Required evidence |
| --- | --- |
| Operation | Import proven definitions as selected-shell native implementations, assign deterministic provisional IDs, report duplicates, then run phase 2 typed validation on the valid candidate subset. |
| Expected state | Commands use `alias_value`; functions use `function_body`. No portable intent is inferred. Invalid entries are not rendered. Every member of a duplicate-name set is `duplicate`, and none is rendered. No phase 4 test invents unknown typed fields or schema versions that cannot exist in memory. |
| Automated evidence | `TestShadowCandidateConstruction` and `TestShadowDuplicateNamesBeforeRender` |
| Approval | Sol/high approved on 2026-09-17. |

### SA4-006 preserves deterministic identity and rendering

| Field | Required evidence |
| --- | --- |
| Operation | Repeat import and render with randomized map iteration and source fixtures at different filesystem paths. |
| Expected state | Internal origin keys, provisional IDs, markers, rendered bytes, and generation hashes are stable. They change when a documented hash input changes. The known-answer origin vector passes. No internal fingerprint appears in a report. No timestamp, absolute path, environment value, or random value appears. |
| Automated evidence | `TestShadowOriginKeyVectors`, `TestShadowRenderDeterministic`, and `TestShadowGenerationHashVectors` |
| Approval | Sol/high approved on 2026-09-17. |

### SA4-007 validates native syntax within hard process limits

| Field | Required evidence |
| --- | --- |
| Operation | Validate good, bad, hanging, noisy, missing-validator, cancellation, aggregate-failure, and descendant-process fixtures. |
| Expected state | Exact absolute executable, argv, and environment match this document. Aggregate success uses one process; failure uses serial bounded bisection and affects only unresolved batches. At most one validator runs, and at most 256 launch. Standard output and error follow their independent bounds and never reach command output. Process and command deadlines apply. Every process group is terminated and reaped. Missing or untrusted validators block results. A malicious executable in an earlier user-writable directory is never invoked. |
| Automated evidence | `TestShadowValidatorMatrix`, `TestShadowValidatorBisectsFailures`, `TestShadowValidatorAttributionDeadline`, `TestShadowValidatorLaunchAndConcurrencyCaps`, `TestShadowRejectsUntrustedValidator`, `TestShadowValidatorKillsProcessGroup`, and `TestShadowValidatorOutputLimits` |
| Approval | Sol/high approved on 2026-09-17. |

### SA4-008 compares structure by origin marker

| Field | Required evidence |
| --- | --- |
| Operation | Change each supported field, reorder definitions, and remove, duplicate, corrupt, or move origin markers. |
| Expected state | Comparison pairs only by valid origin key. Supported changes report exact field names. Missing or ambiguous identity is invalid. Exact function-body and decoded alias-value bytes survive the round trip. Only the explicitly listed outer and metadata formatting is insignificant. No runtime-equivalence claim is made. |
| Automated evidence | `TestShadowComparisonFieldMatrix` and `TestShadowComparisonIdentityFailures` |
| Approval | Sol/high approved on 2026-09-17. |

### SA4-009 blocks and redacts secrets before parsing

| Field | Required evidence |
| --- | --- |
| Operation | Run every raw-byte canary location listed in the secret-handling section through plain, JSON, error, cancellation, and recovered-panic paths. |
| Expected state | Findings become blocked definitions or blocked ranges before rendering. No canary appears in output, logs, snapshots, errors, or panic text. Safe lookalikes continue. |
| Automated evidence | `TestShadowRawSecretScanMatrix` and `TestShadowCanaryNeverEscapes` |
| Approval | Sol/high approved on 2026-09-17. |

### SA4-010 enforces file and resource limits

| Field | Required evidence |
| --- | --- |
| Operation | Exercise every limit at the boundary and one unit over for both config and alias inputs, plus regular files, allowed leaf symlinks, broken links, FIFOs, sockets, directories, devices, duplicate config keys, path replacement, and 10,000 definitions that all fail validation. |
| Expected state | Descriptor-based reads are bounded. FIFO opens are nonblocking. Non-regular inputs and path replacements fail without reading. Validator concurrency never exceeds one and launch count never exceeds 256. Every failure follows the config matrix or limit-outcome table, including global diagnostic ordering and truncation. No operation allocates from an attacker-controlled declared size. |
| Automated evidence | `TestShadowConfigResourceLimits`, `TestShadowResourceLimits`, `TestShadowAllFailingDefinitionsRespectProcessCaps`, `TestShadowDiagnosticTruncation`, `TestShadowNonblockingFileTypeMatrix`, `TestShadowPathReplacement`, and adapter fuzz tests |
| Approval | Sol/high approved on 2026-09-17. |

### SA4-011 emits stable plain and JSON reports

| Field | Required evidence |
| --- | --- |
| Operation | Render a fixed result set containing every status twice under different temporary roots. Force encoding and output-write errors. |
| Expected state | Schema, field presence, units, diagnostic ordering, counts, and exit statuses match this document. Encoding and other pre-write failures emit no JSON. A single stdout write is attempted only after successful buffering. If that write returns a short count or error, a partial prefix may already exist; the command exits `2`, does not retry, and writes no further stdout bytes. Normal reports omit absolute paths, source content, validator output, and internal fingerprints. |
| Automated evidence | `TestShadowPlainReportGolden`, `TestShadowJSONReportGolden`, `TestShadowReportErrorMatrix`, and output hash fixtures |
| Approval | Sol/high approved on 2026-09-17. |

### SA4-012 cannot reach activation, mutation, or sync

| Field | Required evidence |
| --- | --- |
| Operation | Inject fail-on-call doubles around process launch, repository access, mutation lock acquisition, watcher startup, sync, backups, revisions, usage writes, setup, and activation. Run shadow concurrently with another pure reader. |
| Expected state | Only the isolated native validator process launcher is called. Every mutation and repository double remains untouched. Concurrency uses a demonstrably pure reader, not stats or another command that can write. `.git/index` and a repository manifest remain unchanged. |
| Automated evidence | `TestShadowForbiddenCallGraph` and `TestShadowConcurrentPureReader` |
| Approval | Sol/high approved on 2026-09-17. |

### SA4-013 passes the supported environment matrix

| Environment | Required evidence |
| --- | --- |
| Ubuntu 24.04 | CI job `shadow-ubuntu` records OS, Go, Bash, and Zsh versions, runs `TestShadowLinuxMatrix`, and compares plain and JSON output with committed golden hashes. |
| macOS 14 | CI job `shadow-macos` records OS, Go, Apple Bash, and Zsh versions, runs `TestShadowMacOSMatrix`, and compares plain and JSON output with committed golden hashes. |
| Windows 11 WSL 2 | From a clean checkout run `wsl.exe --version`, `uname -a`, `bash --version`, `go version`, `GOCACHE=/tmp/alias-lens-shadow-cache go test ./cmd/alias-lens -run 'TestShadow(Bash|WSL)'`, then `./scripts/verify-shadow-wsl.sh`. The script uses a disposable home, prints `PASS shadow-wsl`, and prints manifest, plain-report, and JSON-report SHA-256 values that must equal committed `testdata/phase4/wsl.sha256` lines. Record exact keystrokes, exit statuses, and output. |
| Approval | Sol/high approved on 2026-09-17. |

### SA4-014 keeps implementation scope narrow

| Field | Required evidence |
| --- | --- |
| Operation | Review `git diff <criteria-commit>...HEAD`, dependencies, and formatter coverage; run `make fmt check`. |
| Expected state | The diff adds read-only shadow inspection only. It does not add persistence, activation, adoption, startup writes, sync, another shell, or a dependency. Any Makefile change is limited to recursive formatting of Go beneath `cmd` and `internal`. |
| Automated evidence | Full phase 4 suite, `make fmt check`, `git diff --check`, and direct `go.mod` and `go.sum` hashes |
| Approval | Sol/high approved on 2026-09-17. |

## Approval record

Sol/high completed four read-only reviews. The first review rejected mutating read paths, ambiguous identity and ranges, underspecified reports and hashes, weak validator bounds, and late secret scanning. Later reviews tightened config reads, metadata grammar, private fingerprints, validator trust and attribution, resource outcomes, report writes, exact body comparison, and process caps. The final review on 2026-09-17 approved SA4-001 through SA4-014 with no unresolved P0 or P1 findings. Phase 4 remains blocked until every entry-gate condition passes.
