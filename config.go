package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type AppConfig struct {
	Repository string                    `json:"repository"`
	AliasFile  string                    `json:"alias_file"`
	Providers  map[string]ProviderConfig `json:"providers"`
}

func runConfigCommand(arguments []string) error {
	config, err := loadConfig()
	if err != nil {
		return err
	}
	if len(arguments) == 0 {
		contents, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(contents))
		fmt.Println("Credentials: GitHub CLI · BITBUCKET_API_TOKEN · GITLAB_TOKEN (never stored in config)")
		return nil
	}

	switch arguments[0] {
	case "provider":
		if len(arguments) < 2 || len(arguments) > 3 {
			return fmt.Errorf("usage: al config provider github [HOST] | bitbucket WORKSPACE | gitlab [HOST]")
		}
		name := strings.ToLower(arguments[1])
		settings := config.Providers[name]
		settings.Enabled = true
		if settings.Protocol == "" {
			settings.Protocol = "auto"
		}
		switch name {
		case "github":
			settings.Host = "github.com"
			if len(arguments) == 3 {
				settings.Host = arguments[2]
			}
		case "bitbucket":
			if len(arguments) != 3 {
				return fmt.Errorf("usage: al config provider bitbucket WORKSPACE")
			}
			settings.Host = "bitbucket.org"
			if !slices.Contains(settings.Workspaces, arguments[2]) {
				settings.Workspaces = append(settings.Workspaces, arguments[2])
			}
		case "gitlab":
			settings.Host = "gitlab.com"
			if len(arguments) == 3 {
				settings.Host = arguments[2]
			}
		default:
			return fmt.Errorf("unsupported provider %q (use github, bitbucket, or gitlab)", name)
		}
		config.Providers[name] = settings
	case "protocol":
		if len(arguments) != 3 {
			return fmt.Errorf("usage: al config protocol PROVIDER auto|ssh|https")
		}
		name, protocol := strings.ToLower(arguments[1]), strings.ToLower(arguments[2])
		settings, exists := config.Providers[name]
		if !exists {
			return fmt.Errorf("configure %s first with: al config provider %s", name, name)
		}
		if protocol != "auto" && protocol != "ssh" && protocol != "https" {
			return fmt.Errorf("protocol must be auto, ssh, or https")
		}
		settings.Protocol = protocol
		config.Providers[name] = settings
	case "disable":
		if len(arguments) != 2 {
			return fmt.Errorf("usage: al config disable PROVIDER")
		}
		name := strings.ToLower(arguments[1])
		settings, exists := config.Providers[name]
		if !exists {
			return fmt.Errorf("provider %s is not configured", name)
		}
		settings.Enabled = false
		config.Providers[name] = settings
	default:
		return fmt.Errorf("usage: al config [provider|protocol|disable]")
	}
	if err := saveConfig(config); err != nil {
		return err
	}
	fmt.Println("Alias Lens provider configuration updated")
	return nil
}

type ProviderConfig struct {
	Enabled    bool     `json:"enabled"`
	Host       string   `json:"host,omitempty"`
	Protocol   string   `json:"protocol,omitempty"`
	Workspaces []string `json:"workspaces,omitempty"`
}

func loadConfig() (AppConfig, error) {
	config := defaultConfig()
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
	if config.Providers == nil {
		config.Providers = defaultConfig().Providers
	}
	return config, nil
}

func defaultConfig() AppConfig {
	return AppConfig{
		AliasFile: ".bash_aliases",
		Providers: map[string]ProviderConfig{
			"github": {Enabled: true, Host: "github.com", Protocol: "auto"},
		},
	}
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
