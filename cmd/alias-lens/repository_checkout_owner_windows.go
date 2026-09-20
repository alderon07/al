//go:build windows

package main

import "os"

func managedRepositoryDirectoryOwnedByUser(os.FileInfo) bool {
	return true
}
