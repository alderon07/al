package app

import (
	"errors"
	"os"
)

func (s *Services) CatalogSyncApplicable(force bool) (bool, error) {
	config, e := s.loadConfig()
	if e != nil {
		return true, e
	}
	installed, e := s.catalogSyncShellInstalled(config.Shell)
	if e != nil {
		return true, e
	}
	if !installed && !force {
		return false, nil
	}

	_, _, err := s.readCatalogSyncRecord()
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return true, err
	}

	return true, nil
}
