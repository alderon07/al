//go:build !windows

package main

import (
	"alias-lens/internal/transaction"
	"fmt"
	"os"
	"path/filepath"
)

func readCatalogNative(path string) ([]byte, string, error) {
	identity, err := transaction.InspectWorkflowTarget(path, shadowSourceLimit, true)
	if err != nil {
		return nil, "", err
	}
	if !identity.Exists {
		return nil, path, os.ErrNotExist
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, "", err
	}
	contents, err := readRegularFile(resolved, shadowSourceLimit)
	if err != nil {
		return nil, "", err
	}
	if hashBytes(contents) != identity.SHA256 {
		return nil, "", fmt.Errorf("native file changed while reading; review al catalog enable again")
	}
	return contents, resolved, nil
}
