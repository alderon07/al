//go:build windows

package main

import (
	"fmt"
	"os"
)

func readObservedPrivateFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("private state is not a regular file")
	}
	return readRegularFile(path, limit)
}
