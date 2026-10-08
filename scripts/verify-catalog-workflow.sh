#!/usr/bin/env bash
set -euo pipefail

repository_root=$(cd "$(dirname "$0")/.." && pwd)
cd "$repository_root"
for required_program in go git bash zsh; do
    command -v "$required_program" >/dev/null || {
        printf 'Catalog verification requires %s.\n' "$required_program" >&2
        exit 1
    }
done

verification_root=$(mktemp -d "${TMPDIR:-/tmp}/alias-lens-catalog-verify.XXXXXX")
chmod 700 "$verification_root"
trap 'rm -rf "$verification_root"' EXIT
binary_path="$verification_root/alias-lens"
go build -buildvcs=false -o "$binary_path" ./cmd/alias-lens
fixture_repository="$verification_root/repository"
mkdir -m 700 "$fixture_repository"
mkdir -m 700 "$fixture_repository/alias-lens"
cat > "$fixture_repository/alias-lens/catalog.json" <<'CATALOG'
{"schema_version":2,"entries":[{"id":"11111111111111111111111111111111","name":"catalog_probe","kind":"command","portable":{"program":"printf","args":["CATALOG_OK:%s\n"],"pass_arguments":true}}]}
CATALOG
chmod 600 "$fixture_repository/alias-lens/catalog.json"
git -C "$fixture_repository" -c init.defaultBranch=main init -q
git -C "$fixture_repository" add -- alias-lens/catalog.json
git -C "$fixture_repository" -c user.name=CatalogVerification -c user.email=verification@example.invalid commit -qm 'test: synthetic catalog'

for selected_shell in bash zsh; do
    disposable_home="$verification_root/$selected_shell-home"
    mkdir -m 700 "$disposable_home"
    env HOME="$disposable_home" ZDOTDIR= ALIAS_LENS_SHELL="$selected_shell" "$binary_path" plan --json init "$fixture_repository" --shell "$selected_shell" > "$verification_root/$selected_shell-plan.json"
    if [ -n "$(find "$disposable_home" -mindepth 1 -print -quit)" ]; then
        printf 'Init preview changed the disposable %s home.\n' "$selected_shell" >&2
        exit 1
    fi
    env HOME="$disposable_home" ZDOTDIR= ALIAS_LENS_SHELL="$selected_shell" "$binary_path" init "$fixture_repository" --shell "$selected_shell" --apply > "$verification_root/$selected_shell-init.txt"
    entry_output=$(env HOME="$disposable_home" ZDOTDIR= ALIAS_LENS_SHELL="$selected_shell" "$binary_path" shell-entry catalog_probe)
    case "$entry_output" in
        *catalog_probe*) ;;
        *) printf 'Installed %s entry was not available.\n' "$selected_shell" >&2; exit 1 ;;
    esac
    env HOME="$disposable_home" ZDOTDIR= ALIAS_LENS_SHELL="$selected_shell" "$binary_path" plan --json catalog rollback --shell "$selected_shell" > "$verification_root/$selected_shell-rollback-plan.json"
    env HOME="$disposable_home" ZDOTDIR= ALIAS_LENS_SHELL="$selected_shell" "$binary_path" catalog rollback --shell "$selected_shell" --apply > "$verification_root/$selected_shell-rollback.txt"
    original_missing_paths=(".$selected_shell"_aliases ".$selected_shell"rc)
    if [ "$selected_shell" = bash ]; then
        original_missing_paths+=(.bash_profile .bash_login .profile)
    fi
    for original_missing in "${original_missing_paths[@]}"; do
        if [ -e "$disposable_home/$original_missing" ]; then
            printf 'Rollback did not restore original absence of %s.\n' "$original_missing" >&2
            exit 1
        fi
    done
done

AL_REQUIRE_PTY_SHELLS=1 go test ./cmd/alias-lens ./internal/app ./internal/tui -run '^(TestCatalog.*PTY|Test.*Init.*PTY)' -count=1
printf 'Catalog bootstrap, read-only preview, installed entries, rollback, and Bash/Zsh PTY verification passed.\n'
