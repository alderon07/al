# Release candidate gates

- Every release tag runs Linux and native macOS checks with required system Bash and Zsh, private temporary directories, the full test suite, and the disposable catalog workflow verifier. Missing shells fail verification.
- Building and publishing release artifacts depend on both platform verification jobs. Preserve the separate read-only build and privileged publication jobs, pinned actions, artifact checksums, SBOMs, and attestations.
- Verify the public Go installation route for the exact tag from outside the checkout in a disposable home and installation directory. Inspect the installed module path and version, and smoke-test version/help without touching real shell files.
- Publish only an explicitly identified release candidate, initially `v1.0.0-rc.1`, after review, full checks, PTYs, and branch CI pass. Verify the resulting prerelease, archives, checksums, SBOMs, and attestations. Do not create stable `v1.0.0` before dated manual platform evidence is recorded.
- Record a separate high review, confirmed corrections, and verification evidence. Keep the unrelated performance evidence edit intact.
