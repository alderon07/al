package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTerminalSafeViewsEscapeRepositoryControlledText(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	malicious := "name\x1b]52;c;payload\a\u202e"
	alias := Alias{
		Name:        malicious,
		Command:     "printf '\x1b[2J'",
		Description: "description\x00",
		Category:    "custom\u2066",
		Tags:        []string{"tag\x1b"},
		Issues:      []string{"issue\a"},
	}
	for name, rendered := range map[string]string{
		"card":  renderAlias(alias, true, 80),
		"plain": plainAliasOutput(alias),
	} {
		for _, raw := range []string{"\x1b]52;c;payload\a", "\u202e", "\x1b[2J", "\x00", "\u2066"} {
			if strings.Contains(rendered, raw) {
				t.Fatalf("%s output contains raw control text %q: %q", name, raw, rendered)
			}
		}
		if !strings.Contains(rendered, `\x1b`) {
			t.Fatalf("%s output did not visibly escape control text: %q", name, rendered)
		}
	}

	confirmation := model{runConfirm: &alias, width: 80, height: 24}
	view := confirmation.runConfirmationView(newMainTUIFrame(80, 24), "Alias Lens")
	if strings.Contains(view, "\x1b]52;c;payload\a") || strings.Contains(view, "\x1b[2J") {
		t.Fatalf("confirmation contains raw repository-controlled escape: %q", view)
	}
	if got := terminalSafeText("safe\u202evalue"); got != `safe\u202evalue` {
		t.Fatalf("bidirectional formatting was not escaped: %q", got)
	}
}

func plainAliasOutput(alias Alias) string {
	var output bytes.Buffer
	printPlainAliasList(&output, []Alias{alias}, "")
	return output.String()
}

func TestRepositoryFileLimitsAndBatchAliasUpdate(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	directory := privateTestHome(t)
	oversized := filepath.Join(directory, "oversized")
	file, err := os.OpenFile(oversized, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(aliasFileLimit + 1); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := readFileLimited(oversized, aliasFileLimit); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized alias file was accepted: %v", err)
	}

	path := filepath.Join(directory, ".bash_aliases")
	if err := os.WriteFile(path, []byte("alias one='true'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	writes := 0
	previousHook := atomicWriteBeforeRename
	atomicWriteBeforeRename = func(target string) error {
		if target == path {
			writes++
		}
		return nil
	}
	t.Cleanup(func() { atomicWriteBeforeRename = previousHook })
	if err := addAliasesToFile(path, []aliasAddition{
		{Name: "two", Command: "git status", Description: "second"},
		{Name: "three", Command: "git log", Description: "third"},
	}); err != nil {
		t.Fatal(err)
	}
	if writes != 1 {
		t.Fatalf("batch import replaced the live alias file %d times, want 1", writes)
	}
	contents, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(contents, []byte("alias two=")) || !bytes.Contains(contents, []byte("alias three=")) {
		t.Fatalf("batch import result = %q, %v", contents, err)
	}
}

func TestManagedRepositoryPathsAndStartupBackupsArePrivate(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	directory := privateTestHome(t)
	root := filepath.Join(directory, "repos")
	target := filepath.Join(root, "github", "owner", "repo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ensureManagedRepositoryPath(root, target); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{root, filepath.Join(root, "github"), filepath.Join(root, "github", "owner"), target} {
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
			t.Fatalf("managed path %s mode = %v, %v", path, info, err)
		}
	}
	outside := filepath.Join(directory, "outside")
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if err := ensureManagedRepositoryPath(root, filepath.Join(root, "link", "repo")); err == nil || !strings.Contains(err.Error(), "real directory") {
		t.Fatalf("managed repository path followed a symlink: %v", err)
	}

	startup := filepath.Join(directory, ".bashrc")
	backup := startup + ".alias-lens.bak"
	if err := os.WriteFile(startup, []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backup, []byte("old backup\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	if err := writeStartupFile(startup, []byte("original\n"), []byte("updated\n")); err == nil {
		t.Fatal("unsafe existing backup metadata accepted")
	}
	if err := os.Chmod(backup, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeStartupFile(startup, []byte("original\n"), []byte("updated\n")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(backup)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("startup backup mode = %v, %v", info, err)
	}
}

func TestRepositoryPushValidatesEveryOutgoingPathAndBlob(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	t.Run("rejects unrelated path", func(t *testing.T) {
		repository, _ := setupPushRepository(t)
		if err := os.WriteFile(filepath.Join(repository, "notes.txt"), []byte("unrelated\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		runGit(t, repository, "add", "notes.txt")
		runGit(t, repository, "commit", "-m", "unrelated")
		_, err := prepareRepositoryPush(AppConfig{Repository: repository, AliasFile: ".bash_aliases"})
		if err == nil || !strings.Contains(err.Error(), "untracked path") {
			t.Fatalf("unrelated outgoing path was accepted: %v", err)
		}
	})

	t.Run("rejects removed secret", func(t *testing.T) {
		repository, _ := setupPushRepository(t)
		path := filepath.Join(repository, ".bash_aliases")
		if err := os.WriteFile(path, []byte("API_TOKEN=not-a-real-token-but-still-private\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		runGit(t, repository, "add", ".bash_aliases")
		runGit(t, repository, "commit", "-m", "secret")
		if err := os.WriteFile(path, []byte("alias safe='true'\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		runGit(t, repository, "add", ".bash_aliases")
		runGit(t, repository, "commit", "-m", "remove secret")
		_, err := prepareRepositoryPush(AppConfig{Repository: repository, AliasFile: ".bash_aliases"})
		if err == nil || !strings.Contains(err.Error(), "may contain a secret") {
			t.Fatalf("removed outgoing secret was accepted: %v", err)
		}
	})

	t.Run("pushes exact allowed snapshot", func(t *testing.T) {
		repository, bare := setupPushRepository(t)
		path := filepath.Join(repository, ".bash_aliases")
		if err := os.WriteFile(path, []byte("alias safe='true'\nalias status='git status'\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		runGit(t, repository, "add", ".bash_aliases")
		runGit(t, repository, "commit", "-m", "allowed")
		runGit(t, repository, "config", "remote.origin.push", "refs/heads/main:refs/heads/injected")
		config := AppConfig{Repository: repository, AliasFile: ".bash_aliases"}
		if err := pushRepository(config); err != nil {
			t.Fatal(err)
		}
		local := strings.TrimSpace(runGit(t, repository, "rev-parse", "HEAD"))
		remote := strings.TrimSpace(runGit(t, bare, "rev-parse", "refs/heads/main"))
		if local != remote {
			t.Fatalf("remote main = %s, want exact snapshot %s", remote, local)
		}
		if output, err := gitOutput("-C", bare, "rev-parse", "--verify", "refs/heads/injected"); err == nil {
			t.Fatalf("configured wildcard push ref was honored: %s", output)
		}
	})

	t.Run("does not publish followed tags", func(t *testing.T) {
		repository, bare := setupPushRepository(t)
		path := filepath.Join(repository, ".bash_aliases")
		if err := os.WriteFile(path, []byte("alias safe='true'\nalias status='git status'\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		runGit(t, repository, "add", ".bash_aliases")
		runGit(t, repository, "commit", "-m", "allowed")
		runGit(t, repository, "tag", "-a", "private-tag", "-m", "private tag message")
		runGit(t, repository, "config", "push.followTags", "true")
		if err := pushRepository(AppConfig{Repository: repository, AliasFile: ".bash_aliases"}); err != nil {
			t.Fatal(err)
		}
		if output, err := gitOutput("-C", bare, "show-ref", "--verify", "refs/tags/private-tag"); err == nil {
			t.Fatalf("unreviewed annotated tag was published: %s", output)
		}
		local := strings.TrimSpace(runGit(t, repository, "rev-parse", "HEAD"))
		remote := strings.TrimSpace(runGit(t, bare, "rev-parse", "refs/heads/main"))
		if local != remote {
			t.Fatalf("remote main = %s, want exact snapshot %s", remote, local)
		}
	})

	t.Run("rejects diverged upstream", func(t *testing.T) {
		repository, _ := setupPushRepository(t)
		baseline := strings.TrimSpace(runGit(t, repository, "rev-parse", "HEAD"))
		path := filepath.Join(repository, ".bash_aliases")
		if err := os.WriteFile(path, []byte("alias remote='true'\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		runGit(t, repository, "add", ".bash_aliases")
		runGit(t, repository, "commit", "-m", "remote")
		runGit(t, repository, "push", "origin", "main")
		runGit(t, repository, "reset", "--hard", baseline)
		if err := os.WriteFile(path, []byte("alias local='true'\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		runGit(t, repository, "add", ".bash_aliases")
		runGit(t, repository, "commit", "-m", "local")
		_, err := prepareRepositoryPush(AppConfig{Repository: repository, AliasFile: ".bash_aliases"})
		if err == nil || !strings.Contains(err.Error(), "diverged") {
			t.Fatalf("diverged upstream was accepted: %v", err)
		}
	})

	t.Run("rejects symlink entry", func(t *testing.T) {
		repository, _ := setupPushRepository(t)
		object := strings.TrimSpace(runGit(t, repository, "hash-object", "-w", ".bash_aliases"))
		runGit(t, repository, "update-index", "--add", "--cacheinfo", "120000,"+object+",.bash_aliases")
		runGit(t, repository, "commit", "-m", "symlink entry")
		_, err := prepareRepositoryPush(AppConfig{Repository: repository, AliasFile: ".bash_aliases"})
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("symlink entry was accepted: %v", err)
		}
	})

	t.Run("reviews literal paths with glob characters", func(t *testing.T) {
		repository, _ := setupPushRepository(t)
		path := "alias[1]"
		if err := os.WriteFile(filepath.Join(repository, path), []byte("API_TOKEN=not-a-real-token-but-still-private\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		runGit(t, repository, "add", "--", path)
		runGit(t, repository, "commit", "-m", "literal path")
		_, err := prepareRepositoryPush(AppConfig{Repository: repository, AliasFile: path})
		if err == nil || !strings.Contains(err.Error(), "may contain a secret") {
			t.Fatalf("literal path secret was accepted: %v", err)
		}
	})

	t.Run("refreshes stale upstream before review", func(t *testing.T) {
		repository, bare := setupPushRepository(t)
		baseline := strings.TrimSpace(runGit(t, repository, "rev-parse", "HEAD"))
		if err := os.WriteFile(filepath.Join(repository, "notes.txt"), []byte("private\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		runGit(t, repository, "add", "notes.txt")
		runGit(t, repository, "commit", "-m", "unrelated")
		runGit(t, repository, "push", "origin", "main")
		runGit(t, bare, "update-ref", "refs/heads/main", baseline)
		aliasPath := filepath.Join(repository, ".bash_aliases")
		if err := os.WriteFile(aliasPath, []byte("alias safe='true'\nalias status='git status'\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		runGit(t, repository, "add", ".bash_aliases")
		runGit(t, repository, "commit", "-m", "allowed")
		_, err := prepareRepositoryPush(AppConfig{Repository: repository, AliasFile: ".bash_aliases"})
		if err == nil || !strings.Contains(err.Error(), "untracked path") {
			t.Fatalf("stale tracking ref hid a republished path: %v", err)
		}
	})

	t.Run("ignores replacement objects during review", func(t *testing.T) {
		repository, _ := setupPushRepository(t)
		baseline := strings.TrimSpace(runGit(t, repository, "rev-parse", "HEAD"))
		if err := os.WriteFile(filepath.Join(repository, "notes.txt"), []byte("private\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		runGit(t, repository, "add", "notes.txt")
		runGit(t, repository, "commit", "-m", "unrelated")
		original := strings.TrimSpace(runGit(t, repository, "rev-parse", "HEAD"))
		baselineTree := strings.TrimSpace(runGit(t, repository, "rev-parse", baseline+"^{tree}"))
		replacement := strings.TrimSpace(runGit(t, repository, "commit-tree", baselineTree, "-p", baseline, "-m", "safe replacement"))
		runGit(t, repository, "replace", original, replacement)
		_, err := prepareRepositoryPush(AppConfig{Repository: repository, AliasFile: ".bash_aliases"})
		if err == nil || !strings.Contains(err.Error(), "untracked path") {
			t.Fatalf("replacement object hid an unsafe outgoing commit: %v", err)
		}
	})
}

func setupPushRepository(t *testing.T) (string, string) {
	t.Helper()
	directory := privateTestHome(t)
	bare := filepath.Join(directory, "remote.git")
	repository := filepath.Join(directory, "repository")
	runGit(t, directory, "init", "--bare", bare)
	if err := os.Mkdir(repository, 0o700); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "init")
	runGit(t, repository, "branch", "-M", "main")
	runGit(t, repository, "config", "user.email", "alias-lens@example.test")
	runGit(t, repository, "config", "user.name", "Alias Lens Test")
	if err := os.WriteFile(filepath.Join(repository, ".bash_aliases"), []byte("alias safe='true'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "add", ".bash_aliases")
	runGit(t, repository, "commit", "-m", "baseline")
	runGit(t, repository, "remote", "add", "origin", bare)
	runGit(t, repository, "push", "-u", "origin", "main")
	return repository, bare
}

func TestReleaseWorkflowPinsPrivilegedDependencies(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	contents, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(contents)
	for _, line := range strings.Split(workflow, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "uses:") {
			continue
		}
		reference := strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, "uses:")))[0]
		separator := strings.LastIndexByte(reference, '@')
		if separator < 0 || len(reference)-separator-1 != 40 {
			t.Fatalf("action is not pinned to a full commit SHA: %s", line)
		}
	}
	for _, expected := range []string{"syft-version: v1.42.3", "version: v2.18.0", "fetch-depth: 0", "persist-credentials: false", "release --clean --skip=publish", "needs: build"} {
		if !strings.Contains(workflow, expected) {
			t.Fatalf("release workflow is missing %q", expected)
		}
	}
	releaseJob := workflow[strings.Index(workflow, "\n  release:"):]
	for _, forbidden := range []string{"goreleaser/goreleaser-action", "anchore/sbom-action", "actions/setup-go"} {
		if strings.Contains(releaseJob, forbidden) {
			t.Fatalf("privileged release job still executes build dependency %q", forbidden)
		}
	}
}
