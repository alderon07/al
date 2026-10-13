#!/usr/bin/env bash
set -euo pipefail
umask 077

distribution=${1:-dist}
archives=("$distribution"/*.tar.gz)
if [[ ${#archives[@]} -ne 4 ]]; then
  printf 'expected 4 release archives, found %d\n' "${#archives[@]}" >&2
  exit 1
fi

scan_directory=$(mktemp -d "${TMPDIR:-/tmp}/al-release-scan.XXXXXX")
trap 'rm -rf "$scan_directory"' EXIT

for archive in "${archives[@]}"; do
  binary_directory=$(mktemp -d "$scan_directory/binary.XXXXXX")
  tar -xzf "$archive" -C "$binary_directory" alias-lens
  if [[ ! -f "$binary_directory/alias-lens" || -L "$binary_directory/alias-lens" ]]; then
    printf '%s does not contain a regular alias-lens executable\n' "$archive" >&2
    exit 1
  fi
  printf 'Scanning %s\n' "$archive"
  go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 -mode=binary "$binary_directory/alias-lens"
done
