package app

import (
	"fmt"
	"path/filepath"
)

type SyncUnitStatus struct {
	File      TrackedFileConfig
	Source    string
	Target    string
	StatePath string
	State     SyncState
	Error     error
}

type SyncStatusSnapshot struct {
	Enabled bool
	Primary SyncUnitStatus
	Tracked []SyncUnitStatus
}

func (s *Services) SyncStatus() (SyncStatusSnapshot, error) {
	config, err := s.loadConfig()
	if err != nil {
		return SyncStatusSnapshot{}, err
	}
	result := SyncStatusSnapshot{Enabled: config.AutoSync.Enabled, Tracked: make([]SyncUnitStatus, 0, len(config.TrackedFiles))}
	statePath, err := s.syncDataPath("sync-state.json")
	if err != nil {
		result.Primary.Error = fmt.Errorf("locate automatic sync status: %w", err)
		return result, result.Primary.Error
	}
	result.Primary.StatePath = statePath
	home := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(statePath))))
	result.Primary.Source = filepath.Join(home, s.activeShellAdapter().AliasFilename())
	if config.Repository != "" {
		result.Primary.Target = filepath.Join(config.Repository, config.AliasFile)
	}
	result.Primary.State, err = loadSyncStateAt(statePath)
	if err != nil {
		result.Primary.Error = fmt.Errorf("read automatic sync status %s: %w; run al data paths to locate it", statePath, err)
	}
	firstError := result.Primary.Error
	for _, tracked := range config.TrackedFiles {
		unit := SyncUnitStatus{File: tracked, Source: tracked.Source, Target: filepath.Join(config.Repository, tracked.RepositoryPath)}
		unit.StatePath, err = s.trackedStatePath(tracked)
		if err != nil {
			unit.Error = fmt.Errorf("locate tracked sync status for %s: %w", tracked.Source, err)
		} else {
			unit.State, err = loadSyncStateAt(unit.StatePath)
			if err != nil {
				unit.Error = fmt.Errorf("read tracked sync status for %s at %s: %w", tracked.Source, unit.StatePath, err)
			}
		}
		if firstError == nil {
			firstError = unit.Error
		}
		result.Tracked = append(result.Tracked, unit)
	}
	return result, firstError
}

type BrowserSyncSnapshot struct {
	Repository      string
	Enabled         bool
	IntervalSeconds int
	Primary         SyncUnitStatus
	Tracked         []SyncUnitStatus
}

func (s *Services) BrowserSyncStatus() (BrowserSyncSnapshot, error) {
	config, err := s.loadConfig()
	if err != nil {
		return BrowserSyncSnapshot{}, err
	}
	result := BrowserSyncSnapshot{Repository: config.Repository, Enabled: config.AutoSync.Enabled, IntervalSeconds: config.AutoSync.IntervalSeconds, Tracked: make([]SyncUnitStatus, 0, len(config.TrackedFiles))}
	source, pathErr := s.aliasesPath()
	if pathErr != nil {
		source = s.aliasDisplayPath()
	}
	relative := config.AliasFile
	if config.Repository == "" {
		relative = ""
	}
	result.Primary = SyncUnitStatus{File: TrackedFileConfig{Source: source, RepositoryPath: relative}, Source: source, Error: pathErr}
	if config.Repository != "" {
		result.Primary.Target = filepath.Join(config.Repository, relative)
	}
	if pathErr == nil {
		result.Primary.State, result.Primary.Error = s.loadSyncState()
	}
	for _, tracked := range config.TrackedFiles {
		unit := SyncUnitStatus{File: tracked, Source: tracked.Source, Target: filepath.Join(config.Repository, tracked.RepositoryPath)}
		unit.StatePath, unit.Error = s.trackedStatePath(tracked)
		if unit.Error == nil {
			unit.State, unit.Error = loadSyncStateAt(unit.StatePath)
		}
		result.Tracked = append(result.Tracked, unit)
	}
	return result, nil
}
