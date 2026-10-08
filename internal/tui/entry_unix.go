//go:build !windows

package tui

import "github.com/alderon07/al/internal/app"

func CatalogConflict(services *app.Services, id string) error {
	return runCatalogConflictReview(services, id)
}
