# WSL test audit acceptance criteria

- Run the full Go suite without cached results on WSL 2, including Bash and Zsh shell integration tests.
- Run every PTY test explicitly and record failures, skips, and terminal dimensions covered by the tests.
- Run race checks for non-PTY tests, coverage, module verification, the WSL shadow fixture, and `make fmt check`.
- Reproduce each discovered defect with a failing regression test before changing production code. Record the failing and passing commands.
- Keep test homes, aliases, repositories, and credentials synthetic and isolated from real user data.
- Record the tested commit, environment, commands, results, and unverified release checklist items in `docs/testing/evidence/`.
- Do not mark the WSL release checklist complete without restart, Windows Terminal, and archive evidence.
- Accept absolute PATH directories with trailing separators, including Windows directories with spaces imported by WSL. Continue rejecting empty, relative, and dot-segment directories.
- Verify portable catalog installation and execution under that PATH with regression and compiled PTY tests.
- Repeat the catalog startup PTY test ten times when investigating the prior process-start failure. Record whether it reproduces.
- Rerun the full suite with coverage and all PTY tests after checking for newer upstream changes. Preserve the earlier audit's failure evidence.
