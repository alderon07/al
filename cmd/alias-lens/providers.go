package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
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
}

func configuredProviders(config AppConfig) []RepoProvider {
	var providers []RepoProvider
	for name, settings := range config.Providers {
		if !settings.Enabled {
			continue
		}
		switch strings.ToLower(name) {
		case "github":
			providers = append(providers, githubProvider{host: defaultString(settings.Host, "github.com"), protocol: defaultString(settings.Protocol, "auto")})
		case "bitbucket":
			providers = append(providers, bitbucketProvider{workspaces: settings.Workspaces, protocol: defaultString(settings.Protocol, "auto")})
		case "gitlab":
			providers = append(providers, gitlabProvider{host: defaultString(settings.Host, "gitlab.com"), protocol: defaultString(settings.Protocol, "auto")})
		}
	}
	sort.Slice(providers, func(i, j int) bool { return providers[i].ID() < providers[j].ID() })
	return providers
}

func listRemoteRepositories(ctx context.Context, config AppConfig, only string) ([]RemoteRepo, []string) {
	var repositories []RemoteRepo
	var warnings []string
	for _, provider := range configuredProviders(config) {
		if only != "" && provider.ID() != only {
			continue
		}
		repos, err := provider.List(ctx)
		if err != nil {
			warnings = append(warnings, provider.Label()+": "+err.Error())
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

func providerByID(config AppConfig, id string) RepoProvider {
	for _, provider := range configuredProviders(config) {
		if provider.ID() == id {
			return provider
		}
	}
	return nil
}

type githubProvider struct {
	host     string
	protocol string
}

func (p githubProvider) ID() string    { return "github" }
func (p githubProvider) Label() string { return "GitHub" }

func (p githubProvider) List(ctx context.Context) ([]RemoteRepo, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, fmt.Errorf("install GitHub CLI and run gh auth login")
	}
	if output, err := exec.CommandContext(ctx, "gh", "auth", "status", "--hostname", p.host).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("run gh auth login: %s", cleanCommandOutput(output))
	}
	endpoint := "user/repos?affiliation=owner,collaborator,organization_member&per_page=100&sort=pushed"
	output, err := exec.CommandContext(ctx, "gh", "api", "--hostname", p.host, endpoint, "--paginate", "--slurp").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("could not list repositories: %s", cleanCommandOutput(output))
	}
	var pages [][]struct {
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
	if err := json.Unmarshal(output, &pages); err != nil {
		return nil, fmt.Errorf("invalid API response: %w", err)
	}
	var repositories []RemoteRepo
	for _, page := range pages {
		for _, repo := range page {
			if repo.Permissions.Push && !repo.Archived {
				repositories = append(repositories, RemoteRepo{Provider: p.ID(), ProviderTag: p.Label(), FullName: repo.FullName, Private: repo.Private, Description: repo.Description, SSHURL: repo.SSHURL, HTTPSURL: repo.HTTPSURL})
			}
		}
	}
	return repositories, nil
}

func (p githubProvider) Clone(ctx context.Context, repo RemoteRepo, destination string) error {
	if useSSH(ctx, p.protocol, p.host) {
		return gitClone(ctx, repo.SSHURL, repo, destination)
	}
	output, err := exec.CommandContext(ctx, "gh", "repo", "clone", repo.FullName, destination, "--", "--depth=1").CombinedOutput()
	if err != nil {
		return fmt.Errorf("clone %s: %s", repo.FullName, cleanCommandOutput(output))
	}
	return nil
}

type bitbucketProvider struct {
	workspaces []string
	protocol   string
}

func (p bitbucketProvider) ID() string    { return "bitbucket" }
func (p bitbucketProvider) Label() string { return "Bitbucket" }

func (p bitbucketProvider) List(ctx context.Context) ([]RemoteRepo, error) {
	token := strings.TrimSpace(os.Getenv("BITBUCKET_API_TOKEN"))
	if token == "" {
		return nil, fmt.Errorf("set BITBUCKET_API_TOKEN (the token is never saved by Alias Lens)")
	}
	if len(p.workspaces) == 0 {
		return nil, fmt.Errorf("add a workspace with: al config provider bitbucket WORKSPACE")
	}
	var repositories []RemoteRepo
	for _, workspace := range p.workspaces {
		endpoint := "https://api.bitbucket.org/2.0/user/workspaces/" + url.PathEscape(workspace) + "/permissions/repositories?pagelen=100&q=permission%3E%22read%22"
		for endpoint != "" {
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
			if err := getJSON(ctx, endpoint, "Authorization", "Bearer "+token, &page); err != nil {
				return nil, fmt.Errorf("workspace %s: %w", workspace, err)
			}
			for _, entry := range page.Values {
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
	if useSSH(ctx, p.protocol, "bitbucket.org") {
		return gitClone(ctx, repo.SSHURL, repo, destination)
	}
	return gitClone(ctx, repo.HTTPSURL, repo, destination)
}

type gitlabProvider struct {
	host     string
	protocol string
}

func (p gitlabProvider) ID() string    { return "gitlab" }
func (p gitlabProvider) Label() string { return "GitLab" }

func (p gitlabProvider) List(ctx context.Context) ([]RemoteRepo, error) {
	token := strings.TrimSpace(os.Getenv("GITLAB_TOKEN"))
	if token == "" {
		return nil, fmt.Errorf("set GITLAB_TOKEN (the token is never saved by Alias Lens)")
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
	for page := 1; ; page++ {
		endpoint := fmt.Sprintf("%s/api/v4/projects?membership=true&min_access_level=30&archived=false&simple=true&per_page=100&page=%d&order_by=last_activity_at&sort=desc", base, page)
		var projects []struct {
			PathWithNamespace string `json:"path_with_namespace"`
			Visibility        string `json:"visibility"`
			Description       string `json:"description"`
			SSHURL            string `json:"ssh_url_to_repo"`
			HTTPSURL          string `json:"http_url_to_repo"`
		}
		if err := getJSON(ctx, endpoint, "PRIVATE-TOKEN", token, &projects); err != nil {
			return nil, err
		}
		for _, project := range projects {
			repositories = append(repositories, RemoteRepo{Provider: p.ID(), ProviderTag: p.Label(), FullName: project.PathWithNamespace, Private: project.Visibility == "private", Description: project.Description, SSHURL: project.SSHURL, HTTPSURL: project.HTTPSURL})
		}
		if len(projects) < 100 {
			break
		}
	}
	return repositories, nil
}

func (p gitlabProvider) Clone(ctx context.Context, repo RemoteRepo, destination string) error {
	host := strings.TrimPrefix(strings.TrimPrefix(strings.TrimRight(p.host, "/"), "https://"), "http://")
	if useSSH(ctx, p.protocol, host) {
		return gitClone(ctx, repo.SSHURL, repo, destination)
	}
	return gitClone(ctx, repo.HTTPSURL, repo, destination)
}

func gitClone(ctx context.Context, cloneURL string, repo RemoteRepo, destination string) error {
	if cloneURL == "" {
		return fmt.Errorf("%s did not provide a clone URL", repo.ProviderTag)
	}
	output, err := exec.CommandContext(ctx, "git", "clone", "--depth=1", "--", cloneURL, destination).CombinedOutput()
	if err != nil {
		return fmt.Errorf("clone %s: %s", repo.FullName, cleanCommandOutput(output))
	}
	return nil
}

func useSSH(ctx context.Context, protocol, host string) bool {
	switch strings.ToLower(protocol) {
	case "ssh":
		return true
	case "https":
		return false
	}
	checkContext, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	output, err := exec.CommandContext(checkContext, "ssh", "-T", "-o", "BatchMode=yes", "-o", "ConnectTimeout=4", "-o", "StrictHostKeyChecking=yes", "git@"+host).CombinedOutput()
	if err == nil {
		return true
	}
	message := strings.ToLower(string(output))
	return strings.Contains(message, "successfully authenticated") || strings.Contains(message, "authenticated via ssh") || strings.Contains(message, "welcome to gitlab")
}

func getJSON(ctx context.Context, endpoint, header, credential string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set(header, credential)
	originHost := request.URL.Hostname()
	client := &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(next *http.Request, via []*http.Request) error {
			if next.URL.Scheme != "https" || next.URL.Hostname() != originHost {
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
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("decode API response: %w", err)
	}
	return nil
}

func trustedNextURL(next, host string) string {
	if next == "" {
		return ""
	}
	parsed, err := url.Parse(next)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() != host {
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
