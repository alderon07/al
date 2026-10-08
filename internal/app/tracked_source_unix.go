//go:build linux || darwin

package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

func openTrustedTrackedSource(path string, validate func(string) error) (*os.File, string, error) {
	if !filepath.IsAbs(path) {
		return nil, "", fmt.Errorf("tracked source must be absolute")
	}
	path = filepath.Clean(path)
	if err := validate(path); err != nil {
		return nil, "", err
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, "", err
	}
	defer func() { unix.Close(fd) }()
	if err := validateTrackedSourceMetadata(fd); err != nil {
		return nil, "", err
	}
	remaining := strings.Split(strings.TrimPrefix(path, "/"), "/")
	resolved := "/"
	links := 0
	for len(remaining) > 0 {
		part := remaining[0]
		remaining = remaining[1:]
		if part == "" || part == "." {
			continue
		}
		candidate := filepath.Join(resolved, part)
		if len(remaining) == 0 {
			if err := validate(candidate); err != nil {
				return nil, "", err
			}
		}
		var stat unix.Stat_t
		if err := unix.Fstatat(fd, part, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			return nil, "", &os.PathError{Op: "inspect tracked source", Path: path, Err: err}
		}
		if stat.Mode&unix.S_IFMT == unix.S_IFLNK {
			links++
			if links > 40 || stat.Uid != uint32(os.Geteuid()) && stat.Uid != 0 {
				return nil, "", fmt.Errorf("tracked source has an unsafe symlink; run al untrack FILE")
			}
			buffer := make([]byte, 4096)
			n, err := unix.Readlinkat(fd, part, buffer)
			if err != nil || n == len(buffer) {
				return nil, "", fmt.Errorf("cannot safely resolve tracked source symlink")
			}
			target := string(buffer[:n])
			if !filepath.IsAbs(target) {
				target = resolved + "/" + target
			}
			validationPath := target
			if len(remaining) > 0 {
				validationPath += "/" + strings.Join(remaining, "/")
			}
			if err := validate(validationPath); err != nil {
				return nil, "", err
			}
			remaining = append(strings.Split(strings.TrimPrefix(target, "/"), "/"), remaining...)
			next, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
			if err != nil {
				return nil, "", err
			}
			unix.Close(fd)
			fd = next
			resolved = "/"
			continue
		}
		flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW | unix.O_NONBLOCK
		if len(remaining) > 0 {
			flags |= unix.O_DIRECTORY
		}
		next, err := unix.Openat(fd, part, flags, 0)
		if err != nil {
			return nil, "", &os.PathError{Op: "open tracked source", Path: path, Err: err}
		}
		if err = unix.Fstat(next, &stat); err != nil {
			unix.Close(next)
			return nil, "", err
		}
		if err := validateTrackedSourceMetadata(next); err != nil {
			unix.Close(next)
			return nil, "", err
		}
		if len(remaining) > 0 {
			stickyRoot := stat.Uid == 0 && stat.Mode&unix.S_ISVTX != 0
			if stat.Mode&unix.S_IFMT != unix.S_IFDIR || stat.Uid != 0 && stat.Uid != uint32(os.Geteuid()) || stat.Mode&0o022 != 0 && !stickyRoot {
				unix.Close(next)
				return nil, "", fmt.Errorf("tracked source has an unsafe parent directory; run al untrack FILE")
			}
			unix.Close(fd)
			fd = next
			resolved = candidate
			continue
		}
		if stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Uid != uint32(os.Geteuid()) || stat.Mode&0o022 != 0 || stat.Nlink != 1 {
			unix.Close(next)
			return nil, "", fmt.Errorf("tracked source has unsafe ownership, permissions, or links; run al untrack FILE")
		}
		return os.NewFile(uintptr(next), path), candidate, nil
	}
	return nil, "", errors.New("tracked source is not a regular file")
}
