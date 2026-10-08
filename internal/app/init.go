//go:build !windows

package app

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/alderon07/al/internal/catalog"
	"github.com/alderon07/al/internal/catalogstore"
	"github.com/alderon07/al/internal/managedgit"
	workflowplan "github.com/alderon07/al/internal/plan"
	"os"
	"path/filepath"
	"strings"
)

func (svc *Services) observeLocalCatalogInit(options CatalogInitOptions) (catalogstore.CatalogSyncRecord, catalog.Catalog, []byte, error) {
	relative, err := cleanRepositoryRelativePath(options.CatalogPath, "catalog path")
	if err != nil || svc.sensitiveConfigPath(relative) || strings.Contains(relative, "\\") {
		return catalogstore.CatalogSyncRecord{}, catalog.Catalog{}, nil, fmt.Errorf("choose a safe relative catalog path")
	}
	repository, err := svc.expandUserPath(options.Source)
	if err != nil {
		return catalogstore.CatalogSyncRecord{}, catalog.Catalog{}, nil, err
	}
	repository, err = filepath.EvalSymlinks(repository)
	if err != nil {
		return catalogstore.CatalogSyncRecord{}, catalog.Catalog{}, nil, fmt.Errorf("local repository is unavailable; use al init /path/to/repository")
	}
	runner, cleanup, err := managedgit.New()
	if err != nil {
		return catalogstore.CatalogSyncRecord{}, catalog.Catalog{}, nil, err
	}
	defer cleanup()
	ctx, cancel := managedGitContext()
	defer cancel()
	root, err := runner.Run(ctx, repository, nil, "rev-parse", "--show-toplevel")
	if err != nil || managedGitText(root) != repository {
		return catalogstore.CatalogSyncRecord{}, catalog.Catalog{}, nil, fmt.Errorf("SOURCE must name the repository root")
	}
	record := catalogstore.CatalogSyncRecord{Version: 1, Repository: repository, CatalogPath: filepath.ToSlash(relative)}
	data, head, blob, err := catalogRepositorySnapshot(ctx, runner, record)
	if err != nil {
		return record, catalog.Catalog{}, nil, err
	}
	value, err := decodeSyncCatalog(data)
	if err != nil {
		return record, value, nil, err
	}
	if err := svc.scanCatalogForSync(value, data); err != nil {
		return record, value, nil, err
	}
	committed, err := runner.Run(ctx, repository, nil, "cat-file", "blob", blob)
	if err != nil {
		return record, value, nil, err
	}
	if _, err := decodeSyncCatalog(committed); err != nil {
		return record, value, nil, fmt.Errorf("committed enrollment base is invalid; commit a valid catalog before init")
	}
	record.BaseBytes = string(committed)
	record.BaseSHA256 = hashBytes(committed)
	record.SourceSHA256 = hashBytes(data)
	record.HEAD = head
	record.Blob = blob
	if remote, err := runner.Run(ctx, repository, nil, "config", "--local", "--no-includes", "--get", "remote.origin.url"); err == nil {
		config, err := svc.loadConfig()
		if err != nil {
			return record, value, nil, err
		}
		transport := managedGitText(remote)
		if locator, err := svc.parseCatalogRemoteLocator(transport, config); err == nil {
			branch, err := runner.Run(ctx, repository, nil, "symbolic-ref", "--quiet", "HEAD")
			if err == nil && catalogstore.ValidRef(managedGitText(branch)) {
				if err := runner.AuditNetworkConfig(ctx, repository, transport); err != nil {
					return record, value, nil, err
				}
				record.Remote = "origin"
				record.RemoteURL = transport
				record.RemoteRef = managedGitText(branch)
				record.Provider = locator.Provider
			}
		}
	}
	return record, value, data, nil
}
func (svc *Services) buildLocalCatalogInitPlan(options CatalogInitOptions, decisions CatalogLifecycleDecisions) (workflowplan.OperationPlan, error) {
	record, value, source, err := svc.observeLocalCatalogInit(options)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	if options.ReviewedSourceSHA != "" && (options.ReviewedSourceSHA != hashBytes(source) || options.ReviewedHEAD != record.HEAD || options.ReviewedBlob != record.Blob) {
		return workflowplan.OperationPlan{}, fmt.Errorf("enrollment source changed during review; restart al init")
	}
	adapter, err := svc.requestedShellAdapter(options.Shell)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	sourcePath := filepath.Join(record.Repository, record.CatalogPath)
	existing, oldState, stateErr := svc.readCatalogSyncRecord()
	if stateErr != nil && !errors.Is(stateErr, os.ErrNotExist) {
		return workflowplan.OperationPlan{}, stateErr
	}
	if stateErr == nil && (existing.Repository != record.Repository || existing.CatalogPath != record.CatalogPath || existing.PushIntent != nil) {
		return workflowplan.OperationPlan{}, fmt.Errorf("a different catalog enrollment exists; resolve it before al init")
	}
	canonical, _ := catalog.Encode(value)
	current, readErr := readRegularFile(svc.localCatalogPath(), catalogstore.MaxDocumentBytes)
	if readErr != nil && !os.IsNotExist(readErr) {
		return workflowplan.OperationPlan{}, readErr
	}
	if readErr == nil && !bytes.Equal(current, canonical) {
		local, err := decodeSyncCatalog(current)
		if err != nil {
			return workflowplan.OperationPlan{}, err
		}
		normalized, _ := catalog.Encode(local)
		if !bytes.Equal(normalized, canonical) {
			return workflowplan.OperationPlan{}, fmt.Errorf("local catalog differs from the enrollment source; review al catalog diff before init")
		}
	}
	lifecycle, err := svc.buildCatalogEnablePlanForCatalog(adapter.Name(), decisions, value, sourcePath)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	inputs := append([]workflowplan.Input{}, lifecycle.Inputs...)
	inputs = append(inputs, svc.planInput("init_source", sourcePath, source), svc.planInput("catalog_sync", svc.catalogSyncPath(), oldState), workflowplan.Input{Role: "init_revision", DisplayPath: "repository HEAD and committed catalog blob", SHA256: hashBytes([]byte(record.HEAD + "\n" + record.Blob))})
	actions := append([]workflowplan.Action{}, lifecycle.Actions...)
	if !bytes.Equal(current, canonical) {
		action, err := svc.catalogSyncAction(svc.localCatalogPath(), "catalog", canonical)
		if err != nil {
			return workflowplan.OperationPlan{}, err
		}
		actions = append(actions, action)
	}
	encoded, err := catalogstore.Encode(record)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	if !bytes.Equal(oldState, encoded) {
		action, err := svc.catalogSyncAction(svc.catalogSyncPath(), "catalog_sync", encoded)
		if err != nil {
			return workflowplan.OperationPlan{}, err
		}
		actions = append(actions, action)
	}
	for i := range actions {
		actions[i].Sequence = i + 1
	}
	return workflowplan.Build("init.local", inputs, actions, lifecycle.Approvals, lifecycle.Diagnostics), nil
}

func remoteCatalogInitSource(preview RemoteCatalogPreview) string { return preview.CloneURL }
func (svc *Services) buildRemoteCatalogInitPlan(options CatalogInitOptions, preview RemoteCatalogPreview, decisions CatalogLifecycleDecisions, stage string) (workflowplan.OperationPlan, error) {
	value, err := decodeSyncCatalog(preview.Bytes)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	adapter, err := svc.requestedShellAdapter(options.Shell)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	destination := svc.managedCatalogDestination(preview)
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		return workflowplan.OperationPlan{}, fmt.Errorf("managed repository destination already exists; enroll its local path")
	}
	if _, _, err := svc.readCatalogSyncRecord(); !errors.Is(err, os.ErrNotExist) {
		if err != nil {
			return workflowplan.OperationPlan{}, err
		}
		return workflowplan.OperationPlan{}, fmt.Errorf("catalog enrollment already exists; resolve it before remote init")
	}
	canonical, _ := catalog.Encode(value)
	current, err := readRegularFile(svc.localCatalogPath(), catalogstore.MaxDocumentBytes)
	if err != nil && !os.IsNotExist(err) {
		return workflowplan.OperationPlan{}, err
	}
	if err == nil && !bytes.Equal(current, canonical) {
		return workflowplan.OperationPlan{}, fmt.Errorf("local catalog differs; preserve it and enroll a local repository")
	}
	lifecycle, err := svc.buildCatalogEnablePlanForCatalog(adapter.Name(), decisions, value, svc.localCatalogPath())
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	actions := append([]workflowplan.Action{}, lifecycle.Actions...)
	inputs := append([]workflowplan.Input{}, lifecycle.Inputs...)
	inputs = append(inputs, workflowplan.Input{Role: "remote_catalog", DisplayPath: "API-pinned catalog revision and blob", SHA256: catalogstore.FramedHash([]byte(preview.Provider), []byte(preview.Host), []byte(preview.Repository), []byte(preview.Revision), []byte(preview.Blob), []byte(preview.CatalogPath), []byte(preview.Transport), []byte(preview.KnownHostsSHA), []byte(preview.KnownHostsIdentity), preview.Bytes)})
	if !bytes.Equal(current, canonical) {
		a, err := svc.catalogSyncAction(svc.localCatalogPath(), "catalog", canonical)
		if err != nil {
			return workflowplan.OperationPlan{}, err
		}
		actions = append(actions, a)
	}
	record := catalogstore.CatalogSyncRecord{Version: 1, Repository: destination, CatalogPath: preview.CatalogPath, BaseBytes: string(preview.Bytes), BaseSHA256: hashBytes(preview.Bytes), SourceSHA256: hashBytes(canonical), HEAD: preview.Revision, Blob: preview.Blob, Remote: "origin", RemoteURL: preview.CloneURL, Transport: preview.Transport, KnownHostsSHA: preview.KnownHostsSHA, KnownHostsIdentity: preview.KnownHostsIdentity, RemoteRef: "refs/heads/" + preview.Branch, Provider: preview.Provider, Managed: true}
	encoded, err := catalogstore.Encode(record)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	a, err := svc.catalogSyncAction(svc.catalogSyncPath(), "catalog_sync", encoded)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	actions = append(actions, a)
	dataRoot := filepath.Dir(destination)
	parents := []string{filepath.Join(svc.homeDirectory(), ".local", "share"), filepath.Join(svc.homeDirectory(), ".local", "share", "alias-lens"), dataRoot}
	for _, parent := range parents {
		identity := observePlanIdentity(parent)
		if identity.FileType == "missing" {
			actions = append(actions, workflowplan.Action{Kind: workflowplan.ActionCreate, TargetRole: "catalog_repository_parent", DisplayPath: svc.displayPrivatePath(parent), Reason: "Create private managed repository parent", Risk: workflowplan.RiskLow, Reversible: true, Target: workflowplan.Target{Path: parent, Scope: workflowplan.TargetScopePrivate, Mode: 0700, Directory: true, RecoveryOrder: 1000, ExpectedIdentity: identity, ExpectedSHA256: hashBytes(nil)}})
		} else if identity.FileType != "directory" {
			return workflowplan.OperationPlan{}, fmt.Errorf("managed repository parent must be a real directory")
		}
	}
	promotion := workflowplan.Action{Kind: workflowplan.ActionClone, TargetRole: "catalog_repository", DisplayPath: svc.displayPrivatePath(destination), Reason: "Install only the API-pinned catalog from a filtered managed repository", Risk: workflowplan.RiskReview, Reversible: true, Target: workflowplan.Target{Path: destination, Scope: workflowplan.TargetScopeRepository, Mode: 0700, RecoveryOrder: 4, ExpectedIdentity: observePlanIdentity(destination), ExpectedSHA256: hashBytes(nil)}}
	if stage != "" {
		promotion, err = svc.managedPromotionAction(stage, destination)
		if err != nil {
			return workflowplan.OperationPlan{}, err
		}
	}
	promotion.Reason = fmt.Sprintf("Install %s/%s path %s at revision %s, blob %s, using %s Git transport", escapePlainText(preview.Host), escapePlainText(preview.Repository), escapePlainText(preview.CatalogPath), preview.Revision, preview.Blob, defaultString(preview.Transport, "https"))
	actions = append(actions, promotion)
	for i := range actions {
		actions[i].Sequence = i + 1
	}
	return workflowplan.Build("init.remote", inputs, actions, lifecycle.Approvals, lifecycle.Diagnostics), nil
}
func remoteInitSemanticPlan(value workflowplan.OperationPlan) workflowplan.OperationPlan {
	value.Actions = append([]workflowplan.Action{}, value.Actions...)
	filtered := []workflowplan.Action{}
	for _, a := range value.Actions {
		if a.TargetRole == "catalog_repository_parent" && a.Target.Directory {
			continue
		}
		a.Sequence = len(filtered) + 1
		filtered = append(filtered, a)
	}
	value.Actions = filtered
	value.Summary.ActionCount = len(filtered)
	for i := range value.Actions {
		if value.Actions[i].Kind == workflowplan.ActionClone {
			value.Actions[i].Target.PromotionSource = ""
			value.Actions[i].Target.PromotionIdentity = workflowplan.Identity{}
			value.Actions[i].Target.PromotionSHA256 = ""
		}
	}
	return value
}
