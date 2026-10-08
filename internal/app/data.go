package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"alias-lens/internal/transaction"
	"alias-lens/internal/usagelog"
)

type DataPath struct {
	Kind string
	Path string
}

func (svc *Services) localDataPaths(home string) ([]DataPath, error) {
	adapter, configuredRepository := svc.readOnlyDataConfig(home)
	aliasPath := filepath.Join(home, adapter.AliasFilename())
	configDirectory := filepath.Join(home, ".config", "alias-lens")
	dataDirectory := filepath.Join(home, ".local", "share", "alias-lens")
	stateDirectory := filepath.Join(home, ".local", "state", "alias-lens")
	paths := []DataPath{
		{Kind: "alias file", Path: aliasPath},
		{Kind: "alias backup", Path: aliasPath + ".alias-lens.bak"},
		{Kind: "configuration", Path: filepath.Join(configDirectory, "config.json")},
		{Kind: "context marks", Path: filepath.Join(configDirectory, "contexts.json")},
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
		{Kind: "mutation lock", Path: filepath.Join(stateDirectory, "mutation.lock")},
		{Kind: "worker lifetime lock", Path: filepath.Join(stateDirectory, "watch-worker.lock")},
		{Kind: "workflow journals", Path: filepath.Join(stateDirectory, "workflows")},
		{Kind: "sync state", Path: stateDirectory},
		{Kind: "catalog snapshots", Path: filepath.Join(stateDirectory, "catalog-snapshots")},
		{Kind: "native snapshots", Path: filepath.Join(stateDirectory, "native-snapshots")},
		{Kind: "native approvals", Path: filepath.Join(stateDirectory, "native-approvals.json")},
		{Kind: "catalog revisions", Path: filepath.Join(stateDirectory, "catalog-revisions")},
		{Kind: "catalog installed", Path: filepath.Join(stateDirectory, "catalog-installed.json")},
		{Kind: "catalog ownership", Path: filepath.Join(stateDirectory, "adoptions.json")},
		{Kind: "catalog rollback", Path: filepath.Join(stateDirectory, "rollback")},
		{Kind: "catalog sync/push", Path: filepath.Join(configDirectory, "catalog-sync.json")},
		{Kind: "catalog conflicts", Path: filepath.Join(configDirectory, "catalog-conflicts")},
		{Kind: "clone stage records", Path: filepath.Join(stateDirectory, "catalog-stages")},
		{Kind: "clone stage roots", Path: filepath.Join(dataDirectory, "catalog-repos", ".stage-*")},
		{Kind: "managed catalogs", Path: filepath.Join(dataDirectory, "catalog-repos")},
	}
	startupPaths, err := adapter.StartupPaths(home, svc.currentPlatform())
	if err != nil {
		return nil, err
	}
	for _, path := range startupPaths {
		paths = append(paths, DataPath{Kind: "shell startup", Path: path})
	}
	if configuredRepository != "" {
		paths = append(paths, DataPath{Kind: "sync repository", Path: configuredRepository})
	}
	return paths, nil
}

func (svc *Services) readOnlyDataConfig(home string) (ShellAdapter, string) {
	adapter := ShellAdapter(svc.mustShellAdapter("bash"))
	adapterFromEnvironment := false
	if selected, err := svc.shellAdapter(svc.dependencies.Environment(activeShellEnvironment)); err == nil {
		adapter = selected
		adapterFromEnvironment = true
	}
	var settings struct {
		Shell      string `json:"shell"`
		Repository string `json:"repository"`
	}
	contents, err := readManagedPrivateFile(filepath.Join(home, ".config", "alias-lens", "config.json"), transaction.MaxPrivateFileSize)
	if err == nil && json.Unmarshal(contents, &settings) == nil {
		if !adapterFromEnvironment {
			if selected, adapterErr := svc.shellAdapter(settings.Shell); adapterErr == nil {
				adapter = selected
			}
		}
	}
	return adapter, settings.Repository
}

func clearManagedPrivateFile(path string) error {
	if _, e := os.Lstat(path); errors.Is(e, os.ErrNotExist) {
		return nil
	}
	return transaction.RemoveWorkflowPrivateFile(path)
}
func clearManagedRevisions(path string) error {
	if _, e := os.Lstat(path); errors.Is(e, os.ErrNotExist) {
		return nil
	}
	id, e := transaction.InspectWorkflowDirectory(path)
	if e != nil {
		return e
	}
	if id.Mode != 0o700 {
		return transaction.ErrUnsafePath
	}
	entries, e := os.ReadDir(path)
	if e != nil {
		return e
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return fmt.Errorf("revision directory contains unexpected directories; inspect al data paths")
		}
		if _, e := readManagedPrivateFile(filepath.Join(path, entry.Name()), transaction.MaxPrivateFileSize); e != nil {
			return e
		}
	}
	for _, entry := range entries {
		if e := transaction.RemoveWorkflowPrivateFile(filepath.Join(path, entry.Name())); e != nil {
			return e
		}
	}
	return nil
}
