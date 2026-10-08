# Catalog bootstrap acceptance

Approved by the implementation request on 2026-10-03. `docs/CATALOG_PLAN.md` controls this first-release implementation. Maps to SW-011, SW-013, and SW-014.

## CB-001 local enrollment

Preview reads the selected relative catalog path and repository HEAD without modifying the repository or managed state. Reject unsafe paths, links, malformed catalogs, secrets, collisions, and incompatible existing repository configuration. Pin working-tree bytes separately from HEAD; dirty working-tree catalogs are not falsely attributed to the committed blob. Interactive cancellation saves nothing. Apply rechecks identities and bytes under the mutation lock and uses the normal lifecycle writer. The existing repository is never copied, deleted, or recloned. Tests compare recursive manifests and staged index entries before and after preview, cancellation, stale application, and successful installation. PTY evidence covers native review and final confirmation.

## CB-002 immutable provider preview

GitHub, GitLab, and Bitbucket use the configured HTTPS host to obtain immutable revision, Git blob identity, and bounded file bytes. Reject locator userinfo, traversal, query, fragment, and cross-host redirects. Never print or persist credentials. Discovery has a two-minute deadline and an 8 MiB catalog limit. A provider without immutable proof fails with local enrollment guidance. Tests use synthetic credentials and cover absent/invalid credentials, immutable mismatches, redirects, bounds, configured hosts, write-access filtering, pagination where applicable, and SSH transport fallback.

## CB-003 staged remote installation

Remote application rechecks the preview and clones to a fresh operation-owned staging directory on the final destination filesystem. Use an owned empty hooks path, no checkout, no submodules, depth one, and blob filtering. Verify filtering capability rather than accepting clone exit alone. Use Git plumbing to read and index only the enrolled catalog path without filters; materialize that file through the safe writer. Pin clone revision and blob to preview. Bound clone to five minutes and staging to 256 MiB; terminate its process group on failure or cancellation. Journal promotion and remove only proven operation-owned artifacts. Tests include malicious hooks/filters/LFS/submodules/config, capability refusal, limits, stale remote objects, existing destinations, promotion crashes, repeated recovery, and unchanged unrelated paths. Subsequent managed sync keeps the same restrictions and refuses unproven fast-forward ancestry.

## Native approvals

Init composes the same per-entry review as enable. In-memory decisions bind exact content, name, ID, kind, shell, and renderer. Persist them only with final confirmed application. `--apply` never grants native approval or ownership. Valid new pending entries may remain inactive; an unavailable replacement of an installed entry blocks re-enable. Invalid declarations block installation. Startup, preview, import, review, sync, and bootstrap never execute user definitions.

Bitbucket proof uses immutable source metadata with exact commit hash, path, commit_file type and bounded size, then raw bytes fetched from the same pinned API authority and commit/path. Link, subrepository, binary and other unsupported attributes are refused. The Git blob ID is computed from the raw bytes, and the later restricted clone must independently match revision, tree path and blob before installation. Source API ETags are not treated as Git blob IDs. This proof chain follows the [official Bitbucket source API](https://developer.atlassian.com/cloud/bitbucket/rest/api-group-source/#api-repositories-workspace-repo-slug-src-commit-path-get).

Compiled CLI PTY tests run the actual `al init` entry point in disposable private homes. They approve a native implementation and ownership range, then cancel final confirmation and verify catalog, enrollment, approvals, ownership and startup bytes remain absent or unchanged. Successful portable `--apply` enrollment starts a fresh Bash shell and invokes the installed definition through the generation loader. Both paths preserve unrelated repository index and worktree content.

Normal compiled Bash bootstrap is tested against the host's root-owned system validator. The normal compiled Zsh test requires root-owned `/bin/zsh` or `/usr/bin/zsh`; missing validators skip locally and fail when `AL_REQUIRE_PTY_SHELLS=1`. A separate helper subprocess captures the actual Bash/Zsh runtime and overrides only its test validator resolver, exercising native review, ownership, final cancellation and successful installation. It is recorded as helper evidence, not normal compiled Zsh release evidence. No production trust checks are changed.

## CB-004 completion isolation in bootstrap PTYs

Fresh-shell bootstrap PTYs must reach the prompt and run the installed entry when the host has insecure Zsh completion directories. The disposable startup fixture initializes completions while excluding insecure directories; it must not accept or execute their completion functions, disable completion registration, weaken production validator checks or alter generated startup code. A synthetic insecure completion directory reproduces the prompt boundary and verifies safe exclusion. Required CI selectors still run normal compiled Bash/Zsh bootstrap, native review, final cancellation and successful application.
