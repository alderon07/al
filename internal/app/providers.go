package app

import (
	"context"
	"github.com/alderon07/al/internal/providers"
	"strings"
)

func providerSettings(config AppConfig) map[string]providers.Settings {
	settings := make(map[string]providers.Settings, len(config.Providers))
	for id, value := range config.Providers {
		settings[id] = providers.Settings{Enabled: value.Enabled, Host: value.Host, Protocol: value.Protocol, Workspaces: append([]string(nil), value.Workspaces...)}
	}
	return settings
}
func (svc *Services) providerRegistry(config AppConfig) *providers.Registry {
	return providers.New(providerSettings(config), providers.Runtime{Transport: svc.dependencies.Transport, Getenv: svc.dependencies.Environment})
}
func (svc *Services) configuredProviders(config AppConfig) []providers.RepoProvider {
	return svc.providerRegistry(config).Configured()
}
func (svc *Services) providerByID(config AppConfig, id string) providers.RepoProvider {
	return svc.providerRegistry(config).ByID(id)
}
func (svc *Services) listRemoteRepositories(ctx context.Context, config AppConfig, only string) ([]providers.RemoteRepo, []string) {
	ctx, cancel := context.WithTimeout(ctx, repositoryCommandTimeout)
	defer cancel()
	return svc.providerRegistry(config).List(ctx, only)
}

func cleanCommandOutput(output []byte) string {
	cleaned := strings.TrimSpace(string(output))
	if cleaned == "" {
		return "command failed"
	}
	return cleaned
}

func (svc *Services) ConnectedProvider(config AppConfig, id, token string) providers.RepoProvider {
	runtime := providers.Runtime{Transport: svc.dependencies.Transport, Getenv: svc.dependencies.Environment}
	if token != "" {
		runtime.Getenv = func(key string) string {
			if key == "BITBUCKET_API_TOKEN" {
				return token
			}
			return svc.dependencies.Environment(key)
		}
	}
	return providers.New(providerSettings(config), runtime).ByID(id)
}
