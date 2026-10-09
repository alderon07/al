# Catalog preview diagnostics

## Acceptance criteria

- `al catalog preview --from bash|zsh` identifies the source line range, a specific reason, and a next step for each reported problem. Reasons distinguish unsupported alias quoting or names, trailing commands, metadata, function boundaries and unsupported expansions, and top-level shell configuration.
- An unsupported function body is described as outside the import grammar, rather than automatically described as syntactically incomplete. When its boundary cannot be proved, explain why the remaining lines are grouped and may contain otherwise valid definitions.
- Guidance preserves shell semantics. Do not recommend blindly replacing double quotes with single quotes when expansion timing would change. Leaving unsupported definitions native is a valid next step. Partial import copies only proven equivalent definitions.
- Diagnostics contain fixed explanatory text, codes and source coordinates. Do not print source snippets, command values, descriptions, tokens, filenames taken from shell code, or terminal control characters. Inspection does not execute user definitions or change user files.
- Keep existing `catalog shadow` plain and JSON golden output, statuses, ranges, stable IDs, parser acceptance boundaries and schema version unchanged. Preview JSON supplies its detailed diagnostics through the existing diagnostic fields without executable source.
- Bound output and diagnostic counts consistently with existing inspection limits. Show report-level failures and truncation notices. Detect failed or short output writes.
- Add synthetic Bash and Zsh fixtures for representative reasons, quoting and comment boundaries, grouped ranges, secret redaction, partial import, and read-only behavior. Verify plain preview through a real PTY at narrow and wide widths.
- Run `make fmt check`; use medium implementation and high review, correct findings and repeat review until resolved. Preserve unrelated staged changes.
