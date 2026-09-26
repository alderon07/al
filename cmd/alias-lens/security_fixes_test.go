package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAutomaticSyncDoesNotActivateRemoteAliasBytes(t *testing.T) {
	home := t.TempDir()
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
	if err := writeSyncStatus("synced", "baseline", hash, hash); err != nil {
		t.Fatal(err)
	}
	remoteBytes := []byte("touch \"$HOME/remote-code-ran\"\nalias safe='git status'\n")
	if err := os.WriteFile(filepath.Join(attacker, ".bash_aliases"), remoteBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, attacker, "add", ".bash_aliases")
	runGit(t, attacker, "commit", "-m", "remote update")
	runGit(t, attacker, "push")

	config := defaultConfig()
	config.Repository = repository
	err := reconcileAliases(config)
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

func TestMissingAliasFileDoesNotRestoreRemoteBytes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	repository := filepath.Join(home, "dotfiles")
	if err := os.Mkdir(repository, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, ".bash_aliases"), []byte("touch \"$HOME/remote-code-ran\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := defaultConfig()
	config.Repository = repository
	if err := saveConfig(config); err != nil {
		t.Fatal(err)
	}
	aliasPath := filepath.Join(home, ".bash_aliases")
	if err := ensureAliasFileExists(aliasPath); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(aliasPath)
	if err != nil || len(contents) != 0 {
		t.Fatalf("missing alias file restored remote bytes: %q, %v", contents, err)
	}
}

func TestManualPullStillImportsRemoteAliasesWithoutTopLevelCode(t *testing.T) {
	home := t.TempDir()
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
	config := defaultConfig()
	config.Repository = repository
	if err := saveConfig(config); err != nil {
		t.Fatal(err)
	}

	message, err := pullRepository()
	if err != nil || !strings.Contains(message, "imported 1 aliases") {
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
	home := t.TempDir()
	t.Setenv("HOME", home)
	config := defaultConfig()
	if err := saveConfig(config); err != nil {
		t.Fatal(err)
	}
	path, err := configPath()
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{path, filepath.Join(filepath.Dir(path), "config-link.json")} {
		if source != path {
			if err := os.Symlink(path, source); err != nil {
				t.Fatal(err)
			}
		}
		err := validateTrackedFileConfig(TrackedFileConfig{Source: source, RepositoryPath: "settings.json"})
		if err == nil || !strings.Contains(err.Error(), "Alias Lens configuration") {
			t.Fatalf("config identity %s was accepted: %v", source, err)
		}
	}
}

func TestLoadConfigRejectsInjectedTrackedScope(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path, err := configPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	config := defaultConfig()
	config.TrackedFiles = []TrackedFileConfig{{Source: path, RepositoryPath: "config.json"}}
	contents, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfig(); err == nil || !strings.Contains(err.Error(), "invalid tracked_files entry") {
		t.Fatalf("injected tracked scope was accepted: %v", err)
	}
}

func TestRepositoryWriteRejectsParentSymlink(t *testing.T) {
	directory := t.TempDir()
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
	_, err := syncRepositoryFiles(AppConfig{Repository: repository, AliasFile: "shell/.bash_aliases"}, source, false)
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("repository symlink was accepted: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(outside, ".bash_aliases")); !os.IsNotExist(statErr) {
		t.Fatalf("write escaped the repository: %v", statErr)
	}
}

func TestRepositoryWriteRejectsSymlinkInsertedAfterValidation(t *testing.T) {
	directory := t.TempDir()
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
	home := t.TempDir()
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
	statePath, err := trackedStatePath(tracked)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeSyncStateAt(statePath, SyncState{LocalHash: contentHash(local), RemoteHash: contentHash(local), Status: "synced"}); err != nil {
		t.Fatal(err)
	}
	err = reconcileTrackedFile(AppConfig{Repository: repository}, tracked)
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("remote tracked-file update was not held for review: %v", err)
	}
	after, readErr := os.ReadFile(localPath)
	if readErr != nil || string(after) != string(local) {
		t.Fatalf("live tracked file changed to %q, %v", after, readErr)
	}
}

func TestOutgoingAliasHistoryIsScannedBeforePush(t *testing.T) {
	repository := t.TempDir()
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
	for _, name := range []string{".netrc", ".npmrc", "production.env", "AUTH_TOKEN.txt", ".pypirc", ".pgpass", ".envrc"} {
		if !sensitiveConfigPath(name) {
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
	home := t.TempDir()
	t.Setenv("HOME", home)
	credential := filepath.Join(home, ".netrc")
	link := filepath.Join(home, "ordinary-settings")
	if err := os.WriteFile(credential, []byte("machine example.test login user password value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(credential, link); err != nil {
		t.Fatal(err)
	}
	if err := validateTrackedFileConfig(TrackedFileConfig{Source: link, RepositoryPath: "ordinary-settings"}); err == nil {
		t.Fatal("safe-looking symlink to .netrc was accepted")
	}
}

func TestWebAPIRequiresAuthenticatedLoopbackRequest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), []byte("alias safe='git status'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler, err := newWebHandler("test-session-token", "127.0.0.1:8787")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		method        string
		host          string
		origin        string
		authorization string
		wantStatus    int
	}{
		{name: "missing token", method: http.MethodGet, host: "127.0.0.1:8787", wantStatus: http.StatusUnauthorized},
		{name: "wrong host", method: http.MethodGet, host: "attacker.example", authorization: "Bearer test-session-token", wantStatus: http.StatusForbidden},
		{name: "cross origin", method: http.MethodGet, host: "127.0.0.1:8787", origin: "https://attacker.example", authorization: "Bearer test-session-token", wantStatus: http.StatusForbidden},
		{name: "wrong method", method: http.MethodPost, host: "127.0.0.1:8787", authorization: "Bearer test-session-token", wantStatus: http.StatusMethodNotAllowed},
		{name: "authenticated", method: http.MethodGet, host: "127.0.0.1:8787", origin: "http://127.0.0.1:8787", authorization: "Bearer test-session-token", wantStatus: http.StatusOK},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, "http://127.0.0.1:8787/api/aliases", nil)
			request.Host = test.host
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if test.authorization != "" {
				request.Header.Set("Authorization", test.authorization)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.wantStatus, response.Body.String())
			}
			if test.wantStatus == http.StatusOK && !strings.Contains(response.Body.String(), `"name":"safe"`) {
				t.Fatalf("authenticated response did not contain aliases: %s", response.Body.String())
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
			}
		})
	}
}
