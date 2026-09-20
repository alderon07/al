//go:build !windows

package transaction

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func openRegularNoFollow(path string) (*os.File, error) {
	descriptor, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(descriptor), path)
	if file == nil {
		_ = unix.Close(descriptor)
		return nil, fmt.Errorf("open private file descriptor")
	}
	return file, nil
}

func createPrivateNoFollow(path string) (*os.File, error) {
	descriptor, err := unix.Open(path, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(descriptor), path)
	if file == nil {
		_ = unix.Close(descriptor)
		return nil, fmt.Errorf("create private file descriptor")
	}
	return file, nil
}

func openLockFile(path string) (*os.File, error) {
	descriptor, err := unix.Open(path, unix.O_RDWR|unix.O_CREAT|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(descriptor), path)
	if file == nil {
		_ = unix.Close(descriptor)
		return nil, fmt.Errorf("open lock file descriptor")
	}
	return file, nil
}

func tryLock(file *os.File) error {
	err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
		return ErrLocked
	}
	return err
}

func unlock(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_UN)
}

func identityFromInfo(info os.FileInfo) (FileIdentity, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return FileIdentity{}, fmt.Errorf("%w: file identity is unavailable", ErrUnsafePath)
	}
	return FileIdentity{
		Exists:     true,
		PlatformID: fmt.Sprintf("%d:%d", uint64(stat.Dev), uint64(stat.Ino)),
		Size:       info.Size(),
		Mode:       uint32(info.Mode().Perm()),
		Owner:      uint64(stat.Uid),
		Group:      uint64(stat.Gid),
		Links:      uint64(stat.Nlink),
	}, nil
}

func validateDirectoryOwner(info os.FileInfo) error {
	identity, err := identityFromInfo(info)
	if err != nil {
		return err
	}
	if identity.Owner != uint64(os.Geteuid()) {
		return fmt.Errorf("%w: private directory owner does not match the current user", ErrUnsafePath)
	}
	return nil
}

func validateFileOwner(identity FileIdentity) error {
	if identity.Owner != uint64(os.Geteuid()) {
		return fmt.Errorf("%w: private file owner does not match the current user", ErrUnsafePath)
	}
	return nil
}

func privateDirectoryModeValid(mode os.FileMode) bool { return mode.Perm() == 0o700 }
func privateFileModeValid(mode os.FileMode) bool      { return mode.Perm() == 0o600 }

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil && !errors.Is(err, syscall.EINVAL) && !errors.Is(err, syscall.ENOTSUP) {
		return err
	}
	return nil
}
