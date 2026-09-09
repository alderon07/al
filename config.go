package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type AppConfig struct {
	Repository string `json:"repository"`
	AliasFile  string `json:"alias_file"`
}

func loadConfig() (AppConfig, error) {
	config := AppConfig{AliasFile: ".bash_aliases"}
	path, err := configPath()
	if err != nil {
		return config, err
	}
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return config, nil
	}
	if err != nil {
		return config, err
	}
	if err := json.Unmarshal(contents, &config); err != nil {
		return config, fmt.Errorf("parse %s: %w", path, err)
	}
	if config.AliasFile == "" {
		config.AliasFile = ".bash_aliases"
	}
	return config, nil
}

func saveConfig(config AppConfig) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	contents, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(contents, '\n'), 0o644)
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "alias-lens", "config.json"), nil
}
