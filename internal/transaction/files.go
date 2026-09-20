package transaction

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func validatePrivateRoot(root string) (string, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve private root: %w", err)
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return "", fmt.Errorf("inspect private root: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", fmt.Errorf("%w: private root is not a real directory", ErrUnsafePath)
	}
	if !privateDirectoryModeValid(info.Mode()) {
		return "", fmt.Errorf("%w: private root mode is %04o, want 0700", ErrUnsafePath, info.Mode().Perm())
	}
	if err := validateDirectoryOwner(info); err != nil {
		return "", err
	}
	return absolute, nil
}

func validatePath(root, path string, leafMayBeMissing bool) (string, error) {
	root, err := validatePrivateRoot(root)
	if err != nil {
		return "", err
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve private path: %w", err)
	}
	relative, err := filepath.Rel(root, absolute)
	if err != nil || relative == "." || relative == ".." || filepath.IsAbs(relative) {
		return "", fmt.Errorf("%w: path is not below the private root", ErrUnsafePath)
	}
	if len(relative) >= 3 && relative[:3] == ".."+string(filepath.Separator) {
		return "", fmt.Errorf("%w: path escapes the private root", ErrUnsafePath)
	}

	parts := splitPath(relative)
	current := root
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		last := index == len(parts)-1
		if errors.Is(statErr, os.ErrNotExist) && last && leafMayBeMissing {
			return absolute, nil
		}
		if statErr != nil {
			return "", fmt.Errorf("inspect private path: %w", statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("%w: private path contains a symbolic link", ErrUnsafePath)
		}
		if !last {
			if !info.IsDir() || !privateDirectoryModeValid(info.Mode()) {
				return "", fmt.Errorf("%w: private path parent is not a mode 0700 directory", ErrUnsafePath)
			}
			if err := validateDirectoryOwner(info); err != nil {
				return "", err
			}
		}
	}
	return absolute, nil
}

func splitPath(path string) []string {
	var parts []string
	for path != "." && path != "" {
		directory, base := filepath.Split(path)
		if base != "" {
			parts = append([]string{base}, parts...)
		}
		path = filepath.Clean(directory)
		if path == string(filepath.Separator) {
			break
		}
	}
	return parts
}

// InspectPrivateFile opens path without following a leaf symbolic link where
// the platform supports that operation. It rejects non-regular, linked,
// incorrectly owned, or non-private files.
func InspectPrivateFile(root, path string, maxBytes int64) (FileIdentity, error) {
	if maxBytes <= 0 {
		maxBytes = MaxPrivateFileSize
	}
	path, err := validatePath(root, path, true)
	if err != nil {
		return FileIdentity{}, err
	}
	file, err := openRegularNoFollow(path)
	if errors.Is(err, os.ErrNotExist) {
		return FileIdentity{Exists: false}, nil
	}
	if err != nil {
		return FileIdentity{}, fmt.Errorf("open private file: %w", err)
	}
	defer file.Close()

	before, err := file.Stat()
	if err != nil {
		return FileIdentity{}, fmt.Errorf("inspect private file: %w", err)
	}
	identity, err := identityFromInfo(before)
	if err != nil {
		return FileIdentity{}, err
	}
	if err := validatePrivateFileInfo(before, identity, maxBytes); err != nil {
		return FileIdentity{}, err
	}

	hash := sha256.New()
	read, err := io.Copy(hash, io.LimitReader(file, maxBytes+1))
	if err != nil {
		return FileIdentity{}, fmt.Errorf("read private file: %w", err)
	}
	if read > maxBytes {
		return FileIdentity{}, fmt.Errorf("%w: private file exceeds %d bytes", ErrUnsafePath, maxBytes)
	}
	after, err := file.Stat()
	if err != nil {
		return FileIdentity{}, fmt.Errorf("reinspect private file: %w", err)
	}
	afterIdentity, err := identityFromInfo(after)
	if err != nil {
		return FileIdentity{}, err
	}
	if !sameOpenIdentity(identity, afterIdentity) || before.Size() != after.Size() {
		return FileIdentity{}, ErrIdentityChanged
	}
	identity.Size = read
	identity.SHA256 = hex.EncodeToString(hash.Sum(nil))
	return identity, nil
}

func validatePrivateFileInfo(info os.FileInfo, identity FileIdentity, maxBytes int64) error {
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: private path is not a regular file", ErrUnsafePath)
	}
	if !privateFileModeValid(info.Mode()) {
		return fmt.Errorf("%w: private file mode is %04o, want 0600", ErrUnsafePath, info.Mode().Perm())
	}
	if identity.Links > 1 {
		return fmt.Errorf("%w: private file has more than one hard link", ErrUnsafePath)
	}
	if info.Size() > maxBytes {
		return fmt.Errorf("%w: private file exceeds %d bytes", ErrUnsafePath, maxBytes)
	}
	return validateFileOwner(identity)
}

func sameIdentity(expected, actual FileIdentity) bool {
	if expected.Exists != actual.Exists {
		return false
	}
	if !expected.Exists {
		return true
	}
	if expected.PlatformID != "" || actual.PlatformID != "" {
		return expected.PlatformID == actual.PlatformID &&
			expected.Size == actual.Size && expected.Mode == actual.Mode &&
			expected.Owner == actual.Owner && expected.Group == actual.Group &&
			expected.Links == actual.Links && expected.SHA256 == actual.SHA256
	}
	return expected.Size == actual.Size && expected.Mode == actual.Mode && expected.SHA256 == actual.SHA256
}

func sameOpenIdentity(expected, actual FileIdentity) bool {
	if !expected.Exists || !actual.Exists {
		return expected.Exists == actual.Exists
	}
	if expected.PlatformID != "" || actual.PlatformID != "" {
		return expected.PlatformID == actual.PlatformID &&
			expected.Mode == actual.Mode && expected.Owner == actual.Owner &&
			expected.Group == actual.Group && expected.Links == actual.Links
	}
	return expected.Mode == actual.Mode && expected.Owner == actual.Owner &&
		expected.Group == actual.Group && expected.Links == actual.Links
}

func hashBytes(contents []byte) string {
	digest := sha256.Sum256(contents)
	return hex.EncodeToString(digest[:])
}

func writeAll(file *os.File, contents []byte) error {
	for len(contents) > 0 {
		written, err := file.Write(contents)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		contents = contents[written:]
	}
	return nil
}
