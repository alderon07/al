# Release candidate publication

The candidate is `v1.0.0-rc.4`, commit `69f669871285845d618b047b7944d9993954f4a0`. The tag was created from a clean managed checkout after [all six branch CI jobs passed](https://github.com/alderon07/al/actions/runs/37852815479). Unrelated edits in the primary checkout were preserved.

Earlier tags remain immutable. `rc.1` stopped before building because public installation succeeded but disposable module-cache cleanup failed. `rc.2` passed both native exact-tag gates, then stopped because the separate build runner lacked Zsh. `rc.3` exposed a partial-output race in the help-footer PTY test. Those runs published no assets. Reviewed corrections supply writable owned Go caches, required shells and private temporary directories for full-check jobs, explicit repository context for checkout-free publication and a complete-divider PTY wait. The wait passed 50 real PTY repetitions and deterministic fragmented-output cases. Runtime layout and performance code were unchanged by these corrections.

## Public Go installation

Exact-tag RC4 verification passed outside the checkout through the public module proxy and checksum service. It verified package `github.com/alderon07/al/cmd/alias-lens`, module `github.com/alderon07/al`, exact version, version/help output and successful cleanup. The command exited zero.

```text
alias-lens 1.0.0-rc.4
PASS go install github.com/alderon07/al/cmd/alias-lens@v1.0.0-rc.4
```

## Publication verification

The [RC4 release workflow](https://github.com/alderon07/al/actions/runs/37853100800) passed both native verification jobs, the fresh build job and publication/attestation signing. The [published release](https://github.com/alderon07/al/releases/tag/v1.0.0-rc.4) is a prerelease, not a draft. Its reviewed installation notes identify the remaining stable gates.

All nine assets were downloaded independently. Their sizes and SHA-256 digests matched public release metadata. The release verifier passed all eight listed archive/SBOM checksums, required archive contents, four-archive/four-SBOM counts and SPDX JSON checks. The checksum file's own SHA-256 is `ecfc97804916a07e69641a1fca852ffc940acb7a345cf7e96530d4502408e3bc`, also matched to public metadata.

Provenance verification passed for all four archives, all four SBOMs and the checksum file with this policy:

```bash
gh attestation verify ASSET --repo alderon07/al \
  --signer-workflow alderon07/al/.github/workflows/release.yml \
  --source-ref refs/tags/v1.0.0-rc.4 \
  --source-digest 69f669871285845d618b047b7944d9993954f4a0 \
  --deny-self-hosted-runners
```

## Native Linux archive

The attested `linux_amd64` archive passed setup, repair and removal in private synthetic Bash and Zsh homes. Both actual shells passed login and nonlogin PTYs, invoking only the synthetic `rc_probe` alias and `al --version`, with expected output, version and zero shell exit status. Removal retained the synthetic alias and mode `0600` and removed the integration. Doctor reported binary, path, alias syntax, startup loading, shell actions and Git as healthy; it exited one for intentionally unconfigured repository, autosync and provider tooling. This was not a fully configured sync test.

The supplemental Zsh harness initially set `ZDOTDIR` to an empty string and did not load its intended startup fixture. It was corrected to the fixture home and synchronized against a startup marker. It used the existing test-only completion initialization. No product change was needed. Failed harness logs and corrected results were retained privately; this record does not count the failed harness as passing.

Host: Linux `7.0.0-34-generic`, x86-64, Go `1.27.1`, Bash `5.2.21`, disposable Zsh `5.9`. Native release CI separately used system Bash and Zsh on Linux and macOS. The other three archives passed integrity/content/provenance inspection; they were not executed on this host.

## Stable release gates

Stable `v1.0.0` remains untagged. Native macOS Terminal.app, Intel and Apple silicon package checks, WSL restart and Windows Terminal checks, and native Linux ARM64 package checks still require dated results. CI results and disposable Linux shell tests do not complete those manual gates. Native macOS and WSL performance remain unmeasured.
