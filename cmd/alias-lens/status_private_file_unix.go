//go:build !windows

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func readObservedPrivateFile(path string, limit int64) ([]byte, error) {
	if _, err := os.Lstat(path); err != nil {
		return nil, err
	}
	if err := validateObservedPrivateParents(path); err != nil {
		return nil, err
	}
	descriptor, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(descriptor), path)
	defer file.Close()
	before, err := file.Stat()
	if err != nil || !before.Mode().IsRegular() {
		return nil, fmt.Errorf("private state is not a regular file")
	}
	beforeStat, ok := before.Sys().(*syscall.Stat_t)
	if !ok || before.Mode().Perm() != 0o600 || beforeStat.Uid != uint32(os.Geteuid()) || beforeStat.Nlink != 1 {
		return nil, fmt.Errorf("private state has unsafe ownership, permissions, or links")
	}
	contents, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(contents)) > limit {
		return nil, fmt.Errorf("private state could not be read safely")
	}
	after, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("private state changed while it was being checked")
	}
	afterStat, ok := after.Sys().(*syscall.Stat_t)
	if !ok || beforeStat.Dev != afterStat.Dev || beforeStat.Ino != afterStat.Ino || before.Size() != after.Size() {
		return nil, fmt.Errorf("private state changed while it was being checked")
	}
	return contents, nil
}

func validateObservedPrivateParents(path string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	roots := []string{filepath.Join(home, ".config", "alias-lens"), filepath.Join(home, ".local", "state", "alias-lens")}
	for _, root := range roots {
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		current := root
		parts := strings.Split(filepath.Dir(relative), string(filepath.Separator))
		for index := -1; index < len(parts); index++ {
			if index >= 0 && parts[index] != "." {
				current = filepath.Join(current, parts[index])
			}
			info, statErr := os.Lstat(current)
			if statErr != nil {
				return statErr
			}
			stat, ok := info.Sys().(*syscall.Stat_t)
			if !ok || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm() != 0o700 || stat.Uid != uint32(os.Geteuid()) {
				return fmt.Errorf("private state path has an unsafe parent directory")
			}
		}
		return nil
	}
	return fmt.Errorf("private state path is outside Alias Lens data directories")
}
