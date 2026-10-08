# Native declaration boundary acceptance

- Native review and approval must prove exactly one declaration with the original name, kind, body, and byte range before parse-only shell validation.
- Unquoted escapes cannot conceal a closing function delimiter. Literal brace arguments cannot extend the declaration range over a top-level command.
- The structural grammar fails closed for substitution, ANSI-C quoting, backticks, process substitution, nested brace groups, and other unsupported lexical forms.
- Ordinary quoted variables, simple quoted and unquoted heredocs containing braces, bounded numeric arithmetic including shifts, trailing comments, and following aliases retain their exact source ranges.
- Heredoc delimiter quote removal supports ordinary single/double quoting and escaped characters, including mixed quoted words; unsupported delimiter constructs refuse the entire function.
- Heredoc data begins only after an unquoted command newline. Multiline quoted arguments or delimiters while a heredoc is pending are unsupported and refuse the function. Unquoted heredoc data with a trailing backslash refuses the function rather than guessing shell line joining; quoted heredoc data retains literal backslashes. Escaped command newlines are unsupported, including token joins that change dollar quoting or substitutions.
- Here-strings (`<<<`) and longer overlapping input-operator runs are unsupported. Operators are consumed atomically so their suffix cannot be interpreted as a heredoc. Ordinary `<<` and `<<-` heredocs remain supported; quoted operator text remains literal.
- Focused tests use synthetic source and disposable homes. Real Bash and Zsh PTYs verify accepted definitions produce no top-level output and run their bodies only upon invocation.
- The shared structural gate is exercised through declaration validation and the existing approval, generation, loader, shell-entry, and fallback-refresh callers.
- Run focused shell tests and repository `make fmt check`, and report unavailable verification explicitly.
- Exact adapter-owned integration remains upgradeable: review masks only a byte-exact block after a proven top-level prefix, preserving all original offsets and enrollment bytes. Modified or nested integration remains unsupported. Pinned integration generation derives its guarded control function from the trusted adapter template.
