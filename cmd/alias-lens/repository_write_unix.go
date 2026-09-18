//go:build linux || darwin

package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// writeRepositoryFile keeps every path resolution relative to an open repository
// descriptor. O_NOFOLLOW on each component prevents a concurrent symlink swap
// from redirecting the temporary file or the final rename outside the repository.
func writeRepositoryFile(repository, relative string, contents []byte, mode os.FileMode) error {
	cleaned, err := cleanRepositoryRelativePath(relative, "repository path")
	if err != nil {
		return err
	}
	root, err := filepath.EvalSymlinks(repository)
	if err != nil {
		return fmt.Errorf("resolve configured repository: %w", err)
	}
	rootFD, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return fmt.Errorf("open configured repository safely: %w", err)
	}
	defer unix.Close(rootFD)
	if repositoryWriteBeforeOpen != nil {
		repositoryWriteBeforeOpen()
	}

	parts := strings.Split(cleaned, string(filepath.Separator))
	directoryFD := rootFD
	defer func() {
		if directoryFD != rootFD {
			_ = unix.Close(directoryFD)
		}
	}()
	for _, component := range parts[:len(parts)-1] {
		nextFD, openErr := unix.Openat(directoryFD, component, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if errors.Is(openErr, unix.ENOENT) {
			if mkdirErr := unix.Mkdirat(directoryFD, component, 0o755); mkdirErr != nil && !errors.Is(mkdirErr, unix.EEXIST) {
				return fmt.Errorf("create repository directory %s: %w", cleaned, mkdirErr)
			}
			nextFD, openErr = unix.Openat(directoryFD, component, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		}
		if openErr != nil {
			return fmt.Errorf("open repository directory %s without following links: %w", cleaned, openErr)
		}
		if directoryFD != rootFD {
			_ = unix.Close(directoryFD)
		}
		directoryFD = nextFD
	}
	name := parts[len(parts)-1]
	if existingFD, openErr := unix.Openat(directoryFD, name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0); openErr == nil {
		var info unix.Stat_t
		statErr := unix.Fstat(existingFD, &info)
		_ = unix.Close(existingFD)
		if statErr != nil {
			return fmt.Errorf("inspect repository file %s: %w", cleaned, statErr)
		}
		if info.Mode&unix.S_IFMT != unix.S_IFREG {
			return fmt.Errorf("repository path %s is not a regular file", cleaned)
		}
	} else if !errors.Is(openErr, unix.ENOENT) {
		return fmt.Errorf("open repository file %s without following links: %w", cleaned, openErr)
	}

	temporaryName, err := repositoryTemporaryName()
	if err != nil {
		return err
	}
	temporaryFD, err := unix.Openat(directoryFD, temporaryName, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW, uint32(mode.Perm()))
	if err != nil {
		return fmt.Errorf("create repository temporary file: %w", err)
	}
	cleanup := true
	defer func() {
		_ = unix.Close(temporaryFD)
		if cleanup {
			_ = unix.Unlinkat(directoryFD, temporaryName, 0)
		}
	}()
	if err := writeDescriptor(temporaryFD, contents); err != nil {
		return err
	}
	if err := unix.Fchmod(temporaryFD, uint32(mode.Perm())); err != nil {
		return fmt.Errorf("set repository file permissions: %w", err)
	}
	if err := unix.Fsync(temporaryFD); err != nil {
		return fmt.Errorf("sync repository file: %w", err)
	}
	if err := unix.Close(temporaryFD); err != nil {
		temporaryFD = -1
		return fmt.Errorf("close repository file: %w", err)
	}
	temporaryFD = -1
	if err := unix.Renameat(directoryFD, temporaryName, directoryFD, name); err != nil {
		return fmt.Errorf("replace repository file %s: %w", cleaned, err)
	}
	cleanup = false
	if err := unix.Fsync(directoryFD); err != nil {
		return fmt.Errorf("sync repository directory: %w", err)
	}
	return nil
}

func repositoryTemporaryName() (string, error) {
	var random [12]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate repository temporary filename: %w", err)
	}
	return ".alias-lens-atomic-" + hex.EncodeToString(random[:]), nil
}

func writeDescriptor(descriptor int, contents []byte) error {
	for len(contents) > 0 {
		written, err := unix.Write(descriptor, contents)
		if err != nil {
			return fmt.Errorf("write repository file: %w", err)
		}
		if written == 0 {
			return fmt.Errorf("write repository file: no progress")
		}
		contents = contents[written:]
	}
	return nil
}
