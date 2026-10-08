package managedgit

import (
	"errors"
	"os/exec"
)

func configureManagedProcess(cmd *exec.Cmd) {}
func terminateManagedProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}

func managedSystemSSH() (string, error) {
	return "", errors.New("controlled SSH requires Unix; enroll a local repository")
}
