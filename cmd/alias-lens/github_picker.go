package main

import terminalui "github.com/alderon07/al/internal/tui"

import (
	"github.com/alderon07/al/internal/providers"

	"fmt"
	"strings"
)

func runRepoPicker(only string) error {
	services := applicationServices()
	ctx, cancel := services.InterruptContext()
	defer cancel()
	config, err := services.Settings.Load()
	if err != nil {
		return err
	}
	var provider providers.RepoProvider
	var repos []providers.RemoteRepo
	var warnings []string
	if only != "" {
		provider, config, err = connectRepoProvider(ctx, config, only)
		if err != nil {
			return err
		}
		repos, err = services.ListProviderRepositories(ctx, provider)
		if err != nil {
			return fmt.Errorf("no repositories available\n%s: %w", provider.Label(), err)
		}
	} else {
		repos, warnings = services.ListRemoteRepositories(ctx, config, "")
	}
	if len(repos) == 0 {
		if len(warnings) > 0 {
			return fmt.Errorf("no repositories available\n%s", strings.Join(warnings, "\n"))
		}
		return fmt.Errorf("no writable repositories found")
	}
	return terminalui.RepositoryPicker(ctx, services, terminalui.RepositoryOptions{Config: config, Provider: provider, Repositories: repos, Warnings: warnings})
}
