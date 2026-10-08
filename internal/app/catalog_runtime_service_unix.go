//go:build !windows

package app

import (
	catalog "alias-lens/internal/catalog"
	"fmt"
	"path/filepath"
)

type CatalogRuntimeRequest struct {
	Shell  string
	Root   string
	Native string
}

func (svc *Services) CatalogRuntime(request CatalogRuntimeRequest) (string, error) {
	if !filepath.IsAbs(request.Root) || !filepath.IsAbs(request.Native) {
		return "", fmt.Errorf("catalog-loader needs pinned paths")
	}
	manifest, err := svc.readCatalogGeneration(request.Root, request.Native, request.Shell)
	if err != nil {
		return "", err
	}
	return catalogRuntimeHandoff(request.Shell, manifest.Entries), nil
}
func (svc *Services) Catalog() (catalog.Catalog, error) {
	path := svc.localCatalogPath()
	return readCatalogFile(path)
}
