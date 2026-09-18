//go:build windows

package main

import "fmt"

func runCatalogCommand(arguments []string) (int, error) {
	return 2, fmt.Errorf("catalog shadow inspection supports Bash and Zsh on Linux, WSL, and macOS")
}
