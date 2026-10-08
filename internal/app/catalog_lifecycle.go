//go:build !windows

package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	neutralcatalog "github.com/alderon07/al/internal/catalog"
	"github.com/alderon07/al/internal/catalogrender"
	"github.com/alderon07/al/internal/catalogstore"
	workflowplan "github.com/alderon07/al/internal/plan"
	shellapi "github.com/alderon07/al/internal/shell"
)

type CatalogLifecycleDecisions struct {
	Approvals    []catalogstore.ApprovalRecord
	Adoptions    []catalogstore.Adoption
	ConfirmedAt  string
	Executable   string
	SourcePath   string
	SourceSHA256 string
	StartupPaths []string
}

func readLifecycleFile(path string, destination any) ([]byte, error) {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	data, err := catalogstore.ReadPrivateBytes(path, catalogstore.MaxDocumentBytes)
	if err != nil {
		return nil, err
	}
	if err := catalogstore.Decode(data, destination); err != nil {
		return nil, err
	}
	return data, nil
}

func (svc *Services) lifecycleAction(path, role string, planned []byte) (workflowplan.Action, error) {
	before, err := readRegularFile(path, catalogstore.MaxDocumentBytes)
	kind := workflowplan.ActionReplace
	if errors.Is(err, os.ErrNotExist) {
		before = nil
		kind = workflowplan.ActionCreate
	} else if err != nil {
		return workflowplan.Action{}, err
	}
	target := plannedTarget(path, before, planned)
	if role == "native_alias_file" || role == "startup" {
		target, err = plannedUserTarget(path, before, planned)
		if err != nil {
			return workflowplan.Action{}, err
		}
	}
	return workflowplan.Action{Kind: kind, TargetRole: role, DisplayPath: svc.displayPrivatePath(path), Reason: lifecycleRoleReason(role), Risk: workflowplan.RiskReview, Backup: before != nil, Reversible: true, Target: target}, nil
}

func lifecycleRoleReason(role string) string {
	reasons := map[string]string{
		"generation":          "Save verified shell declarations",
		"generation_manifest": "Save immutable generation identity and inputs",
		"catalog_snapshot":    "Save the reviewed catalog source snapshot",
		"native_snapshot":     "Save native input proof for installed generation review",
		"rollback_native":     "Preserve original native enrollment baseline for offline rollback",
		"rollback_startup":    "Preserve original startup enrollment baseline for offline rollback",
		"rollback_record":     "Record enrollment baseline paths and original file existence",
		"startup":             "Install verified catalog loader and pinned shell integration",
		"native_alias_file":   "Update reviewed enrolled native fallbacks",
		"active_pointer":      "Activate the verified immutable generation",
		"installed":           "Record exact installed generation membership",
		"approvals":           "Save reviewed native implementation approvals",
		"adoptions":           "Save exact enrolled fallback ownership ranges",
		"catalog_revision":    "Preserve a timestamped catalog revision before editing",
		"catalog":             "Save catalog edits pending explicit enable",
	}
	if reason := reasons[role]; reason != "" {
		return reason
	}
	return "Save reviewed " + role
}

func catalogNativeActionReason(shell string, adoptions catalogstore.AdoptionsFile, entries map[string]neutralcatalog.Entry, included map[string]bool, refresh, relocateIntegration bool) string {
	parts := []string{}
	for _, record := range adoptions.Records {
		if record.Shell != shell {
			continue
		}
		entry, exists := entries[record.EntryID]
		switch {
		case !refresh:
			parts = append(parts, fmt.Sprintf("Retain enrolled fallback %q unchanged", record.Name))
		case !exists:
			parts = append(parts, fmt.Sprintf("Remove enrolled fallback %q for deleted catalog entry", record.Name))
		case !included[record.EntryID]:
			parts = append(parts, fmt.Sprintf("Remove enrolled fallback %q excluded by shell/profile/platform conditions", record.Name))
		case entry.Name != record.Name:
			parts = append(parts, fmt.Sprintf("Rename enrolled fallback %q to %q with reviewed declaration", record.Name, entry.Name))
		default:
			parts = append(parts, fmt.Sprintf("Refresh enrolled fallback %q with reviewed declaration", record.Name))
		}
	}
	if relocateIntegration {
		parts = append(parts, "Relocate exact owned native integration to guarded pinned startup integration")
	}
	if len(parts) == 0 {
		return lifecycleRoleReason("native_alias_file")
	}
	return strings.Join(parts, "; ")
}

func (svc *Services) buildCatalogEnablePlan(shell string, decisions CatalogLifecycleDecisions) (workflowplan.OperationPlan, error) {
	value, err := readCatalogFile(svc.localCatalogPath())
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	return svc.buildCatalogEnablePlanForCatalog(shell, decisions, value, svc.localCatalogPath())
}

func (svc *Services) buildCatalogEnablePlanForCatalog(shell string, decisions CatalogLifecycleDecisions, value neutralcatalog.Catalog, sourcePath string) (workflowplan.OperationPlan, error) {
	if err := svc.validateCatalogNames(value); err != nil {
		return workflowplan.OperationPlan{}, err
	}
	adapter, err := svc.shellAdapter(shell)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	canonical, diagnostics := neutralcatalog.Encode(value)
	if len(diagnostics) > 0 {
		return workflowplan.OperationPlan{}, fmt.Errorf("catalog is invalid")
	}
	if decisions.SourceSHA256 != "" && decisions.SourceSHA256 != hashBytes(canonical) {
		return workflowplan.OperationPlan{}, fmt.Errorf("catalog changed after review; run al catalog review again")
	}
	observed, err := svc.observeConfig()
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	nativePath := filepath.Join(svc.homeDirectory(), adapter.AliasFilename())
	native, resolvedNativePath, err := readCatalogNative(nativePath)
	if errors.Is(err, os.ErrNotExist) {
		native = []byte{}
		resolvedNativePath = nativePath
	} else if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	if err := validateCatalogNativeControls(shell, native); err != nil {
		return workflowplan.OperationPlan{}, err
	}
	inputs := []workflowplan.Input{svc.planInput("catalog", sourcePath, canonical), svc.planInput("native_alias_file", nativePath, native)}
	approvals := catalogstore.ApprovalFile{Version: 2, Records: []catalogstore.ApprovalRecord{}}
	approvalBytes, err := readLifecycleFile(svc.catalogApprovalsPath(), &approvals)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	inputs = append(inputs, svc.planInput("approvals", svc.catalogApprovalsPath(), approvalBytes))
	approved := map[catalogstore.ApprovalKey]bool{}
	for _, record := range approvals.Records {
		approved[record.Key] = true
	}
	for _, record := range decisions.Approvals {
		if !approved[record.Key] {
			approvals.Records = append(approvals.Records, record)
			approved[record.Key] = true
		}
	}
	adoptions := catalogstore.AdoptionsFile{Version: 1, Records: []catalogstore.Adoption{}}
	adoptionBytes, err := readLifecycleFile(svc.catalogAdoptionsPath(), &adoptions)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	inputs = append(inputs, svc.planInput("adoptions", svc.catalogAdoptionsPath(), adoptionBytes))
	for _, record := range decisions.Adoptions {
		replaced := false
		for i := range adoptions.Records {
			old := adoptions.Records[i]
			if old.Shell == record.Shell && old.EntryID == record.EntryID {
				record.OriginalPath = old.OriginalPath
				record.OriginalSHA256 = old.OriginalSHA256
				adoptions.Records[i] = record
				replaced = true
				break
			}
		}
		if !replaced {
			adoptions.Records = append(adoptions.Records, record)
		}
	}
	installed := catalogstore.InstalledFile{Version: 1, Records: []catalogstore.InstalledState{}}
	installedBytes, err := readLifecycleFile(svc.catalogInstalledPath(), &installed)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	inputs = append(inputs, svc.planInput("installed", svc.catalogInstalledPath(), installedBytes))
	var previous *catalogstore.InstalledState
	for i := range installed.Records {
		if installed.Records[i].Shell == shell {
			previous = &installed.Records[i]
		}
	}
	var prior catalogstore.GenerationManifest
	if previous != nil {
		savedNative, e := catalogstore.ReadPrivateBytes(filepath.Join(svc.catalogStateRoot(), "native-snapshots", previous.NativeInputSHA256+".native"), ShadowSourceLimit)
		if e != nil {
			return workflowplan.OperationPlan{}, fmt.Errorf("installed native snapshot is unavailable; use al catalog rollback")
		}
		prior, err = svc.readCatalogImmutableGeneration(svc.catalogGeneratedRoot(shell), resolvedNativePath, shell, previous.GenerationID, savedNative)
		if err != nil {
			return workflowplan.OperationPlan{}, fmt.Errorf("installed generation is invalid; use al catalog rollback")
		}
	}
	rendered, problems := catalogrender.RenderV2(value, catalogrender.RenderV2Context{Shell: shell, Platform: svc.currentPlatform(), Profiles: observed.Config.Profiles, Approvals: approved, ValidateNative: svc.validateCatalogNativeDeclaration, ResolveExecutable: func(program string) (string, error) {
		return resolveCatalogExecutable(program, svc.dependencies.Environment("PATH"))
	}})
	if len(problems) > 0 {
		return workflowplan.OperationPlan{}, fmt.Errorf("catalog has unsafe declarations; review al catalog shadow")
	}
	included := map[string]bool{}
	for _, id := range rendered.IncludedEntryIDs {
		included[id] = true
	}
	entries := map[string]neutralcatalog.Entry{}
	for _, entry := range value.Entries {
		entries[entry.ID] = entry
	}
	for _, old := range prior.Entries {
		if current, exists := entries[old.Entry.ID]; exists && !included[current.ID] && !svc.catalogIntentionallyDisabled(current, shell, svc.currentPlatform(), observed.Config.Profiles) {
			return workflowplan.OperationPlan{}, fmt.Errorf("installed entry %s has no reviewed available replacement; review al catalog approve %s --shell %s", old.Entry.Name, current.Name, shell)
		}
	}
	owners := map[string]catalogstore.Adoption{}
	for _, record := range adoptions.Records {
		if record.Shell == shell {
			owners[record.Name] = record
		}
	}
	reviewSource, err := shellapi.CatalogNativeReviewSource(adapter, native)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	nativeEntries := importShadowSource(shell, reviewSource)
	collisions := map[string]int{}
	for _, record := range nativeEntries {
		if record.Entry != nil {
			collisions[record.Name]++
		}
	}
	for _, entry := range rendered.Entries {
		if collisions[entry.Entry.Name] > 0 {
			owner, ok := owners[entry.Entry.Name]
			if !ok || owner.EntryID != entry.Entry.ID || collisions[entry.Entry.Name] != 1 {
				return workflowplan.OperationPlan{}, fmt.Errorf("%s needs exact ownership review; enter al catalog adopt %s --shell %s", entry.Entry.Name, entry.Entry.Name, shell)
			}
		}
	}
	replacements := map[string]string{}
	for _, entry := range rendered.Entries {
		replacements[entry.Entry.ID] = entry.Declaration
	}
	refreshed, updatedAdoptions, err := refreshCatalogFallbacks(shell, nativePath, native, adoptions, entries, included, previous != nil, replacements)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	withoutIntegration, integrationStart, err := catalogRemoveOwnedIntegration(adapter, refreshed)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	if integrationStart >= 0 {
		removed := len(refreshed) - len(withoutIntegration)
		for index := range updatedAdoptions.Records {
			record := &updatedAdoptions.Records[index]
			if record.Shell != shell {
				continue
			}
			if record.Start >= integrationStart+removed {
				record.Start -= removed
				record.End -= removed
			} else if record.End > integrationStart {
				return workflowplan.OperationPlan{}, fmt.Errorf("owned fallback overlaps native integration; repeat ownership review")
			}
			record.FileSHA256 = hashBytes(withoutIntegration)
		}
		refreshed = withoutIntegration
	}
	declarations := make([][]byte, 0, len(rendered.Entries))
	entryList := make([]neutralcatalog.Entry, 0, len(rendered.Entries))
	for _, entry := range rendered.Entries {
		entryList = append(entryList, entry.Entry)
		declarations = append(declarations, []byte(entry.Declaration))
	}
	if err := svc.validateCatalogDeclarations(shell, entryList, declarations, refreshed); err != nil {
		return workflowplan.OperationPlan{}, err
	}
	manifest := catalogstore.GenerationManifest{Version: 1, Shell: shell, Renderer: shell + "/v2", Platform: svc.currentPlatform(), SourceSHA256: hashBytes(canonical), NativeInputSHA256: hashBytes(refreshed), NativePath: resolvedNativePath, NativePolicy: "regular-user", Profiles: observed.Config.Profiles, Entries: rendered.Entries, IncludedEntryIDs: rendered.IncludedEntryIDs, Confirmations: rendered.Confirmations, ExecutableResolutions: rendered.ExecutableResolutions}
	leaf := observePlanIdentity(nativePath)
	if leaf.FileType == "symlink" {
		manifest.NativeSourcePath = nativePath
		manifest.NativeSourceLinkTarget = leaf.LinkTarget
		manifest.NativeSourceIdentity = fmt.Sprintf("%d:%d", leaf.Device, leaf.Inode)
	}
	manifest, manifestBytes, err := catalogstore.BuildGeneration(manifest, rendered.Body)
	if err != nil {
		return workflowplan.OperationPlan{}, fmt.Errorf("build generation: %w", err)
	}
	executable := decisions.Executable
	if executable == "" {
		executable, err = svc.dependencies.Executable()
		if err != nil {
			return workflowplan.OperationPlan{}, err
		}
		executable, err = filepath.EvalSymlinks(executable)
		if err != nil {
			return workflowplan.OperationPlan{}, err
		}
	}
	startup, err := svc.planCatalogStartup(adapter, manifest, previous, executable, refreshed, decisions.StartupPaths)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	for _, edit := range startup {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := svc.validateSyntax(ctx, shell, edit.After)
		cancel()
		if err != nil {
			return workflowplan.OperationPlan{}, fmt.Errorf("startup integration failed isolated parse-only validation")
		}
	}
	actions := []workflowplan.Action{}
	appendAction := func(path, role string, data []byte) error {
		if role == "generation" || role == "generation_manifest" || role == "catalog_snapshot" || role == "native_snapshot" {
			if existing, e := readRegularFile(path, catalogstore.MaxDocumentBytes); e == nil && !bytes.Equal(existing, data) {
				return fmt.Errorf("immutable catalog artifact differs; use al catalog rollback")
			} else if e != nil && !os.IsNotExist(e) {
				return e
			}
		}
		action, e := svc.lifecycleAction(path, role, data)
		if e != nil {
			return e
		}
		if role == "native_alias_file" {
			action.Reason = catalogNativeActionReason(shell, adoptions, entries, included, previous != nil, integrationStart >= 0)
		}

		if _, statErr := os.Lstat(path); statErr == nil && bytes.Equal(data, func() []byte { b, _ := readRegularFile(path, catalogstore.MaxDocumentBytes); return b }()) {
			return nil
		}
		if role == "native_snapshot" {
			action.Target.Scope = "private"
			action.Target.RecoveryOrder = 10
		}
		actions = append(actions, action)
		return nil
	}
	root := svc.catalogGeneratedRoot(shell)
	for _, file := range []struct {
		path, role string
		data       []byte
	}{{filepath.Join(svc.catalogStateRoot(), "native-snapshots", manifest.NativeInputSHA256+".native"), "native_snapshot", refreshed}, {filepath.Join(svc.catalogStateRoot(), "catalog-snapshots", manifest.SourceSHA256+".json"), "catalog_snapshot", canonical}, {filepath.Join(root, manifest.ID+".json"), "generation_manifest", manifestBytes}, {filepath.Join(root, manifest.ID+".sh"), "generation", rendered.Body}} {
		if err := appendAction(file.path, file.role, file.data); err != nil {
			return workflowplan.OperationPlan{}, err
		}
	}
	rollbackID := ""
	if previous != nil {
		rollbackID = previous.RollbackID
	}
	baseline := filepath.Join(svc.catalogStateRoot(), "rollback", hashBytes([]byte(shell+nativePath+hashBytes(native)))+".native")
	if rollbackID == "" {
		rollbackID = hashBytes([]byte(shell + nativePath + hashBytes(native)))
		if err := appendAction(baseline, "rollback_native", native); err != nil {
			return workflowplan.OperationPlan{}, err
		}
	}
	startupRecords := []catalogstore.StartupRecord{}
	for _, edit := range startup {
		originalPath := filepath.Join(svc.catalogStateRoot(), "rollback", rollbackID+"-"+hashBytes([]byte(edit.Path))+".startup")
		if previous == nil {
			if err := appendAction(originalPath, "rollback_startup", edit.Before); err != nil {
				return workflowplan.OperationPlan{}, err
			}
		}
		startupRecords = append(startupRecords, catalogstore.StartupRecord{Path: edit.Path, SHA256: hashBytes(edit.After), OriginalPath: originalPath, OriginalSHA256: hashBytes(edit.Before), OriginalExists: func() bool { _, e := os.Lstat(edit.Path); return e == nil }(), Route: edit.Route})
		if previous != nil {
			for _, old := range previous.StartupRecords {
				if old.Path == edit.Path {
					startupRecords[len(startupRecords)-1].OriginalExists = old.OriginalExists
					startupRecords[len(startupRecords)-1].OriginalSHA256 = old.OriginalSHA256
					startupRecords[len(startupRecords)-1].OriginalPath = old.OriginalPath
				}
			}
		}
		inputs = append(inputs, svc.planInput("startup", edit.Path, edit.Before))
		if err := appendAction(edit.Path, "startup", edit.After); err != nil {
			return workflowplan.OperationPlan{}, err
		}
	}
	if previous == nil {
		record := catalogstore.RollbackRecord{Version: 1, ID: rollbackID, Shell: shell, CreatedAt: decisions.ConfirmedAt, NativePath: nativePath, NativeExists: func() bool { _, e := os.Lstat(nativePath); return e == nil }(), OriginalPath: baseline, OriginalSHA256: hashBytes(native), StartupRecords: startupRecords}
		if record.CreatedAt == "" {
			record.CreatedAt = "2026-10-03T00:00:00Z"
		}
		data, e := catalogstore.Encode(record)
		if e != nil {
			return workflowplan.OperationPlan{}, fmt.Errorf("encode rollback: %w", e)
		}
		if err := appendAction(filepath.Join(svc.catalogStateRoot(), "rollback", rollbackID+".json"), "rollback_record", data); err != nil {
			return workflowplan.OperationPlan{}, err
		}
	}
	if err := appendAction(nativePath, "native_alias_file", refreshed); err != nil {
		return workflowplan.OperationPlan{}, err
	}
	pointer, _ := catalogstore.EncodePointer(manifest.ID)
	if err := appendAction(filepath.Join(root, "active"), "active_pointer", pointer); err != nil {
		return workflowplan.OperationPlan{}, err
	}
	state := catalogstore.InstalledState{Shell: shell, GenerationID: manifest.ID, Renderer: shell + "/v2", NativeInputSHA256: hashBytes(refreshed), StartupRecords: startupRecords, RollbackID: rollbackID}
	if previous != nil {
		*previous = state
	} else {
		installed.Records = append(installed.Records, state)
	}
	for _, file := range []struct {
		path, role string
		value      any
	}{{svc.catalogApprovalsPath(), "approvals", approvals}, {svc.catalogAdoptionsPath(), "adoptions", updatedAdoptions}, {svc.catalogInstalledPath(), "installed", installed}} {
		data, e := catalogstore.Encode(file.value)
		if e != nil {
			return workflowplan.OperationPlan{}, fmt.Errorf("encode %s: %w", file.role, e)
		}
		if err := appendAction(file.path, file.role, data); err != nil {
			return workflowplan.OperationPlan{}, err
		}
	}
	for i := range actions {
		actions[i].Sequence = i + 1
	}
	return workflowplan.Build("catalog.enable", inputs, actions, nil, nil), nil
}

func (svc *Services) catalogIntentionallyDisabled(entry neutralcatalog.Entry, shell, platform string, profiles []string) bool {
	copy := entry
	copy.Kind = "command"
	copy.Portable = &neutralcatalog.Portable{Program: "printf", Args: []string{}, PassArguments: true}
	copy.Native = nil
	resolved, problems := neutralcatalog.Resolve(neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{copy}}, neutralcatalog.ResolveContext{Shell: shell, Platform: platform, Profiles: profiles})
	return len(problems) == 0 && len(resolved) == 1 && !resolved[0].Available
}

func refreshCatalogFallbacks(shell, path string, native []byte, adoptions catalogstore.AdoptionsFile, entries map[string]neutralcatalog.Entry, included map[string]bool, refresh bool, replacements map[string]string) ([]byte, catalogstore.AdoptionsFile, error) {
	records := []catalogstore.Adoption{}
	for _, record := range adoptions.Records {
		if record.Shell == shell {
			if record.Path != path || record.FileSHA256 != hashBytes(native) || record.Start < 0 || record.End > len(native) || record.End <= record.Start || string(native[record.Start:record.End]) != record.Definition {
				return nil, adoptions, fmt.Errorf("native ownership changed; repeat al catalog adopt %s --shell %s", record.Name, shell)
			}
			records = append(records, record)
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Start > records[j].Start })
	updated := append([]byte{}, native...)
	if refresh {
		for _, record := range records {
			replacement := ""
			if entry, ok := entries[record.EntryID]; ok && included[record.EntryID] {
				replacement = replacements[entry.ID]
				if replacement == "" {
					return nil, adoptions, fmt.Errorf("native fallback needs a reviewed replacement")
				}

			}
			updated = append(append(append([]byte{}, updated[:record.Start]...), []byte(replacement)...), updated[record.End:]...)
		}
	}
	result := catalogstore.AdoptionsFile{Version: 1, Records: []catalogstore.Adoption{}}
	adapter, err := shellapi.New(shell)
	if err != nil {
		return nil, adoptions, err
	}
	reviewSource, err := shellapi.CatalogNativeReviewSource(adapter, updated)
	if err != nil {
		return nil, adoptions, err
	}
	parsed := importShadowSource(shell, reviewSource)
	for _, record := range adoptions.Records {
		if record.Shell != shell {
			result.Records = append(result.Records, record)
			continue
		}
		entry, exists := entries[record.EntryID]
		if !exists || !included[record.EntryID] {
			if refresh {
				continue
			}
		}
		name := record.Name
		if refresh {
			name = entry.Name
		}
		found := false
		for _, span := range parsed {
			if span.Entry != nil && span.Name == name {
				record.Name = name
				record.Kind = span.Kind
				record.Start = span.StartByte
				record.End = span.EndByte
				record.Definition = string(updated[record.Start:record.End])
				record.DefinitionSHA256 = hashBytes([]byte(record.Definition))
				record.FileSHA256 = hashBytes(updated)
				found = true
				break
			}
		}
		if !found {
			return nil, adoptions, fmt.Errorf("owned fallback no longer has one complete declaration")
		}
		result.Records = append(result.Records, record)
	}
	return updated, result, nil
}

func (svc *Services) planCatalogStartup(adapter ShellAdapter, manifest catalogstore.GenerationManifest, previous *catalogstore.InstalledState, executable string, native []byte, explicit []string) ([]catalogStartupEdit, error) {
	paths, err := adapter.CatalogStartupPaths(svc.homeDirectory(), svc.currentPlatform(), explicit)
	if err != nil {
		return nil, err
	}
	result := []catalogStartupEdit{}
	names := map[string]bool{}
	for _, entry := range manifest.Entries {
		names[entry.Entry.Name] = true
	}
	for _, path := range paths {
		before, e := readRegularFile(path, ShadowSourceLimit)
		if errors.Is(e, os.ErrNotExist) {
			before = nil
		} else if e != nil {
			return nil, e
		}
		contents := before
		previousInsertion := -1
		if previous != nil {
			matched := false
			for _, record := range previous.StartupRecords {
				if record.Path == path {
					if record.SHA256 != hashBytes(before) {
						return nil, fmt.Errorf("startup changed; review al setup --repair")
					}
					matched = true
				}
			}
			if !matched {
				return nil, fmt.Errorf("startup route changed; use al catalog rollback")
			}
			text := string(contents)
			start := bytes.Index(contents, []byte(catalogLoaderStart))
			end := bytes.Index(contents, []byte(catalogLoaderEnd))
			if start < 0 || end < start {
				return nil, fmt.Errorf("catalog startup block is incomplete")
			}
			end += len(catalogLoaderEnd)
			if start > 0 && text[start-1] == '\n' {
				start--
			}
			if end < len(text) && text[end] == '\n' {
				end++
			}
			previousInsertion = start
			text = text[:start] + text[end:]
			contents = []byte(text)
		}
		route, viaPrimary := adapter.CatalogStartupRoute(path, contents, svc.homeDirectory())
		if viaPrimary {
			result = append(result, catalogStartupEdit{Path: path, Before: before, After: contents, Route: route})
			continue
		}
		preserved, sourceNative, insertion, e := catalogStartupPlacementAt(contents, adapter.Name(), svc.homeDirectory(), manifest.NativePath, names)
		if e != nil {
			return nil, e
		}
		integration := ""
		if !bytes.Contains(native, []byte(adapter.Integration())) {
			for _, entry := range importShadowSource(adapter.Name(), native) {
				if entry.Entry != nil && catalogProtectedName(entry.Name) && !(entry.Name == "al" && entry.Kind == "command") {
					return nil, fmt.Errorf("native control name prevents integration; run al setup --repair")
				}
			}
			integration, err = catalogPinnedIntegration(adapter, executable)
			if err != nil {
				return nil, err
			}
		}
		block := catalogLoaderBlock(adapter.Name(), manifest.NativePath, executable, svc.catalogGeneratedRoot(adapter.Name()), sourceNative)
		if previousInsertion >= 0 {
			insertion = previousInsertion
		}
		installedBlock := []byte(strings.Replace(block, catalogLoaderEnd, integration+catalogLoaderEnd, 1))
		updated := append(append(append([]byte{}, preserved[:insertion]...), installedBlock...), preserved[insertion:]...)
		result = append(result, catalogStartupEdit{Path: path, Before: before, After: updated, Route: route})
	}
	return result, nil
}

func (svc *Services) lifecycleTimestamp() string {
	return svc.dependencies.Now().UTC().Format(time.RFC3339Nano)
}
