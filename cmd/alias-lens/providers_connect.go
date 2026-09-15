package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/x/term"
)

const bitbucketTokenURL = "https://id.atlassian.com/manage-profile/security/api-tokens"

type connectableRepoProvider interface {
	RepoProvider
	Connect(context.Context) error
}

var readProviderSecret = func(prompt string) (string, error) {
	if !term.IsTerminal(os.Stdin.Fd()) {
		return "", fmt.Errorf("an interactive terminal is required")
	}
	fmt.Fprint(os.Stderr, prompt)
	secret, err := term.ReadPassword(os.Stdin.Fd())
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(secret)), nil
}

func connectRepoProvider(ctx context.Context, config AppConfig, id string) (RepoProvider, AppConfig, error) {
	settings := config.Providers[id]
	var provider connectableRepoProvider
	switch id {
	case "github":
		settings.Host = defaultString(settings.Host, "github.com")
		provider = &githubProvider{host: settings.Host, protocol: defaultString(settings.Protocol, "auto")}
	case "bitbucket":
		settings.Host = "bitbucket.org"
		provider = &bitbucketProvider{workspaces: settings.Workspaces, protocol: defaultString(settings.Protocol, "auto")}
	case "gitlab":
		settings.Host = defaultString(settings.Host, "gitlab.com")
		provider = &gitlabProvider{host: settings.Host, protocol: defaultString(settings.Protocol, "auto")}
	default:
		return nil, config, fmt.Errorf("unsupported provider %q", id)
	}
	if err := provider.Connect(ctx); err != nil {
		return nil, config, err
	}
	settings.Enabled = true
	settings.Protocol = defaultString(settings.Protocol, "auto")
	if config.Providers == nil {
		config.Providers = make(map[string]ProviderConfig)
	}
	config.Providers[id] = settings
	if err := saveConfig(config); err != nil {
		return nil, config, err
	}
	return provider, config, nil
}

func (p *bitbucketProvider) Connect(context.Context) error {
	if token := strings.TrimSpace(os.Getenv("BITBUCKET_API_TOKEN")); token != "" {
		p.token = token
		return nil
	}
	fmt.Fprintln(os.Stderr, "Alias Lens: Bitbucket Cloud uses scoped API tokens.")
	fmt.Fprintln(os.Stderr, "Create one with workspace read and repository read/write access:")
	fmt.Fprintln(os.Stderr, bitbucketTokenURL)
	token, err := readProviderSecret("Bitbucket API token: ")
	if err != nil {
		return fmt.Errorf("read Bitbucket API token: %w", err)
	}
	if token == "" {
		return fmt.Errorf("Bitbucket API token cannot be empty")
	}
	p.token = token
	return nil
}

func (p *gitlabProvider) Connect(ctx context.Context) error {
	if strings.TrimSpace(os.Getenv("GITLAB_TOKEN")) != "" {
		return nil
	}
	host, err := p.hostname()
	if err != nil {
		return err
	}
	glab, err := exec.LookPath("glab")
	if err != nil {
		return fmt.Errorf("GitLab CLI is required; install glab, then rerun al repo gitlab")
	}
	if err := exec.CommandContext(ctx, glab, "auth", "status", "--hostname", host).Run(); err == nil {
		return nil
	}

	fmt.Fprintln(os.Stderr, "Alias Lens: GitLab sign-in is required. Opening GitLab CLI login...")
	login := exec.CommandContext(ctx, glab, "auth", "login", "--hostname", host, "--web", "--git-protocol", "https")
	login.Stdin = os.Stdin
	login.Stdout = os.Stdout
	login.Stderr = os.Stderr
	if err := login.Run(); err != nil {
		return fmt.Errorf("GitLab sign-in failed: %w", err)
	}
	if err := exec.CommandContext(ctx, glab, "auth", "status", "--hostname", host).Run(); err != nil {
		return fmt.Errorf("GitLab CLI did not report an authenticated account after sign-in")
	}
	return nil
}
