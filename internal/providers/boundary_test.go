package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestConfiguredAuthorityBeforeCredentials(t *testing.T) {
	calls := 0
	registry := New(map[string]Settings{"github": {Enabled: true, Host: "enterprise.invalid:8443"}}, Runtime{Credential: func(context.Context, string, string) (string, error) { calls++; return "synthetic", nil }, Transport: catalogProviderRoundTrip(func(*http.Request) (*http.Response, error) {
		calls++
		t.Fatal("untrusted authority reached HTTP")
		return nil, nil
	})})
	provider := registry.ByID("github").(ImmutableProvider)
	for _, host := range []string{"other.invalid", "enterprise.invalid", "credential@enterprise.invalid:8443"} {
		if _, err := provider.PreviewCatalog(context.Background(), Locator{Provider: "github", Host: host, Repository: "synthetic/repo"}, "catalog.json"); err == nil {
			t.Fatal("foreign authority accepted")
		}
		if err := registry.ProveAncestry(context.Background(), strings.Repeat("a", 40), Object{Provider: "github", Host: host, Repository: "synthetic/repo", Revision: strings.Repeat("b", 40)}); err == nil {
			t.Fatal("foreign ancestry authority accepted")
		}
	}
	if calls != 0 {
		t.Fatal("credential lookup happened before authority validation")
	}
}

func TestEnterpriseAncestryAndRegistryRuntimeIsolation(t *testing.T) {
	var workers sync.WaitGroup
	for _, identity := range []string{"one", "two"} {
		identity := identity
		workers.Add(1)
		go func() {
			defer workers.Done()
			host := identity + ".enterprise.invalid:8443"
			runtime := Runtime{Credential: func(_ context.Context, provider, authority string) (string, error) {
				if provider != "github" || authority != host {
					t.Error("credential authority changed")
				}
				return "synthetic-" + identity, nil
			}, Transport: catalogProviderRoundTrip(func(request *http.Request) (*http.Response, error) {
				if request.URL.Host != host || request.Header.Get("Authorization") != "Bearer synthetic-"+identity {
					t.Error("registry runtime crossed instances")
				}
				base := strings.Repeat("a", 40)
				data, _ := json.Marshal(map[string]any{"status": "ahead", "base_commit": map[string]string{"sha": base}, "merge_base_commit": map[string]string{"sha": base}})
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(data)))}, nil
			})}
			settings := map[string]Settings{"github": {Enabled: true, Host: host}}
			registry := New(settings, runtime)
			settings["github"] = Settings{Enabled: true, Host: "changed.invalid"}
			if err := registry.ProveAncestry(context.Background(), strings.Repeat("a", 40), Object{Provider: "github", Host: host, Repository: "synthetic/repo", Revision: strings.Repeat("b", 40)}); err != nil {
				t.Error(err)
			}
		}()
	}
	workers.Wait()
}

func TestProviderRedirectAndPaginationPreserveAuthority(t *testing.T) {
	for _, next := range []string{"https://api.bitbucket.org:8443/next", "https://user@api.bitbucket.org/next", "http://api.bitbucket.org/next"} {
		if trustedNextURL(next, "api.bitbucket.org") != "" {
			t.Fatal("untrusted pagination authority accepted")
		}
	}
	runtime := Runtime{Transport: catalogProviderRoundTrip(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "configured.invalid" {
			t.Fatal("credential crossed authority")
		}
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://configured.invalid:8443/next"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}
	var target any
	if err := runtime.fetchJSON(context.Background(), "https://configured.invalid/start", "Authorization", "Bearer synthetic", &target); err == nil {
		t.Fatal("credential redirect across ports accepted")
	}
}

func TestCredentialOnlyAPIDiscoveryAndBitbucketClone(t *testing.T) {
	for _, id := range []string{"bitbucket", "gitlab"} {
		for _, ambient := range []string{"", "ambient-synthetic"} {
			t.Run(id+ambient, func(t *testing.T) {
				requests := 0
				runtime := Runtime{Getenv: func(string) string { return ambient }, Credential: func(_ context.Context, provider, host string) (string, error) {
					if provider != id {
						t.Fatal("credential provider changed")
					}
					return "synthetic", nil
				}, Transport: catalogProviderRoundTrip(func(request *http.Request) (*http.Response, error) {
					requests++
					data := `[{"path_with_namespace":"synthetic/repo","visibility":"private"}]`
					if id == "bitbucket" {
						data = `{"values":[{"permission":"write","repository":{"full_name":"synthetic/repo"}}]}`
						if request.Header.Get("Authorization") != "Bearer synthetic" {
							t.Fatal("injected Bitbucket credential missing")
						}
					} else if request.Header.Get("PRIVATE-TOKEN") != "synthetic" {
						t.Fatal("injected GitLab credential missing")
					}
					return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(data))}, nil
				})}
				provider := New(map[string]Settings{id: {Enabled: true, Host: id + ".com", Protocol: "https", Workspaces: []string{"synthetic"}}}, runtime).ByID(id)
				repos, err := provider.List(context.Background())
				if err != nil || len(repos) != 1 || requests != 1 {
					t.Fatal("injected API discovery failed", err)
				}
			})
		}
	}
	calls, commands := 0, 0
	runtime := Runtime{Getenv: func(string) string { return "ambient-synthetic" }, Command: func(ctx context.Context, name string, args ...string) *exec.Cmd {
		commands++
		return exec.CommandContext(ctx, "sh", "-c", `test "$BITBUCKET_API_TOKEN" = synthetic && test "$("$GIT_ASKPASS" Password)" = synthetic`)
	}, Credential: func(context.Context, string, string) (string, error) { calls++; return "synthetic", nil }}
	provider := New(map[string]Settings{"bitbucket": {Enabled: true, Protocol: "https"}}, runtime).ByID("bitbucket")
	for _, source := range []string{"https://other.invalid/synthetic/repo.git", "https://bitbucket.org:8443/synthetic/repo.git", "https://user:password@bitbucket.org/synthetic/repo.git", "https://bitbucket.org/other/repo.git"} {
		if err := provider.Clone(context.Background(), RemoteRepo{FullName: "synthetic/repo", HTTPSURL: source}, "unused"); err == nil {
			t.Fatal("untrusted token clone accepted")
		}
	}
	if calls != 0 || commands != 0 {
		t.Fatal("credential or process used for untrusted clone authority")
	}
	if err := provider.Clone(context.Background(), RemoteRepo{ProviderTag: "Bitbucket", FullName: "synthetic/repo", HTTPSURL: "https://bitbucket.org/synthetic/repo.git"}, t.TempDir()); err != nil {
		t.Fatal("credential-only clone failed", err)
	}
	if calls != 1 || commands != 1 {
		t.Fatal("credential-only clone bypassed injected runtime")
	}
}

func TestBitbucketAutoRejectsForgedURLsBeforeSSHProbe(t *testing.T) {
	for _, probeSuccess := range []bool{false, true} {
		for _, repo := range []RemoteRepo{{FullName: "synthetic/repo", HTTPSURL: "https://other.invalid/synthetic/repo.git", SSHURL: "git@bitbucket.org:synthetic/repo.git"}, {FullName: "synthetic/repo", HTTPSURL: "https://bitbucket.org/synthetic/repo.git", SSHURL: "git@other.invalid:synthetic/repo.git"}} {
			calls := 0
			runtime := Runtime{Credential: func(context.Context, string, string) (string, error) { calls++; return "synthetic", nil }, Command: func(ctx context.Context, name string, args ...string) *exec.Cmd {
				calls++
				status := "1"
				if probeSuccess {
					status = "0"
				}
				return exec.CommandContext(ctx, "sh", "-c", "exit "+status)
			}}
			provider := New(map[string]Settings{"bitbucket": {Enabled: true, Protocol: "auto"}}, runtime).ByID("bitbucket")
			if err := provider.Clone(context.Background(), repo, "unused"); err == nil || calls != 0 {
				t.Fatal("forged clone reached credential/SSH/git execution")
			}
		}
	}
}

func TestBitbucketAutoHTTPSOnlySkipsSSHProbe(t *testing.T) {
	commands := 0
	runtime := Runtime{Credential: func(context.Context, string, string) (string, error) { return "synthetic", nil }, Command: func(ctx context.Context, name string, args ...string) *exec.Cmd {
		commands++
		if name != "git" {
			t.Fatal("HTTPS-only repository probed SSH")
		}
		return exec.CommandContext(ctx, "sh", "-c", `test "$BITBUCKET_API_TOKEN" = synthetic && test "$("$GIT_ASKPASS" Password)" = synthetic`)
	}}
	provider := New(map[string]Settings{"bitbucket": {Enabled: true, Protocol: "auto"}}, runtime).ByID("bitbucket")
	if err := provider.Clone(context.Background(), RemoteRepo{ProviderTag: "Bitbucket", FullName: "synthetic/repo", HTTPSURL: "https://bitbucket.org/synthetic/repo.git"}, t.TempDir()); err != nil || commands != 1 {
		t.Fatal("HTTPS-only auto clone failed", err)
	}
}

func TestBitbucketSSHOnlyRejectsUnsafeRepositoryBeforeExecution(t *testing.T) {
	calls := 0
	runtime := Runtime{Credential: func(context.Context, string, string) (string, error) { calls++; return "synthetic", nil }, Command: func(ctx context.Context, name string, args ...string) *exec.Cmd {
		calls++
		return exec.CommandContext(ctx, "sh", "-c", "exit 0")
	}}
	provider := New(map[string]Settings{"bitbucket": {Enabled: true, Protocol: "ssh"}}, runtime).ByID("bitbucket")
	if err := provider.Clone(context.Background(), RemoteRepo{FullName: "synthetic/../repo", SSHURL: "git@bitbucket.org:synthetic/../repo.git"}, "unused"); err == nil || calls != 0 {
		t.Fatal("unsafe SSH repository reached execution")
	}
	if err := provider.Clone(context.Background(), RemoteRepo{FullName: "synthetic/repo", SSHURL: "git@bitbucket.org:synthetic/repo.git"}, t.TempDir()); err != nil || calls != 1 {
		t.Fatal("safe SSH-only repository refused", err)
	}
}

func TestGitHubListPaginationAndWriteFilteringUsingRuntime(t *testing.T) {
	directory := t.TempDir()
	pages := make([]string, 3)
	first := make([]map[string]any, 100)
	for index := range first {
		first[index] = map[string]any{"full_name": fmt.Sprintf("synthetic/read-only-%d", index), "permissions": map[string]bool{"push": false}}
	}
	first[0] = map[string]any{"full_name": "synthetic/first-writable", "private": true, "description": "synthetic first page", "ssh_url": "git@enterprise.invalid:synthetic/first-writable.git", "clone_url": "https://enterprise.invalid/synthetic/first-writable.git", "permissions": map[string]bool{"push": true}}
	first[1] = map[string]any{"full_name": "synthetic/archived-first", "archived": true, "permissions": map[string]bool{"push": true}}
	second := []map[string]any{{"full_name": "synthetic/second-writable", "permissions": map[string]bool{"push": true}}, {"full_name": "synthetic/read-only-second", "permissions": map[string]bool{"push": false}}, {"full_name": "synthetic/archived-second", "archived": true, "permissions": map[string]bool{"push": true}}}
	for index, value := range []any{nil, first, second} {
		pages[index] = filepath.Join(directory, fmt.Sprintf("page-%d.json", index))
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(pages[index], data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	calls := [][]string{}
	lookups := 0
	runtime := Runtime{LookPath: func(name string) (string, error) {
		lookups++
		if name != "gh" {
			t.Fatal("unexpected command lookup")
		}
		return "synthetic-gh", nil
	}, Command: func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name != "gh" {
			t.Fatal("unexpected provider command")
		}
		calls = append(calls, append([]string(nil), args...))
		if len(calls) > 3 {
			t.Fatal("discovery requested an extra page")
		}
		command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestGitHubListRuntimeJSONHelper$")
		command.Env = append(os.Environ(), "AL_PROVIDER_JSON_HELPER=1", "AL_PROVIDER_JSON_PATH="+pages[len(calls)-1])
		return command
	}}
	provider := New(map[string]Settings{"github": {Enabled: true, Host: "enterprise.invalid", Protocol: "https"}}, runtime).ByID("github")
	repos, err := provider.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantCalls := [][]string{{"auth", "status", "--hostname", "enterprise.invalid"}, {"api", "--hostname", "enterprise.invalid", "user/repos?affiliation=owner,collaborator,organization_member&per_page=100&sort=pushed&page=1"}, {"api", "--hostname", "enterprise.invalid", "user/repos?affiliation=owner,collaborator,organization_member&per_page=100&sort=pushed&page=2"}}
	if lookups != 1 || !reflect.DeepEqual(calls, wantCalls) {
		t.Fatal("provider hostname/auth/pagination calls changed")
	}
	names := []string{}
	for _, repo := range repos {
		names = append(names, repo.FullName)
		if repo.Provider != "github" || repo.ProviderTag != "GitHub" {
			t.Fatal("provider identity changed")
		}
	}
	if !reflect.DeepEqual(names, []string{"synthetic/first-writable", "synthetic/second-writable"}) {
		t.Fatal("write access or archive filtering changed")
	}
	if !repos[0].Private || repos[0].Description != "synthetic first page" || repos[0].SSHURL != "git@enterprise.invalid:synthetic/first-writable.git" || repos[0].HTTPSURL != "https://enterprise.invalid/synthetic/first-writable.git" {
		t.Fatal("repository metadata changed")
	}
}

func TestGitHubListRuntimeJSONHelper(t *testing.T) {
	if os.Getenv("AL_PROVIDER_JSON_HELPER") != "1" {
		return
	}
	data, err := os.ReadFile(os.Getenv("AL_PROVIDER_JSON_PATH"))
	if err != nil {
		t.Fatal("cannot read synthetic provider fixture", err)
	}
	if _, err := os.Stdout.Write(data); err != nil {
		t.Fatal("cannot emit synthetic provider fixture", err)
	}
	os.Exit(0)
}
