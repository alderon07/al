package app

import (
	"alias-lens/internal/providers"
	"context"
)

func (svc *Services) ListProviderRepositories(ctx context.Context, provider providers.RepoProvider) ([]providers.RemoteRepo, error) {
	listContext, cancel := context.WithTimeout(ctx, repositoryCommandTimeout)
	defer cancel()
	return provider.List(listContext)
}
