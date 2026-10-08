//go:build windows

package managedgit

import "os"

func openRegularFile(path string) (*os.File, error) {
	return os.Open(path)
}
