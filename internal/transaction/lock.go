package transaction

import (
	"errors"
	"fmt"
	"os"
)

// Lock is an operating-system-backed exclusive mutation lock. The lock file is
// persistent. Close releases the kernel lock but never removes or replaces the
// inode, so age cannot create two lock owners.
type Lock struct {
	file *os.File
}

func AcquireLock(root, path string) (*Lock, error) {
	path, err := validatePath(root, path, true)
	if err != nil {
		return nil, err
	}
	file, err := openLockFile(path)
	if err != nil {
		return nil, fmt.Errorf("open mutation lock: %w", err)
	}
	fail := func(cause error) (*Lock, error) {
		_ = file.Close()
		return nil, cause
	}
	info, err := file.Stat()
	if err != nil {
		return fail(fmt.Errorf("inspect mutation lock: %w", err))
	}
	identity, err := identityFromInfo(info)
	if err != nil {
		return fail(err)
	}
	if err := validatePrivateFileInfo(info, identity, MaxPrivateFileSize); err != nil {
		return fail(fmt.Errorf("unsafe mutation lock: %w", err))
	}
	if err := tryLock(file); err != nil {
		if errors.Is(err, ErrLocked) {
			return fail(ErrLocked)
		}
		return fail(fmt.Errorf("lock mutation file: %w", err))
	}
	return &Lock{file: file}, nil
}

func (lock *Lock) Close() error {
	if lock == nil || lock.file == nil {
		return nil
	}
	file := lock.file
	lock.file = nil
	unlockErr := unlock(file)
	closeErr := file.Close()
	if unlockErr != nil {
		return fmt.Errorf("unlock mutation file: %w", unlockErr)
	}
	return closeErr
}
