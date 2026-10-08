package providers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/alderon07/al/internal/catalog"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type catalogProviderRoundTrip func(*http.Request) (*http.Response, error)

func (f catalogProviderRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func syntheticCatalogProviderBytes(t *testing.T) []byte {
	t.Helper()
	data, d := catalog.Encode(catalog.Catalog{SchemaVersion: 2, Entries: []catalog.Entry{{ID: "11111111111111111111111111111111", Name: "demo", Kind: "command", Portable: &catalog.Portable{Program: "printf", Args: []string{"synthetic"}}}}})
	if len(d) > 0 {
		t.Fatal(d)
	}
	return data
}

func TestRemoteProviderRefusesMismatchedBlobAndLocators(t *testing.T) {
	data := syntheticCatalogProviderBytes(t)
	preview := Object{Revision: strings.Repeat("a", 40), Blob: strings.Repeat("b", 40)}
	if _, err := decodeRemoteCatalog(preview, base64.StdEncoding.EncodeToString(data), int64(len(data))); err == nil {
		t.Fatal("mismatched blob accepted")
	}
	for _, source := range []string{"http://github.com/synthetic/repo", "https://credential@github.com/synthetic/repo", "https://github.com/synthetic/../repo", "https://github.com/synthetic/repo?token=synthetic", "https://github.com/synthetic/repo#branch", "https://github.com:444/synthetic/repo", "https://other.invalid/synthetic/repo"} {
		if _, err := New(defaultSettings(), testRuntime()).ParseLocator(source); err == nil {
			t.Fatal("unsafe locator accepted", source)
		}
	}
}
func TestProviderMissingCredentialsAndWriteAccess(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("PATH", t.TempDir())
	if _, err := (githubProvider{runtime: testRuntime(), host: "github.com"}).PreviewCatalog(context.Background(), Locator{Provider: "github", Host: "github.com", Repository: "synthetic/repo"}, "catalog.json"); err == nil {
		t.Fatal("missing credentials accepted")
	}
	t.Setenv("GH_TOKEN", "synthetic")
	original := catalogProviderTransport
	t.Cleanup(func() { catalogProviderTransport = original })
	catalogProviderTransport = catalogProviderRoundTrip(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"default_branch":"main","permissions":{"push":false}}`)), Header: http.Header{}}, nil
	})
	if _, err := (githubProvider{runtime: testRuntime(), host: "github.com"}).PreviewCatalog(context.Background(), Locator{Provider: "github", Host: "github.com", Repository: "synthetic/repo"}, "catalog.json"); err == nil {
		t.Fatal("write access filter missing")
	}
}

func TestGitLabImmutableCatalogProof(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	t.Setenv("SSH_AUTH_SOCK", "")
	data := syntheticCatalogProviderBytes(t)
	revision := strings.Repeat("a", 40)
	t.Setenv("GITLAB_TOKEN", "synthetic-credential")
	original := catalogProviderTransport
	t.Cleanup(func() { catalogProviderTransport = original })
	for _, damage := range []string{"", "revision", "path", "blob", "unauthorized", "redirect"} {
		t.Run(damage, func(t *testing.T) {
			catalogProviderTransport = catalogProviderRoundTrip(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != "gitlab.com" || r.Header.Get("PRIVATE-TOKEN") != "synthetic-credential" {
					t.Fatal("credential authority mismatch")
				}
				if damage == "unauthorized" {
					return &http.Response{StatusCode: 401, Body: io.NopCloser(strings.NewReader("synthetic-credential")), Header: http.Header{}}, nil
				}
				if damage == "redirect" {
					return &http.Response{StatusCode: 302, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{"Location": []string{"https://other.invalid/"}}}, nil
				}
				var response any
				switch {
				case strings.Contains(r.URL.Path, "/repository/files/"):
					commit, path, blob := revision, "catalog.json", BlobSHA(data)
					if damage == "revision" {
						commit = strings.Repeat("b", 40)
					}
					if damage == "path" {
						path = "other.json"
					}
					if damage == "blob" {
						blob = strings.Repeat("b", 40)
					}
					response = map[string]any{"blob_id": blob, "commit_id": commit, "file_path": path, "size": len(data), "encoding": "base64", "content": base64.StdEncoding.EncodeToString(data)}
				case strings.Contains(r.URL.Path, "/repository/commits/"):
					response = map[string]any{"id": revision}
				default:
					response = map[string]any{"default_branch": "main", "permissions": map[string]any{"project_access": map[string]int{"access_level": 40}}}
				}
				b, _ := json.Marshal(response)
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(b))), Header: http.Header{}}, nil
			})
			preview, err := (gitlabProvider{runtime: testRuntime(), host: "gitlab.com"}).PreviewCatalog(context.Background(), Locator{Provider: "gitlab", Host: "gitlab.com", Repository: "synthetic/repo"}, "catalog.json")
			if damage == "" {
				if err != nil || preview.Blob != BlobSHA(data) {
					t.Fatal("immutable proof failed", err)
				}
			} else if err == nil || strings.Contains(err.Error(), "synthetic-credential") {
				t.Fatal("corrupt provider response accepted or credential leaked")
			}
		})
	}
}

func TestBitbucketImmutableCatalogProofAndPagination(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	t.Setenv("SSH_AUTH_SOCK", "")
	data := syntheticCatalogProviderBytes(t)
	revision := strings.Repeat("a", 40)
	t.Setenv("BITBUCKET_API_TOKEN", "synthetic-credential")
	originalTransport, originalGet := catalogProviderTransport, providerGetJSON
	t.Cleanup(func() { catalogProviderTransport = originalTransport; providerGetJSON = originalGet })
	for _, damage := range []string{"", "revision", "path", "link", "binary", "size", "unauthorized", "redirect"} {
		t.Run(damage, func(t *testing.T) {
			pages := 0
			providerGetJSON = func(ctx context.Context, endpoint, header, credential string, target any) error {
				if !strings.HasPrefix(endpoint, "https://api.bitbucket.org/") || credential != "Bearer synthetic-credential" {
					t.Fatal("list credential authority mismatch")
				}
				pages++
				body := `{"next":"https://api.bitbucket.org/2.0/next","values":[{"permission":"read","repository":{"full_name":"synthetic/read-only"}}]}`
				if pages == 2 {
					body = `{"values":[{"permission":"write","repository":{"full_name":"synthetic/repo"}}]}`
				}
				return json.Unmarshal([]byte(body), target)
			}
			catalogProviderTransport = catalogProviderRoundTrip(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != "api.bitbucket.org" || r.Header.Get("Authorization") != "Bearer synthetic-credential" {
					t.Fatal("credential authority mismatch")
				}
				if damage == "unauthorized" {
					return &http.Response{StatusCode: 401, Body: io.NopCloser(strings.NewReader("synthetic-credential")), Header: http.Header{}}, nil
				}
				if damage == "redirect" {
					return &http.Response{StatusCode: 302, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{"Location": []string{"https://other.invalid/"}}}, nil
				}
				var response any
				switch {
				case strings.Contains(r.URL.Path, "/src/"):
					if r.URL.Query().Get("format") != "meta" {
						return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(data))), Header: http.Header{}}, nil
					}
					commit, path, size, attrs := revision, "catalog.json", len(data), []string{}
					if damage == "revision" {
						commit = strings.Repeat("b", 40)
					}
					if damage == "path" {
						path = "other.json"
					}
					if damage == "size" {
						size++
					}
					if damage == "link" || damage == "binary" {
						attrs = []string{damage}
					}
					response = map[string]any{"type": "commit_file", "path": path, "size": size, "attributes": attrs, "commit": map[string]string{"hash": commit}}
				case strings.Contains(r.URL.Path, "/commit/"):
					response = map[string]string{"hash": revision}
				default:
					response = map[string]any{"mainbranch": map[string]string{"name": "main"}}
				}
				b, _ := json.Marshal(response)
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(b))), Header: http.Header{}}, nil
			})
			preview, err := (bitbucketProvider{runtime: testRuntime(), workspaces: []string{"synthetic"}}).PreviewCatalog(context.Background(), Locator{Provider: "bitbucket", Host: "bitbucket.org", Repository: "synthetic/repo"}, "catalog.json")
			if damage == "" {
				if err != nil || preview.Blob != BlobSHA(data) || pages != 2 {
					t.Fatal("immutable pagination proof failed", err)
				}
			} else if err == nil || strings.Contains(err.Error(), "synthetic-credential") {
				t.Fatal("unsafe proof accepted or credential leaked")
			}
		})
	}
}

func TestProviderQualifiedCatalogSource(t *testing.T) {
	config := defaultSettings()
	github := config["github"]
	github.Enabled = true
	github.Host = "github.example.invalid"
	config["github"] = github
	gitlab := config["gitlab"]
	gitlab.Enabled = true
	gitlab.Host = "https://gitlab.example.invalid:8443"
	config["gitlab"] = gitlab
	bitbucket := config["bitbucket"]
	bitbucket.Enabled = true
	config["bitbucket"] = bitbucket
	for _, fixture := range []struct{ source, provider, host, repository string }{{"github:synthetic/repo", "github", "github.example.invalid", "synthetic/repo"}, {"gitlab:synthetic/nested/repo", "gitlab", "gitlab.example.invalid:8443", "synthetic/nested/repo"}, {"bitbucket:synthetic/repo", "bitbucket", "bitbucket.org", "synthetic/repo"}} {
		locator, err := New(config, testRuntime()).ParseLocator(fixture.source)
		if err != nil || locator.Provider != fixture.provider || locator.Host != fixture.host || locator.Repository != fixture.repository {
			t.Fatal("configured shorthand failed", err)
		}
	}
	for _, source := range []string{"github:synthetic/../repo", "github:synthetic/repo?secret", "github:synthetic/nested/repo", "unknown:synthetic/repo", "gitlab:synthetic/%2e%2e/repo"} {
		if _, err := New(config, testRuntime()).ParseLocator(source); err == nil {
			t.Fatal("unsafe shorthand accepted")
		}
	}
}

func TestGitLabListUsesAuthenticatedCLIWithoutEnvironmentToken(t *testing.T) {
	directory := t.TempDir()
	glabPath := filepath.Join(directory, "glab")
	script := `#!/bin/sh
if [ "$1" = "api" ]; then
  printf '%s\n' '[{"path_with_namespace":"team/dotfiles","visibility":"private","description":"Shell files","ssh_url_to_repo":"git@gitlab.com:team/dotfiles.git","http_url_to_repo":"https://gitlab.com/team/dotfiles.git"}]'
  exit 0
fi
exit 2
`
	if err := os.WriteFile(glabPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory)
	t.Setenv("GITLAB_TOKEN", "")

	repositories, err := (&gitlabProvider{runtime: testRuntime(), host: "gitlab.com"}).List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(repositories) != 1 || repositories[0].FullName != "team/dotfiles" || !repositories[0].Private {
		t.Fatalf("repositories = %#v", repositories)
	}
}

func TestBitbucketListDiscoversWorkspaces(t *testing.T) {
	t.Setenv("BITBUCKET_API_TOKEN", "")
	originalGetJSON := providerGetJSON
	providerGetJSON = func(_ context.Context, endpoint, _, credential string, target any) error {
		if credential != "Bearer temporary-session-value" {
			t.Fatalf("credential header = %q", credential)
		}
		payload := `{"values":[{"workspace":{"slug":"team"}}]}`
		if strings.Contains(endpoint, "/permissions/repositories") {
			payload = `{"values":[{"permission":"write","repository":{"full_name":"team/dotfiles","is_private":true,"description":"Shell files"}}]}`
		}
		return json.Unmarshal([]byte(payload), target)
	}
	defer func() { providerGetJSON = originalGetJSON }()

	provider := &bitbucketProvider{runtime: tokenTestRuntime("temporary-session-value")}
	repositories, err := provider.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(repositories) != 1 || repositories[0].FullName != "team/dotfiles" || !repositories[0].Private {
		t.Fatalf("repositories = %#v", repositories)
	}
}

func TestBitbucketHTTPSCloneKeepsTokenOutOfArgumentsAndRemote(t *testing.T) {
	directory := t.TempDir()
	logPath := filepath.Join(directory, "git.log")
	gitPath := filepath.Join(directory, "git")
	script := `#!/bin/sh
printf 'args=%s\n' "$*" > "$BITBUCKET_CLONE_LOG"
printf 'askpass=%s\n' "$GIT_ASKPASS" >> "$BITBUCKET_CLONE_LOG"
printf 'answer=' >> "$BITBUCKET_CLONE_LOG"
"$GIT_ASKPASS" 'Password for Bitbucket' >> "$BITBUCKET_CLONE_LOG"
exit 0
`
	if err := os.WriteFile(gitPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory)
	t.Setenv("BITBUCKET_CLONE_LOG", logPath)

	repository := RemoteRepo{ProviderTag: "Bitbucket", FullName: "team/dotfiles"}
	cloneURL := "https://x-bitbucket-api-token-auth@bitbucket.org/team/dotfiles.git"
	if err := (Runtime{}).gitCloneWithBitbucketToken(context.Background(), cloneURL, repository, filepath.Join(directory, "checkout"), "temporary-session-value"); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	log := string(contents)
	if strings.Contains(strings.Split(log, "\n")[0], "temporary-session-value") {
		t.Fatalf("clone arguments contained token: %s", log)
	}
	if !strings.Contains(log, "answer=temporary-session-value") {
		t.Fatalf("askpass did not receive the in-memory token: %s", log)
	}
	var askPassPath string
	for _, line := range strings.Split(log, "\n") {
		if strings.HasPrefix(line, "askpass=") {
			askPassPath = strings.TrimPrefix(line, "askpass=")
		}
	}
	if askPassPath == "" {
		t.Fatal("git did not receive an askpass helper")
	}
	if _, err := os.Stat(askPassPath); !os.IsNotExist(err) {
		t.Fatalf("temporary askpass helper still exists: %v", err)
	}
}

func TestProviderResponseAndPaginationLimits(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	var target any
	if err := decodeProviderResponse(bytes.NewReader(bytes.Repeat([]byte("x"), providerResponseLimit+1)), &target); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized provider response was accepted: %v", err)
	}

	if _, _, err := (Runtime{}).outputBounded(context.Background(), 1024, "sh", "-c", "while :; do printf x; done"); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized CLI output was accepted: %v", err)
	}

	previousGetJSON := providerGetJSON
	calls := 0
	providerGetJSON = func(_ context.Context, endpoint, _, _ string, target any) error {
		calls++
		payload := fmt.Sprintf(`{"next":%q,"values":[]}`, endpoint)
		return json.Unmarshal([]byte(payload), target)
	}
	t.Cleanup(func() { providerGetJSON = previousGetJSON })
	provider := bitbucketProvider{runtime: tokenTestRuntime("test"), workspaces: []string{"workspace"}}
	if _, err := provider.List(context.Background()); err == nil || !strings.Contains(err.Error(), "repeated") {
		t.Fatalf("pagination cycle was accepted: %v", err)
	}
	if calls != 1 {
		t.Fatalf("pagination cycle made %d requests, want 1", calls)
	}

	workspaces := make([]string, providerPageLimit+1)
	for index := range workspaces {
		workspaces[index] = fmt.Sprintf("workspace-%d", index)
	}
	calls = 0
	providerGetJSON = func(_ context.Context, _, _, _ string, target any) error {
		calls++
		return json.Unmarshal([]byte(`{"values":[]}`), target)
	}
	provider = bitbucketProvider{runtime: tokenTestRuntime("test"), workspaces: workspaces}
	if _, err := provider.List(context.Background()); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("aggregate pagination limit was not enforced: %v", err)
	}
	if calls != providerPageLimit {
		t.Fatalf("aggregate pagination limit made %d requests, want %d", calls, providerPageLimit)
	}
}

func TestTrustedNextURLRejectsCredentialRedirects(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	if got := trustedNextURL("https://api.bitbucket.org/2.0/example?page=2", "api.bitbucket.org"); got == "" {
		t.Fatal("trusted Bitbucket pagination URL was rejected")
	}
	if got := trustedNextURL("https://evil.example/steal", "api.bitbucket.org"); got != "" {
		t.Fatalf("untrusted pagination URL was accepted: %s", got)
	}
}

var catalogProviderTransport http.RoundTripper = http.DefaultTransport
var providerGetJSON func(context.Context, string, string, string, any) error

func testRuntime() Runtime {
	transport, getJSON := catalogProviderTransport, providerGetJSON
	return Runtime{Transport: catalogProviderRoundTrip(func(request *http.Request) (*http.Response, error) {
		if getJSON != nil && (strings.Contains(request.URL.Path, "/permissions/repositories") || strings.Contains(request.URL.Path, "/user/workspaces") || strings.Contains(request.URL.Path, "/next")) {
			header := "Authorization"
			if request.Header.Get("PRIVATE-TOKEN") != "" {
				header = "PRIVATE-TOKEN"
			}
			var value any
			if err := getJSON(request.Context(), request.URL.String(), header, request.Header.Get(header), &value); err != nil {
				return nil, err
			}
			data, err := json.Marshal(value)
			if err != nil {
				return nil, err
			}
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(data))}, nil
		}
		return transport.RoundTrip(request)
	})}
}
func privateTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if err := os.Chmod(home, 0700); err != nil {
		t.Fatal(err)
	}
	return home
}
func defaultSettings() map[string]Settings {
	return map[string]Settings{"github": {Enabled: true, Host: "github.com", Protocol: "auto"}, "gitlab": {Host: "gitlab.com", Protocol: "auto"}, "bitbucket": {Host: "bitbucket.org", Protocol: "auto"}}
}

func tokenTestRuntime(token string) Runtime {
	runtime := testRuntime()
	runtime.Credential = func(context.Context, string, string) (string, error) { return token, nil }
	return runtime
}
