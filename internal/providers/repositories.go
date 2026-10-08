package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	providerResponseLimit = 8 << 20
	providerPageLimit     = 100
	providerResultLimit   = 10_000
)

type RemoteRepo struct {
	Provider    string
	ProviderTag string
	FullName    string
	Private     bool
	Description string
	SSHURL      string
	HTTPSURL    string
}

type RepoProvider interface {
	ID() string
	Label() string
	List(context.Context) ([]RemoteRepo, error)
	Clone(context.Context, RemoteRepo, string) error
	Host() string
	SSHAvailable(context.Context) bool
}

func (registry *Registry) Configured() []RepoProvider {
	var providers []RepoProvider
	for name, settings := range registry.settings {
		if !settings.Enabled {
			continue
		}
		switch strings.ToLower(name) {
		case "github":
			providers = append(providers, githubProvider{runtime: registry.runtime, host: defaultString(settings.Host, "github.com"), protocol: defaultString(settings.Protocol, "auto")})
		case "bitbucket":
			providers = append(providers, bitbucketProvider{runtime: registry.runtime, workspaces: settings.Workspaces, protocol: defaultString(settings.Protocol, "auto")})
		case "gitlab":
			providers = append(providers, gitlabProvider{runtime: registry.runtime, host: defaultString(settings.Host, "gitlab.com"), protocol: defaultString(settings.Protocol, "auto")})
		}
	}
	sort.Slice(providers, func(i, j int) bool { return providers[i].ID() < providers[j].ID() })
	return providers
}

func (registry *Registry) List(ctx context.Context, only string) ([]RemoteRepo, []string) {
	var repositories []RemoteRepo
	var warnings []string
	for _, provider := range registry.Configured() {
		if only != "" && provider.ID() != only {
			continue
		}
		repos, err := provider.List(ctx)
		if err != nil {
			warnings = append(warnings, provider.Label()+": "+err.Error())
			continue
		}
		if len(repositories)+len(repos) > providerResultLimit {
			warnings = append(warnings, provider.Label()+fmt.Sprintf(": combined repository discovery exceeds %d results", providerResultLimit))
			continue
		}
		repositories = append(repositories, repos...)
	}
	sort.SliceStable(repositories, func(i, j int) bool {
		if repositories[i].Provider == repositories[j].Provider {
			return repositories[i].FullName < repositories[j].FullName
		}
		return repositories[i].Provider < repositories[j].Provider
	})
	return repositories, warnings
}

func (registry *Registry) ByID(id string) RepoProvider {
	for _, provider := range registry.Configured() {
		if provider.ID() == id {
			return provider
		}
	}
	return nil
}

type githubProvider struct {
	runtime  Runtime
	host     string
	protocol string
}

func (p githubProvider) ID() string    { return "github" }
func (p githubProvider) Label() string { return "GitHub" }

func (p githubProvider) List(ctx context.Context) ([]RemoteRepo, error) {
	if _, err := p.runtime.lookPath("gh"); err != nil {
		return nil, fmt.Errorf("GitHub CLI is not installed; install gh, then run al repo github")
	}
	if stdout, stderr, err := p.runtime.outputBounded(ctx, providerResponseLimit, "gh", "auth", "status", "--hostname", p.host); err != nil {
		return nil, fmt.Errorf("run al repo github to sign in: %s", cleanProviderCommandError(stdout, stderr, err))
	}
	type githubRepository struct {
		FullName    string `json:"full_name"`
		Private     bool   `json:"private"`
		Archived    bool   `json:"archived"`
		Description string `json:"description"`
		SSHURL      string `json:"ssh_url"`
		HTTPSURL    string `json:"clone_url"`
		Permissions struct {
			Push bool `json:"push"`
		} `json:"permissions"`
	}
	var repositories []RemoteRepo
	examined := 0
	for page := 1; page <= providerPageLimit; page++ {
		endpoint := fmt.Sprintf("user/repos?affiliation=owner,collaborator,organization_member&per_page=100&sort=pushed&page=%d", page)
		output, stderr, err := p.runtime.outputBounded(ctx, providerResponseLimit, "gh", "api", "--hostname", p.host, endpoint)
		if err != nil {
			return nil, fmt.Errorf("could not list repositories: %s", cleanProviderCommandError(output, stderr, err))
		}
		var results []githubRepository
		if err := json.Unmarshal(output, &results); err != nil {
			return nil, fmt.Errorf("invalid API response: %w", err)
		}
		for _, repo := range results {
			examined++
			if examined > providerResultLimit {
				return nil, fmt.Errorf("GitHub repository discovery exceeds %d results", providerResultLimit)
			}
			if repo.Permissions.Push && !repo.Archived {
				repositories = append(repositories, RemoteRepo{Provider: p.ID(), ProviderTag: p.Label(), FullName: repo.FullName, Private: repo.Private, Description: repo.Description, SSHURL: repo.SSHURL, HTTPSURL: repo.HTTPSURL})
			}
		}
		if len(results) < 100 {
			return repositories, nil
		}
	}
	return nil, fmt.Errorf("GitHub repository discovery exceeds %d pages", providerPageLimit)
}

func (p githubProvider) Clone(ctx context.Context, repo RemoteRepo, destination string) error {
	if p.runtime.useSSH(ctx, p.protocol, p.host) {
		return p.runtime.gitClone(ctx, repo.SSHURL, repo, destination)
	}
	output, err := p.runtime.command(ctx, "gh", "repo", "clone", repo.FullName, destination, "--", "--depth=1").CombinedOutput()
	if err != nil {
		return fmt.Errorf("clone %s: %s", repo.FullName, cleanCommandOutput(output))
	}
	return nil
}

type bitbucketProvider struct {
	runtime    Runtime
	workspaces []string
	protocol   string
}

func (p bitbucketProvider) ID() string    { return "bitbucket" }
func (p bitbucketProvider) Label() string { return "Bitbucket" }

func (p bitbucketProvider) List(ctx context.Context) ([]RemoteRepo, error) {
	token, err := p.runtime.apiToken(ctx, p.ID(), p.Host(), "BITBUCKET_API_TOKEN")
	if err != nil {
		return nil, err
	}
	if token == "" {
		return nil, fmt.Errorf("run al repo bitbucket to enter a temporary API token")
	}
	workspaces := p.workspaces
	if len(workspaces) == 0 {
		var err error
		workspaces, err = p.listWorkspaces(ctx, token)
		if err != nil {
			return nil, err
		}
	}
	var repositories []RemoteRepo
	examined := 0
	pages := 0
	for _, workspace := range workspaces {
		endpoint := "https://api.bitbucket.org/2.0/user/workspaces/" + url.PathEscape(workspace) + "/permissions/repositories?pagelen=100&q=permission%3E%22read%22"
		seenPages := make(map[string]struct{})
		for endpoint != "" {
			pages++
			if pages > providerPageLimit {
				return nil, fmt.Errorf("Bitbucket repository discovery exceeds %d pages", providerPageLimit)
			}
			if _, seen := seenPages[endpoint]; seen {
				return nil, fmt.Errorf("workspace %s returned a repeated repository page", workspace)
			}
			seenPages[endpoint] = struct{}{}
			var page struct {
				Next   string `json:"next"`
				Values []struct {
					Permission string `json:"permission"`
					Repository struct {
						FullName    string `json:"full_name"`
						Private     bool   `json:"is_private"`
						Description string `json:"description"`
					} `json:"repository"`
				} `json:"values"`
			}
			if err := p.runtime.getJSON(ctx, endpoint, "Authorization", "Bearer "+token, &page); err != nil {
				return nil, fmt.Errorf("workspace %s: %w", workspace, err)
			}
			for _, entry := range page.Values {
				examined++
				if examined > providerResultLimit {
					return nil, fmt.Errorf("Bitbucket repository discovery exceeds %d results", providerResultLimit)
				}
				if entry.Permission != "write" && entry.Permission != "admin" {
					continue
				}
				fullName := entry.Repository.FullName
				repositories = append(repositories, RemoteRepo{Provider: p.ID(), ProviderTag: p.Label(), FullName: fullName, Private: entry.Repository.Private, Description: entry.Repository.Description, SSHURL: "git@bitbucket.org:" + fullName + ".git", HTTPSURL: "https://x-bitbucket-api-token-auth@bitbucket.org/" + fullName + ".git"})
			}
			endpoint = trustedNextURL(page.Next, "api.bitbucket.org")
		}
	}
	return repositories, nil
}

func (p bitbucketProvider) Clone(ctx context.Context, repo RemoteRepo, destination string) error {
	if _, err := cleanRemoteRepositoryParts(repo.FullName); err != nil || strings.ContainsAny(repo.FullName, "?#%\\\r\n") {
		return fmt.Errorf("invalid Bitbucket clone repository")
	}
	protocol := strings.ToLower(p.protocol)
	if repo.HTTPSURL != "" || protocol != "ssh" {
		if err := validateBitbucketClone(repo.HTTPSURL, repo.FullName); err != nil {
			return err
		}
	}
	if repo.SSHURL != "" {
		if repo.SSHURL != "git@bitbucket.org:"+repo.FullName+".git" && repo.SSHURL != "ssh://git@bitbucket.org/"+repo.FullName+".git" {
			return fmt.Errorf("Bitbucket clone needs its configured SSH repository authority")
		}
	} else if protocol == "ssh" {
		return fmt.Errorf("%s did not provide a clone URL", repo.ProviderTag)
	}
	if repo.SSHURL != "" && p.runtime.useSSH(ctx, p.protocol, "bitbucket.org") {
		return p.runtime.gitClone(ctx, repo.SSHURL, repo, destination)
	}

	token, err := p.runtime.apiToken(ctx, p.ID(), p.Host(), "BITBUCKET_API_TOKEN")
	if err != nil {
		return err
	}
	if token != "" {
		return p.runtime.gitCloneWithBitbucketToken(ctx, repo.HTTPSURL, repo, destination, token)
	}
	return p.runtime.gitClone(ctx, repo.HTTPSURL, repo, destination)
}

func (p bitbucketProvider) listWorkspaces(ctx context.Context, token string) ([]string, error) {
	endpoint := "https://api.bitbucket.org/2.0/user/workspaces?pagelen=100"
	seen := make(map[string]bool)
	seenPages := make(map[string]struct{})
	var workspaces []string
	pages := 0
	examined := 0
	for endpoint != "" {
		pages++
		if pages > providerPageLimit {
			return nil, fmt.Errorf("Bitbucket workspace discovery exceeds %d pages", providerPageLimit)
		}
		if _, found := seenPages[endpoint]; found {
			return nil, fmt.Errorf("Bitbucket workspace discovery returned a repeated page")
		}
		seenPages[endpoint] = struct{}{}
		var page struct {
			Next   string `json:"next"`
			Values []struct {
				Slug      string `json:"slug"`
				Workspace struct {
					Slug string `json:"slug"`
				} `json:"workspace"`
			} `json:"values"`
		}
		if err := p.runtime.getJSON(ctx, endpoint, "Authorization", "Bearer "+token, &page); err != nil {
			return nil, fmt.Errorf("discover Bitbucket workspaces: %w", err)
		}
		for _, entry := range page.Values {
			examined++
			if examined > providerResultLimit {
				return nil, fmt.Errorf("Bitbucket workspace discovery exceeds %d results", providerResultLimit)
			}
			slug := defaultString(entry.Workspace.Slug, entry.Slug)
			if slug != "" && !seen[slug] {
				seen[slug] = true
				workspaces = append(workspaces, slug)
			}
		}
		endpoint = trustedNextURL(page.Next, "api.bitbucket.org")
	}
	if len(workspaces) == 0 {
		return nil, fmt.Errorf("no Bitbucket workspaces are visible to this token")
	}
	sort.Strings(workspaces)
	return workspaces, nil
}

type gitlabProvider struct {
	runtime  Runtime
	host     string
	protocol string
}

func (p gitlabProvider) ID() string    { return "gitlab" }
func (p gitlabProvider) Label() string { return "GitLab" }

func (p gitlabProvider) List(ctx context.Context) ([]RemoteRepo, error) {
	if _, err := p.hostname(); err != nil {
		return nil, err
	}
	token, err := p.runtime.apiToken(ctx, p.ID(), p.Host(), "GITLAB_TOKEN")
	if err != nil {
		return nil, err
	}
	base := strings.TrimRight(p.host, "/")
	if !strings.Contains(base, "://") {
		base = "https://" + base
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Host == "" || parsed.Scheme != "https" {
		return nil, fmt.Errorf("host must be a valid HTTPS GitLab URL")
	}
	var repositories []RemoteRepo
	examined := 0
	for page := 1; page <= providerPageLimit; page++ {
		endpoint := fmt.Sprintf("%s/api/v4/projects?membership=true&min_access_level=30&archived=false&simple=true&per_page=100&page=%d&order_by=last_activity_at&sort=desc", base, page)
		var projects []struct {
			PathWithNamespace string `json:"path_with_namespace"`
			Visibility        string `json:"visibility"`
			Description       string `json:"description"`
			SSHURL            string `json:"ssh_url_to_repo"`
			HTTPSURL          string `json:"http_url_to_repo"`
		}
		if token != "" {
			if err := p.runtime.getJSON(ctx, endpoint, "PRIVATE-TOKEN", token, &projects); err != nil {
				return nil, err
			}
		} else {
			host, err := p.hostname()
			if err != nil {
				return nil, err
			}
			glab, err := p.runtime.lookPath("glab")
			if err != nil {
				return nil, fmt.Errorf("run al repo gitlab to connect GitLab CLI")
			}
			relativeEndpoint := strings.TrimPrefix(endpoint, base+"/api/v4/")
			output, stderr, err := p.runtime.outputBounded(ctx, providerResponseLimit, glab, "api", "--hostname", host, relativeEndpoint)
			if err != nil {
				return nil, fmt.Errorf("could not list GitLab repositories: %s", cleanProviderCommandError(output, stderr, err))
			}
			if err := json.Unmarshal(output, &projects); err != nil {
				return nil, fmt.Errorf("invalid GitLab CLI response: %w", err)
			}
		}
		for _, project := range projects {
			examined++
			if examined > providerResultLimit {
				return nil, fmt.Errorf("GitLab repository discovery exceeds %d results", providerResultLimit)
			}
			repositories = append(repositories, RemoteRepo{Provider: p.ID(), ProviderTag: p.Label(), FullName: project.PathWithNamespace, Private: project.Visibility == "private", Description: project.Description, SSHURL: project.SSHURL, HTTPSURL: project.HTTPSURL})
		}
		if len(projects) < 100 {
			return repositories, nil
		}
	}
	return nil, fmt.Errorf("GitLab repository discovery exceeds %d pages", providerPageLimit)
}

func (p gitlabProvider) hostname() (string, error) {
	base := strings.TrimRight(p.host, "/")
	if !strings.Contains(base, "://") {
		base = "https://" + base
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Host == "" || parsed.Scheme != "https" || parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") {
		return "", fmt.Errorf("host must be a valid HTTPS GitLab URL")
	}
	return parsed.Host, nil
}

func (p gitlabProvider) Clone(ctx context.Context, repo RemoteRepo, destination string) error {
	host := strings.TrimPrefix(strings.TrimPrefix(strings.TrimRight(p.host, "/"), "https://"), "http://")
	if p.runtime.useSSH(ctx, p.protocol, host) {
		return p.runtime.gitClone(ctx, repo.SSHURL, repo, destination)
	}
	return p.runtime.gitClone(ctx, repo.HTTPSURL, repo, destination)
}

func (runtime Runtime) gitClone(ctx context.Context, cloneURL string, repo RemoteRepo, destination string) error {
	if cloneURL == "" {
		return fmt.Errorf("%s did not provide a clone URL", repo.ProviderTag)
	}
	output, err := runtime.command(ctx, "git", "clone", "--depth=1", "--", cloneURL, destination).CombinedOutput()
	if err != nil {
		return fmt.Errorf("clone %s: %s", repo.FullName, cleanCommandOutput(output))
	}
	return nil
}

func (runtime Runtime) gitCloneWithBitbucketToken(ctx context.Context, cloneURL string, repo RemoteRepo, destination, token string) error {
	if err := validateBitbucketClone(cloneURL, repo.FullName); err != nil {
		return err
	}
	directory, err := os.MkdirTemp("", "alias-lens-askpass-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	askPass := filepath.Join(directory, "askpass")
	script := "#!/bin/sh\ncase \"$1\" in\n  *sername*) printf '%s\\n' 'x-bitbucket-api-token-auth' ;;\n  *) printf '%s\\n' \"$BITBUCKET_API_TOKEN\" ;;\nesac\n"
	if err := os.WriteFile(askPass, []byte(script), 0o700); err != nil {
		return err
	}
	command := runtime.command(ctx, "git", "clone", "--depth=1", "--", cloneURL, destination)
	command.Env = environmentWith(map[string]string{
		"BITBUCKET_API_TOKEN": token,
		"GIT_ASKPASS":         askPass,
		"GIT_TERMINAL_PROMPT": "0",
	})
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("clone %s: %s", repo.FullName, cleanCommandOutput(output))
	}
	return nil
}

func environmentWith(overrides map[string]string) []string {
	environment := make([]string, 0, len(os.Environ())+len(overrides))
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, replaced := overrides[key]; !replaced {
			environment = append(environment, entry)
		}
	}
	for key, value := range overrides {
		environment = append(environment, key+"="+value)
	}
	return environment
}

func (runtime Runtime) useSSH(ctx context.Context, protocol, host string) bool {
	switch strings.ToLower(protocol) {
	case "ssh":
		return true
	case "https":
		return false
	}
	checkContext, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	output, err := runtime.command(checkContext, "ssh", "-T", "-o", "BatchMode=yes", "-o", "ConnectTimeout=4", "-o", "StrictHostKeyChecking=yes", "git@"+host).CombinedOutput()
	if err == nil {
		return true
	}
	message := strings.ToLower(string(output))
	return strings.Contains(message, "successfully authenticated") || strings.Contains(message, "authenticated via ssh") || strings.Contains(message, "welcome to gitlab")
}

func (runtime Runtime) fetchJSON(ctx context.Context, endpoint, header, credential string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	if request.URL.Scheme != "https" || request.URL.Host == "" || request.URL.User != nil {
		return fmt.Errorf("provider endpoint must use configured HTTPS authority")
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set(header, credential)
	originHost := request.URL.Host
	client := &http.Client{
		Transport: runtime.Transport,
		Timeout:   20 * time.Second,
		CheckRedirect: func(next *http.Request, via []*http.Request) error {
			if next.URL.Scheme != "https" || next.URL.Host != originHost || next.URL.User != nil {
				return fmt.Errorf("refusing credential redirect to %s", next.URL.Host)
			}
			return nil
		},
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("API returned %s", response.Status)
	}
	if err := decodeProviderResponse(response.Body, target); err != nil {
		return err
	}
	return nil
}

func decodeProviderResponse(source io.Reader, target any) error {
	contents, err := io.ReadAll(io.LimitReader(source, providerResponseLimit+1))
	if err != nil {
		return fmt.Errorf("read API response: %w", err)
	}
	if len(contents) > providerResponseLimit {
		return fmt.Errorf("API response exceeds %d bytes", providerResponseLimit)
	}
	if err := json.Unmarshal(contents, target); err != nil {
		return fmt.Errorf("decode API response: %w", err)
	}
	return nil
}

func cleanProviderCommandError(stdout, stderr []byte, commandErr error) string {
	if len(stderr) > 0 {
		return terminalSafeText(cleanCommandOutput(stderr))
	}
	if len(stdout) > 0 {
		return terminalSafeText(cleanCommandOutput(stdout))
	}
	return terminalSafeText(commandErr.Error())
}

func trustedNextURL(next, host string) string {
	if next == "" {
		return ""
	}
	parsed, err := url.Parse(next)
	if err != nil || parsed.Scheme != "https" || parsed.Host != host || parsed.User != nil {
		return ""
	}
	return next
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func cleanCommandOutput(output []byte) string {
	cleaned := strings.TrimSpace(string(output))
	if cleaned == "" {
		return "command failed"
	}
	return cleaned
}

func validateBitbucketClone(cloneURL, repository string) error {
	if _, err := cleanRemoteRepositoryParts(repository); err != nil || strings.ContainsAny(repository, "?#%\\\r\n") {
		return fmt.Errorf("invalid Bitbucket clone repository")
	}
	parsed, err := url.Parse(cloneURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "bitbucket.org" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.RawPath != "" || parsed.Path != "/"+repository+".git" {
		return fmt.Errorf("Bitbucket clone needs its configured HTTPS repository authority")
	}
	if parsed.User != nil {
		if _, password := parsed.User.Password(); password || parsed.User.Username() != "x-bitbucket-api-token-auth" {
			return fmt.Errorf("Bitbucket clone URL must not contain credentials")
		}
	}
	return nil
}
