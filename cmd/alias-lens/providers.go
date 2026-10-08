package main

import (
	"alias-lens/internal/providers"
	"context"
	"strings"
)

func providerSettings(config AppConfig) map[string]providers.Settings {
	settings := make(map[string]providers.Settings, len(config.Providers))
	for id, value := range config.Providers {
		settings[id] = providers.Settings{Enabled: value.Enabled, Host: value.Host, Protocol: value.Protocol, Workspaces: append([]string(nil), value.Workspaces...)}
	}
	return settings
}
func providerRegistry(config AppConfig) *providers.Registry {
	return providers.New(providerSettings(config), providers.Runtime{Transport: catalogProviderTransport})
}
func configuredProviders(config AppConfig) []providers.RepoProvider {
	return providerRegistry(config).Configured()
}
func providerByID(config AppConfig, id string) providers.RepoProvider {
	return providerRegistry(config).ByID(id)
}
func listRemoteRepositories(ctx context.Context, config AppConfig, only string) ([]providers.RemoteRepo, []string) {
	return providerRegistry(config).List(ctx, only)
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
