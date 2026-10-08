//go:build windows

package app

import "os"

func openRegularFile(path string) (*os.File, error) {
	return os.Open(path)
}
