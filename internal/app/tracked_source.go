package app

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

type trackedSourceObservation struct {
	contents []byte
	resolved string
	info     os.FileInfo
}

func (s *SettingsService) validateTrackedSourcePath(path string) error {
	if s.sensitiveConfigPath(path) {
		return fmt.Errorf("refusing to track a credential-shaped file; run al untrack FILE")
	}
	if err := s.dependencies.GuardTrackedSource(path); err != nil {
		return err
	}
	config, err := s.configPath()
	if err != nil {
		return err
	}
	contexts, err := s.dependencies.ContextPath()
	if err != nil {
		return err
	}
	if s.sameFilePath(path, config) {
		return fmt.Errorf("refusing to track Alias Lens configuration; run al untrack FILE")
	}
	if s.sameFilePath(path, contexts) {
		return fmt.Errorf("refusing to track private context marks; run al untrack FILE")
	}
	return nil
}

func (s *SettingsService) observeTrackedSource(path string) (trackedSourceObservation, error) {
	file, resolved, err := openTrustedTrackedSource(path, s.validateTrackedSourcePath)
	if err != nil {
		return trackedSourceObservation{}, err
	}
	defer file.Close()
	before, err := file.Stat()
	if err != nil {
		return trackedSourceObservation{}, err
	}
	contents, err := readOpenedFile(file, trackedFileLimit)
	if err != nil {
		return trackedSourceObservation{}, err
	}
	after, err := file.Stat()
	if err != nil || !sameTrackedSourceInfo(before, after) {
		return trackedSourceObservation{}, fmt.Errorf("tracked source changed while reading; run al watch to retry")
	}
	return trackedSourceObservation{contents: contents, resolved: resolved, info: after}, nil
}

func sameTrackedSourceInfo(left, right os.FileInfo) bool {
	return left != nil && right != nil && os.SameFile(left, right) && left.Mode() == right.Mode() && left.Size() == right.Size() && left.ModTime().Equal(right.ModTime())
}

func (s *SettingsService) verifyTrackedSource(path string, observed trackedSourceObservation) error {
	current, err := s.observeTrackedSource(path)
	if err != nil {
		return err
	}
	if filepath.Clean(current.resolved) != filepath.Clean(observed.resolved) || !sameTrackedSourceInfo(current.info, observed.info) || !bytes.Equal(current.contents, observed.contents) {
		return fmt.Errorf("tracked source changed before publication; run al watch to retry")
	}
	return nil
}
