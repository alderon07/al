# Remove unused migration paths

## Goal

Alias Lens has no released data formats and no users other than its developer. The code should define the current configuration and catalog formats directly instead of carrying migration commands for unreleased formats.

## Acceptance criteria

- Configuration files must use the current version, version 2. Missing, older, and newer versions produce an actionable error without changing the file.
- Catalog files must use the current schema, version 2. Version 1 is rejected and is not decoded or rewritten.
- The `al config migrate`, `al catalog migrate`, and matching `al plan` forms are removed from routing, help, and completion.
- Profile changes update only a current configuration and do not create migration-only backups.
- Current configuration, catalog import, catalog diff, status, and profile workflows keep working.
- Report schemas, transaction journal schemas, and the normal alias-file workflow remain unchanged. They are current contracts, not migration paths.
