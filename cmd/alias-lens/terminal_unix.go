//go:build !windows

package main

import terminalui "github.com/alderon07/al/internal/tui"

func runCatalogConflictReview(id string) error {
	return terminalui.CatalogConflict(applicationServices(), id)
}
