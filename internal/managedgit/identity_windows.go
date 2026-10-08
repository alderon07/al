//go:build windows

package managedgit

import (
	"os"
)

func observePlanIdentity(path string) fileIdentity {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return fileIdentity{FileType: "missing"}
	}
	if err != nil {
		return fileIdentity{FileType: "unreadable"}
	}
	identity := fileIdentity{FileType: planFileType(info.Mode()), Mode: uint32(info.Mode().Perm()), LinkCount: 1}
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
