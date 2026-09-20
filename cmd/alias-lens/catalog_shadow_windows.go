//go:build windows

package main

import "fmt"

func runCatalogCommand(arguments []string) (int, error) {
	if handled, code, err := runCatalogWorkflowCommand(arguments); handled {
		return code, err
	}
	if len(arguments) > 0 && arguments[0] == "shadow" {
		return 2, fmt.Errorf("catalog shadow inspection supports Bash and Zsh on Linux, WSL, and macOS")
	}
	return 2, catalogCommandUsageError()
}
