//go:build windows

package app

import "os"

func managedRepositoryDirectoryOwnedByUser(os.FileInfo) bool {
	return true
}
