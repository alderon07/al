package providers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

type Settings struct {
	Enabled        bool
	Host, Protocol string
	Workspaces     []string
}
type Runtime struct {
	Transport  http.RoundTripper
	Credential func(context.Context, string, string) (string, error)
	Getenv     func(string) string
	LookPath   func(string) (string, error)
	Command    func(context.Context, string, ...string) *exec.Cmd
}
type Registry struct {
	settings map[string]Settings
	runtime  Runtime
}

func New(settings map[string]Settings, runtime Runtime) *Registry {
	copySettings := make(map[string]Settings, len(settings))
	for id, value := range settings {
		value.Workspaces = append([]string(nil), value.Workspaces...)
		copySettings[id] = value
	}
	return &Registry{settings: copySettings, runtime: runtime}
}
func (runtime Runtime) getenv(name string) string {
	if runtime.Getenv != nil {
		return runtime.Getenv(name)
	}
	return os.Getenv(name)
}
func (runtime Runtime) lookPath(name string) (string, error) {
	if runtime.LookPath != nil {
		return runtime.LookPath(name)
	}
	return exec.LookPath(name)
}
func (runtime Runtime) command(ctx context.Context, name string, args ...string) *exec.Cmd {
	if runtime.Command != nil {
		return runtime.Command(ctx, name, args...)
	}
	return exec.CommandContext(ctx, name, args...)
}
func (runtime Runtime) getJSON(ctx context.Context, endpoint, header, credential string, target any) error {
	return runtime.fetchJSON(ctx, endpoint, header, credential, target)
}
func (p githubProvider) Host() string    { return p.host }
func (p gitlabProvider) Host() string    { host, _ := p.hostname(); return host }
func (p bitbucketProvider) Host() string { return "bitbucket.org" }
func (p githubProvider) SSHAvailable(ctx context.Context) bool {
	return p.runtime.useSSH(ctx, "auto", p.host)
}
func (p gitlabProvider) SSHAvailable(ctx context.Context) bool {
	host := p.Host()
	return p.runtime.useSSH(ctx, "auto", host)
}
func (p bitbucketProvider) SSHAvailable(ctx context.Context) bool {
	return p.runtime.useSSH(ctx, "auto", "bitbucket.org")
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

func (runtime Runtime) outputBounded(ctx context.Context, limit int, name string, arguments ...string) ([]byte, []byte, error) {
	command := runtime.command(ctx, name, arguments...)
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

func (runtime Runtime) apiToken(ctx context.Context, provider, host, environment string) (string, error) {
	if runtime.Credential != nil {
		return runtime.Token(ctx, provider, host)
	}
	return strings.TrimSpace(runtime.getenv(environment)), nil
}
