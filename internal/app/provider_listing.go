package app

import (
	"context"
	"github.com/alderon07/al/internal/providers"
)

func (svc *Services) ListProviderRepositories(ctx context.Context, provider providers.RepoProvider) ([]providers.RemoteRepo, error) {
	listContext, cancel := context.WithTimeout(ctx, repositoryCommandTimeout)
	defer cancel()
	return provider.List(listContext)
}
