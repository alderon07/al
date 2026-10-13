//go:build !linux && !windows

package testpty

import (
	"os"
	"os/exec"

	"github.com/creack/pty/v2"
)

func Start(command *exec.Cmd) (*os.File, error) {
	return pty.Start(command)
}

func StartWithSize(command *exec.Cmd, size *pty.Winsize) (*os.File, error) {
	return pty.StartWithSize(command, size)
}
