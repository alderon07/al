#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir"' EXIT

home_dir="$work_dir/home"
binary="$work_dir/alias-lens"
mkdir -p "$home_dir"
chmod 700 "$home_dir"
printf '%s\n' \
  '# List files' \
  '# al: tags=files platforms=linux,wsl favorite=true category=files' \
  "alias ll='ls -la'" \
  '' \
  'cproj() {' \
  ' cd "$HOME/code"' \
  '}' > "$home_dir/.bash_aliases"
chmod 600 "$home_dir/.bash_aliases"

manifest_hash() {
  local alias_hash
  alias_hash=$(sha256sum "$home_dir/.bash_aliases" | awk '{print $1}')
  printf '.bash_aliases 600 %s\n' "$alias_hash" | sha256sum | awk '{print $1}'
}

before_manifest=$(manifest_hash)
cd "$repo_root"
go build -buildvcs=false -o "$binary" ./cmd/alias-lens
HOME="$home_dir" "$binary" catalog shadow --shell bash > "$work_dir/plain.txt"
HOME="$home_dir" "$binary" catalog shadow --shell bash --json > "$work_dir/report.json"
after_manifest=$(manifest_hash)

plain_hash=$(sha256sum "$work_dir/plain.txt" | awk '{print $1}')
json_hash=$(sha256sum "$work_dir/report.json" | awk '{print $1}')
expected_file="$repo_root/cmd/alias-lens/testdata/phase4/wsl.sha256"
expected_manifest=$(awk '$1 == "manifest" {print $2}' "$expected_file")
expected_plain=$(awk '$1 == "plain" {print $2}' "$expected_file")
expected_json=$(awk '$1 == "json" {print $2}' "$expected_file")

if [[ "$before_manifest" != "$after_manifest" || "$after_manifest" != "$expected_manifest" ]]; then
  printf 'FAIL shadow-wsl: alias manifest changed or did not match the fixture\n' >&2
  exit 1
fi
if [[ "$plain_hash" != "$expected_plain" || "$json_hash" != "$expected_json" ]]; then
  printf 'FAIL shadow-wsl: report hash mismatch\n' >&2
  exit 1
fi

printf 'PASS shadow-wsl\n'
printf 'manifest %s\n' "$after_manifest"
printf 'plain %s\n' "$plain_hash"
printf 'json %s\n' "$json_hash"
