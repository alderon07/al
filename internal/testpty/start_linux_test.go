package testpty

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty/v2"
	"golang.org/x/sys/unix"
)

func TestControllingTerminalPTY(t *testing.T) {
	shell, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, shell, "--noprofile", "--norc", "-c", "stty size; read -r line; exit 41")
	command.Env = []string{"HOME=" + t.TempDir(), "PATH=/usr/bin:/bin", "BASH_ENV=/dev/null", "ENV=/dev/null"}
	command.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
	terminal, err := StartWithSize(command, &pty.Winsize{Rows: 18, Cols: 48})
	if err != nil {
		t.Fatal(err)
	}
	defer terminal.Close()
	defer func() {
		if command.ProcessState == nil {
			command.Process.Kill()
			command.Wait()
		}
	}()
	owner, err := unix.IoctlGetInt(int(terminal.Fd()), unix.TIOCGSID)
	if err != nil || owner != command.Process.Pid {
		t.Fatalf("controlling terminal owner = %d, expected %d: %v", owner, command.Process.Pid, err)
	}
	flags, err := unix.FcntlInt(terminal.Fd(), unix.F_GETFD, 0)
	if err != nil || flags&unix.FD_CLOEXEC == 0 {
		t.Fatalf("master descriptor lacks close-on-exec: flags=%d error=%v", flags, err)
	}
	if command.SysProcAttr.Pdeathsig != syscall.SIGTERM {
		t.Fatal("existing process attributes changed")
	}
	var output bytes.Buffer
	drained := make(chan struct{})
	go func() {
		io.Copy(&output, terminal)
		close(drained)
	}()
	if _, err := terminal.Write([]byte("ready\n")); err != nil {
		t.Fatal(err)
	}
	var exitError *exec.ExitError
	if err := command.Wait(); !errors.As(err, &exitError) || exitError.ExitCode() != 41 {
		t.Fatalf("shell exit status changed: %v", err)
	}
	select {
	case <-drained:
	case <-ctx.Done():
		t.Fatal("PTY output did not drain")
	}
	if !strings.Contains(output.String(), "18 48\r\n") {
		t.Fatalf("shell did not inherit terminal dimensions: %q", output.String())
	}
}

func TestCallerStreamsPTY(t *testing.T) {
	shell, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, shell, "--noprofile", "--norc", "-c", "printf synthetic-output; printf synthetic-error >&2")
	command.Env = []string{"HOME=" + t.TempDir(), "PATH=/usr/bin:/bin", "BASH_ENV=/dev/null", "ENV=/dev/null"}
	var output, diagnostic bytes.Buffer
	command.Stdout, command.Stderr = &output, &diagnostic
	terminal, err := Start(command)
	if err != nil {
		t.Fatal(err)
	}
	defer terminal.Close()
	if err := command.Wait(); err != nil {
		t.Fatal(err)
	}
	if output.String() != "synthetic-output" || diagnostic.String() != "synthetic-error" {
		t.Fatalf("caller streams changed: stdout=%q stderr=%q", output.String(), diagnostic.String())
	}
}

func TestStartFailureClosesDescriptorsPTY(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-executable")
	start := func() {
		t.Helper()
		terminal, err := Start(exec.Command(missing))
		if terminal != nil || !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("unexpected failed-start result: terminal=%v error=%v", terminal, err)
		}
	}
	start()
	before, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 25; i++ {
		start()
	}
	after, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("failed starts leaked descriptors: before=%d after=%d", len(before), len(after))
	}
}
