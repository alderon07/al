//go:build windows

package shell

import "os"

func openRegularFile(path string) (*os.File, error) {
	return os.Open(path)
}
