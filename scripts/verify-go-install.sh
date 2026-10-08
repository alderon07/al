#!/usr/bin/env bash
set -euo pipefail
umask 077

candidate_version=${1:?Usage: verify-go-install.sh VERSION}
if [[ ! "$candidate_version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
    printf 'Expected a published semantic version, received %s.\n' "$candidate_version" >&2
    exit 1
fi
candidate_go=$(command -v go)
candidate_root=$(mktemp -d "${TMPDIR:-/tmp}/al-go-install.XXXXXX")
trap 'rm -rf "$candidate_root"' EXIT
mkdir -m 700 "$candidate_root/home" "$candidate_root/bin" "$candidate_root/work"
cd "$candidate_root/work"
env HOME="$candidate_root/home" GOBIN="$candidate_root/bin" GOWORK=off GOENV=off GOFLAGS= \
    GOPROXY=https://proxy.golang.org,direct GOSUMDB=sum.golang.org GOPRIVATE= GONOPROXY= GONOSUMDB= \
    "$candidate_go" install "github.com/alderon07/al/cmd/alias-lens@$candidate_version"
installed_binary="$candidate_root/bin/alias-lens"
"$candidate_go" version -m "$installed_binary" > "$candidate_root/build-info.txt"
if ! awk -v version="$candidate_version" '
    $1 == "path" && $2 == "github.com/alderon07/al/cmd/alias-lens" { path_ok = 1 }
    $1 == "mod" && $2 == "github.com/alderon07/al" && $3 == version { version_ok = 1 }
    END { exit !(path_ok && version_ok) }
' "$candidate_root/build-info.txt"; then
    printf 'Installed binary does not identify the requested module and version.\n' >&2
    exit 1
fi
env HOME="$candidate_root/home" "$installed_binary" --version
env HOME="$candidate_root/home" "$installed_binary" --help > "$candidate_root/help.txt"
test -s "$candidate_root/help.txt"
printf 'PASS go install github.com/alderon07/al/cmd/alias-lens@%s\n' "$candidate_version"
