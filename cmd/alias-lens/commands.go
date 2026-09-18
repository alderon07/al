package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

var repositoryCommandTimeout = 45 * time.Second

func interruptContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func commandOutput(ctx context.Context, timeout time.Duration, name string, arguments ...string) ([]byte, error) {
	commandContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	output, err := exec.CommandContext(commandContext, name, arguments...).CombinedOutput()
	if commandContext.Err() != nil {
		return output, fmt.Errorf("%s stopped: %w", name, commandContext.Err())
	}
	return output, err
}

func gitOutput(arguments ...string) ([]byte, error) {
	ctx, cancel := interruptContext()
	defer cancel()
	output, err := commandOutput(ctx, repositoryCommandTimeout, "git", arguments...)
	if err != nil && len(output) == 0 {
		output = []byte(err.Error())
	}
	return output, err
}
