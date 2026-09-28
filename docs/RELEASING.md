# Releasing Alias Lens

Alias Lens needs a stable GitHub release before it can be installed from a Homebrew formula. The release workflow builds `alias-lens` for macOS and Linux on Intel and ARM, publishes the archives, and adds `checksums.txt` to the GitHub release.

The first supported release routes are GitHub archives and `go install github.com/alderon07/al/cmd/alias-lens@latest`. WSL uses the Linux archive. Native Windows shells are not supported, so do not publish a Windows package until their parser, writer, startup integration, execution behavior, and key bindings are implemented and tested.

Read [the compatibility policy](COMPATIBILITY.md) before changing commands, flags, metadata, JSON output, or configuration fields.

## Prepare a release candidate

1. Choose one commit for every platform check. Push it without a tag.
2. Complete the platform checklists and save dated, sanitized evidence:

   - [Linux](testing/RELEASE_LINUX.md)
   - [WSL 2](testing/RELEASE_WSL.md)
   - [macOS](testing/RELEASE_MACOS.md)

3. Run the repository checks:

   ```bash
   make fmt check
   go test -race -count=1 -skip PTY ./...
   go mod verify
   go mod tidy -diff
   go list -m -u -mod=readonly all
   govulncheck ./...
   actionlint .github/workflows/*.yml
   goreleaser check
   ```

   Review available dependency updates. An update is not automatically a release blocker, but every vulnerability finding needs a written decision.

4. Confirm that every GitHub Actions job passes for the release commit.
5. Review `git status --short`, `git diff --check`, and the files that the release archives contain. Do not tag a dirty worktree.
6. Inspect the staged file list and diff for tokens, private keys, shell alias files, environment files, and local config before the final commit and push.
7. Tag a stable semantic version and push it:

   ```bash
   git tag -a v1.0.0 -m "Alias Lens v1.0.0"
   git push origin v1.0.0
   ```

The tag starts `.github/workflows/release.yml`.

## Checks for the release owner

These checks require repository access, another operating system, or a decision about the public release. They cannot be completed by one Linux preflight run.

- [ ] Confirm that the release commit is pushed, immutable, and identical on Linux, WSL, and macOS.
- [ ] Review the generated release notes. Add migration, compatibility, security, or known-issue notes that GitHub cannot infer from commit titles.
- [ ] Confirm that every required GitHub Actions job passes for the tag.
- [ ] Confirm that the GitHub release contains four archives, four SPDX JSON SBOMs, and `checksums.txt`.
- [ ] Verify `checksums.txt` on Linux and macOS.
- [ ] Extract and smoke-test each archive on its native architecture. Do not treat cross-compilation as an ARM64 or Intel runtime test.
- [ ] Verify each archive and SBOM with `gh attestation verify FILE --repo alderon07/al`.
- [ ] Test `go install github.com/alderon07/al/cmd/alias-lens@VERSION` in a temporary Go environment.
- [ ] Publish or update the personal Homebrew tap only after the macOS checklist passes.
- [ ] Confirm that the install commands and release links in `README.md` work for the published tag.
- [ ] Keep the release as a prerelease until every required manual result has dated evidence. Publish it only after reviewing all failures and exceptions.

The personal Homebrew tap is a separate repository. Keep AUR, Debian, RPM, and other package repositories as later distribution work rather than blocking the first release.

## Publish through a personal tap

A personal tap works before the project qualifies for `homebrew/core`.

1. Create a public GitHub repository named `homebrew-tap` under `alderon07`.
2. Download the immutable source archive and calculate its checksum:

   ```bash
   curl -LO https://github.com/alderon07/al/archive/refs/tags/v1.0.0.tar.gz
   shasum -a 256 v1.0.0.tar.gz
   ```

3. Copy `packaging/homebrew/alias-lens.rb.tmpl` to `Formula/alias-lens.rb` in the tap. Replace `VERSION` and `SHA256` with the release version and checksum.
4. Test the formula on macOS and Linux:

   ```bash
   brew install --build-from-source ./Formula/alias-lens.rb
   brew test ./Formula/alias-lens.rb
   brew audit --strict ./Formula/alias-lens.rb
   ```

5. Push the formula to the tap. Users can then install it with:

   ```bash
   brew install alderon07/tap/alias-lens
   alias-lens setup
   ```

Update the formula URL and SHA-256 checksum for every release. `brew bump-formula-pr` can prepare that update after the first formula exists.

## Submit to homebrew/core

Submit the source formula to `Homebrew/homebrew-core` once Alias Lens has a stable release, a compatible open source license, and enough independent usage to meet [Homebrew's package acceptance policy](https://docs.brew.sh/Package-Acceptance-Policy). Homebrew currently expects a self-submitted GitHub project to have at least 90 forks, 90 watchers, or 225 stars. Until then, the personal tap gives users the same `brew install` and `brew upgrade` workflow.
