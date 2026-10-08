# Performance optimization evidence

Acceptance criteria preceded implementation in `docs/acceptance/PERFORMANCE_OPTIMIZATION.md`. Inputs are synthetic, and file observations and PTYs use disposable homes. The original performance baseline evidence is preserved.

Structural import previously hashed the complete source for each accepted entry. It now computes one SHA-256 digest per invocation and frames that digest with the original shell and byte offsets. New known-answer framing and changed-input tests preserve origin IDs. Bash and Zsh 10,000-entry medians are 32.506 and 22.967 ms, compared with 2,242.572 and 2,240.928 ms in the earlier repeated Linux baseline. This is an improvement above 98%. The original report and the new [structural report](PERFORMANCE_STRUCTURAL_AFTER_2026-10-08.txt) contain all three samples at 100, 1,000, and 10,000 entries. These runs occurred at different times, so allocation and algorithmic evidence support the result more strongly than a precise speed ratio.

Search uses two local edit-distance rows throughout each calculation. It retains the original byte comparison, normalization, threshold and stable ranking. No buffer is shared across calls. At 10,000 entries, typo allocations fall from 310,016 to 230,016 and no-match allocations from 310,003 to 230,003, a reduction of 25.8%. Typo allocated bytes fall from 11,736,465 to 5,336,448. No-match allocated bytes fall from 20,960,120 to 6,880,094. Timing samples vary enough that this pass does not claim a no-match search speed improvement.

The state decoder now validates object fields, required fields and container shapes during its existing token pass. Per-call type metadata avoids repeating reflection for every entry. The generic JSON tree and its separate required-field walk are removed. Typed decoding with unknown-field rejection, record validation and transactional destination assignment remain. The pass still rejects duplicate and escaped duplicate fields, case variants, all null values, oversized input, excessive nesting, trailing input and invalid UTF-8. Native maps and array elements retain their own schemas. Generation verification reuses a single normalized manifest for its identity and exact canonical encoding. Constant protected shell names are initialized once and only read afterwards.

The private state reader allocates from the already validated descriptor size, reads that exact length and requires EOF on a separate one-byte probe. Existing nofollow directory and leaf opening, ownership, permission and hardlink checks remain. Descriptor and path identities are compared again after reading. Growth, shrinkage, replacement and caller-limit violations fail. No file content, decoded values, generation observations or user data are cached between calls.

Focused tests passed for codec contracts, nested required/unknown/case/escaped duplicate fields, null fields, wrong shapes and scalar types, truncation, size bounds, destination preservation, origin framing and drift, and edit distances. All `internal/catalogstore` tests passed after the final sized-reader change. Focused actual Bash and Zsh PTYs passed for installed loader startup, native ownership renewal, exact membership, readonly helpers and surviving control masks. After the final sized-reader change, focused loader/startup, native-drift, readonly and private-integrity tests passed again: app 6.467 seconds, shell 0.979 seconds and catalogstore 0.031 seconds. The genuine disposable Zsh 5.9 runtime was selected through `/tmp/al-zsh-runtime/build/bin` with packaged completion functions through `/tmp/al-bootstrap-zsh-fpath`.

Before measurements used a precompiled application benchmark executable built after the module-path rewrite and before these algorithm changes. After measurements used another precompiled executable. Parent and shell agent coordinated a window with no other agent builds, tests or benchmarks. Filesystem caches were warm, CPU scheduling and outside host activity were uncontrolled. The [before report](PERFORMANCE_CORE_BEFORE_2026-10-08.txt) and [final after report](PERFORMANCE_CORE_AFTER_2026-10-08.txt) retain all repeated samples. Each row below uses 10,000 entries and the median of three per-run averages. Bytes are total Go allocation, not peak resident memory.

| Operation | Before ms/op | After ms/op | Before B/op | After B/op | Before allocs/op | After allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Typo search | 32.843 | 22.771 | 11,736,465 | 5,336,448 | 310,016 | 230,016 |
| No-match search | 38.910 | 28.619 | 20,960,120 | 6,880,094 | 310,003 | 230,003 |
| Bash installed load | 717.732 | 507.474 | 323,884,312 | 253,266,256 | 2,625,863 | 1,600,348 |
| Zsh installed load | 705.535 | 478.756 | 323,778,304 | 253,179,232 | 2,615,844 | 1,590,354 |

Installed allocated bytes fall by 21.8% in both shells. Allocation counts fall by 39.1% in Bash and 39.2% in Zsh. Final search bytes differ by a few bytes from the earlier after samples because of runtime overhead; the allocation count reduction remains 25.8%. Final timing samples improve, although an earlier candidate window had noisy no-match timing. The original baseline was measured at another time and is not substituted for these quiet before results.

The before application executable SHA-256 was `bcc18646b9df559ee8b3576e97ec787d2afaff61b5fb0221f5a8f9f4a80e1491`; final after was `f191859c60a798f5857b84d3c5c5c4a41289206b74a3815a08d66474388a1ebe`. The structural after executable was `56ada461722e9965c0e065936d107236c118dba4cfe7b7e352ed203ac1513a85`.

A separate high review inspected these changes and their direct callers and reported no findings. The parent owns the complete `make fmt check` gate; its result is recorded separately.

Native macOS and WSL performance, cold caches, child-process memory and full interactive readiness remain unmeasured. Shell startup optimization is recorded separately by its owner.
