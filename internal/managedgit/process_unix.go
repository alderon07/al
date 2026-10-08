//go:build !windows

package managedgit

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func configureManagedProcess(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }
func terminateManagedProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}

func managedSystemSSH() (string, error) {
	path, err := trustedSystemSSH()
	if err != nil {
		return "", errors.New("SSH needs a trusted system ssh; enroll a local repository instead")
	}
	return path, nil
}

func trustedSystemSSH() (string, error) {
	names := []string{"/bin/ssh", "/usr/bin/ssh"}
	for _, name := range names {
		resolved, err := filepath.EvalSymlinks(name)
		if err != nil {
			continue
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 || !rootOwned(info) {
			continue
		}
		trusted := true
		for directory := filepath.Dir(resolved); ; directory = filepath.Dir(directory) {
			info, err = os.Stat(directory)
			if err != nil || !info.IsDir() || !rootOwned(info) {
				trusted = false
				break
			}
			if directory == string(filepath.Separator) {
				break
			}
		}
		if trusted {
			return resolved, nil
		}
	}
	return "", errors.New("trusted ssh validator is not installed")
}
func rootOwned(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == 0 && info.Mode().Perm()&0o022 == 0
}
