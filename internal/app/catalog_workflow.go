package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"os"
	"path/filepath"
	"strings"

	neutralcatalog "alias-lens/internal/catalog"
)

func (svc *Services) localCatalogPath() string {
	home, _ := svc.dependencies.HomeDir()
	return filepath.Join(home, ".config", "alias-lens", "catalog.json")
}

func readCatalogFile(path string) (neutralcatalog.Catalog, error) {
	contents, err := readRegularFile(path, neutralcatalog.MaxDocumentBytes)
	if err != nil {
		return neutralcatalog.Catalog{}, err
	}
	value, diagnostics := neutralcatalog.Decode(contents)
	if len(diagnostics) > 0 {
		return neutralcatalog.Catalog{}, fmt.Errorf("the catalog data format is not valid")
	}
	return value, nil
}

func (svc *Services) readInstalledCatalogSnapshot(shell string) (neutralcatalog.Catalog, error) {
	if value, handled, err := svc.readLifecycleInstalledSnapshot(shell); handled || err != nil {
		return value, err
	}
	if shell != "bash" && shell != "zsh" {
		return neutralcatalog.Catalog{}, fmt.Errorf("choose Bash or Zsh with --shell")
	}
	home, err := svc.dependencies.HomeDir()
	if err != nil {
		return neutralcatalog.Catalog{}, err
	}
	statePath := filepath.Join(home, ".local", "state", "alias-lens", "catalog-state.json")
	contents, err := svc.readObservedPrivateFile(statePath, 1<<20)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return neutralcatalog.Catalog{}, fmt.Errorf("No saved catalog is installed for %s yet", friendlyShellName(shell))
		}
		return neutralcatalog.Catalog{}, fmt.Errorf("Alias Lens cannot read the installed catalog record")
	}
	var state catalogStateFile
	if err := json.Unmarshal(contents, &state); err != nil || state.SchemaVersion != 1 {
		return neutralcatalog.Catalog{}, fmt.Errorf("Alias Lens cannot use the installed catalog record because its data format is not supported")
	}
	hash := state.InstalledShells[shell].SourceCatalogSHA256
	if len(hash) != 64 || strings.Trim(hash, "0123456789abcdef") != "" {
		return neutralcatalog.Catalog{}, fmt.Errorf("No saved catalog is installed for %s yet", friendlyShellName(shell))
	}
	snapshotPath := filepath.Join(home, ".local", "state", "alias-lens", "catalog-snapshots", hash+".json")
	snapshot, err := svc.readObservedPrivateFile(snapshotPath, neutralcatalog.MaxDocumentBytes)
	if err != nil {
		return neutralcatalog.Catalog{}, fmt.Errorf("Alias Lens cannot read the installed catalog copy")
	}
	value, diagnostics := neutralcatalog.Decode(snapshot)
	if len(diagnostics) > 0 {
		return neutralcatalog.Catalog{}, fmt.Errorf("the installed catalog copy is not valid")
	}
	canonical, diagnostics := neutralcatalog.Encode(value)
	if len(diagnostics) > 0 || hashBytes(canonical) != hash {
		return neutralcatalog.Catalog{}, fmt.Errorf("the installed catalog copy does not match its saved fingerprint")
	}
	return value, nil
}

func (svc *Services) observedCatalogRepositoryPath() (string, error) {
	home, err := svc.dependencies.HomeDir()
	if err != nil {
		return "", err
	}
	state, err := svc.inspectCatalogState(filepath.Join(home, ".local", "state", "alias-lens", "catalog-state.json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("Alias Lens cannot read the saved catalog location")
	}
	path := state.CatalogSync.RepositoryPath
	if path == "" {
		path = filepath.Join("alias-lens", "catalog.json")
	}
	path, err = cleanRepositoryRelativePath(path, "catalog repository path")
	if err != nil {
		return "", fmt.Errorf("the saved catalog location is not safe; enter al doctor")
	}
	return path, nil
}

func encodeCatalogJSON(value any) ([]byte, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return nil, fmt.Errorf("prepare JSON output: %w", err)
	}
	return output.Bytes(), nil
}
