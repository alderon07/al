package app

import (
	"fmt"
	"path/filepath"
)

func (s *SettingsService) trackFile(source string, requestedPath *string, remove bool) error {
	config, err := s.loadConfig()
	if err != nil {
		return err
	}
	source, err = s.expandUserPath(source)
	if err != nil {
		return err
	}
	if remove {
		var kept []TrackedFileConfig
		for _, tracked := range config.TrackedFiles {
			if tracked.Source != source {
				kept = append(kept, tracked)
			}
		}
		config.TrackedFiles = kept
		return s.saveConfig(config)
	}
	repositoryPath := filepath.Base(source)
	if requestedPath != nil {
		repositoryPath = *requestedPath
	}
	tracked := TrackedFileConfig{Source: source, RepositoryPath: repositoryPath}
	if err := s.validateTrackedFileConfig(tracked); err != nil {
		return err
	}
	tracked.RepositoryPath = filepath.Clean(tracked.RepositoryPath)
	for _, existing := range config.TrackedFiles {
		if existing.Source == source || filepath.Clean(existing.RepositoryPath) == tracked.RepositoryPath {
			return fmt.Errorf("that source or repository path is already tracked")
		}
	}
	config.TrackedFiles = append(config.TrackedFiles, tracked)
	if err := s.saveConfig(config); err != nil {
		return err
	}
	if config.AutoSync.Enabled {
		return s.dependencies.StartAutoSync()
	}
	return nil
}

func (s *SettingsService) Track(source string, repositoryPath *string) error {
	return s.trackFile(source, repositoryPath, false)
}
func (s *SettingsService) Untrack(source string) error { return s.trackFile(source, nil, true) }
