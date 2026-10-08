//go:build !windows

package main

import (
	shellapi "alias-lens/internal/shell"

	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	neutralcatalog "alias-lens/internal/catalog"
)

func resolveCatalogExecutable(program, searchPath string) (string, error) {
	if searchPath == "" {
		return "", fmt.Errorf("PATH is empty; set an absolute PATH before al catalog enable")
	}
	directories := filepath.SplitList(searchPath)
	for _, directory := range directories {
		if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
			return "", fmt.Errorf("PATH contains an empty or relative directory; set an absolute PATH before al catalog enable")
		}
	}
	for _, directory := range directories {
		candidate := filepath.Join(directory, program)
		info, err := os.Stat(candidate)
		if err == nil && info.Mode().IsRegular() {
			if _, err := exec.LookPath(candidate); err != nil {
				continue
			}
			resolved, err := filepath.EvalSymlinks(candidate)
			if err != nil {
				return "", fmt.Errorf("cannot pin the executable for %s", program)
			}
			return resolved, nil
		}
	}
	return "", fmt.Errorf("%s is unavailable in the captured PATH", program)
}
func catalogProtectedName(name string) bool { return shellapi.CatalogProtectedName(name) }
func validateCatalogNames(value neutralcatalog.Catalog) error {
	return shellapi.ValidateCatalogNames(value)
}
func validateCatalogDeclaration(shell string, entry neutralcatalog.Entry, declaration []byte) error {
	return shellapi.ValidateCatalogDeclaration(shell, entry, declaration)
}
func validateCatalogNativeControls(shell string, native []byte) error {
	return shellapi.ValidateCatalogNativeControls(shell, native)
}
func validateCatalogDeclarations(shell string, entries []neutralcatalog.Entry, declarations [][]byte, native []byte) error {
	return shellapi.ValidateCatalogDeclarations(shell, entries, declarations, native, shadowSyntaxValidator)
}
func validateCatalogNativeDeclaration(shell string, declaration []byte) error {
	return shellapi.ValidateCatalogNativeDeclaration(shell, declaration)
}
