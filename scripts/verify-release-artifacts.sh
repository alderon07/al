#!/usr/bin/env bash
set -euo pipefail

distribution=${1:-dist}
archives=("$distribution"/*.tar.gz)
sboms=("$distribution"/*.sbom.json)
required=(alias-lens LICENSE NOTICE README.md THIRD_PARTY_LICENSES.txt THIRD_PARTY_NOTICES.md)

if [[ ${#archives[@]} -ne 4 ]]; then
  printf 'expected 4 release archives, found %d\n' "${#archives[@]}" >&2
  exit 1
fi
if [[ ${#sboms[@]} -ne 4 ]]; then
  printf 'expected 4 archive SBOMs, found %d\n' "${#sboms[@]}" >&2
  exit 1
fi
if [[ ! -f "$distribution/checksums.txt" ]]; then
  printf 'missing %s/checksums.txt\n' "$distribution" >&2
  exit 1
fi

(
  cd "$distribution"
  sha256sum -c checksums.txt
)

for archive in "${archives[@]}"; do
  contents=$(tar -tzf "$archive")
  for filename in "${required[@]}"; do
    if ! grep -Fxq "$filename" <<<"$contents"; then
      printf '%s is missing %s\n' "$archive" "$filename" >&2
      exit 1
    fi
  done
done

for sbom in "${sboms[@]}"; do
  if ! grep -Fq '"spdxVersion"' "$sbom"; then
    printf '%s is not an SPDX JSON document\n' "$sbom" >&2
    exit 1
  fi
done
