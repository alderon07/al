package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

func managedRepositoryRoot(home string) string {
	return filepath.Join(home, ".local", "share", "alias-lens", "repos")
}

func managedRepositoryDestination(home string, repo RemoteRepo) (string, error) {
	provider, err := cleanRepositoryPathComponent(repo.Provider)
	if err != nil {
		return "", fmt.Errorf("invalid repository provider: %w", err)
	}
	parts, err := cleanRemoteRepositoryParts(repo.FullName)
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{managedRepositoryRoot(home), provider}, parts...)...), nil
}

func cleanRepositoryPathComponent(component string) (string, error) {
	if component == "" || component == "." || component == ".." || strings.ContainsAny(component, `/\`) {
		return "", fmt.Errorf("unsafe path component %q", component)
	}
	return component, nil
}

func cleanRemoteRepositoryParts(fullName string) ([]string, error) {
	if fullName == "" || strings.HasPrefix(fullName, "/") || strings.Contains(fullName, `\`) {
		return nil, fmt.Errorf("invalid remote repository name %q", fullName)
	}
	parts := strings.Split(fullName, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("remote repository name %q must include its namespace", fullName)
	}
	for _, part := range parts {
		if _, err := cleanRepositoryPathComponent(part); err != nil {
			return nil, fmt.Errorf("invalid remote repository name %q: %w", fullName, err)
		}
	}
	return parts, nil
}
