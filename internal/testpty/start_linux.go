package testpty

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty/v2"
	"golang.org/x/sys/unix"
)

func Start(command *exec.Cmd) (*os.File, error) {
	return StartWithSize(command, nil)
}

func StartWithSize(command *exec.Cmd, size *pty.Winsize) (*os.File, error) {
	master, slave, err := open()
	if err != nil {
		return nil, err
	}
	defer slave.Close()
	if size != nil {
		if err := pty.Setsize(master, size); err != nil {
			master.Close()
			return nil, err
		}
	}
	if command.Stdin == nil {
		command.Stdin = slave
	}
	if command.Stdout == nil {
		command.Stdout = slave
	}
	if command.Stderr == nil {
		command.Stderr = slave
	}
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.Setsid = true
	command.SysProcAttr.Setctty = true
	if err := command.Start(); err != nil {
		master.Close()
		return nil, err
	}
	return master, nil
}

func open() (*os.File, *os.File, error) {
	fd, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("open PTY master: %w", err)
	}
	master := os.NewFile(uintptr(fd), "ptmx")
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		master.Close()
		return nil, nil, fmt.Errorf("unlock PTY slave: %w", err)
	}
	peer, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), unix.TIOCGPTPEER, unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC)
	if errno != 0 {
		master.Close()
		return nil, nil, fmt.Errorf("open PTY slave from master: %w", errno)
	}
	return master, os.NewFile(peer, "pts-peer"), nil
}
