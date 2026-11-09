# macOS private workflows

Acceptance criteria, written before implementation:

- Darwin workflow file and directory inspections permit the OS-managed `com.apple.provenance` attribute. Unknown extended attributes remain refused. Linux continues to refuse all extended attributes.
- Private writes, appends, directory promotion, and interrupted-workflow recovery use the same metadata policy. Recovery completes and a second recovery is harmless when provenance is the only extended metadata.
- Darwin paths below the root-owned `/tmp` to `/private/tmp` and `/var` to `/private/var` redirects work for workflow writes and catalog state reads. Other parent symlinks, leaf symlinks, changed identities, hard links, and unsafe ownership or permissions retain their existing protections.
- Opening a revision preview without loaded revisions returns an error. The changed-live-file test checks revision loading and diff creation before using the diff.
- The module selects Go 1.27.3, and `govulncheck ./...` no longer reports GO-2026-6809 with that toolchain.
- Run focused regression tests, PTY coverage, and `make fmt check`. Record platform limits explicitly; Linux execution alone does not verify Darwin behavior.

Verification on Linux:

- `AL_REQUIRE_PTY_SHELLS=1 make fmt check` passed with Go 1.27.3, including Bash and Zsh PTY coverage.
- `go test ./cmd/alias-lens -run TestCompiledCatalogRecoveryPTY -count=1 -v` passed at widths 52 and 140 for successful and blocked recovery.
- Focused workflow, private-file, and revision-preview tests passed.
- `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` reported no vulnerabilities. Build metadata confirms Go 1.27.3.
- Darwin amd64 and arm64 test binaries cross-compiled. Native macOS provenance, system-parent, and recovery execution remains pending.

Subsequent [user-reported macOS 27.0.1 arm64 validation](../testing/evidence/MACOS_ARM64_2026-11-09.md) passed the full suite and native setup flows on `03b1279`. Terminal.app and Intel execution remain open.
