package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func managedRepositoryRoot(home string) string {
	return filepath.Join(home, ".local", "share", "alias-lens", "repos")
}

func ensureManagedRepositoryDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("managed repository path %s is not a real directory", path)
	}
	if !managedRepositoryDirectoryOwnedByUser(info) {
		return fmt.Errorf("managed repository path %s is not owned by the current user", path)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return fmt.Errorf("make managed repository path private: %w", err)
	}
	info, err = os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode().Perm() != 0o700 {
		return fmt.Errorf("managed repository path %s is not private", path)
	}
	return nil
}

func ensureManagedRepositoryPath(root, target string) error {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("managed repository path %s is outside %s", target, root)
	}
	if err := ensureManagedRepositoryDirectory(root); err != nil {
		return err
	}
	if relative == "." {
		return nil
	}
	current := root
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		if err := ensureManagedRepositoryDirectory(current); err != nil {
			return err
		}
	}
	return nil
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
