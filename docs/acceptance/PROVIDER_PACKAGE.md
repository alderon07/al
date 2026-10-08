# Provider package extraction acceptance

## Boundary

`internal/providers` owns provider repository discovery, immutable HTTPS object reads, locator parsing, HTTP response bounds, pagination, object identity proof, and provider-specific credential/clone handoffs. It accepts provider settings and explicit runtime dependencies rather than application configuration. It must not import the command, application, or TUI packages.

The application owns configuration persistence, interactive sign-in prompts, catalog schema validation and secret scanning, reviewed SSH transport choice, mutation sessions, approval decisions, and enrollment. API responses can prove immutable bytes without deciding whether catalog entries are safe to install.

## Criteria

- PP-001: GitHub, GitLab, and Bitbucket discovery and immutable reads live behind provider interfaces in the package. The picker remains provider neutral. Application code does not duplicate provider HTTP implementations or depend on concrete provider types for ordinary discovery.
- PP-002: Provider settings are passed explicitly. HTTP clients/transports, credential access and needed command execution seams are injectable. Production credentials stay out of persistent app config, arguments, URLs/remotes, fixtures, and diagnostics.
- PP-003: Keep configured HTTPS authority checks, cross-host redirect refusal, response/deadline limits, pagination, write access filtering, revision/blob/path identity proof, and capability errors. Missing and invalid credentials preserve actionable behavior. Preserve selected SSH fallback policy.
- PP-004: App-level catalog decoding and whole-catalog secret scanning still occur before preview/enrollment/merge/application use. Provider decoding preserves size, encoding and Git object hash checks. Separating responsibilities must not remove either layer.
- PP-005: Interactive connection prompts and configuration saving remain at the command/application boundary and do not hold a mutation lock during interaction. Existing provider-neutral interfaces remain usable for future adapters.
- PP-006: Move isolated provider tests with their implementation, keep command connection/bootstrap/sync tests with callers. Use synthetic credentials and disposable homes/repos. Existing immutable provider, paging, invalid/missing credential, redirect, filtering and SSH tests pass. Run `make fmt check`, focused race tests and the affected bootstrap/sync PTYs.
- PP-007: Keep command output, public JSON/record versions, provider IDs, enrollment formats, shell bytes and unrelated Git state unchanged. No dependency updates or network calls against production repositories.

## Review

Medium implementation, separate high review, medium corrections and repeated high review. Review the bounded extraction against its pre-extraction snapshot and record concrete failures and resolutions in package organization evidence.
