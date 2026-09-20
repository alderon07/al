package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type observedConfig struct {
	Config            AppConfig
	Present           bool
	MigrationRequired bool
}

// observeConfig reads configuration without migrating it, creating a file, or
// changing any timestamp owned by Alias Lens.
func observeConfig() (observedConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return observedConfig{}, err
	}
	path := filepath.Join(home, ".config", "alias-lens", "config.json")
	contents, err := readObservedPrivateFile(path, 1<<20)
	if errors.Is(err, os.ErrNotExist) {
		return observedConfig{Config: defaultConfig()}, nil
	}
	if err != nil {
		return observedConfig{}, fmt.Errorf("read settings: %w", err)
	}
	var fields map[string]json.RawMessage
	if err := decodeUniqueJSON(contents, &fields); err != nil {
		return observedConfig{}, fmt.Errorf("settings are not valid JSON")
	}
	version := 0
	if raw, exists := fields["version"]; exists {
		if err := json.Unmarshal(raw, &version); err != nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return observedConfig{}, fmt.Errorf("settings have an invalid data format version")
		}
	}
	if version < 0 || version > currentConfigVersion {
		return observedConfig{}, fmt.Errorf("settings use an unsupported data format")
	}
	if version <= 1 {
		for _, field := range []string{"profiles", "shortcut_profile", "footer"} {
			if _, exists := fields[field]; exists {
				return observedConfig{}, fmt.Errorf("settings data format %d cannot contain %q", version, field)
			}
		}
	}
	config := defaultConfig()
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return observedConfig{}, fmt.Errorf("settings contain an unsupported field or value")
	}
	if err := validateAppConfig(config); err != nil {
		return observedConfig{}, fmt.Errorf("settings are invalid: %w", err)
	}
	return observedConfig{Config: config, Present: true, MigrationRequired: version < currentConfigVersion}, nil
}
