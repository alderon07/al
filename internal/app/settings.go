package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"alias-lens/internal/presentation"
	"alias-lens/internal/shell"
	"alias-lens/internal/shortcuts"
	"os"
	"path/filepath"
	"regexp"

	"strings"

	"alias-lens/internal/transaction"
)

const currentConfigVersion = 2

var profileNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

type AppConfig struct {
	observed        []byte
	Version         int                           `json:"version"`
	Repository      string                        `json:"repository"`
	AliasFile       string                        `json:"alias_file"`
	Shell           string                        `json:"shell"`
	Profiles        []string                      `json:"profiles,omitempty"`
	ShortcutProfile string                        `json:"shortcut_profile,omitempty"`
	Shortcuts       map[string]string             `json:"shortcuts,omitempty"`
	Providers       map[string]ProviderConfig     `json:"providers"`
	AutoSync        AutoSyncConfig                `json:"auto_sync"`
	TrackedFiles    []TrackedFileConfig           `json:"tracked_files,omitempty"`
	Appearance      presentation.AppearanceConfig `json:"appearance"`
	Footer          presentation.FooterConfig     `json:"footer"`
}

type AutoSyncConfig struct {
	Enabled         bool `json:"enabled"`
	IntervalSeconds int  `json:"interval_seconds"`
}

type TrackedFileConfig struct {
	Source         string `json:"source"`
	RepositoryPath string `json:"repository_path"`
}

func (s *SettingsService) expandUserPath(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := s.dependencies.HomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return filepath.Abs(path)
}

func (s *SettingsService) sensitiveConfigPath(path string) bool {
	cleaned := strings.ToLower(filepath.Clean(path))
	name := filepath.Base(cleaned)
	if name == ".env" || name == ".envrc" || strings.HasPrefix(name, ".env.") || strings.HasSuffix(name, ".env") {
		return true
	}
	credentialFiles := map[string]bool{
		".authinfo": true, ".authinfo.gpg": true, ".git-credentials": true,
		".netrc": true, "_netrc": true, ".npmrc": true, ".pypirc": true,
		".pgpass": true, ".my.cnf": true, "credentials": true,
	}
	if credentialFiles[name] {
		return true
	}
	for _, component := range strings.Split(cleaned, string(filepath.Separator)) {
		switch component {
		case ".aws", ".docker", ".gnupg", ".ssh", "gcloud":
			return true
		}
	}
	if name == "id_rsa" || name == "id_dsa" || name == "id_ecdsa" || name == "id_ed25519" {
		return true
	}
	return strings.Contains(name, "credential") || strings.Contains(name, "password") || strings.Contains(name, "passwd") || strings.Contains(name, "secret") || strings.Contains(name, "token") || strings.HasSuffix(name, ".pem") || strings.HasSuffix(name, ".key") || strings.HasSuffix(name, ".p12") || strings.HasSuffix(name, ".pfx")
}

func (s *SettingsService) validateTrackedFileConfig(tracked TrackedFileConfig) error {
	if err := s.validateTrackedRegistryEntry(tracked); err != nil {
		return err
	}
	file, _, err := openTrustedTrackedSource(tracked.Source, s.validateTrackedSourcePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if file != nil {
		file.Close()
	}
	return nil
}

func (s *SettingsService) validateTrackedRegistryEntry(tracked TrackedFileConfig) error {
	if !filepath.IsAbs(tracked.Source) {
		return fmt.Errorf("tracked file source must be an absolute path: %s", tracked.Source)
	}
	if s.sensitiveConfigPath(tracked.Source) {
		return fmt.Errorf("refusing to track a credential-shaped file: %s", tracked.Source)
	}
	configFile, err := s.configPath()
	if err != nil {
		return err
	}
	if filepath.Clean(tracked.Source) == filepath.Clean(configFile) {
		return fmt.Errorf("refusing to track Alias Lens configuration: %s", tracked.Source)
	}
	contextsFile, err := s.dependencies.ContextPath()
	if err != nil {
		return err
	}
	if filepath.Clean(tracked.Source) == filepath.Clean(contextsFile) {
		return fmt.Errorf("refusing to track private context marks: %s", tracked.Source)
	}
	if _, err := s.dependencies.ValidateRepositoryPath(tracked.RepositoryPath, "tracked repository path"); err != nil {
		return err
	}
	return nil
}

func (s *SettingsService) sameFilePath(left, right string) bool {
	if filepath.Clean(left) == filepath.Clean(right) {
		return true
	}
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	return leftErr == nil && rightErr == nil && os.SameFile(leftInfo, rightInfo)
}

func (s *SettingsService) validateAppConfig(config AppConfig) error {
	if _, err := shell.New(config.Shell); err != nil {
		return fmt.Errorf("invalid shell in configuration: %w", err)
	}
	if _, err := s.dependencies.ValidateRepositoryPath(config.AliasFile, "alias_file"); err != nil {
		return err
	}
	if config.ShortcutProfile != "" {
		if _, err := shortcuts.ParseProfile(config.ShortcutProfile); err != nil {
			return fmt.Errorf("invalid shortcut_profile in configuration: %w", err)
		}
	}
	if err := shortcuts.ValidateOverrides(config.ShortcutProfile, config.Shortcuts); err != nil {
		return fmt.Errorf("invalid shortcuts in configuration: %w", err)
	}
	if len(config.Profiles) > 32 {
		return fmt.Errorf("invalid profiles in configuration: keep 32 or fewer profile names")
	}
	for index, profile := range config.Profiles {
		if !profileNamePattern.MatchString(profile) {
			return fmt.Errorf("invalid profile name %q: use a lowercase letter first, followed by lowercase letters, numbers, _ or -", profile)
		}
		if index > 0 && config.Profiles[index-1] >= profile {
			return fmt.Errorf("invalid profiles in configuration: names must be unique and sorted")
		}
	}
	if err := presentation.ValidateFooterConfig(config.Footer); err != nil {
		return fmt.Errorf("invalid footer configuration: %w", err)
	}
	if err := presentation.ValidateAppearanceConfig(config.Appearance); err != nil {
		return fmt.Errorf("invalid appearance configuration: %w", err)
	}
	seenSources := make(map[string]bool)
	seenRepositoryPaths := make(map[string]bool)
	for _, tracked := range config.TrackedFiles {
		if err := s.validateTrackedRegistryEntry(tracked); err != nil {
			return fmt.Errorf("invalid tracked_files entry: %w; remove it from config.json", err)
		}
		source := filepath.Clean(tracked.Source)
		repositoryPath := filepath.Clean(tracked.RepositoryPath)
		if seenSources[source] || seenRepositoryPaths[repositoryPath] {
			return fmt.Errorf("invalid tracked_files entry: duplicate source or repository path; remove it from config.json")
		}
		seenSources[source] = true
		seenRepositoryPaths[repositoryPath] = true
	}
	return nil
}

type ProviderConfig struct {
	Enabled    bool     `json:"enabled"`
	Host       string   `json:"host,omitempty"`
	Protocol   string   `json:"protocol,omitempty"`
	Workspaces []string `json:"workspaces,omitempty"`
}

func (s *SettingsService) loadConfig() (AppConfig, error) {
	config := s.defaultConfig()
	path, err := s.configPath()
	if err != nil {
		return config, err
	}
	contents, err := s.dependencies.ReadPrivate(path, transaction.MaxPrivateFileSize)
	if errors.Is(err, os.ErrNotExist) {
		config.observed, _ = json.Marshal(config)
		return config, nil
	}
	if err != nil {
		return config, err
	}
	var fields map[string]json.RawMessage
	if err := decodeUniqueJSON(contents, &fields); err != nil {
		return config, fmt.Errorf("parse %s: %w", path, err)
	}
	raw, versionPresent := fields["version"]
	if !versionPresent {
		return config, fmt.Errorf("parse %s: configuration version is required; recreate the file with al setup", path)
	}
	var version int
	if err := json.Unmarshal(raw, &version); err != nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return config, fmt.Errorf("parse %s: invalid configuration version", path)
	}
	if version != currentConfigVersion {
		if version > currentConfigVersion {
			return config, fmt.Errorf("parse %s: configuration version %d is newer than this Alias Lens supports; update Alias Lens", path, version)
		}
		return config, fmt.Errorf("parse %s: configuration version %d is unsupported; recreate the file with al setup", path, version)
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return config, fmt.Errorf("parse %s: %w", path, err)
	}
	config = s.ensureConfigDefaults(config)
	if err := s.validateAppConfig(config); err != nil {
		return s.defaultConfig(), fmt.Errorf("parse %s: %w", path, err)
	}
	config.observed, _ = json.Marshal(config)
	return config, nil
}

func (s *SettingsService) ensureConfigDefaults(config AppConfig) AppConfig {
	if config.Shell == "" {
		config.Shell = "bash"
	}
	if config.AliasFile == "" {
		adapter, err := shell.New(config.Shell)
		if err != nil {
			adapter, _ = shell.New("bash")
		}
		config.AliasFile = adapter.AliasFilename()
	}
	if config.Providers == nil {
		config.Providers = s.defaultConfig().Providers
	}
	if config.AutoSync.IntervalSeconds < 5 {
		config.AutoSync.IntervalSeconds = 15
	}
	if config.Footer.Message == "" && config.Footer.Icon == "" {
		config.Footer = presentation.DefaultFooterConfig()
	} else if config.Footer.Icon == "" {
		config.Footer.Icon = presentation.DefaultFooterConfig().Icon
	}
	if config.Footer.Alignment == "" {
		config.Footer.Alignment = presentation.DefaultFooterConfig().Alignment
	}
	if config.Footer.Tone == "" {
		config.Footer.Tone = presentation.DefaultFooterConfig().Tone
	}
	if config.Footer.Rule == "" {
		config.Footer.Rule = presentation.DefaultFooterConfig().Rule
	}
	if config.Appearance.Brand == "" && config.Appearance.ArtStyle == "" {
		config.Appearance = presentation.DefaultAppearanceConfig()
	} else {
		if config.Appearance.Brand == "" {
			config.Appearance.Brand = presentation.DefaultAppearanceConfig().Brand
		}
		if config.Appearance.ArtStyle == "" {
			config.Appearance.ArtStyle = presentation.DefaultAppearanceConfig().ArtStyle
		}
		if config.Appearance.MarkerStyle == "" {
			config.Appearance.MarkerStyle = presentation.DefaultAppearanceConfig().MarkerStyle
		}
	}
	return config
}

func (s *SettingsService) defaultConfig() AppConfig {
	return AppConfig{
		Version:   currentConfigVersion,
		AliasFile: ".bash_aliases",
		Shell:     "bash",
		Providers: map[string]ProviderConfig{
			"github": {Enabled: true, Host: "github.com", Protocol: "auto"},
		},
		AutoSync:   AutoSyncConfig{IntervalSeconds: 15},
		Appearance: presentation.DefaultAppearanceConfig(),
		Footer:     presentation.DefaultFooterConfig(),
	}
}

func (s *SettingsService) saveConfig(config AppConfig) error {
	return s.dependencies.Mutate(func(writer settingsWriter) error { return s.saveConfigInSession(writer, config) })
}
func (s *SettingsService) updateConfig(change func(*AppConfig) error) error {
	return s.dependencies.Mutate(func(writer settingsWriter) error { return s.updateConfigInSession(writer, change) })
}
func (s *SettingsService) updateConfigInSession(writer settingsWriter, change func(*AppConfig) error) error {
	config, e := s.loadConfig()
	if e != nil {
		return e
	}
	if e = change(&config); e != nil {
		return e
	}
	return s.saveConfigInSession(writer, config)
}
func (s *SettingsService) saveConfigInSession(writer settingsWriter, config AppConfig) error {
	_, e := s.configPath()
	if e != nil {
		return e
	}
	config = s.ensureConfigDefaults(config)
	config.Version = currentConfigVersion
	if len(config.observed) > 0 {
		current, e := s.loadConfig()
		if e != nil {
			return e
		}
		before := map[string]json.RawMessage{}
		after := map[string]json.RawMessage{}
		fresh := map[string]json.RawMessage{}
		if e = json.Unmarshal(config.observed, &before); e != nil {
			return e
		}
		b, e := json.Marshal(config)
		if e != nil {
			return e
		}
		if e = json.Unmarshal(b, &after); e != nil {
			return e
		}
		b, e = json.Marshal(current)
		if e != nil {
			return e
		}
		if e = json.Unmarshal(b, &fresh); e != nil {
			return e
		}
		keys := map[string]bool{}
		for k := range before {
			keys[k] = true
		}
		for k := range after {
			keys[k] = true
		}
		for k := range keys {
			if bytes.Equal(before[k], after[k]) {
				continue
			}
			if !bytes.Equal(fresh[k], before[k]) && !bytes.Equal(fresh[k], after[k]) {
				return fmt.Errorf("configuration changed after reading it; run the command again")
			}
			if v, ok := after[k]; ok {
				fresh[k] = v
			} else {
				delete(fresh, k)
			}
		}
		b, e = json.Marshal(fresh)
		if e != nil {
			return e
		}
		var merged AppConfig
		if e = json.Unmarshal(b, &merged); e != nil {
			return e
		}
		config = s.ensureConfigDefaults(merged)
	}
	if e = s.validateAppConfig(config); e != nil {
		return e
	}
	contents, e := json.MarshalIndent(config, "", "  ")
	if e != nil {
		return e
	}
	return writer.WriteSettings(append(contents, '\n'))
}
func (s *SettingsService) saveConfigFile(path string, config AppConfig) error {
	expected, e := s.configPath()
	if e != nil {
		return e
	}
	if filepath.Clean(path) != expected {
		return transaction.ErrUnsafePath
	}
	return s.saveConfig(config)
}

func (s *SettingsService) configPath() (string, error) {
	home, err := s.dependencies.HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "alias-lens", "config.json"), nil
}
