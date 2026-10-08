# Release candidate gates

- Every release tag runs Linux and native macOS checks with required system Bash and Zsh, private temporary directories, the full test suite, and the disposable catalog workflow verifier. Missing shells fail verification.
- Building and publishing release artifacts depend on both platform verification jobs. Preserve the separate read-only build and privileged publication jobs, pinned actions, artifact checksums, SBOMs, and attestations.
- Verify the public Go installation route for the exact tag from outside the checkout in a disposable home and installation directory. Inspect the installed module path and version, and smoke-test version/help without touching real shell files.
- Publish only an explicitly identified release candidate, initially `v1.0.0-rc.1`, after review, full checks, PTYs, and branch CI pass. Verify the resulting prerelease, archives, checksums, SBOMs, and attestations. Do not create stable `v1.0.0` before dated manual platform evidence is recorded.
- Record a separate high review, confirmed corrections, and verification evidence. Keep the unrelated performance evidence edit intact.
- Every job that runs the full checks, including the fresh artifact build job, supplies system Bash and Zsh, uses a private temporary directory, and sets `AL_REQUIRE_PTY_SHELLS=1`. An earlier job's shell installation does not satisfy a later job's checks.
- The publication job resolves the repository explicitly through `GH_REPO` while remaining checkout-free. Verify repository resolution with a read-only GitHub CLI command from a directory without Git metadata.
