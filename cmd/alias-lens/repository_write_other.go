//go:build !linux && !darwin

package main

import (
	"os"
	"path/filepath"
)

func writeRepositoryFile(repository, relative string, contents []byte, mode os.FileMode) error {
	if repositoryWriteBeforeOpen != nil {
		repositoryWriteBeforeOpen()
	}
	target, err := repositoryFilePath(repository, relative)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	target, err = repositoryFilePath(repository, relative)
	if err != nil {
		return err
	}
	return writeFileAtomically(target, contents, mode)
}
