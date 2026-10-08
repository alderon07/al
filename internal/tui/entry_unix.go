//go:build !windows

package tui

import "alias-lens/internal/app"

func CatalogConflict(services *app.Services, id string) error {
	return runCatalogConflictReview(services, id)
}
