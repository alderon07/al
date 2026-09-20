package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	workflowplan "alias-lens/internal/plan"
)

const currentConfigVersion = 2

var profileNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

type AppConfig struct {
	Version         int                       `json:"version"`
	Repository      string                    `json:"repository"`
	AliasFile       string                    `json:"alias_file"`
	Shell           string                    `json:"shell"`
	Profiles        []string                  `json:"profiles,omitempty"`
	ShortcutProfile string                    `json:"shortcut_profile,omitempty"`
	Providers       map[string]ProviderConfig `json:"providers"`
	AutoSync        AutoSyncConfig            `json:"auto_sync"`
	TrackedFiles    []TrackedFileConfig       `json:"tracked_files,omitempty"`
	Appearance      AppearanceConfig          `json:"appearance"`
	Footer          FooterConfig              `json:"footer"`
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
		repositoryPath = arguments[1]
	}
	tracked := TrackedFileConfig{Source: source, RepositoryPath: repositoryPath}
	if err := validateTrackedFileConfig(tracked); err != nil {
		return err
	}
	tracked.RepositoryPath = filepath.Clean(tracked.RepositoryPath)
	for _, existing := range config.TrackedFiles {
		if existing.Source == source || filepath.Clean(existing.RepositoryPath) == tracked.RepositoryPath {
			return fmt.Errorf("that source or repository path is already tracked")
		}
	}
	config.TrackedFiles = append(config.TrackedFiles, tracked)
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

func validateTrackedFileConfig(tracked TrackedFileConfig) error {
	if !filepath.IsAbs(tracked.Source) {
		return fmt.Errorf("tracked file source must be an absolute path: %s", tracked.Source)
	}
	if sensitiveConfigPath(tracked.Source) {
		return fmt.Errorf("refusing to track a credential-shaped file: %s", tracked.Source)
	}
	if resolved, err := filepath.EvalSymlinks(tracked.Source); err == nil && sensitiveConfigPath(resolved) {
		return fmt.Errorf("refusing to track a link to a credential-shaped file: %s", tracked.Source)
	}
	configFile, err := configPath()
	if err != nil {
		return err
	}
	if sameFilePath(tracked.Source, configFile) {
		return fmt.Errorf("refusing to track Alias Lens configuration: %s", tracked.Source)
	}
	if _, err := cleanRepositoryRelativePath(tracked.RepositoryPath, "tracked repository path"); err != nil {
		return err
	}
	return nil
}

func sameFilePath(left, right string) bool {
	if filepath.Clean(left) == filepath.Clean(right) {
		return true
	}
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	return leftErr == nil && rightErr == nil && os.SameFile(leftInfo, rightInfo)
}

func validateAppConfig(config AppConfig) error {
	if _, err := shellAdapter(config.Shell); err != nil {
		return fmt.Errorf("invalid shell in configuration: %w", err)
	}
	if _, err := cleanRepositoryRelativePath(config.AliasFile, "alias_file"); err != nil {
		return err
	}
	if config.ShortcutProfile != "" {
		if _, err := parseShortcutProfile(config.ShortcutProfile); err != nil {
			return fmt.Errorf("invalid shortcut_profile in configuration: %w", err)
		}
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
	if err := validateFooterConfig(config.Footer); err != nil {
		return fmt.Errorf("invalid footer configuration: %w", err)
	}
	if err := validateAppearanceConfig(config.Appearance); err != nil {
		return fmt.Errorf("invalid appearance configuration: %w", err)
	}
	seenSources := make(map[string]bool)
	seenRepositoryPaths := make(map[string]bool)
	for _, tracked := range config.TrackedFiles {
		if err := validateTrackedFileConfig(tracked); err != nil {
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

func runConfigCommand(arguments []string) error {
	if len(arguments) > 0 && arguments[0] == "profile" {
		return runConfigProfileCommand(arguments[1:])
	}
	if len(arguments) == 1 && arguments[0] == "migrate" {
		return runConfigMigrationCommand()
	}
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
	case "shell":
		if len(arguments) != 2 {
			return fmt.Errorf("usage: al config shell bash|zsh")
		}
		adapter, err := shellAdapter(arguments[1])
		if err != nil {
			return err
		}
		oldAdapter, _ := shellAdapter(config.Shell)
		if oldAdapter != nil && config.AliasFile == oldAdapter.AliasFilename() {
			config.AliasFile = adapter.AliasFilename()
		}
		config.Shell = adapter.Name()
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
	case "footer-message":
		if len(arguments) != 2 {
			return fmt.Errorf("usage: al config footer-message MESSAGE")
		}
		config.Footer.Message = arguments[1]
	case "footer-icon":
		if len(arguments) != 2 {
			return fmt.Errorf("usage: al config footer-icon ICON")
		}
		config.Footer.Icon = arguments[1]
	case "footer-reset":
		if len(arguments) != 1 {
			return fmt.Errorf("usage: al config footer-reset")
		}
		config.Footer = defaultFooterConfig()
	default:
		return fmt.Errorf("usage: al config [shell|provider|protocol|disable|profile|footer-message|footer-icon|footer-reset]")
	}
	if err := saveConfig(config); err != nil {
		return err
	}
	fmt.Println("Alias Lens configuration updated")
	return nil
}

func runConfigProfileCommand(arguments []string) error {
	if len(arguments) == 1 && arguments[0] == "list" {
		observed, err := observeConfig()
		if err != nil {
			return err
		}
		if len(observed.Config.Profiles) == 0 {
			fmt.Println("No machine profiles are active.")
			fmt.Println("Add one with: al config profile add NAME")
			return nil
		}
		fmt.Println("Active machine profiles:")
		for _, profile := range observed.Config.Profiles {
			fmt.Println("-", profile)
		}
		return nil
	}
	if len(arguments) != 2 || (arguments[0] != "add" && arguments[0] != "remove") {
		return fmt.Errorf("usage: al config profile list|add NAME|remove NAME")
	}
	preview, err := buildProfilePlan(arguments[0], arguments[1])
	if err != nil {
		return err
	}
	fmt.Print(workflowplan.RenderPlain(preview))
	if len(preview.Actions) == 0 {
		fmt.Println("Nothing needed to change.")
		return nil
	}
	path, err := configPath()
	if err != nil {
		return err
	}
	action, name := arguments[0], arguments[1]
	if err := applyPrivatePlan(filepath.Dir(path), preview, func() (workflowplan.OperationPlan, error) {
		return buildProfilePlan(action, name)
	}); err != nil {
		return err
	}
	if action == "add" {
		fmt.Printf("Machine profile %q is now active.\n", name)
	} else {
		fmt.Printf("Machine profile %q is no longer active.\n", name)
	}
	fmt.Printf("Your current aliases were left unchanged. Enter al catalog preview --from %s to review the new selection.\n", activeShellNameForConfig())
	return nil
}

func runConfigMigrationCommand() error {
	preview, err := buildConfigMigrationPlan()
	if err != nil {
		return err
	}
	fmt.Print(workflowplan.RenderPlain(preview))
	if len(preview.Actions) == 0 {
		fmt.Println("Nothing needed to change.")
		return nil
	}
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := applyPrivatePlan(filepath.Dir(path), preview, buildConfigMigrationPlan); err != nil {
		return err
	}
	fmt.Println("Settings now use the current data format. Your choices did not change.")
	return nil
}

func activeShellNameForConfig() string {
	observed, err := observeConfig()
	if err == nil && (observed.Config.Shell == "bash" || observed.Config.Shell == "zsh") {
		return observed.Config.Shell
	}
	return "bash"
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
	var fields map[string]json.RawMessage
	if err := decodeUniqueJSON(contents, &fields); err != nil {
		return config, fmt.Errorf("parse %s: %w", path, err)
	}
	rawVersion := 0
	versionPresent := false
	if raw, exists := fields["version"]; exists {
		versionPresent = true
		if err := json.Unmarshal(raw, &rawVersion); err != nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return config, fmt.Errorf("parse %s: invalid configuration version", path)
		}
	}
	if rawVersion <= 1 {
		for _, field := range []string{"profiles", "shortcut_profile", "footer"} {
			if _, exists := fields[field]; exists {
				return config, fmt.Errorf("parse %s: configuration version %d cannot contain %q; remove that field or set it after migration", path, rawVersion, field)
			}
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return config, fmt.Errorf("parse %s: %w", path, err)
	}
	if !versionPresent {
		config.Version = 0
	}
	var migrated bool
	config, migrated, err = migrateConfig(config)
	if err != nil {
		return defaultConfig(), fmt.Errorf("parse %s: %w", path, err)
	}
	config = ensureConfigDefaults(config)
	if err := validateAppConfig(config); err != nil {
		return defaultConfig(), fmt.Errorf("parse %s: %w", path, err)
	}
	if migrated {
		if err := saveConfigFile(path, config, contents); err != nil {
			return defaultConfig(), fmt.Errorf("migrate %s: %w", path, err)
		}
	}
	return config, nil
}

func migrateConfig(config AppConfig) (AppConfig, bool, error) {
	switch config.Version {
	case 0:
		config.Version = currentConfigVersion
		return config, true, nil
	case 1:
		config.Version = currentConfigVersion
		return config, true, nil
	case currentConfigVersion:
		return config, false, nil
	default:
		return config, false, fmt.Errorf("configuration version %d is newer than this Alias Lens supports; update Alias Lens", config.Version)
	}
}

func ensureConfigDefaults(config AppConfig) AppConfig {
	if config.Shell == "" {
		config.Shell = "bash"
	}
	if config.AliasFile == "" {
		adapter, err := shellAdapter(config.Shell)
		if err != nil {
			adapter = bashShellAdapter{}
		}
		config.AliasFile = adapter.AliasFilename()
	}
	if config.Providers == nil {
		config.Providers = defaultConfig().Providers
	}
	if config.AutoSync.IntervalSeconds < 5 {
		config.AutoSync.IntervalSeconds = 15
	}
	if config.Footer.Message == "" && config.Footer.Icon == "" {
		config.Footer = defaultFooterConfig()
	} else if config.Footer.Icon == "" {
		config.Footer.Icon = defaultFooterConfig().Icon
	}
	if config.Footer.Alignment == "" {
		config.Footer.Alignment = defaultFooterConfig().Alignment
	}
	if config.Footer.Tone == "" {
		config.Footer.Tone = defaultFooterConfig().Tone
	}
	if config.Footer.Rule == "" {
		config.Footer.Rule = defaultFooterConfig().Rule
	}
	if config.Appearance.Brand == "" && config.Appearance.ArtStyle == "" {
		config.Appearance = defaultAppearanceConfig()
	} else {
		if config.Appearance.Brand == "" {
			config.Appearance.Brand = defaultAppearanceConfig().Brand
		}
		if config.Appearance.ArtStyle == "" {
			config.Appearance.ArtStyle = defaultAppearanceConfig().ArtStyle
		}
		if config.Appearance.MarkerStyle == "" {
			config.Appearance.MarkerStyle = defaultAppearanceConfig().MarkerStyle
		}
	}
	return config
}

func defaultConfig() AppConfig {
	return AppConfig{
		Version:   currentConfigVersion,
		AliasFile: ".bash_aliases",
		Shell:     "bash",
		Providers: map[string]ProviderConfig{
			"github": {Enabled: true, Host: "github.com", Protocol: "auto"},
		},
		AutoSync:   AutoSyncConfig{IntervalSeconds: 15},
		Appearance: defaultAppearanceConfig(),
		Footer:     defaultFooterConfig(),
	}
}

func saveConfig(config AppConfig) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	config = ensureConfigDefaults(config)
	config.Version = currentConfigVersion
	if err := validateAppConfig(config); err != nil {
		return err
	}
	return saveConfigFile(path, config, nil)
}

func saveConfigFile(path string, config AppConfig, backup []byte) error {
	directoryPath := filepath.Dir(path)
	if err := os.MkdirAll(directoryPath, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(directoryPath, 0o700); err != nil {
		return err
	}
	contents, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	if len(backup) > 0 {
		if err := writePrivateBackup(path+".alias-lens.bak", backup); err != nil {
			return err
		}
	}
	temporary, err := os.CreateTemp(directoryPath, ".config-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(append(contents, '\n')); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return syncDirectory(directoryPath)
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil && !errors.Is(err, fs.ErrInvalid) {
		return err
	}
	return nil
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "alias-lens", "config.json"), nil
}
