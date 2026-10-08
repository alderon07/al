//go:build !windows

package main

import (
	"alias-lens/internal/app"
	"errors"
	"fmt"
)

func runCatalogCheck(strict bool) (int, error) {
	result, err := applicationServices().CheckCatalog()
	if err != nil {
		var invalid *app.CatalogCheckError
		if errors.As(err, &invalid) {
			return 1, err
		}
		return 2, err
	}
	fmt.Printf("Catalog checked: %d installed candidates, %d pending approvals, %d unavailable entries.\n", result.Installed, result.Pending, result.Unavailable)
	if strict && result.Pending+result.Unavailable > 0 {
		return 1, nil
	}
	return 0, nil
}
