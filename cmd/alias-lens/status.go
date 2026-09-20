package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	neutralcatalog "alias-lens/internal/catalog"
	"alias-lens/internal/catalogrender"
	workflowstate "alias-lens/internal/state"
	"alias-lens/internal/transaction"
)

type catalogStateFile struct {
	SchemaVersion   int                             `json:"schema_version"`
	InstalledShells map[string]installedShellRecord `json:"installed_shells"`
	CatalogSync     catalogSyncRecord               `json:"catalog_sync"`
}

type installedShellRecord struct {
	ActiveGenerationSHA256 string `json:"active_generation_sha256"`
	SourceCatalogSHA256    string `json:"source_catalog_sha256"`
	ResolvedStateSHA256    string `json:"resolved_state_sha256"`
	LoaderSHA256           string `json:"loader_sha256"`
}

type catalogSyncRecord struct {
	RepositoryPath     string `json:"repository_path"`
	BaseCatalogSHA256  string `json:"base_catalog_sha256"`
	LocalCatalogSHA256 string `json:"local_catalog_sha256"`
	RemoteCatalogHash  string `json:"remote_catalog_sha256"`
}

var errInvalidObservedCatalog = errors.New("catalog contents are invalid")

func runStatusCommand(arguments []string) (int, error) {
	jsonOutput := false
	if len(arguments) == 1 && arguments[0] == "--json" {
		jsonOutput = true
	} else if len(arguments) != 0 {
		return 2, fmt.Errorf("usage: al status [--json]")
	}
	report := inspectWorkflowStatus()
	if jsonOutput {
		output, err := workflowstate.EncodeJSON(report)
		if err != nil {
			return 1, fmt.Errorf("prepare status report: %w", err)
		}
		if _, err := os.Stdout.Write(output); err != nil {
			return 1, err
		}
	} else {
		fmt.Print(workflowstate.RenderPlain(report))
	}
	return workflowstate.ExitCode(report), nil
}

func inspectWorkflowStatus() workflowstate.Report {
	inputs := workflowstate.Inputs{
		Mode: workflowstate.ModeLegacy,
		Config: workflowstate.ConfigObservation{
			Present:       true,
			SchemaVersion: currentConfigVersion,
		},
		Sync: workflowstate.SyncObservation{},
	}
	observed, configErr := observeConfig()
	if configErr != nil {
		if errors.Is(configErr, os.ErrPermission) {
			inputs.Config.Unreadable = true
		} else {
			inputs.Config.Invalid = true
		}
	} else {
		inputs.Config.Present = observed.Present
		version := observed.Config.Version
		if !observed.Present {
			version = currentConfigVersion
		}
		inputs.Config.SchemaVersion = version
		inputs.Sync.Configured = observed.Config.Repository != ""
	}

	catalog, catalogHash, catalogErr := inspectLocalCatalog()
	switch {
	case errors.Is(catalogErr, os.ErrNotExist):
		inputs.Catalog.Present = false
	case errors.Is(catalogErr, errInvalidObservedCatalog):
		inputs.Catalog.Present = true
		inputs.Catalog.Invalid = true
	case catalogErr != nil:
		inputs.Catalog.Present = true
		inputs.Catalog.Unreadable = true
	default:
		inputs.Catalog = workflowstate.CatalogObservation{
			Present: true, SchemaVersion: catalog.SchemaVersion,
			EntryCount: len(catalog.Entries), SHA256: catalogHash,
		}
	}

	home, homeErr := os.UserHomeDir()
	var stored catalogStateFile
	var storedErr error
	if homeErr == nil {
		stored, storedErr = inspectCatalogState(filepath.Join(home, ".local", "state", "alias-lens", "catalog-state.json"))
		inputs.Recovery = combineRecovery(
			inspectRecovery(filepath.Join(home, ".local", "state", "alias-lens"), filepath.Join(home, ".local", "state", "alias-lens", "transactions")),
			inspectRecovery(filepath.Join(home, ".config", "alias-lens"), filepath.Join(home, ".config", "alias-lens", "transactions")),
		)
		if storedErr != nil {
			inputs.Recovery.Blocked = true
		}
	}
	if len(stored.InstalledShells) > 0 {
		inputs.Mode = workflowstate.ModeCatalog
		if configErr == nil {
			if _, installed := stored.InstalledShells[observed.Config.Shell]; !installed {
				inputs.Mode = workflowstate.ModeMixed
			}
		}
	}
	if inputs.Mode == workflowstate.ModeLegacy {
		// Legacy synchronization has its own stable status command. The catalog
		// status contract must not reinterpret a legacy alias repository.
		inputs.Sync = workflowstate.SyncObservation{}
	}
	if inputs.Catalog.Present {
		approvals, approvalsErr := inspectNativeApprovals(home)
		shellNames := []string{}
		for shell := range stored.InstalledShells {
			shellNames = append(shellNames, shell)
		}
		if len(shellNames) == 0 && configErr == nil {
			shellNames = append(shellNames, observed.Config.Shell)
		}
		sort.Strings(shellNames)
		for _, shell := range shellNames {
			record, installed := stored.InstalledShells[shell]
			resolved, renderDiagnostics := catalogrender.Render(catalog, catalogrender.RenderContext{Shell: shell, Platform: currentPlatform(), Profiles: observed.Config.Profiles, Approvals: approvals})
			observation := workflowstate.ShellObservation{Name: shell, Resolved: workflowstate.ResolvedSummary{SHA256: resolved.ResolvedSHA256, EligibleEntries: len(resolved.IncludedEntryIDs), PendingApprovals: len(resolved.PendingApprovals)}, Unreadable: configErr != nil || storedErr != nil || approvalsErr != nil || len(renderDiagnostics) > 0}
			if installed {
				observation.Installed = &workflowstate.InstalledObservation{
					GenerationSHA256:       record.ActiveGenerationSHA256,
					ResolvedStateSHA256:    record.ResolvedStateSHA256,
					RecordedLoaderSHA256:   record.LoaderSHA256,
					ActiveGenerationSHA256: inspectActiveGeneration(home, shell),
					OnDiskGenerationSHA256: inspectGeneratedFile(home, shell, record.ActiveGenerationSHA256),
					OnDiskLoaderSHA256:     inspectLoaderHash(home, shell),
				}
			}
			inputs.Shells = append(inputs.Shells, observation)
		}
	}
	if inputs.Sync.Configured && configErr == nil {
		inputs.Sync.BaseSHA256 = stored.CatalogSync.BaseCatalogSHA256
		inputs.Sync.LocalSHA256 = catalogHash
		path := stored.CatalogSync.RepositoryPath
		if path == "" {
			path = filepath.Join("alias-lens", "catalog.json")
		}
		path, pathErr := cleanRepositoryRelativePath(path, "catalog repository path")
		if pathErr != nil {
			inputs.Sync.Invalid = true
			return workflowstate.Resolve(inputs)
		}
		repositoryCatalog, pathErr := repositoryFilePath(observed.Config.Repository, path)
		if pathErr != nil {
			inputs.Sync.Invalid = true
			return workflowstate.Resolve(inputs)
		}
		_, remoteHash, err := inspectRepositoryCatalogAt(repositoryCatalog)
		if err != nil {
			inputs.Sync.Invalid = true
		} else {
			inputs.Sync.RemoteSHA256 = remoteHash
		}
	}
	return workflowstate.Resolve(inputs)
}

func inspectNativeApprovals(home string) (map[catalogrender.NativeApprovalKey]bool, error) {
	result := map[catalogrender.NativeApprovalKey]bool{}
	if home == "" {
		return result, nil
	}
	contents, err := readObservedPrivateFile(filepath.Join(home, ".local", "state", "alias-lens", "native-approvals.json"), 1<<20)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	var file struct {
		SchemaVersion int `json:"schema_version"`
		Approvals     []struct {
			EntryID              string `json:"entry_id"`
			Shell                string `json:"shell"`
			Kind                 string `json:"kind"`
			ImplementationSHA256 string `json:"implementation_sha256"`
			Renderer             string `json:"renderer"`
		} `json:"approvals"`
	}
	if json.Unmarshal(contents, &file) != nil || file.SchemaVersion != 1 {
		return nil, fmt.Errorf("native approvals are invalid")
	}
	for _, item := range file.Approvals {
		key := catalogrender.NativeApprovalKey{EntryID: item.EntryID, Shell: item.Shell, Kind: item.Kind, ImplementationSHA256: item.ImplementationSHA256, Renderer: item.Renderer}
		result[key] = true
	}
	return result, nil
}

func inspectLoaderHash(home, shell string) string {
	adapter, err := shellAdapter(shell)
	if err != nil {
		return ""
	}
	paths, err := adapter.StartupPaths(home, currentPlatform())
	if err != nil || len(paths) == 0 {
		return ""
	}
	block := bashAliasLoader
	if shell == "zsh" {
		block = zshAliasLoader
	}
	contents, err := readRegularFile(paths[0], 1<<20)
	if err != nil || !strings.Contains(string(contents), block) {
		return ""
	}
	return hashBytes([]byte(block))
}

func combineRecovery(values ...workflowstate.RecoveryObservation) workflowstate.RecoveryObservation {
	result := workflowstate.RecoveryObservation{}
	for _, value := range values {
		result.Required = result.Required || value.Required
		result.Blocked = result.Blocked || value.Blocked
	}
	return result
}

func inspectLocalCatalog() (neutralcatalog.Catalog, string, error) {
	return inspectCatalogAt(localCatalogPath())
}

func inspectCatalogAt(path string) (neutralcatalog.Catalog, string, error) {
	contents, err := readObservedPrivateFile(path, neutralcatalog.MaxDocumentBytes)
	return decodeObservedCatalog(contents, err)
}

func inspectRepositoryCatalogAt(path string) (neutralcatalog.Catalog, string, error) {
	contents, err := readRegularFile(path, neutralcatalog.MaxDocumentBytes)
	return decodeObservedCatalog(contents, err)
}

func decodeObservedCatalog(contents []byte, err error) (neutralcatalog.Catalog, string, error) {
	if err != nil {
		return neutralcatalog.Catalog{}, "", err
	}
	value, diagnostics := neutralcatalog.Decode(contents)
	if len(diagnostics) > 0 {
		return neutralcatalog.Catalog{}, "", errInvalidObservedCatalog
	}
	canonical, diagnostics := neutralcatalog.Encode(value)
	if len(diagnostics) > 0 {
		return neutralcatalog.Catalog{}, "", errInvalidObservedCatalog
	}
	sum := sha256.Sum256(canonical)
	return value, hex.EncodeToString(sum[:]), nil
}

func inspectCatalogState(path string) (catalogStateFile, error) {
	contents, err := readObservedPrivateFile(path, 1<<20)
	if errors.Is(err, os.ErrNotExist) {
		return catalogStateFile{}, nil
	}
	if err != nil {
		return catalogStateFile{}, err
	}
	var state catalogStateFile
	if json.Unmarshal(contents, &state) != nil || state.SchemaVersion != 1 {
		return catalogStateFile{}, fmt.Errorf("catalog state is invalid")
	}
	return state, nil
}

func inspectRecovery(root, directory string) workflowstate.RecoveryObservation {
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return workflowstate.RecoveryObservation{}
	}
	if err != nil {
		return workflowstate.RecoveryObservation{Blocked: true}
	}
	result := workflowstate.RecoveryObservation{}
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".journal" {
			analysis, analyzeErr := transaction.AnalyzeRecovery(root, filepath.Join(directory, entry.Name()))
			if analyzeErr != nil || analysis.State == transaction.RecoveryAmbiguous {
				result.Blocked = true
				continue
			}
			result.Required = true
		}
	}
	return result
}

func inspectActiveGeneration(home, shell string) string {
	contents, err := readObservedPrivateFile(filepath.Join(home, ".config", "alias-lens", "generated", shell, "active"), 65)
	if err != nil {
		return ""
	}
	value := string(contents)
	for len(value) > 0 && (value[len(value)-1] == '\n' || value[len(value)-1] == '\r') {
		value = value[:len(value)-1]
	}
	return value
}

func inspectGeneratedFile(home, shell, generation string) string {
	if len(generation) != 64 || len(hexOnly(generation)) != 64 {
		return ""
	}
	extension := ".sh"
	if shell == "zsh" {
		extension = ".zsh"
	}
	contents, err := readObservedPrivateFile(filepath.Join(home, ".config", "alias-lens", "generated", shell, generation+extension), neutralcatalog.MaxDocumentBytes)
	if err != nil {
		return ""
	}
	marker := []byte("# generation-sha256: " + generation + "\n")
	if !bytesContains(contents, marker) {
		return ""
	}
	return generation
}

func bytesContains(contents, needle []byte) bool {
	if len(needle) == 0 || len(contents) < len(needle) {
		return false
	}
	for index := 0; index <= len(contents)-len(needle); index++ {
		matched := true
		for offset := range needle {
			if contents[index+offset] != needle[offset] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func hexOnly(value string) string {
	result := ""
	for _, character := range value {
		if (character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') {
			result += string(character)
		}
	}
	return result
}
