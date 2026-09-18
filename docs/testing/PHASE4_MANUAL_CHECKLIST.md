# Phase 4 shadow-mode evidence

Use this checklist on the release candidate after the automated suite passes. Shadow mode must leave the alias file and every Alias Lens data path unchanged.

## WSL 2

From a clean checkout in WSL Ubuntu, run:

```bash
wsl.exe --version
uname -a
bash --version | head -1
go version
GOCACHE=/tmp/alias-lens-shadow-cache go test ./cmd/alias-lens -run 'TestShadow(Bash|Linux|NeverExecutes|Validator)' -count=1
./scripts/verify-shadow-wsl.sh
```

Save the commands, exit statuses, and output in `docs/testing/evidence/PHASE4_WSL2_YYYY-MM-DD.md`. The script must print `PASS shadow-wsl` and hashes that match `cmd/alias-lens/testdata/phase4/wsl.sha256`.

Close WSL with `wsl.exe --shutdown`, reopen it, and run the script once more. This catches dependencies on a process or temporary state left by the first session.

## Ubuntu and macOS

The `shadow-ubuntu` and `shadow-macos` CI jobs record the OS, Go, Bash, and Zsh versions. Both jobs must pass before release. Keep the workflow URL with the release evidence.
