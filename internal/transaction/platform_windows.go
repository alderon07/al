//go:build windows

package transaction

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func rejectReparsePoint(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: private path is a symbolic link", ErrUnsafePath)
	}
	return nil
}

func openRegularNoFollow(path string) (*os.File, error) {
	if err := rejectReparsePoint(path); err != nil {
		return nil, err
	}
	return os.Open(path)
}

func createPrivateNoFollow(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
}

func openLockFile(path string) (*os.File, error) {
	if _, err := os.Lstat(path); err == nil {
		if err := rejectReparsePoint(path); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
}

func tryLock(file *os.File) error {
	overlapped := new(windows.Overlapped)
	err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, overlapped)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return ErrLocked
	}
	return err
}

func unlock(file *os.File) error {
	return windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, new(windows.Overlapped))
}

func identityFromInfo(info os.FileInfo) (FileIdentity, error) {
	return FileIdentity{
		Exists: true,
		Size:   info.Size(),
		Mode:   uint32(info.Mode().Perm()),
		Links:  1,
	}, nil
}

func validateDirectoryOwner(os.FileInfo) error   { return nil }
func validateFileOwner(FileIdentity) error       { return nil }
func syncDirectory(string) error                 { return nil }
func privateDirectoryModeValid(os.FileMode) bool { return true }
func privateFileModeValid(os.FileMode) bool      { return true }
