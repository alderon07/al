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
	Repository   string                    `json:"repository"`
	AliasFile    string                    `json:"alias_file"`
	Providers    map[string]ProviderConfig `json:"providers"`
	AutoSync     AutoSyncConfig            `json:"auto_sync"`
	TrackedFiles []TrackedFileConfig       `json:"tracked_files,omitempty"`
}

type AutoSyncConfig struct {
	Enabled         bool `json:"enabled"`
	IntervalSeconds int  `json:"interval_seconds"`
}

type TrackedFileConfig struct {
	Source         string `json:"source"`
	RepositoryPath string `json:"repository_path"`
}

func runTrackCommand(arguments []string, remove bool) error {
	if len(arguments) < 1 || len(arguments) > 2 {
		return fmt.Errorf("usage: al %s SOURCE [REPOSITORY_PATH]", map[bool]string{true: "untrack", false: "track"}[remove])
	}
	config, err := loadConfig()
	if err != nil {
		return err
	}
	source, err := expandUserPath(arguments[0])
	if err != nil {
		return err
	}
	if sensitiveConfigPath(source) {
		return fmt.Errorf("refusing to track a credential-shaped file: %s", source)
	}
	if remove {
		var kept []TrackedFileConfig
		for _, tracked := range config.TrackedFiles {
			if tracked.Source != source {
				kept = append(kept, tracked)
			}
		}
		config.TrackedFiles = kept
		return saveConfig(config)
	}
	repositoryPath := filepath.Base(source)
	if len(arguments) == 2 {
		repositoryPath = filepath.Clean(arguments[1])
	}
	if filepath.IsAbs(repositoryPath) || repositoryPath == ".." || strings.HasPrefix(repositoryPath, ".."+string(filepath.Separator)) {
		return fmt.Errorf("repository path must stay inside the configured repository")
	}
	for _, tracked := range config.TrackedFiles {
		if tracked.Source == source || tracked.RepositoryPath == repositoryPath {
			return fmt.Errorf("that source or repository path is already tracked")
		}
	}
	config.TrackedFiles = append(config.TrackedFiles, TrackedFileConfig{Source: source, RepositoryPath: repositoryPath})
	if err := saveConfig(config); err != nil {
		return err
	}
	if config.AutoSync.Enabled {
		return ensureWatchProcess()
	}
	return nil
}

func expandUserPath(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return filepath.Abs(path)
}

func sensitiveConfigPath(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	return name == ".env" || strings.HasPrefix(name, ".env.") || strings.Contains(name, "credential") || strings.Contains(name, "secret") || strings.HasSuffix(name, ".pem") || strings.HasSuffix(name, ".key") || strings.HasSuffix(name, ".p12") || strings.HasSuffix(name, ".pfx")
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
	config = ensureConfigDefaults(config)
	return config, nil
}

func ensureConfigDefaults(config AppConfig) AppConfig {
	if config.AliasFile == "" {
		config.AliasFile = ".bash_aliases"
	}
	if config.Providers == nil {
		config.Providers = defaultConfig().Providers
	}
	if config.AutoSync.IntervalSeconds < 5 {
		config.AutoSync.IntervalSeconds = 15
	}
	return config
}

func defaultConfig() AppConfig {
	return AppConfig{
		AliasFile: ".bash_aliases",
		Providers: map[string]ProviderConfig{
			"github": {Enabled: true, Host: "github.com", Protocol: "auto"},
		},
		AutoSync: AutoSyncConfig{IntervalSeconds: 15},
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
