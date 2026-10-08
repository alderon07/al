# Go installation acceptance

- The module declares `github.com/alderon07/al`. Every Go import of an Alias Lens package uses that prefix, including platform-specific source and tests.
- The installed executable remains `alias-lens`, and shell integration continues to expose `al`. Private storage paths, command output contracts, dependency versions, and the minimum Go version remain unchanged.
- `make fmt check`, `go mod verify`, and `go mod tidy -diff` pass. Actual Bash and Zsh PTYs verify the installed command and existing setup behavior in disposable homes.
- Verify `go install github.com/alderon07/al/cmd/alias-lens@VERSION` from outside the checkout for the published candidate tag. Use a disposable home and installation directory, inspect the installed binary's module identity and selected version, and run its version/help commands. The request expanded to publishing an RC; its tag verification job owns this gate before artifact publication. A final stable release still requires verification with its published tag.
- Record medium implementation, separate high review, any corrections, and sanitized verification evidence. Preserve unrelated work, inspect staged paths, and scan for private data before committing.
