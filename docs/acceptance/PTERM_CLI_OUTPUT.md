# Style human-facing command output with PTerm

## Acceptance criteria

- Human-facing terminal commands use a consistent, restrained PTerm presentation for headings, results, and warnings.
- Diagnostic lists and other structured results remain readable at narrow and wide terminal widths.
- Redirected output, `NO_COLOR`, dumb terminals, JSON output, shell integration output, and completion output remain plain and usable by scripts.
- Private alias text, credentials, and paths receive no extra logging or display from styling or progress feedback.
- The Bubble Tea interface stays the default experience and PTerm does not render inside it.
- PTY checks cover styled terminal output and plain piped output. `make fmt check` passes.
