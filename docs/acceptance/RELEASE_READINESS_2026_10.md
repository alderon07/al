# Release readiness acceptance criteria

- Preserve concurrent workspace edits and prepare the release from an isolated, committed checkout.
- Make normal repository commands and every CI/release job select patched Go 1.27.2. Raise the language minimum only as required by security-fixed dependencies.
- Reproduce the unpatched toolchain selection before changing `go.mod`. Verify automatic selection, compiled build metadata, and source and binary vulnerability scans afterward.
- Review dependency advisories and available updates. Change dependencies only when a security finding or release defect warrants it.
- Remove GO-2026-5970, GO-2026-6629, and GO-2026-5024 by selecting fixed `golang.org/x/text` and `golang.org/x/sys` versions. Record their Go 1.25.0 minimum and update installation instructions.
- Require binary vulnerability scans of all four archived executables in both the release-candidate CI job and the release build job before artifacts reach publication. Fail on scanner errors or findings.
- Pass the full required-shell suite, repeated PTYs, non-PTY race checks, module checks, Actionlint, and `make fmt check` on the prepared tree.
- Verify GoReleaser 2.18.0 and Syft 1.42.3 downloads against their published checksums. Build four archives and four SPDX SBOMs, then verify checksums, required contents, build metadata, and native Linux archive behavior.
- Exercise the compiled terminal at 48x18, 80x24, and 160x40, including tmux, with isolated synthetic data.
- Run the Bash and Zsh catalog workflow and preserve the WSL shadow hashes.
- Obtain Linux and native macOS branch CI results for the exact prepared commit. Keep packaging and publication permissions separate.
- Record remaining WSL restart, Windows Terminal, native architecture, and macOS manual gates without marking them complete from cross-builds or CI alone.
- Prepare the selected candidate and its review evidence. Do not publish a stable release while mandatory manual platform gates remain open.
- Prioritize WSL readiness. Prepare persistent synthetic Bash and Zsh homes and reproducible pre/post-restart checks for the exact candidate, and record Windows Terminal evidence separately from Unix PTYs.

## PTY launch reliability

- Reproduce the intermittent controlling-terminal launch error on the existing launcher before changing fixtures. Preserve the diagnostic failures in the evidence.
- Share a Linux PTY fixture launcher that opens the master and its descriptor-linked slave with `O_NOCTTY` and close-on-exec. Keep the existing launcher on other Unix systems.
- Preserve terminal size, existing process attributes, caller-provided standard streams, process-group cancellation, and start-error cleanup. Do not retry failed launches or skip required shells.
- Verify controlling-terminal ownership and terminal dimensions with real Bash PTYs. Verify that failed starts do not leak descriptors.
- Run the failing startup case repeatedly under the same focused tracing and run all required-shell PTYs afterward. Keep the exact kernel cause distinct from the verified launcher behavior.
