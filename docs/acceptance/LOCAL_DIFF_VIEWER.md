# Local catalog diff viewer acceptance criteria

Status: approved for implementation by the user on 2026-09-19. Native terminal and browser evidence can be recorded after implementation. This approval does not close the phase 4 WSL restart gate.

## Scope

Alias Lens can use the open-source `@pierre/diffs` package to display exact catalog changes in its optional local browser interface.

The terminal interface remains the default. Go owns catalog parsing, semantic comparison, plans, conflicts, and file changes. Pierre displays a typed result and does not make workflow decisions.

The viewer is available through `al catalog diff --web` or an equivalent catalog-diff route in `al --web`. The existing `al diff` command keeps its 1.x meaning and syntax.

## Acceptance criteria

### DV-001 stays private and local

- The server binds only to loopback.
- Every API request requires the unpredictable session token and the existing Host and Origin checks.
- Alias and function text never appears in a URL, log, error, telemetry request, or persistent browser storage.
- JavaScript, CSS, fonts, syntax data, and themes are served from the embedded binary. The page makes no third-party request.
- The normal semantic-diff JSON continues to contain hashes and byte counts, not implementation text.

### DV-002 keeps exact text behind a deliberate action

- The first view uses plain-language cards and counts.
- Exact before and after text loads only after the user chooses **Show exact changes**.
- The detail response is bounded and available only during the authenticated local session.
- Closing the server removes access to the detail response.

### DV-003 is read-only

- Opening, filtering, changing layout, and viewing exact text do not create, migrate, recover, rewrite, stage, commit, or synchronize a file.
- The first release of the viewer has no apply, merge, approve, or edit action.
- Browser code cannot supply semantic identities, plan actions, conflict outcomes, or write targets to Go.

### DV-004 remains optional

- The installed `alias-lens` binary works without Node.js, npm, a network connection, or a browser.
- Frontend dependencies are pinned by a lockfile and compiled before release into `go:embed` assets.
- A missing browser does not break `al catalog diff`, the TUI, setup, sync, or shell integration.
- Third-party license notices include Pierre and its distributed dependencies.

### DV-005 remains understandable and accessible

- The summary leads with added, removed, and changed entries in familiar words.
- Color is not the only difference indicator.
- Controls have keyboard focus, accessible names, and visible focus states.
- Unified and split layouts remain usable at narrow and wide widths.
- Long commands wrap or scroll without hiding the entry name or change type.
- Reduced-motion preferences disable nonessential animation.

## Required evidence

- Automated API tests for missing and invalid authentication, hostile Host and Origin headers, response limits, and no-store headers.
- Automated canary tests proving that exact command text is absent from summary JSON, URLs, logs, and initial HTML.
- Automated tests proving that viewer requests do not change catalog, configuration, state, repository, alias, or startup files.
- A production frontend build from a pinned lockfile followed by `make fmt check`.
- Manual browser checks at narrow and wide widths, with keyboard-only navigation, light and dark system themes, and reduced motion.
- A binary-size comparison recorded before release.

## Recorded automated evidence

- `npm audit --omit=dev` reported no known vulnerabilities on 2026-09-19.
- The production bundle is 2,729,339 bytes after limiting syntax data to the JSON language used by catalog comparisons. The initial all-language bundle was 10,805,362 bytes.
- A Linux amd64 release build is 18,195,854 bytes, compared with 14,860,286 bytes at `HEAD` before this work: an increase of 3,335,568 bytes (22.4%). This is a release tradeoff to review, not a claim that manual browser evidence is complete.

## Deferred behavior

Applying changes, resolving conflicts, approving native implementations, and editing entries in the browser require separate acceptance criteria. They are not part of this viewer.
