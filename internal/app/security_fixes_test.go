package app

import (
	"encoding/json"

	"os"
	"path/filepath"

	"strings"
	"testing"
)

func TestAutomaticSyncDoesNotActivateRemoteAliasBytes(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	bare := filepath.Join(home, "remote.git")
	seed := filepath.Join(home, "seed")
	repository := filepath.Join(home, "dotfiles")
	attacker := filepath.Join(home, "attacker")
	if err := os.Mkdir(seed, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, home, "init", "--bare", bare)
	runGit(t, seed, "init")
	runGit(t, seed, "config", "user.email", "alias-lens@example.test")
	runGit(t, seed, "config", "user.name", "Alias Lens Test")
	baseline := []byte("alias safe='git status'\n")
	if err := os.WriteFile(filepath.Join(seed, ".bash_aliases"), baseline, 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, seed, "add", ".bash_aliases")
	runGit(t, seed, "commit", "-m", "baseline")
	runGit(t, seed, "remote", "add", "origin", bare)
	runGit(t, seed, "push", "-u", "origin", "HEAD")
	runGit(t, home, "clone", bare, repository)
	runGit(t, home, "clone", bare, attacker)
	runGit(t, attacker, "config", "user.email", "remote-writer@example.test")
	runGit(t, attacker, "config", "user.name", "Remote Writer")

	aliasPath := filepath.Join(home, ".bash_aliases")
	if err := os.WriteFile(aliasPath, baseline, 0o600); err != nil {
		t.Fatal(err)
	}
	hash := contentHash(baseline)
	if err := DefaultServices().writeSyncStatus("synced", "baseline", hash, hash); err != nil {
		t.Fatal(err)
	}
	remoteBytes := []byte("touch \"$HOME/remote-code-ran\"\nalias safe='git status'\n")
	if err := os.WriteFile(filepath.Join(attacker, ".bash_aliases"), remoteBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, attacker, "add", ".bash_aliases")
	runGit(t, attacker, "commit", "-m", "remote update")
	runGit(t, attacker, "push")

	config := DefaultServices().defaultConfig()
	config.Repository = repository
	err := DefaultServices().reconcileAliases(config)
	if err == nil || !strings.Contains(err.Error(), "al sync --pull") {
		t.Fatalf("remote update was not deferred for approval: %v", err)
	}
	contents, readErr := os.ReadFile(aliasPath)
	if readErr != nil || string(contents) != string(baseline) {
		t.Fatalf("live aliases changed to %q, %v", contents, readErr)
	}
	if info, statErr := os.Stat(filepath.Join(repository, ".bash_aliases")); statErr != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("repository alias copy mode = %v, %v; want 0600", info, statErr)
	}
	if _, statErr := os.Stat(filepath.Join(home, "remote-code-ran")); !os.IsNotExist(statErr) {
		t.Fatalf("remote top-level command ran: %v", statErr)
	}
}

func TestManualPullStillImportsRemoteAliasesWithoutTopLevelCode(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	bare := filepath.Join(home, "remote.git")
	seed := filepath.Join(home, "seed")
	repository := filepath.Join(home, "dotfiles")
	if err := os.Mkdir(seed, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, home, "init", "--bare", bare)
	runGit(t, seed, "init")
	runGit(t, seed, "config", "user.email", "alias-lens@example.test")
	runGit(t, seed, "config", "user.name", "Alias Lens Test")
	remote := []byte("touch \"$HOME/remote-code-ran\"\nalias local='git status'\nalias remote='git push'\n")
	if err := os.WriteFile(filepath.Join(seed, ".bash_aliases"), remote, 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, seed, "add", ".bash_aliases")
	runGit(t, seed, "commit", "-m", "aliases")
	runGit(t, seed, "remote", "add", "origin", bare)
	runGit(t, seed, "push", "-u", "origin", "HEAD")
	runGit(t, home, "clone", bare, repository)
	aliasPath := filepath.Join(home, ".bash_aliases")
	if err := os.WriteFile(aliasPath, []byte("alias local='git status'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := DefaultServices().defaultConfig()
	config.Repository = repository
	if err := DefaultServices().saveConfig(config); err != nil {
		t.Fatal(err)
	}

	message, err := DefaultServices().pullRepository()
	if err != nil || !strings.Contains(message, "imported 1 aliases") || !strings.Contains(message, "still differ") || !strings.Contains(message, "al diff") {
		t.Fatalf("manual pull = %q, %v", message, err)
	}
	contents, err := os.ReadFile(aliasPath)
	if err != nil || !strings.Contains(string(contents), "alias remote='git push'") || strings.Contains(string(contents), "touch ") {
		t.Fatalf("manual pull wrote unsafe or incomplete content: %q, %v", contents, err)
	}
	if info, statErr := os.Stat(filepath.Join(repository, ".bash_aliases")); statErr != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("repository alias copy mode = %v, %v; want 0600", info, statErr)
	}
	if _, statErr := os.Stat(filepath.Join(home, "remote-code-ran")); !os.IsNotExist(statErr) {
		t.Fatalf("manual pull executed remote top-level code: %v", statErr)
	}
}

func TestTrackedFileValidationRejectsAliasLensConfigIdentity(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	config := DefaultServices().defaultConfig()
	if err := DefaultServices().saveConfig(config); err != nil {
		t.Fatal(err)
	}
	path, err := DefaultServices().configPath()
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{path, filepath.Join(filepath.Dir(path), "config-link.json")} {
		if source != path {
			if err := os.Symlink(path, source); err != nil {
				t.Fatal(err)
			}
		}
		err := DefaultServices().validateTrackedFileConfig(TrackedFileConfig{Source: source, RepositoryPath: "settings.json"})
		if err == nil || !strings.Contains(err.Error(), "Alias Lens configuration") {
			t.Fatalf("config identity %s was accepted: %v", source, err)
		}
	}
}

func TestLoadConfigRejectsInjectedTrackedScope(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	path, err := DefaultServices().configPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	config := DefaultServices().defaultConfig()
	config.TrackedFiles = []TrackedFileConfig{{Source: path, RepositoryPath: "config.json"}}
	contents, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := DefaultServices().loadConfig(); err == nil || !strings.Contains(err.Error(), "invalid tracked_files entry") {
		t.Fatalf("injected tracked scope was accepted: %v", err)
	}
}

func TestRepositoryWriteRejectsParentSymlink(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	directory := privateTestHome(t)
	repository := filepath.Join(directory, "dotfiles")
	outside := filepath.Join(directory, "outside")
	if err := os.Mkdir(repository, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "init")
	if err := os.Symlink(outside, filepath.Join(repository, "shell")); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(directory, ".bash_aliases")
	if err := os.WriteFile(source, []byte("alias safe='git status'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := DefaultServices().syncRepositoryFiles(AppConfig{Repository: repository, AliasFile: "shell/.bash_aliases"}, source, false)
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("repository symlink was accepted: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(outside, ".bash_aliases")); !os.IsNotExist(statErr) {
		t.Fatalf("write escaped the repository: %v", statErr)
	}
}

func TestRepositoryWriteRejectsSymlinkInsertedAfterValidation(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	directory := privateTestHome(t)
	repository := filepath.Join(directory, "dotfiles")
	outside := filepath.Join(directory, "outside")
	if err := os.Mkdir(repository, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	repositoryWriteBeforeOpen = func() {
		if err := os.Symlink(outside, filepath.Join(repository, "shell")); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { repositoryWriteBeforeOpen = nil })
	err := writeRepositoryFile(repository, "shell/.bash_aliases", []byte("alias safe='true'\n"), 0o600)
	if err == nil {
		t.Fatal("repository write followed a symlink inserted after validation")
	}
	if _, statErr := os.Stat(filepath.Join(outside, ".bash_aliases")); !os.IsNotExist(statErr) {
		t.Fatalf("write escaped the repository: %v", statErr)
	}
}

func TestAutomaticSyncDoesNotApplyRemoteTrackedFile(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	localPath := filepath.Join(home, ".bashrc")
	repository := filepath.Join(home, "dotfiles")
	if err := os.Mkdir(repository, 0o755); err != nil {
		t.Fatal(err)
	}
	local := []byte("export SAFE=1\n")
	remote := []byte("touch \"$HOME/remote-ran\"\n")
	if err := os.WriteFile(localPath, local, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "bashrc"), remote, 0o600); err != nil {
		t.Fatal(err)
	}
	tracked := TrackedFileConfig{Source: localPath, RepositoryPath: "bashrc"}
	statePath, err := DefaultServices().trackedStatePath(tracked)
	if err != nil {
		t.Fatal(err)
	}
	if err := DefaultServices().writeSyncStateAt(statePath, SyncState{LocalHash: contentHash(local), RemoteHash: contentHash(local), Status: "synced"}); err != nil {
		t.Fatal(err)
	}
	err = DefaultServices().reconcileTrackedFile(AppConfig{Repository: repository}, tracked)
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("remote tracked-file update was not held for review: %v", err)
	}
	after, readErr := os.ReadFile(localPath)
	if readErr != nil || string(after) != string(local) {
		t.Fatalf("live tracked file changed to %q, %v", after, readErr)
	}
}

func TestOutgoingAliasHistoryIsScannedBeforePush(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	repository := privateTestHome(t)
	runGit(t, repository, "init")
	runGit(t, repository, "config", "user.email", "alias-lens@example.test")
	runGit(t, repository, "config", "user.name", "Alias Lens Test")
	path := filepath.Join(repository, ".bash_aliases")
	if err := os.WriteFile(path, []byte("alias safe='true'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "add", ".bash_aliases")
	runGit(t, repository, "commit", "-m", "baseline")
	baseline := strings.TrimSpace(runGit(t, repository, "rev-parse", "HEAD"))
	runGit(t, repository, "update-ref", "refs/remotes/origin/main", baseline)
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
	if err := scanOutgoingAliasHistory(repository, ".bash_aliases"); err == nil || !strings.Contains(err.Error(), "outgoing alias commit") {
		t.Fatalf("outgoing secret history was accepted: %v", err)
	}
}

func TestCredentialFileNamesAndFormatsAreRejected(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	for _, name := range []string{".netrc", ".npmrc", "production.env", "AUTH_TOKEN.txt", ".pypirc", ".pgpass", ".envrc"} {
		if !DefaultServices().sensitiveConfigPath(name) {
			t.Errorf("credential filename %s was accepted", name)
		}
	}
	for _, contents := range []string{
		"machine example.test login user password value\n",
		"//registry.example/:_authToken=value\n",
		"//registry.example/:_auth=BASE64VALUE\n",
		"PRODUCTION_TOKEN=value\n",
	} {
		if findings := findSecretFindings([]byte(contents)); len(findings) == 0 {
			t.Errorf("credential format was not detected: %q", contents)
		}
	}
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	credential := filepath.Join(home, ".netrc")
	link := filepath.Join(home, "ordinary-settings")
	if err := os.WriteFile(credential, []byte("machine example.test login user password value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(credential, link); err != nil {
		t.Fatal(err)
	}
	if err := DefaultServices().validateTrackedFileConfig(TrackedFileConfig{Source: link, RepositoryPath: "ordinary-settings"}); err == nil {
		t.Fatal("safe-looking symlink to .netrc was accepted")
	}
}
