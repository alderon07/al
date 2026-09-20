//go:build !windows

package main

import (
	"os"
	"syscall"
)

func managedRepositoryDirectoryOwnedByUser(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Geteuid())
}
