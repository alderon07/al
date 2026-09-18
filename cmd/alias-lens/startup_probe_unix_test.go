//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/creack/pty/v2"
)

func TestPlainCommandStartupDoesNotProbeTerminal(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestStartupProbeHelper$")
	command.Env = append(os.Environ(), "ALIAS_LENS_STARTUP_PROBE_HELPER=1", "TERM=xterm-256color")
	terminal, err := pty.Start(command)
	if err != nil {
		t.Fatal(err)
	}
	defer terminal.Close()
	finished := make(chan error, 1)
	go func() { finished <- command.Wait() }()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("plain child command failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		_ = command.Process.Kill()
		<-finished
		t.Fatal("plain command waited for an unanswered terminal probe")
	}
}

func TestStartupProbeHelper(t *testing.T) {
	if os.Getenv("ALIAS_LENS_STARTUP_PROBE_HELPER") != "1" {
		return
	}
	fmt.Println("ready")
}
