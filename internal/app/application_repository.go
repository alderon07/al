package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"alias-lens/internal/providers"
)

type RepositoryConfiguration struct {
	Name      string
	Directory string
}

func (svc *Services) cloneAndConfigureRepository(ctx context.Context, config AppConfig, connected providers.RepoProvider, repo providers.RemoteRepo) (RepositoryConfiguration, error) {
	home, err := svc.dependencies.HomeDir()
	if err != nil {
		return RepositoryConfiguration{}, err
	}
	root := managedRepositoryRoot(home)
	if err = ensureManagedRepositoryDirectory(root); err != nil {
		return RepositoryConfiguration{}, err
	}
	destination, err := managedRepositoryDestination(home, repo)
	if err != nil {
		return RepositoryConfiguration{}, err
	}
	if err = ensureManagedRepositoryPath(root, destination); err != nil {
		return RepositoryConfiguration{}, err
	}
	if _, err := os.Stat(filepath.Join(destination, ".git")); os.IsNotExist(err) {
		provider := connected
		if provider == nil || provider.ID() != repo.Provider {
			provider = svc.providerByID(config, repo.Provider)
		}
		if provider == nil {
			return RepositoryConfiguration{}, fmt.Errorf("provider %s is no longer configured", repo.Provider)
		}
		cloneContext, cancel := context.WithTimeout(ctx, repositoryCommandTimeout)
		defer cancel()
		if err = provider.Clone(cloneContext, repo, destination); err != nil {
			return RepositoryConfiguration{}, err
		}
	}
	if err = svc.configureRepository(destination); err != nil {
		return RepositoryConfiguration{}, err
	}
	return RepositoryConfiguration{Name: repo.FullName, Directory: destination}, nil
}
