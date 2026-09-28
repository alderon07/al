# Keep workflow lint compatible with the project Go version

## Acceptance criteria

- The release candidate job runs actionlint with the Go version selected from `go.mod`.
- Actionlint checks every workflow file without changing the project's Go requirement.
- The release candidate job proceeds past workflow syntax validation.
- `make fmt check` passes.
