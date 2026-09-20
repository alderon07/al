//go:build windows

package main

import (
	"os"

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
	identity := workflowplan.Identity{FileType: planFileType(info.Mode()), Mode: uint32(info.Mode().Perm()), LinkCount: 1}
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
