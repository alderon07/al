package managedgit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

type fileIdentity struct {
	FileType                               string
	Mode                                   uint32
	Device, Inode, LinkCount, Owner, Group uint64
	LinkTarget                             string
}

func readRegularFile(path string, limit int64) ([]byte, error) {
	file, err := openRegularFile(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readOpenedFile(file, limit)
}

func readOpenedFile(file *os.File, limit int64) ([]byte, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file")
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("file exceeds %d bytes", limit)
	}
	contents, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(contents)) > limit {
		return nil, fmt.Errorf("file exceeds %d bytes", limit)
	}
	return contents, nil
}

func cleanRepositoryPathComponent(component string) (string, error) {
	if component == "" || component == "." || component == ".." || strings.ContainsAny(component, `/\`) {
		return "", fmt.Errorf("unsafe path component %q", component)
	}
	return component, nil
}

func cleanRemoteRepositoryParts(fullName string) ([]string, error) {
	if fullName == "" || strings.HasPrefix(fullName, "/") || strings.Contains(fullName, `\`) {
		return nil, fmt.Errorf("invalid remote repository name %q", fullName)
	}
	parts := strings.Split(fullName, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("remote repository name %q must include its namespace", fullName)
	}
	for _, part := range parts {
		if _, err := cleanRepositoryPathComponent(part); err != nil {
			return nil, fmt.Errorf("invalid remote repository name %q: %w", fullName, err)
		}
	}
	return parts, nil
}

func hashBytes(contents []byte) string {
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:])
}
