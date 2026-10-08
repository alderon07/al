//go:build !windows

package main

import terminalui "alias-lens/internal/tui"

func runCatalogConflictReview(id string) error {
	return terminalui.CatalogConflict(applicationServices(), id)
}
