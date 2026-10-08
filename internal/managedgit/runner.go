package managedgit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"
)

const managedGitOutputLimit = 8 << 20

type Transport struct {
	CloneURL           string
	Transport          string
	KnownHostsSHA      string
	KnownHostsIdentity string
	Home               string
	AgentSocket        string
}

type Runner struct {
	Program    string
	Home       string
	Hooks      string
	PATH       string
	IndexFile  string
	TracePath  string
	Username   string
	Password   string
	SSHCommand string
	SSHSocket  string
	Authority  string
	CommitDate string
}

func New() (Runner, func(), error) {
	program, err := exec.LookPath("git")
	if err != nil {
		return Runner{}, nil, fmt.Errorf("install Git before running al init")
	}
	program, err = filepath.Abs(program)
	if err != nil {
		return Runner{}, nil, err
	}
	root, err := os.MkdirTemp("", "alias-lens-managed-git-")
	if err != nil {
		return Runner{}, nil, err
	}
	os.Chmod(root, 0700)
	hooks := filepath.Join(root, "hooks")
	if err := os.Mkdir(hooks, 0700); err != nil {
		os.RemoveAll(root)
		return Runner{}, nil, err
	}
	return Runner{Program: program, Home: root, Hooks: hooks, PATH: os.Getenv("PATH")}, func() { os.RemoveAll(root) }, nil
}
func (r Runner) Run(ctx context.Context, repository string, input []byte, args ...string) ([]byte, error) {
	for _, arg := range args {
		if arg == "fetch" || arg == "push" || arg == "clone" || arg == "ls-remote" {
			return nil, errors.New("network Git requires a pinned HTTPS transport")
		}
	}
	return r.run(ctx, repository, input, args...)
}
func (r Runner) run(ctx context.Context, repository string, input []byte, args ...string) ([]byte, error) {
	options := []string{"-c", "core.hooksPath=" + r.Hooks, "-c", "core.fsmonitor=false", "-c", "core.attributesFile=/dev/null", "-c", "credential.helper=", "-c", "submodule.recurse=false", "-c", "protocol.ext.allow=never", "-c", "protocol.file.allow=never", "-c", "protocol.version=2", "-c", "http.followRedirects=false", "-c", "filter.lfs.required=false", "-c", "filter.lfs.smudge=", "-c", "filter.lfs.clean=", "-c", "filter.lfs.process=", "-c", "maintenance.auto=false", "-c", "gc.auto=0", "-c", "commit.gpgsign=false", "-c", "user.name=Alias Lens", "-c", "user.email=alias-lens@localhost"}
	if repository != "" {
		options = append(options, "-C", repository)
	}
	options = append(options, args...)
	command := exec.CommandContext(ctx, r.Program, options...)
	command.Env = []string{"PATH=" + r.PATH, "HOME=" + r.Home, "XDG_CONFIG_HOME=" + r.Home, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_SYSTEM=" + os.DevNull, "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_LFS_SKIP_SMUDGE=1", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1", "LC_ALL=C"}
	if r.CommitDate != "" {
		command.Env = append(command.Env, "GIT_AUTHOR_DATE="+r.CommitDate, "GIT_COMMITTER_DATE="+r.CommitDate)
	}
	if r.IndexFile != "" {
		command.Env = append(command.Env, "GIT_INDEX_FILE="+r.IndexFile)
	}
	if r.TracePath != "" {
		command.Env = append(command.Env, "GIT_TRACE_PACKET="+r.TracePath)
	}
	if r.Password != "" {
		scriptPath := filepath.Join(r.Home, "askpass")
		script := `#!/bin/sh
case "$1" in
 "Username for 'https://$AL_MANAGED_GIT_AUTHORITY':"|"Username for 'https://$AL_MANAGED_GIT_AUTHORITY': ") printf '%s\n' "$AL_MANAGED_GIT_USERNAME" ;;
 "Password for 'https://$AL_MANAGED_GIT_USERNAME@$AL_MANAGED_GIT_AUTHORITY':"|"Password for 'https://$AL_MANAGED_GIT_USERNAME@$AL_MANAGED_GIT_AUTHORITY': ") printf '%s\n' "$AL_MANAGED_GIT_PASSWORD" ;;
 *) exit 1 ;;
esac
`
		if err := os.WriteFile(scriptPath, []byte(script), 0700); err != nil {
			return nil, errors.New("cannot prepare controlled Git credential handoff")
		}
		command.Env = append(command.Env, "GIT_ASKPASS="+scriptPath, "AL_MANAGED_GIT_AUTHORITY="+r.Authority, "AL_MANAGED_GIT_USERNAME="+r.Username, "AL_MANAGED_GIT_PASSWORD="+r.Password)
	}
	if r.SSHCommand != "" {
		command.Env = append(command.Env, "GIT_SSH_COMMAND="+r.SSHCommand, "GIT_SSH_VARIANT=ssh", "SSH_AUTH_SOCK="+r.SSHSocket)
	}
	command.Stdin = bytes.NewReader(input)
	return runManagedGitCommand(ctx, command, managedGitOutputLimit)
}

type managedOutput struct {
	mu       sync.Mutex
	bytes    bytes.Buffer
	limit    int
	overflow bool
	stop     func()
}

func (w *managedOutput) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.bytes.Len()+len(b) > w.limit {
		w.overflow = true
		if w.stop != nil {
			w.stop()
		}
		return len(b), nil
	}
	return w.bytes.Write(b)
}
func runManagedGitCommand(ctx context.Context, cmd *exec.Cmd, limit int) ([]byte, error) {
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	out := &managedOutput{limit: limit, stop: cancel}
	cmd.Stdout = out
	stderr := &managedOutput{limit: limit, stop: cancel}
	cmd.Stderr = stderr
	configureManagedProcess(cmd)
	cmd.Cancel = func() error { return terminateManagedProcess(cmd) }
	cmd.WaitDelay = time.Second
	if err := cmd.Start(); err != nil {
		return nil, errors.New("Git could not start")
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var err error
	select {
	case err = <-done:
	case <-child.Done():
		terminateManagedProcess(cmd)
		err = <-done
		if err == nil {
			err = child.Err()
		}
	}
	if err != nil || out.overflow || stderr.overflow {
		return nil, errors.New("restricted Git operation failed; verify repository access and retry al init or al sync")
	}
	return out.bytes.Bytes(), nil
}

func (r Runner) RunNetwork(ctx context.Context, repository, transport string, input []byte, args ...string) ([]byte, error) {
	parsed, err := url.Parse(transport)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("managed Git needs a credential-free configured HTTPS URL")
	}
	if r.Password != "" && r.Authority != parsed.Host {
		return nil, errors.New("Git credential authority differs from pinned transport")
	}
	if repository != "" {
		if err := r.AuditNetworkConfig(ctx, repository, transport); err != nil {
			return nil, err
		}
	}
	present := false
	for i, arg := range args {
		if arg == transport || i > 0 && args[i-1] == "-c" && arg == "remote.origin.url="+transport {
			present = true
		}
	}
	if !present {
		return nil, errors.New("managed Git transport is not pinned in arguments")
	}
	return r.run(ctx, repository, input, args...)
}
func (r Runner) AuditNetworkConfig(ctx context.Context, repository, transport string) error {
	gitdir := filepath.Join(repository, ".git")
	info, e := os.Lstat(gitdir)
	if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("linked and indirect Git directories are unsupported for managed transport")
	}
	if _, e := os.Lstat(filepath.Join(gitdir, "config.worktree")); !errors.Is(e, os.ErrNotExist) {
		return errors.New("Git worktree transport configuration is unsupported")
	}
	if _, e := os.Lstat(filepath.Join(gitdir, "commondir")); !errors.Is(e, os.ErrNotExist) {
		return errors.New("shared Git directories are unsupported for managed transport")
	}
	data, err := r.run(ctx, repository, nil, "config", "--local", "--no-includes", "--null", "--list")
	if err != nil {
		return err
	}
	for _, record := range bytes.Split(data, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		parts := bytes.SplitN(record, []byte{'\n'}, 2)
		key := strings.ToLower(string(parts[0]))
		value := ""
		if len(parts) == 2 {
			value = string(parts[1])
		}
		unsafe := strings.HasPrefix(key, "url.") || strings.HasPrefix(key, "include.") || strings.HasPrefix(key, "includeif.") || strings.HasPrefix(key, "credential.") || strings.HasPrefix(key, "http.") || strings.HasPrefix(key, "filter.") || strings.HasPrefix(key, "protocol.") || key == "core.sshcommand" || key == "core.gitproxy" || key == "core.askpass" || strings.HasPrefix(key, "extensions.") || key == "core.worktree"
		if strings.HasPrefix(key, "remote.") {
			safe := key == "remote.origin.url" && (value == transport || func() bool { ssh, e := managedSSHURL(transport); return e == nil && value == ssh }()) || key == "remote.origin.fetch" && value == "+refs/heads/*:refs/remotes/origin/*" || key == "remote.origin.promisor" && value == "true" || key == "remote.origin.partialclonefilter" && value == "blob:none"
			for _, pinned := range []string{transport, func() string { ssh, _ := managedSSHURL(transport); return ssh }()} {
				if pinned != "" && (key == "remote."+strings.ToLower(pinned)+".promisor" && value == "true" || key == "remote."+strings.ToLower(pinned)+".partialclonefilter" && value == "blob:none") {
					safe = true
				}
			}
			unsafe = unsafe || !safe
		}
		if unsafe {
			return errors.New("repository has unsafe transport configuration; enroll a clean local repository")
		}
	}
	return nil
}

func managedSSHURL(httpsSource string) (string, error) {
	u, err := url.Parse(httpsSource)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Hostname() == "" || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || strings.ContainsAny(u.Hostname(), " /\\\r\n\x00") {
		return "", errors.New("invalid SSH repository authority")
	}
	if _, err := cleanRemoteRepositoryParts(strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), ".git")); err != nil {
		return "", errors.New("invalid SSH repository path")
	}
	return "ssh://git@" + u.Hostname() + u.EscapedPath(), nil
}
func ReadKnownHosts(home string) ([]byte, string, error) {
	if !filepath.IsAbs(home) {
		return nil, "", errors.New("SSH needs an absolute home directory; enroll a local repository instead")
	}
	path := filepath.Join(home, ".ssh", "known_hosts")
	identity := observePlanIdentity(path)
	if identity.FileType != "regular" || identity.Owner != uint64(os.Geteuid()) || identity.LinkCount != 1 || identity.Mode&0022 != 0 || !managedSSHParents(path) {
		return nil, "", errors.New("SSH needs an owned regular ~/.ssh/known_hosts; enroll a local repository instead")
	}
	data, err := readRegularFile(path, 1<<20)
	if err != nil || len(data) == 0 || !reflect.DeepEqual(observePlanIdentity(path), identity) {
		return nil, "", errors.New("SSH known hosts changed; review ~/.ssh/known_hosts and retry")
	}
	return data, hashBytes(data), nil
}
func managedSSHParents(path string) bool {
	if !filepath.IsAbs(path) {
		return false
	}
	privateChild := false
	for directory := filepath.Dir(path); ; directory = filepath.Dir(directory) {
		info, err := os.Lstat(directory)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
		identity := observePlanIdentity(directory)
		if info.Mode().Perm()&0022 != 0 {
			if !privateChild || identity.Owner != 0 || info.Mode()&os.ModeSticky == 0 {
				return false
			}
		} else if identity.Owner != 0 && identity.Owner != uint64(os.Geteuid()) {
			return false
		}
		if identity.Owner == uint64(os.Geteuid()) && info.Mode().Perm()&0077 == 0 {
			privateChild = true
		}
		if directory == string(filepath.Separator) {
			return true
		}
	}
}

func (r Runner) RunTransport(ctx context.Context, repository string, preview Transport, input []byte, args ...string) ([]byte, error) {
	if preview.Transport != "ssh" {
		return r.RunNetwork(ctx, repository, preview.CloneURL, input, args...)
	}
	transport, err := managedSSHURL(preview.CloneURL)
	if err != nil {
		return nil, err
	}
	hosts, sha, err := ReadKnownHosts(preview.Home)
	if err != nil {
		return nil, err
	}
	if preview.KnownHostsSHA != "" && preview.KnownHostsSHA != sha || preview.KnownHostsIdentity != "" && preview.KnownHostsIdentity != KnownHostsIdentity(preview.Home) {
		return nil, errors.New("SSH known hosts changed after review")
	}
	socket := preview.AgentSocket
	id := observePlanIdentity(socket)
	info, socketErr := os.Lstat(socket)
	if !filepath.IsAbs(socket) || socketErr != nil || info.Mode()&os.ModeSocket == 0 || id.Owner != uint64(os.Geteuid()) || id.LinkCount != 1 || !managedSSHParents(socket) {
		return nil, errors.New("SSH needs a safe owned agent socket; enroll a local repository instead")
	}
	ssh, err := managedSystemSSH()
	if err != nil {
		return nil, err
	}
	hostsPath := filepath.Join(r.Home, "known_hosts")
	if err := os.WriteFile(hostsPath, hosts, 0600); err != nil {
		return nil, err
	}
	r.SSHCommand = managedSSHCommand(ssh, hostsPath, socket)
	r.SSHSocket = socket
	r.Password = ""
	r.Username = ""
	r.Authority = ""
	if repository != "" {
		if err := r.AuditNetworkConfig(ctx, repository, preview.CloneURL); err != nil {
			return nil, err
		}
	}
	pinned := false
	rewritten := append([]string(nil), args...)
	for i, arg := range rewritten {
		if arg == preview.CloneURL {
			rewritten[i] = transport
			pinned = true
		} else if i > 0 && rewritten[i-1] == "-c" && arg == "remote.origin.url="+preview.CloneURL {
			rewritten[i] = "remote.origin.url=" + transport
			pinned = true
		}
	}
	if !pinned {
		return nil, errors.New("SSH operation lacks the exact pinned repository")
	}
	return r.run(ctx, repository, input, rewritten...)
}

func managedShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func managedSSHCommand(ssh, hostsPath, socket string) string {
	return managedShellQuote(ssh) + " -F " + managedShellQuote(os.DevNull) + " -oBatchMode=yes -oStrictHostKeyChecking=yes -oGlobalKnownHostsFile=" + managedShellQuote(os.DevNull) + " -oUserKnownHostsFile=" + managedShellQuote(hostsPath) + " -oIdentityAgent=" + managedShellQuote(socket) + " -oIdentityFile=none -oForwardAgent=no -oProxyCommand=none -oProxyJump=none -oRequestTTY=no -p 22"
}

func KnownHostsIdentity(home string) string {
	if !filepath.IsAbs(home) {
		return ""
	}
	id := observePlanIdentity(filepath.Join(home, ".ssh", "known_hosts"))
	return fmt.Sprintf("%d:%d:%d:%d", id.Device, id.Inode, id.Owner, id.Mode)
}
