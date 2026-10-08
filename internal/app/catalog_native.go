//go:build !windows

package app

import (
	"fmt"
	"github.com/alderon07/al/internal/transaction"
	"os"
	"path/filepath"
)

func readCatalogNative(path string) ([]byte, string, error) {
	identity, err := transaction.InspectWorkflowTarget(path, ShadowSourceLimit, true)
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
	contents, err := readRegularFile(resolved, ShadowSourceLimit)
	if err != nil {
		return nil, "", err
	}
	if hashBytes(contents) != identity.SHA256 {
		return nil, "", fmt.Errorf("native file changed while reading; review al catalog enable again")
	}
	return contents, resolved, nil
}
