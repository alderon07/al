package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type ObservedConfig struct {
	Config  AppConfig
	Present bool
}

// observeConfig reads configuration without creating a file or changing any
// timestamp owned by Alias Lens.
func (s *SettingsService) observeConfig() (ObservedConfig, error) {
	home, err := s.dependencies.HomeDir()
	if err != nil {
		return ObservedConfig{}, err
	}
	path := filepath.Join(home, ".config", "alias-lens", "config.json")
	contents, err := s.dependencies.ReadObserved(path, 1<<20)
	if errors.Is(err, os.ErrNotExist) {
		return ObservedConfig{Config: s.defaultConfig()}, nil
	}
	if err != nil {
		return ObservedConfig{}, fmt.Errorf("read settings: %w", err)
	}
	var fields map[string]json.RawMessage
	if err := decodeUniqueJSON(contents, &fields); err != nil {
		return ObservedConfig{}, fmt.Errorf("settings are not valid JSON")
	}
	raw, exists := fields["version"]
	if !exists {
		return ObservedConfig{}, fmt.Errorf("settings have no data format version; recreate them with al setup")
	}
	var version int
	if err := json.Unmarshal(raw, &version); err != nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return ObservedConfig{}, fmt.Errorf("settings have an invalid data format version")
	}
	if version != currentConfigVersion {
		if version > currentConfigVersion {
			return ObservedConfig{}, fmt.Errorf("settings use a newer data format; update Alias Lens")
		}
		return ObservedConfig{}, fmt.Errorf("settings use an unsupported data format; recreate them with al setup")
	}
	config := s.defaultConfig()
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return ObservedConfig{}, fmt.Errorf("settings contain an unsupported field or value")
	}
	if err := s.validateAppConfig(config); err != nil {
		return ObservedConfig{}, fmt.Errorf("settings are invalid: %w", err)
	}
	return ObservedConfig{Config: config, Present: true}, nil
}
