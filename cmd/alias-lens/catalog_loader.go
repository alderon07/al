//go:build !windows

package main

import (
	"alias-lens/internal/app"
	"fmt"
	"os"
)

func runCatalogLoader(arguments []string) error {
	shell, root, native := "", "", ""
	for i := 0; i < len(arguments); i++ {
		if i+1 >= len(arguments) {
			return fmt.Errorf("incomplete catalog-loader options")
		}
		value := arguments[i+1]
		switch arguments[i] {
		case "--shell":
			shell = value
		case "--root":
			root = value
		case "--native":
			native = value
		default:
			return fmt.Errorf("unknown catalog-loader option")
		}
		i++
	}
	handoff, err := applicationServices().CatalogRuntime(app.CatalogRuntimeRequest{Shell: shell, Root: root, Native: native})
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(os.Stdout, handoff)
	return err
}
