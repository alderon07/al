package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"alias-lens/cmd/alias-lens/internal/usagelog"
)

type dataPath struct {
	Kind string
	Path string
}

func runDataCommand(arguments []string) error {
	if len(arguments) != 1 {
		return fmt.Errorf("usage: al data paths|clear-usage|clear-revisions")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	switch arguments[0] {
	case "paths":
		paths, err := localDataPaths(home)
		if err != nil {
			return err
		}
		for _, item := range paths {
			fmt.Printf("%-18s %s\n", item.Kind, item.Path)
		}
		return nil
	case "clear-usage":
		if err := removeIfPresent(usagelog.Path(home)); err != nil {
			return fmt.Errorf("clear usage data: %w", err)
		}
		fmt.Println("Alias Lens usage data cleared.")
		return nil
	case "clear-revisions":
		path := filepath.Join(home, ".local", "share", "alias-lens", "revisions")
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("clear revisions: %w", err)
		}
		fmt.Println("Alias Lens revisions cleared. Alias-file backups were kept.")
		return nil
	default:
		return fmt.Errorf("usage: al data paths|clear-usage|clear-revisions")
	}
}

func localDataPaths(home string) ([]dataPath, error) {
	adapter, configuredRepository := readOnlyDataConfig(home)
	aliasPath := filepath.Join(home, adapter.AliasFilename())
	configDirectory := filepath.Join(home, ".config", "alias-lens")
	dataDirectory := filepath.Join(home, ".local", "share", "alias-lens")
	stateDirectory := filepath.Join(home, ".local", "state", "alias-lens")
	paths := []dataPath{
		{Kind: "alias file", Path: aliasPath},
		{Kind: "alias backup", Path: aliasPath + ".alias-lens.bak"},
		{Kind: "configuration", Path: filepath.Join(configDirectory, "config.json")},
		{Kind: "portable catalog", Path: filepath.Join(configDirectory, "catalog.json")},
		{Kind: "Bash suggestions", Path: filepath.Join(configDirectory, "completion.bash")},
		{Kind: "Zsh suggestions", Path: filepath.Join(configDirectory, "completion.zsh")},
		{Kind: "generated shells", Path: filepath.Join(configDirectory, "generated")},
		{Kind: "change backups", Path: filepath.Join(configDirectory, "backups")},
		{Kind: "change journals", Path: filepath.Join(configDirectory, "transactions")},
		{Kind: "theme", Path: filepath.Join(configDirectory, "theme.json")},
		{Kind: "usage", Path: usagelog.Path(home)},
		{Kind: "revisions", Path: filepath.Join(dataDirectory, "revisions")},
		{Kind: "cloned repos", Path: filepath.Join(dataDirectory, "repos")},
		{Kind: "sync state", Path: stateDirectory},
		{Kind: "catalog snapshots", Path: filepath.Join(stateDirectory, "catalog-snapshots")},
		{Kind: "native approvals", Path: filepath.Join(stateDirectory, "native-approvals.json")},
	}
	startupPaths, err := adapter.StartupPaths(home, currentPlatform())
	if err != nil {
		return nil, err
	}
	for _, path := range startupPaths {
		paths = append(paths, dataPath{Kind: "shell startup", Path: path})
	}
	if configuredRepository != "" {
		paths = append(paths, dataPath{Kind: "sync repository", Path: configuredRepository})
	}
	return paths, nil
}

func readOnlyDataConfig(home string) (ShellAdapter, string) {
	adapter := ShellAdapter(bashShellAdapter{})
	adapterFromEnvironment := false
	if selected, err := shellAdapter(os.Getenv(activeShellEnvironment)); err == nil {
		adapter = selected
		adapterFromEnvironment = true
	}
	var settings struct {
		Shell      string `json:"shell"`
		Repository string `json:"repository"`
	}
	contents, err := os.ReadFile(filepath.Join(home, ".config", "alias-lens", "config.json"))
	if err == nil && json.Unmarshal(contents, &settings) == nil {
		if !adapterFromEnvironment {
			if selected, adapterErr := shellAdapter(settings.Shell); adapterErr == nil {
				adapter = selected
			}
		}
	}
	return adapter, settings.Repository
}

func removeIfPresent(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
