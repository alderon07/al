# Releasing Alias Lens

Alias Lens needs a stable GitHub release before it can be installed from a Homebrew formula. The release workflow builds `alias-lens` for macOS and Linux on Intel and ARM, publishes the archives, and adds `checksums.txt` to the GitHub release.

## Prepare the first release

1. Add an open source `LICENSE` file and replace `LICENSE` in `packaging/homebrew/alias-lens.rb.tmpl` with its Homebrew SPDX identifier.
2. Run the repository checks:

   ```bash
   gofmt -w *.go
   go test ./...
   go vet ./...
   go build -buildvcs=false -o /tmp/alias-lens-release .
   git diff --check
   ```

3. Commit and push the release setup.
4. Tag a stable semantic version and push it:

   ```bash
   git tag -a v0.1.0 -m "Alias Lens v0.1.0"
   git push origin v0.1.0
   ```

The tag starts `.github/workflows/release.yml`. Confirm that the GitHub release contains four archives and `checksums.txt` before publishing a formula.

## Publish through a personal tap

A personal tap works before the project qualifies for `homebrew/core`.

1. Create a public GitHub repository named `homebrew-tap` under `alderon07`.
2. Download the immutable source archive and calculate its checksum:

   ```bash
   curl -LO https://github.com/alderon07/al/archive/refs/tags/v0.1.0.tar.gz
   shasum -a 256 v0.1.0.tar.gz
   ```

3. Copy `packaging/homebrew/alias-lens.rb.tmpl` to `Formula/alias-lens.rb` in the tap. Replace `VERSION`, `SHA256`, and `LICENSE` with the release version, checksum, and SPDX license identifier.
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

Submit the source formula to `Homebrew/homebrew-core` once Alias Lens has a stable release, a compatible open source license, and enough independent usage to meet Homebrew's package acceptance policy. Homebrew currently expects a self-submitted GitHub project to have at least 90 forks, 90 watchers, or 225 stars. Until then, the personal tap gives users the same `brew install` and `brew upgrade` workflow.
