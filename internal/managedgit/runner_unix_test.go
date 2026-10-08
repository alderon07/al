//go:build !windows

package managedgit

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestManagedCredentialFillControlledPrompt(t *testing.T) {
	runner, cleanup, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	runner.Authority = "example.invalid"
	runner.Username = "synthetic-user"
	runner.Password = "synthetic-password"
	result, err := runner.Run(context.Background(), "", []byte("protocol=https\nhost=example.invalid\n\n"), "credential", "fill")
	if err != nil {
		t.Fatal("controlled credential prompt failed")
	}
	if !strings.Contains(string(result), "username=synthetic-user\n") || !strings.Contains(string(result), "password=synthetic-password\n") {
		t.Fatal("controlled credential handoff mismatch")
	}
	if _, err := runner.Run(context.Background(), "", []byte("protocol=https\nhost=other.invalid\n\n"), "credential", "fill"); err == nil {
		t.Fatal("foreign authority credential prompt accepted")
	}
}

func TestManagedStderrOverflowRefused(t *testing.T) {
	if _, err := runManagedGitCommand(context.Background(), exec.Command("sh", "-c", "printf 123456789 >&2; exit 0"), 4); err == nil {
		t.Fatal("successful stderr overflow accepted")
	}
}

func TestControlledSSHOptionsAndTokenAbsence(t *testing.T) {
	command := managedSSHCommand("/usr/bin/ssh", "/private/known_hosts", "/private/agent")
	for _, option := range []string{"-F '/dev/null'", "-oBatchMode=yes", "-oStrictHostKeyChecking=yes", "-oGlobalKnownHostsFile='/dev/null'", "-oUserKnownHostsFile='/private/known_hosts'", "-oIdentityAgent='/private/agent'", "-oIdentityFile=none", "-oForwardAgent=no", "-oProxyCommand=none", "-oProxyJump=none", "-oRequestTTY=no", "-p 22"} {
		if !strings.Contains(command, option) {
			t.Fatal("unsafe SSH options")
		}
	}
	runner, cleanup, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	fake := filepath.Join(runner.Home, "fake-git")
	os.WriteFile(fake, []byte("#!/bin/sh\nif env | /usr/bin/grep -E 'TOKEN|PASSWORD|AUTHORITY|USERNAME' >/dev/null; then exit 9; fi\nprintf '%s' \"$GIT_SSH_COMMAND\"\n"), 0700)
	runner.Program = fake
	runner.SSHCommand = command
	runner.SSHSocket = "/private/agent"
	t.Setenv("GH_TOKEN", "synthetic-private-token")
	output, err := runner.run(context.Background(), "", nil, "ls-remote", "ssh://git@example.invalid/synthetic/repo.git")
	if err != nil || string(output) != command {
		t.Fatal("SSH environment leaked credentials or options changed")
	}
	transport, err := managedSSHURL("https://example.invalid:8443/synthetic/repo.git")
	if err != nil || transport != "ssh://git@example.invalid/synthetic/repo.git" {
		t.Fatal("SSH authority did not use canonical hostname")
	}
}

func TestManagedSSHKnownHostsUnsafeSourcesRefused(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	sshDirectory := filepath.Join(home, ".ssh")
	os.Mkdir(sshDirectory, 0700)
	hosts := filepath.Join(sshDirectory, "known_hosts")
	source := filepath.Join(home, "synthetic-hosts")
	os.WriteFile(source, []byte("synthetic.invalid ssh-ed25519 synthetic\n"), 0600)
	os.Symlink(source, hosts)
	if _, _, err := ReadKnownHosts(home); err == nil {
		t.Fatal("known-host link accepted")
	}
	os.Remove(hosts)
	os.Link(source, hosts)
	if _, _, err := ReadKnownHosts(home); err == nil {
		t.Fatal("known-host hardlink accepted")
	}
	os.Remove(hosts)
	os.WriteFile(hosts, []byte("synthetic.invalid ssh-ed25519 synthetic\n"), 0666)
	os.Chmod(hosts, 0666)
	if _, _, err := ReadKnownHosts(home); err == nil {
		t.Fatal("writable known-host file accepted")
	}
}

func TestControlledSSHRunTransportPinnedInputs(t *testing.T) {
	if _, err := managedSystemSSH(); err != nil {
		if os.Getenv("AL_REQUIRE_PTY_SHELLS") == "1" {
			t.Fatal(err)
		}
		t.Skip("controlled transport requires root-owned system ssh")
	}
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	os.Mkdir(filepath.Join(home, ".ssh"), 0700)
	hostsPath := filepath.Join(home, ".ssh", "known_hosts")
	hosts := []byte("synthetic.invalid ssh-ed25519 synthetic\n")
	os.WriteFile(hostsPath, hosts, 0644)
	socket := filepath.Join(home, "agent")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	t.Setenv("SSH_AUTH_SOCK", socket)
	t.Setenv("GH_TOKEN", "synthetic-token")
	runner, cleanup, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	marker := filepath.Join(home, "called")
	fake := filepath.Join(home, "fake-git")
	script := "#!/bin/sh\nif env | /usr/bin/grep -E 'TOKEN|PASSWORD|AUTHORITY|USERNAME' >/dev/null; then exit 9; fi\nprintf called > " + managedShellQuote(marker) + "\nfor arg do last=$arg; done\nprintf '%s\\n%s' \"$GIT_SSH_COMMAND\" \"$last\"\n"
	os.WriteFile(fake, []byte(script), 0700)
	runner.Program = fake
	runner.Password = "synthetic-token"
	preview := Transport{Home: home, AgentSocket: socket, CloneURL: "https://synthetic.invalid:8443/owner/repo.git", Transport: "ssh", KnownHostsSHA: hashBytes(hosts), KnownHostsIdentity: KnownHostsIdentity(home)}
	output, err := runner.RunTransport(context.Background(), "", preview, nil, "ls-remote", "--", preview.CloneURL)
	if err != nil || !strings.Contains(string(output), "ssh://git@synthetic.invalid/owner/repo.git") || !strings.Contains(string(output), "-oStrictHostKeyChecking=yes") {
		t.Fatal("safe SSH handoff failed", err)
	}
	for _, damage := range []string{"changed-hosts", "mismatched-url", "unsafe-socket"} {
		os.Remove(marker)
		args := []string{"ls-remote", "--", preview.CloneURL}
		if damage == "changed-hosts" {
			os.WriteFile(hostsPath, []byte("modified\n"), 0644)
		}
		if damage == "mismatched-url" {
			args[len(args)-1] = "https://other.invalid/owner/repo.git"
		}
		if damage == "unsafe-socket" {
			preview.AgentSocket = hostsPath
		}
		if _, err := runner.RunTransport(context.Background(), "", preview, nil, args...); err == nil {
			t.Fatal("unsafe SSH pinned input accepted")
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatal("unsafe SSH reached child")
		}
		os.WriteFile(hostsPath, hosts, 0644)
		preview.AgentSocket = socket
	}
}

func privateTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if err := os.Chmod(home, 0700); err != nil {
		t.Fatal(err)
	}
	return home
}

func TestManagedGitDisablesHooksAndInheritedConfig(t *testing.T) {
	repo := setupRunnerRepository(t)
	marker := filepath.Join(t.TempDir(), "executed")
	hook := filepath.Join(repo, ".git", "hooks", "post-checkout")
	os.WriteFile(hook, []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0700)
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "alias.injected")
	t.Setenv("GIT_CONFIG_VALUE_0", "!touch '"+marker+"'")
	runner, cleanup, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if _, err := runner.Run(context.Background(), repo, nil, "injected"); err == nil {
		t.Fatal("inherited git config used")
	}
	if _, err := runner.Run(context.Background(), repo, nil, "checkout", "--detach", "HEAD"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("hook/config executed")
	}
}
func TestManagedGitRefusesLocalNetworkRewrites(t *testing.T) {
	repo := setupRunnerRepository(t)
	transport := "https://example.invalid/synthetic/repository.git"
	runner, cleanup, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := runner.AuditNetworkConfig(context.Background(), repo, transport); err != nil {
		t.Fatal("clean repository refused", err)
	}
	for _, setting := range []struct{ key, value string }{{"url.https://other.invalid/.insteadOf", "https://example.invalid/"}, {"core.sshCommand", "touch synthetic-sentinel"}, {"remote.origin.uploadpack", "touch synthetic-sentinel"}, {"include.path", "/synthetic/included"}, {"http.proxy", "https://other.invalid"}, {"credential.helper", "!touch synthetic-sentinel"}} {
		runGit(t, repo, "config", setting.key, setting.value)
		if err := runner.AuditNetworkConfig(context.Background(), repo, transport); err == nil || err.Error() != "repository has unsafe transport configuration; enroll a clean local repository" {
			t.Fatalf("unsafe config policy failed for %s: %v", setting.key, err)
		}
		runGit(t, repo, "config", "--unset", setting.key)
	}
}
func TestManagedGitRefusesWorktreeConfig(t *testing.T) {
	repo := setupRunnerRepository(t)
	transport := "https://example.invalid/synthetic/repository.git"
	runGit(t, repo, "config", "extensions.worktreeConfig", "true")
	os.WriteFile(filepath.Join(repo, ".git", "config.worktree"), []byte("[url \"https://other.invalid/\"]\n insteadOf = https://example.invalid/\n"), 0600)
	runner, cleanup, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := runner.AuditNetworkConfig(context.Background(), repo, transport); err == nil || err.Error() != "Git worktree transport configuration is unsupported" {
		t.Fatal("worktree rewrite policy failed", err)
	}
}

func setupRunnerRepository(t *testing.T) string {
	t.Helper()
	repository := t.TempDir()
	runGit(t, repository, "init", "-q")
	runGit(t, repository, "-c", "user.name=Synthetic", "-c", "user.email=synthetic@localhost", "commit", "--allow-empty", "-qm", "synthetic")
	return repository
}
func runGit(t *testing.T, repository string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repository}, args...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("synthetic Git fixture operation failed: %v", err)
	}
	return string(output)
}

func TestManagedSSHRelativeHomeRefusedWithoutLoop(t *testing.T) {
	if os.Getenv("AL_MANAGEDGIT_RELATIVE_HOSTS_HELPER") == "1" {
		if _, _, err := ReadKnownHosts("."); err == nil {
			t.Fatal("relative home accepted")
		}
		if managedSSHParents(filepath.Join(".ssh", "known_hosts")) {
			t.Fatal("relative SSH parents accepted")
		}
		if KnownHostsIdentity(".") != "" {
			t.Fatal("relative home produced reviewed identity")
		}
		return
	}
	home := privateTestHome(t)
	if err := os.Mkdir(filepath.Join(home, ".ssh"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".ssh", "known_hosts"), []byte("synthetic.invalid ssh-ed25519 synthetic\n"), 0600); err != nil {
		t.Fatal(err)
	}
	data, sha, err := ReadKnownHosts(home)
	if err != nil || string(data) != "synthetic.invalid ssh-ed25519 synthetic\n" || sha != hashBytes(data) || KnownHostsIdentity(home) == "" {
		t.Fatal("absolute home refused", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestManagedSSHRelativeHomeRefusedWithoutLoop$")
	command.Dir = home
	command.Env = append(os.Environ(), "AL_MANAGEDGIT_RELATIVE_HOSTS_HELPER=1", "HOME="+home)
	if err := command.Run(); err != nil {
		t.Fatal("relative-home refusal subprocess failed", err)
	}
	if ctx.Err() != nil {
		t.Fatal("relative-home policy looped")
	}
}

func TestManagedPushDoesNotFollowTags(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	repository := setupRunnerRepository(t)
	remote := filepath.Join(t.TempDir(), "remote.git")
	runGit(t, repository, "init", "--bare", remote)
	runGit(t, repository, "-c", "user.name=Synthetic", "-c", "user.email=synthetic@localhost", "tag", "-am", "synthetic private message", "synthetic-private")
	runGit(t, repository, "config", "push.followTags", "true")
	runner, cleanup, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if _, err := runner.run(context.Background(), repository, nil, "-c", "protocol.file.allow=always", "push", "--", "file://"+remote, "HEAD:refs/heads/catalog"); err != nil {
		t.Fatal(err)
	}
	refs := strings.Fields(runGit(t, remote, "for-each-ref", "--format=%(refname)"))
	if len(refs) != 1 || refs[0] != "refs/heads/catalog" {
		t.Fatalf("managed push expanded explicit refspec: %v", refs)
	}
	if strings.TrimSpace(runGit(t, repository, "config", "push.followTags")) != "true" {
		t.Fatal("local followTags setting changed")
	}
}

func TestManagedPushDoesNotExecuteSigners(t *testing.T) {
	for _, signing := range []struct{ name, mode, format, program string }{
		{"true", "true", "openpgp", "gpg.program"},
		{"if-asked", "if-asked", "openpgp", "gpg.program"},
		{"format-program", "true", "x509", "gpg.x509.program"},
		{"ssh-key-command", "if-asked", "ssh", "gpg.ssh.defaultKeyCommand"},
	} {
		t.Run(signing.name, func(t *testing.T) {
			t.Setenv("HOME", privateTestHome(t))
			repository := setupRunnerRepository(t)
			remote := filepath.Join(t.TempDir(), "remote.git")
			runGit(t, repository, "init", "--bare", remote)
			runGit(t, remote, "config", "receive.certNonceSeed", "synthetic-seed")
			marker := filepath.Join(t.TempDir(), "signer-called")
			signer := filepath.Join(t.TempDir(), "signer")
			if err := os.WriteFile(signer, []byte("#!/bin/sh\nprintf called > '"+marker+"'\nexit 1\n"), 0700); err != nil {
				t.Fatal(err)
			}
			runGit(t, repository, "config", "push.gpgSign", signing.mode)
			runGit(t, repository, "config", "gpg.format", signing.format)
			runGit(t, repository, "config", signing.program, signer)
			if signing.format != "ssh" {
				runGit(t, repository, "config", "user.signingKey", "synthetic-key")
			}
			runner, cleanup, err := New()
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			runner.Password = "synthetic-password"
			if err := runner.AuditNetworkConfig(context.Background(), repository, "https://example.invalid/synthetic/repo.git"); err != nil {
				t.Fatal("repository with signing settings refused", err)
			}
			if _, err := runner.run(context.Background(), repository, nil, "-c", "protocol.file.allow=always", "push", "--", "file://"+remote, "HEAD:refs/heads/catalog"); err != nil {
				t.Fatal("managed unsigned push failed", err)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("managed push executed configured signer")
			}
			if strings.TrimSpace(runGit(t, remote, "rev-parse", "refs/heads/catalog")) != strings.TrimSpace(runGit(t, repository, "rev-parse", "HEAD")) {
				t.Fatal("unsigned push did not publish intended branch")
			}
			if strings.TrimSpace(runGit(t, repository, "config", "push.gpgSign")) != signing.mode || strings.TrimSpace(runGit(t, repository, "config", signing.program)) != signer {
				t.Fatal("local signing settings changed")
			}
		})
	}
}
