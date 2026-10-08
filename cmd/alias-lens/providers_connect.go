package main

import "alias-lens/internal/presentation"

import (
	"alias-lens/internal/providers"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/x/term"
)

const bitbucketTokenURL = "https://id.atlassian.com/manage-profile/security/api-tokens"

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

func connectRepoProvider(ctx context.Context, config AppConfig, id string) (providers.RepoProvider, AppConfig, error) {
	settings := config.Providers[id]
	token := ""
	switch id {
	case "github":
		settings.Host = presentation.DefaultString(settings.Host, "github.com")
		if err := connectGitHub(ctx, settings.Host); err != nil {
			return nil, config, err
		}
	case "gitlab":
		settings.Host = presentation.DefaultString(settings.Host, "gitlab.com")
		hostProvider := providers.New(map[string]providers.Settings{"gitlab": {Enabled: true, Host: settings.Host}}, providers.Runtime{}).ByID("gitlab")
		if err := connectGitLab(ctx, hostProvider.Host()); err != nil {
			return nil, config, err
		}
	case "bitbucket":
		settings.Host = "bitbucket.org"
		var err error
		token, err = connectBitbucket(ctx)
		if err != nil {
			return nil, config, err
		}
	default:
		return nil, config, fmt.Errorf("unsupported provider %q", id)
	}
	settings.Enabled = true
	settings.Protocol = presentation.DefaultString(settings.Protocol, "auto")
	if config.Providers == nil {
		config.Providers = make(map[string]ProviderConfig)
	}
	config.Providers[id] = settings
	if err := saveConfig(config); err != nil {
		return nil, config, err
	}
	return applicationServices().ConnectedProvider(config, id, token), config, nil
}
func connectBitbucket(ctx context.Context) (string, error) {
	if token := strings.TrimSpace(os.Getenv("BITBUCKET_API_TOKEN")); token != "" {
		return token, nil
	}
	fmt.Fprintln(os.Stderr, "Alias Lens: Bitbucket Cloud uses scoped API tokens.")
	fmt.Fprintln(os.Stderr, "Create one with workspace read and repository read/write access:")
	fmt.Fprintln(os.Stderr, bitbucketTokenURL)
	token, err := readProviderSecret("Bitbucket API token: ")
	if err != nil {
		return "", fmt.Errorf("read Bitbucket API token: %w", err)
	}
	if token == "" {
		return "", fmt.Errorf("Bitbucket API token cannot be empty")
	}
	return token, nil
}

func connectGitLab(ctx context.Context, host string) error {
	if strings.TrimSpace(os.Getenv("GITLAB_TOKEN")) != "" {
		return nil
	}
	if host == "" {
		return fmt.Errorf("host must be a valid HTTPS GitLab URL")
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

func connectGitHub(ctx context.Context, host string) error {
	gh, err := exec.LookPath("gh")
	if err != nil {
		return fmt.Errorf("GitHub CLI is required; install gh, then rerun al repo github")
	}
	if err := exec.CommandContext(ctx, gh, "auth", "status", "--hostname", host).Run(); err == nil {
		return nil
	}

	fmt.Fprintln(os.Stderr, "Alias Lens: GitHub sign-in is required. Opening GitHub CLI login...")
	login := exec.CommandContext(ctx, gh, "auth", "login", "--hostname", host, "--web", "--git-protocol", "https")
	login.Stdin = os.Stdin
	login.Stdout = os.Stdout
	login.Stderr = os.Stderr
	if err := login.Run(); err != nil {
		return fmt.Errorf("GitHub sign-in failed: %w", err)
	}
	if err := exec.CommandContext(ctx, gh, "auth", "status", "--hostname", host).Run(); err != nil {
		return fmt.Errorf("GitHub CLI did not report an authenticated account after sign-in")
	}
	return nil
}
