//go:build !windows

package main

import (
	"os"
	"syscall"

	workflowplan "alias-lens/internal/plan"
)

func observePlanIdentity(path string) workflowplan.Identity {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return workflowplan.Identity{FileType: "missing"}
	}
	if err != nil {
		return workflowplan.Identity{FileType: "unreadable"}
	}
	identity := workflowplan.Identity{FileType: planFileType(info.Mode()), Mode: uint32(info.Mode().Perm())}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		identity.Device = uint64(stat.Dev)
		identity.Inode = uint64(stat.Ino)
		identity.LinkCount = uint64(stat.Nlink)
		identity.Owner = uint64(stat.Uid)
		identity.Group = uint64(stat.Gid)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		identity.LinkTarget, _ = os.Readlink(path)
	}
	return identity
}

func planFileType(mode os.FileMode) string {
	switch {
	case mode.IsRegular():
		return "regular"
	case mode&os.ModeSymlink != 0:
		return "symlink"
	case mode.IsDir():
		return "directory"
	default:
		return "other"
	}
}
