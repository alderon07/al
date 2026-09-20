package main

import (
	"bytes"
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

type boundedCommandBuffer struct {
	buffer   bytes.Buffer
	limit    int
	overflow bool
	stop     func()
}

func (buffer *boundedCommandBuffer) Write(contents []byte) (int, error) {
	remaining := buffer.limit - buffer.buffer.Len()
	if remaining >= len(contents) {
		return buffer.buffer.Write(contents)
	}
	if remaining > 0 {
		_, _ = buffer.buffer.Write(contents[:remaining])
	}
	buffer.overflow = true
	if buffer.stop != nil {
		buffer.stop()
	}
	return len(contents), nil
}

func commandOutputBounded(ctx context.Context, limit int, name string, arguments ...string) ([]byte, []byte, error) {
	command := exec.CommandContext(ctx, name, arguments...)
	stdout := &boundedCommandBuffer{limit: limit}
	stderr := &boundedCommandBuffer{limit: limit}
	stop := func() {
		if command.Process != nil {
			_ = command.Process.Kill()
		}
	}
	stdout.stop = stop
	stderr.stop = stop
	command.Stdout = stdout
	command.Stderr = stderr
	err := command.Run()
	if stdout.overflow || stderr.overflow {
		return stdout.buffer.Bytes(), stderr.buffer.Bytes(), fmt.Errorf("%s output exceeds %d bytes", name, limit)
	}
	if ctx.Err() != nil {
		return stdout.buffer.Bytes(), stderr.buffer.Bytes(), fmt.Errorf("%s stopped: %w", name, ctx.Err())
	}
	return stdout.buffer.Bytes(), stderr.buffer.Bytes(), err
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

func gitOutputBounded(limit int, arguments ...string) ([]byte, error) {
	ctx, cancel := interruptContext()
	defer cancel()
	commandContext, stop := context.WithTimeout(ctx, repositoryCommandTimeout)
	defer stop()
	stdout, stderr, err := commandOutputBounded(commandContext, limit, "git", arguments...)
	if err != nil {
		if len(stderr) > 0 {
			return stdout, fmt.Errorf("%s", cleanCommandOutput(stderr))
		}
		return stdout, err
	}
	return stdout, nil
}
