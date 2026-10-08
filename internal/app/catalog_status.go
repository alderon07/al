//go:build !windows

package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	neutralcatalog "alias-lens/internal/catalog"
	"alias-lens/internal/catalogrender"
	"alias-lens/internal/catalogstore"
	workflowstate "alias-lens/internal/state"
)

func (svc *Services) readLifecycleInstalledSnapshot(shell string) (neutralcatalog.Catalog, bool, error) {
	records := catalogstore.InstalledFile{Version: 1, Records: []catalogstore.InstalledState{}}
	_, err := readLifecycleFile(svc.catalogInstalledPath(), &records)
	if err != nil {
		return neutralcatalog.Catalog{}, true, err
	}
	for _, record := range records.Records {
		if record.Shell == shell {
			data, err := catalogstore.ReadPrivateBytes(filepath.Join(svc.catalogGeneratedRoot(shell), record.GenerationID+".json"), catalogstore.MaxDocumentBytes)
			if err != nil {
				return neutralcatalog.Catalog{}, true, err
			}
			var manifest catalogstore.GenerationManifest
			if err := catalogstore.Decode(data, &manifest); err != nil {
				return neutralcatalog.Catalog{}, true, err
			}
			native, err := catalogstore.ReadPrivateBytes(filepath.Join(svc.catalogStateRoot(), "native-snapshots", manifest.NativeInputSHA256+".native"), ShadowSourceLimit)
			if err != nil {
				return neutralcatalog.Catalog{}, true, err
			}
			verified, err := svc.readCatalogImmutableGeneration(svc.catalogGeneratedRoot(shell), manifest.NativePath, shell, record.GenerationID, native)
			if err != nil || verified.ID != record.GenerationID {
				return neutralcatalog.Catalog{}, true, fmt.Errorf("installed immutable package cannot be verified")
			}
			snapshot, err := catalogstore.ReadPrivateBytes(filepath.Join(svc.catalogStateRoot(), "catalog-snapshots", manifest.SourceSHA256+".json"), neutralcatalog.MaxDocumentBytes)
			if err != nil {
				return neutralcatalog.Catalog{}, true, err
			}
			value, problems := neutralcatalog.Decode(snapshot)
			if len(problems) > 0 {
				return neutralcatalog.Catalog{}, true, fmt.Errorf("installed catalog snapshot is invalid")
			}
			canonical, _ := neutralcatalog.Encode(value)
			if hashBytes(canonical) != manifest.SourceSHA256 {
				return neutralcatalog.Catalog{}, true, fmt.Errorf("installed snapshot fingerprint changed")
			}
			included := neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{}}
			for _, entry := range verified.Entries {
				included.Entries = append(included.Entries, entry.Entry)
			}
			return neutralcatalog.Normalize(included), true, nil
		}
	}
	return neutralcatalog.Catalog{}, false, nil
}

func (svc *Services) observeLifecycleStatus(inputs *workflowstate.Inputs) {
	records := catalogstore.InstalledFile{Version: 1, Records: []catalogstore.InstalledState{}}
	_, err := readLifecycleFile(svc.catalogInstalledPath(), &records)
	if err != nil {
		inputs.Recovery.Blocked = true
		return
	}
	if len(records.Records) == 0 {
		return
	}
	config, err := svc.observeConfig()
	if err != nil {
		inputs.Recovery.Blocked = true
		return
	}
	inputs.Mode = workflowstate.ModeMixed
	for _, record := range records.Records {
		if record.Shell == config.Config.Shell {
			inputs.Mode = workflowstate.ModeCatalog
		}
	}
	value, catalogHash, err := svc.inspectLocalCatalog()
	approvals := catalogstore.ApprovalFile{Version: 2, Records: []catalogstore.ApprovalRecord{}}
	_, approvalErr := readLifecycleFile(svc.catalogApprovalsPath(), &approvals)
	approved := map[catalogstore.ApprovalKey]bool{}
	for _, record := range approvals.Records {
		approved[record.Key] = true
	}
	for _, record := range records.Records {
		observation := workflowstate.ShellObservation{Name: record.Shell, Unreadable: err != nil || approvalErr != nil}
		rendered, problems := catalogrender.RenderV2(value, catalogrender.RenderV2Context{Shell: record.Shell, Platform: svc.currentPlatform(), Profiles: config.Config.Profiles, Approvals: approved, ValidateNative: svc.validateCatalogNativeDeclaration, ResolveExecutable: func(program string) (string, error) {
			return resolveCatalogExecutable(program, svc.dependencies.Environment("PATH"))
		}})
		observation.Resolved = workflowstate.ResolvedSummary{SHA256: catalogResolvedFingerprint(catalogHash, record.Shell, svc.currentPlatform(), config.Config.Profiles, rendered.IncludedEntryIDs, rendered.Confirmations, rendered.ExecutableResolutions), EligibleEntries: len(rendered.IncludedEntryIDs), PendingApprovals: len(rendered.PendingApprovals)}
		observation.Unreadable = observation.Unreadable || len(problems) > 0
		adapter, _ := svc.shellAdapter(record.Shell)
		manifest, verifyErr := svc.readCatalogGeneration(svc.catalogGeneratedRoot(record.Shell), filepath.Join(svc.homeDirectory(), adapter.AliasFilename()), record.Shell)
		actualID := ""
		sourceHash := ""
		if verifyErr == nil {
			actualID = manifest.ID
			sourceHash = catalogResolvedFingerprint(manifest.SourceSHA256, manifest.Shell, manifest.Platform, manifest.Profiles, manifest.IncludedEntryIDs, manifest.Confirmations, manifest.ExecutableResolutions)
			if manifest.ID != record.GenerationID || manifest.NativeInputSHA256 != record.NativeInputSHA256 || manifest.Renderer != record.Renderer {
				observation.Blocked = true
			}
		} else {
			observation.Blocked = true
		}
		expectedStartup := ""
		actualStartup := ""
		for _, startup := range record.StartupRecords {
			expectedStartup += startup.SHA256
			current, e := readRegularFile(startup.Path, ShadowSourceLimit)
			if e != nil {
				observation.Unreadable = true
			} else {
				actualStartup += hashBytes(current)
			}
		}
		observation.Installed = &workflowstate.InstalledObservation{GenerationSHA256: record.GenerationID, ResolvedStateSHA256: sourceHash, RecordedLoaderSHA256: hashBytes([]byte(expectedStartup)), ActiveGenerationSHA256: actualID, OnDiskGenerationSHA256: actualID, OnDiskLoaderSHA256: hashBytes([]byte(actualStartup))}
		kept := inputs.Shells[:0]
		for _, old := range inputs.Shells {
			if old.Name != record.Shell {
				kept = append(kept, old)
			}
		}
		inputs.Shells = append(kept, observation)
	}
}

func catalogResolvedFingerprint(source, shell, platform string, profiles, ids []string, approvals []catalogstore.ApprovalKey, resolutions []catalogstore.ExecutableResolution) string {
	profiles = append([]string(nil), profiles...)
	sort.Strings(profiles)
	ids = append([]string(nil), ids...)
	sort.Strings(ids)
	approvals = append([]catalogstore.ApprovalKey(nil), approvals...)
	sort.Slice(approvals, func(i, j int) bool {
		a, _ := json.Marshal(approvals[i])
		b, _ := json.Marshal(approvals[j])
		return string(a) < string(b)
	})
	resolutions = append([]catalogstore.ExecutableResolution(nil), resolutions...)
	sort.Slice(resolutions, func(i, j int) bool { return resolutions[i].EntryID < resolutions[j].EntryID })
	data, _ := json.Marshal(struct {
		Source, Shell, Platform, Renderer string
		Profiles, IDs                     []string
		Approvals                         []catalogstore.ApprovalKey
		Resolutions                       []catalogstore.ExecutableResolution
	}{source, shell, platform, shell + "/v2", profiles, ids, approvals, resolutions})
	return hashBytes(data)
}

func (svc *Services) readEnrolledRepositoryCatalog() (neutralcatalog.Catalog, bool, error) {
	record, _, err := svc.readCatalogSyncRecord()
	if errors.Is(err, os.ErrNotExist) {
		return neutralcatalog.Catalog{}, false, nil
	}
	if err != nil {
		return neutralcatalog.Catalog{}, true, err
	}
	path, err := repositoryFilePath(record.Repository, record.CatalogPath)
	if err != nil {
		return neutralcatalog.Catalog{}, true, err
	}
	value, err := readCatalogFile(path)
	return value, true, err
}
