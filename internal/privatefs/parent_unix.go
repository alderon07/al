//go:build !windows

package privatefs

import (
	"runtime"

	"golang.org/x/sys/unix"
)

func OpenDirectoryComponent(parent int, name string, atRoot bool) (int, error) {
	flags := unix.O_RDONLY | unix.O_DIRECTORY | unix.O_NOFOLLOW | unix.O_CLOEXEC
	fd, err := unix.Openat(parent, name, flags, 0)
	if err == nil || runtime.GOOS != "darwin" || !atRoot || name != "tmp" && name != "var" {
		return fd, err
	}
	var link unix.Stat_t
	if e := unix.Fstatat(parent, name, &link, unix.AT_SYMLINK_NOFOLLOW); e != nil || link.Mode&unix.S_IFMT != unix.S_IFLNK || link.Uid != 0 {
		return -1, err
	}
	buffer := make([]byte, 256)
	n, e := unix.Readlinkat(parent, name, buffer)
	if e != nil || string(buffer[:n]) != "private/"+name && string(buffer[:n]) != "/private/"+name {
		return -1, err
	}
	private, e := unix.Openat(parent, "private", flags, 0)
	if e != nil {
		return -1, e
	}
	defer unix.Close(private)
	var info unix.Stat_t
	if e = unix.Fstat(private, &info); e != nil {
		return -1, e
	}
	if info.Uid != 0 || info.Mode&0o022 != 0 {
		return -1, unix.EPERM
	}
	return unix.Openat(private, name, flags, 0)
}
