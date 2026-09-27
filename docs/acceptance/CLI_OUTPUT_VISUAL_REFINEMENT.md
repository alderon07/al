# Refine command output layout

## Acceptance criteria

- Interactive human-facing reports use one Alias Lens heading style, a compact separator, and clear spacing between the heading and results.
- Status labels align, doctor checks keep their OK/FIX words, and long values remain readable at 48 and 120 terminal columns.
- Interactive search results show each alias as a compact name, command, and description group rather than a tab-separated row.
- The heading, success, warning, and muted text use colors consistent with the terminal interface without relying on color alone.
- Piped output, `NO_COLOR`, `TERM=dumb`, JSON, shell output, and completions retain their current plain bytes.
- PTerm sync progress remains legible, and no additional private values are printed.
- `make fmt check` and real PTY checks pass.
