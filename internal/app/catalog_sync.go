package app

import (
	"alias-lens/internal/catalog"
	"alias-lens/internal/catalogstore"
	"alias-lens/internal/managedgit"
	workflowplan "alias-lens/internal/plan"
	workflowstate "alias-lens/internal/state"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (svc *Services) catalogSyncPath() string {
	return filepath.Join(filepath.Dir(svc.localCatalogPath()), "catalog-sync.json")
}
func (svc *Services) readCatalogSyncRecord() (catalogstore.CatalogSyncRecord, []byte, error) {
	var record catalogstore.CatalogSyncRecord
	data, err := catalogstore.ReadPrivateBytes(svc.catalogSyncPath(), catalogstore.MaxDocumentBytes)
	if err != nil {
		return record, nil, err
	}
	if err := catalogstore.Decode(data, &record); err != nil {
		return record, nil, err
	}
	return record, data, nil
}
func (svc *Services) catalogSyncShellInstalled(shell string) (bool, error) {
	var file catalogstore.InstalledFile
	data, err := catalogstore.ReadPrivateBytes(filepath.Join(svc.homeDirectory(), ".local", "state", "alias-lens", "catalog-installed.json"), catalogstore.MaxDocumentBytes)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := catalogstore.Decode(data, &file); err != nil {
		return false, err
	}
	for _, record := range file.Records {
		if record.Shell == shell {
			return true, nil
		}
	}
	return false, nil
}
func decodeSyncCatalog(data []byte) (catalog.Catalog, error) {
	value, d := catalog.Decode(data)
	if len(d) > 0 {
		return catalog.Catalog{}, fmt.Errorf("catalog sync input is invalid; run al catalog diff")
	}
	return value, nil
}
func (svc *Services) scanCatalogForSync(value catalog.Catalog, data []byte) error {
	if len(findSecretFindings(data)) > 0 {
		return fmt.Errorf("catalog contains likely secrets; remove them before al sync --push")
	}
	for _, entry := range value.Entries {
		for _, implementation := range entry.Native {
			for _, value := range []*string{implementation.AliasValue, implementation.FunctionBody} {
				if value != nil && len(findSecretFindings([]byte(*value))) > 0 {
					return fmt.Errorf("catalog contains likely secrets; remove them before al sync --push")
				}
			}
		}
		if entry.Portable != nil {
			if len(findSecretFindings([]byte(strings.Join(append([]string{entry.Portable.Program}, entry.Portable.Args...), "\n")))) > 0 {
				return fmt.Errorf("catalog contains likely secrets; remove them before al sync --push")
			}
		}
	}
	return nil
}
func catalogRepositorySnapshot(ctx context.Context, r managedgit.Runner, record catalogstore.CatalogSyncRecord) ([]byte, string, string, error) {
	head, err := r.Run(ctx, record.Repository, nil, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return nil, "", "", err
	}
	commit := managedGitText(head)
	path, err := repositoryFilePath(record.Repository, record.CatalogPath)
	if err != nil {
		return nil, "", "", err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, "", "", fmt.Errorf("enrolled catalog must be a regular repository file")
	}
	data, err := readRegularFile(path, catalogstore.MaxDocumentBytes)
	if err != nil {
		return nil, "", "", err
	}
	tree, err := r.Run(ctx, record.Repository, nil, "ls-tree", "-z", commit, "--", record.CatalogPath)
	if err != nil {
		return nil, "", "", err
	}
	parts := strings.SplitN(string(tree), "\t", 2)
	if len(parts) != 2 || strings.TrimSuffix(parts[1], "\x00") != filepath.ToSlash(record.CatalogPath) {
		return nil, "", "", fmt.Errorf("catalog path is not a committed regular file; commit it before al init")
	}
	fields := strings.Fields(parts[0])
	if len(fields) != 3 || fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755") {
		return nil, "", "", fmt.Errorf("catalog path must identify a committed regular blob")
	}
	return data, commit, fields[2], nil
}
func (svc *Services) catalogSyncRemoteSnapshot(ctx context.Context, runner managedgit.Runner, record catalogstore.CatalogSyncRecord) ([]byte, string, string, error) {
	if record.RemoteURL == "" {
		return catalogRepositorySnapshot(ctx, runner, record)
	}
	config, err := svc.loadConfig()
	if err != nil {
		return nil, "", "", err
	}
	if err := runner.AuditNetworkConfig(ctx, record.Repository, record.RemoteURL); err != nil {
		return nil, "", "", err
	}
	remote, err := svc.discoverRemoteCatalogAtRef(ctx, config, record.RemoteURL, record.CatalogPath, record.RemoteRef)
	if err != nil {
		return nil, "", "", err
	}
	if err := svc.proveCatalogRemoteAncestry(ctx, config, record, remote); err != nil {
		return nil, "", "", err
	}
	return remote.Bytes, remote.Revision, remote.Blob, nil
}
func (svc *Services) buildCatalogSyncPullPlan() (workflowplan.OperationPlan, error) {
	record, state, err := svc.readCatalogSyncRecord()
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	if record.PushIntent != nil {
		return workflowplan.OperationPlan{}, fmt.Errorf("a catalog push is unresolved; run al sync --push to reconcile it")
	}
	localBytes, err := readRegularFile(svc.localCatalogPath(), catalogstore.MaxDocumentBytes)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	runner, cleanup, err := managedgit.New()
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	defer cleanup()
	ctx, cancel := managedGitContext()
	defer cancel()
	remoteBytes, head, blob, err := svc.catalogSyncRemoteSnapshot(ctx, runner, record)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	base, err := decodeSyncCatalog([]byte(record.BaseBytes))
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	local, err := decodeSyncCatalog(localBytes)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	remote, err := decodeSyncCatalog(remoteBytes)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	merged := catalog.ThreeWayMerge(base, local, remote)
	inputs := []workflowplan.Input{svc.planInput("catalog_sync", svc.catalogSyncPath(), state), svc.planInput("catalog", svc.localCatalogPath(), localBytes), svc.planInput("repository_catalog", filepath.Join(record.Repository, record.CatalogPath), remoteBytes), {Role: "repository_head", DisplayPath: "repository HEAD", SHA256: hashBytes([]byte(head + "\n" + blob)), Internal: workflowplan.InternalInput{}}}
	if merged.Catalog == nil {

		id := catalogstore.FramedHash([]byte(record.BaseBytes), localBytes, remoteBytes)
		root := filepath.Join(filepath.Dir(svc.catalogSyncPath()), "catalog-conflicts", id)
		metadata := catalogConflictMetadata{Version: 1, BaseSHA256: record.BaseSHA256, LocalSHA256: hashBytes(localBytes), RemoteSHA256: hashBytes(remoteBytes), RemoteRevision: head, RemoteBlob: blob, Conflicts: merged.Conflicts}
		encoded, _ := json.MarshalIndent(metadata, "", "  ")
		encoded = append(encoded, '\n')
		if len(encoded) > catalogstore.MaxDocumentBytes {
			return workflowplan.OperationPlan{}, fmt.Errorf("catalog conflict metadata exceeds limit")
		}
		actions := []workflowplan.Action{}
		for _, item := range []struct {
			name string
			data []byte
		}{{"base.json", []byte(record.BaseBytes)}, {"local.json", localBytes}, {"remote.json", remoteBytes}, {"conflicts.json", encoded}} {
			a, err := svc.catalogSyncAction(filepath.Join(root, item.name), "catalog_conflict", item.data)
			if err != nil {
				return workflowplan.OperationPlan{}, err
			}
			a.Sequence = len(actions) + 1
			actions = append(actions, a)
		}
		return workflowplan.Build("catalog.sync.conflict", inputs, actions, nil, []workflowplan.Diagnostic{{Code: "semantic_conflict", Message: "Catalog changes conflict. Apply to save private conflict copies; inspect al catalog diff before resolving."}}), nil

	}
	planned, diagnostics := catalog.Encode(*merged.Catalog)
	if len(diagnostics) > 0 {
		return workflowplan.OperationPlan{}, fmt.Errorf("combined catalog is invalid")
	}
	record.BaseBytes = string(remoteBytes)
	record.BaseSHA256 = hashBytes(remoteBytes)
	record.SourceSHA256 = hashBytes(planned)
	record.HEAD = head
	record.Blob = blob
	next, err := catalogstore.Encode(record)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	actions := []workflowplan.Action{}
	if !bytes.Equal(localBytes, planned) {
		revision, err := svc.catalogSyncRevisionAction(localBytes)
		if err != nil {
			return workflowplan.OperationPlan{}, err
		}
		if revision != nil {
			revision.Sequence = 1
			actions = append(actions, *revision)
		}
	}
	for _, item := range []struct {
		path, role    string
		before, after []byte
	}{{svc.localCatalogPath(), "catalog", localBytes, planned}, {svc.catalogSyncPath(), "catalog_sync", state, next}} {
		if bytes.Equal(item.before, item.after) {
			continue
		}
		a, err := svc.catalogSyncAction(item.path, item.role, item.after)
		if err != nil {
			return workflowplan.OperationPlan{}, err
		}
		a.Sequence = len(actions) + 1
		actions = append(actions, a)
	}
	return workflowplan.Build("catalog.sync.pull", inputs, actions, nil, nil), nil
}
func (svc *Services) catalogSyncAction(path, role string, planned []byte) (workflowplan.Action, error) {
	before, err := readRegularFile(path, catalogstore.MaxDocumentBytes)
	kind := workflowplan.ActionReplace
	if errors.Is(err, os.ErrNotExist) {
		before = nil
		kind = workflowplan.ActionCreate
	} else if err != nil {
		return workflowplan.Action{}, err
	}
	target := plannedTarget(path, before, planned)
	target.Scope = workflowplan.TargetScopePrivate
	target.Mode = 0600
	return workflowplan.Action{Sequence: 1, Kind: kind, TargetRole: role, DisplayPath: svc.displayPrivatePath(path), Reason: "Save catalog synchronization state", Risk: workflowplan.RiskReview, Backup: before != nil, Reversible: true, Target: target}, nil
}

func (svc *Services) inspectCatalogSync() error {
	record, _, err := svc.readCatalogSyncRecord()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if record.PushIntent != nil {
		return fmt.Errorf("catalog push needs reconciliation; run al sync --push")
	}
	runner, cleanup, err := managedgit.New()
	if err != nil {
		return err
	}
	defer cleanup()
	ctx, cancel := managedGitContext()
	defer cancel()
	remote, _, _, err := svc.catalogSyncRemoteSnapshot(ctx, runner, record)
	if err != nil {
		return err
	}
	if hashBytes(remote) != record.BaseSHA256 {
		return fmt.Errorf("repository catalog changed; review al sync --pull")
	}
	return nil
}

func (svc *Services) rejectCatalogFallbackSync(config AppConfig) error {
	installed, err := svc.catalogSyncShellInstalled(config.Shell)
	if err != nil {
		return err
	}
	if installed {
		return fmt.Errorf("this shell uses catalog generations; native fallback sync is disabled, use al sync with catalog enrollment")
	}
	return nil
}
func (svc *Services) rejectCatalogFallbackTracking(source string) error {
	for _, shell := range []string{"bash", "zsh"} {
		installed, err := svc.catalogSyncShellInstalled(shell)
		if err != nil {
			return err
		}
		if installed {
			adapter, err := svc.shellAdapter(shell)
			if err != nil {
				return err
			}
			if svc.sameFilePath(source, filepath.Join(svc.homeDirectory(), adapter.AliasFilename())) {
				return fmt.Errorf("installed catalog native fallbacks cannot be tracked; use catalog enrollment")
			}
		}
	}
	return nil
}

type CatalogPushPreview struct {
	Record catalogstore.CatalogSyncRecord
	State  []byte
	Local  []byte
	HEAD   string
	Date   string
}

func (svc *Services) prepareCatalogPush() (CatalogPushPreview, error) {
	record, state, err := svc.readCatalogSyncRecord()
	if err != nil {
		return CatalogPushPreview{}, err
	}
	if record.Remote == "" || record.RemoteURL == "" || !catalogstore.ValidRef(record.RemoteRef) {
		return CatalogPushPreview{}, fmt.Errorf("catalog enrollment needs a configured HTTPS origin and branch; rerun al init with that local repository")
	}
	local, err := readRegularFile(svc.localCatalogPath(), catalogstore.MaxDocumentBytes)
	if err != nil {
		return CatalogPushPreview{}, err
	}
	value, err := decodeSyncCatalog(local)
	if err != nil {
		return CatalogPushPreview{}, err
	}
	canonical, d := catalog.Encode(value)
	if len(d) > 0 {
		return CatalogPushPreview{}, fmt.Errorf("local catalog is invalid")
	}
	if err := svc.scanCatalogForSync(value, canonical); err != nil {
		return CatalogPushPreview{}, err
	}
	runner, cleanup, err := managedgit.New()
	if err != nil {
		return CatalogPushPreview{}, err
	}
	defer cleanup()
	ctx, cancel := managedGitContext()
	defer cancel()
	head, err := runner.Run(ctx, record.Repository, nil, "rev-parse", "HEAD")
	if err != nil {
		return CatalogPushPreview{}, err
	}
	if err := runner.AuditNetworkConfig(ctx, record.Repository, record.RemoteURL); err != nil {
		return CatalogPushPreview{}, err
	}
	return CatalogPushPreview{Record: record, State: state, Local: canonical, HEAD: managedGitText(head), Date: svc.dependencies.Now().UTC().Format(time.RFC3339Nano)}, nil
}
func createCatalogPathCommit(ctx context.Context, r managedgit.Runner, preview CatalogPushPreview) (string, string, error) {
	parent := preview.HEAD
	if preview.Record.RemoteURL != "" {
		parent = preview.Record.HEAD
	}
	if _, err := r.Run(ctx, preview.Record.Repository, nil, "merge-base", "--is-ancestor", preview.Record.HEAD, parent); err != nil {
		return "", "", fmt.Errorf("catalog push ancestry is not proven; enroll a complete local repository")
	}
	r.IndexFile = filepath.Join(r.Home, "catalog-index")
	r.CommitDate = preview.Date
	if _, err := r.Run(ctx, preview.Record.Repository, nil, "read-tree", parent); err != nil {
		return "", "", err
	}
	blob, err := r.Run(ctx, preview.Record.Repository, preview.Local, "hash-object", "-w", "--stdin")
	if err != nil {
		return "", "", err
	}
	blobID := managedGitText(blob)
	if _, err := r.Run(ctx, preview.Record.Repository, nil, "update-index", "--add", "--cacheinfo", "100644,"+blobID+","+preview.Record.CatalogPath); err != nil {
		return "", "", err
	}
	tree, err := r.Run(ctx, preview.Record.Repository, nil, "write-tree")
	if err != nil {
		return "", "", err
	}
	commit, err := r.Run(ctx, preview.Record.Repository, []byte("Update enrolled Alias Lens catalog\n"), "commit-tree", managedGitText(tree), "-p", parent)
	if err != nil {
		return "", "", err
	}
	return managedGitText(commit), blobID, nil
}
func (svc *Services) scanCatalogOutgoingHistory(ctx context.Context, r managedgit.Runner, record catalogstore.CatalogSyncRecord, sourceCommit string) error {
	history, err := r.Run(ctx, record.Repository, nil, "rev-list", record.HEAD+".."+sourceCommit, "--", record.CatalogPath)
	if err != nil {
		return err
	}
	commits := strings.Fields(string(history))
	if len(commits) > 10000 {
		return fmt.Errorf("outgoing catalog history exceeds safety limit")
	}
	total := 0
	for _, commit := range commits {
		data, err := r.Run(ctx, record.Repository, nil, "show", commit+":"+record.CatalogPath)
		if err != nil {
			return fmt.Errorf("outgoing catalog history cannot be inspected; push refused")
		}
		total += len(data)
		if total > 64<<20 {
			return fmt.Errorf("outgoing catalog history exceeds safety limit")
		}
		value, err := decodeSyncCatalog(data)
		if err != nil {
			return fmt.Errorf("outgoing enrolled history has an invalid catalog; push refused")
		}
		if err := svc.scanCatalogForSync(value, data); err != nil {
			return err
		}
	}
	return nil
}
func (svc *Services) catalogPushStatePlan(record catalogstore.CatalogSyncRecord, local []byte, writeRepository bool) (workflowplan.OperationPlan, error) {
	encoded, err := catalogstore.Encode(record)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	state, err := svc.catalogSyncAction(svc.catalogSyncPath(), "catalog_sync", encoded)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	actions := []workflowplan.Action{state}
	if writeRepository {
		path, err := repositoryFilePath(record.Repository, record.CatalogPath)
		if err != nil {
			return workflowplan.OperationPlan{}, err
		}
		before, err := readRegularFile(path, catalogstore.MaxDocumentBytes)
		if err != nil {
			return workflowplan.OperationPlan{}, err
		}
		if !bytes.Equal(before, local) {
			target := plannedTarget(path, before, local)
			target.Scope = workflowplan.TargetScopeRepository
			target.RecoveryOrder = 3
			actions = append(actions, workflowplan.Action{Sequence: 2, Kind: workflowplan.ActionReplace, TargetRole: "repository_catalog", DisplayPath: svc.displayPrivatePath(path), Reason: "Update only the enrolled catalog path", Risk: workflowplan.RiskReview, Backup: true, Reversible: true, Target: target})
		}
	}
	return workflowplan.Build("catalog.sync.push.intent", nil, actions, nil, nil), nil
}
func (svc *Services) reconcileCatalogPush(session *mutationSession, record catalogstore.CatalogSyncRecord, runner managedgit.Runner, ctx context.Context) error {
	intent := record.PushIntent
	if intent == nil {
		return nil
	}
	if err := svc.verifyCatalogPushIntent(ctx, runner, record); err != nil {
		return err
	}
	config, err := svc.loadConfig()
	if err != nil {
		return err
	}
	remote, err := svc.discoverRemoteCatalogAtRef(ctx, config, record.RemoteURL, record.CatalogPath, record.RemoteRef)
	if err != nil {
		return err
	}
	if remote.Revision != intent.SourceCommit {
		if remote.Revision != intent.ExpectedRemote {
			return fmt.Errorf("remote changed during uncertain catalog push; preserve local state and review al sync --pull")
		}
		remote.Transport = record.Transport
		remote.KnownHostsSHA = record.KnownHostsSHA
		remote.KnownHostsIdentity = record.KnownHostsIdentity
		if err := svc.verifyCatalogPushIntent(ctx, runner, record); err != nil {
			return err
		}
		token, err := svc.catalogProviderToken(ctx, record.Provider, remote.Host)
		if err != nil {
			return err
		}
		runner.Username = map[string]string{"github": "x-access-token", "gitlab": "oauth2", "bitbucket": "x-token-auth"}[record.Provider]
		runner.Password = token
		runner.Authority = remote.Host
		if _, err := runner.RunTransport(ctx, record.Repository, remote.managedTransport(), nil, "push", "--", record.RemoteURL, intent.SourceCommit+":"+intent.DestinationRef); err != nil {
			return fmt.Errorf("catalog push outcome is uncertain; run al sync --push to reconcile the pinned intent")
		}
		remote, err = svc.discoverRemoteCatalogAtRef(ctx, config, record.RemoteURL, record.CatalogPath, record.RemoteRef)
		if err != nil || remote.Revision != intent.SourceCommit {
			return fmt.Errorf("catalog push outcome is uncertain; run al sync --push to reconcile the pinned intent")
		}
	}
	if hashBytes(remote.Bytes) != intent.CatalogSHA256 {
		return fmt.Errorf("remote catalog differs from pinned push intent; reconciliation refused")
	}
	if record.Managed {
		head, err := runner.Run(ctx, record.Repository, nil, "rev-parse", "HEAD")
		if err != nil {
			return err
		}
		branch, e := runner.Run(ctx, record.Repository, nil, "symbolic-ref", "--quiet", "HEAD")
		if e != nil || managedGitText(branch) != intent.DestinationRef {
			return fmt.Errorf("local catalog branch changed during push; pinned intent was preserved")
		}
		if managedGitText(head) != intent.SourceCommit {
			if managedGitText(head) != intent.LocalHEAD {
				return fmt.Errorf("local catalog history changed during push; pinned intent was preserved")
			}
			if _, err := runner.Run(ctx, record.Repository, nil, "update-ref", intent.DestinationRef, intent.SourceCommit, managedGitText(head)); err != nil {
				return err
			}
		}
		if _, err := runner.Run(ctx, record.Repository, nil, "update-index", "--add", "--cacheinfo", "100644,"+remote.Blob+","+record.CatalogPath); err != nil {
			return err
		}
	}
	record.HEAD = remote.Revision
	record.Blob = remote.Blob
	record.BaseBytes = string(remote.Bytes)
	record.BaseSHA256 = hashBytes(remote.Bytes)
	record.SourceSHA256 = hashBytes(remote.Bytes)
	record.PushIntent = nil
	plan, err := svc.catalogPushStatePlan(record, nil, false)
	if err != nil {
		return err
	}
	return session.ApplyPlan(plan, func() (workflowplan.OperationPlan, error) { return svc.catalogPushStatePlan(record, nil, false) })
}

func (svc *Services) runCatalogSyncPushInSession(session *mutationSession, preview CatalogPushPreview) error {
	fresh, err := svc.prepareCatalogPush()
	if err != nil {
		return err
	}
	if !bytes.Equal(preview.State, fresh.State) || !bytes.Equal(preview.Local, fresh.Local) || preview.HEAD != fresh.HEAD {
		return fmt.Errorf("catalog changed after push preview; review again")
	}
	runner, cleanup, err := managedgit.New()
	if err != nil {
		return err
	}
	defer cleanup()
	ctx, cancel := managedGitContext()
	defer cancel()
	if fresh.Record.PushIntent != nil {
		return svc.reconcileCatalogPush(session, fresh.Record, runner, ctx)
	}
	config, err := svc.loadConfig()
	if err != nil {
		return err
	}
	remote, err := svc.discoverRemoteCatalogAtRef(ctx, config, fresh.Record.RemoteURL, fresh.Record.CatalogPath, fresh.Record.RemoteRef)
	if err != nil {
		return err
	}
	if remote.Revision != fresh.Record.HEAD || hashBytes(remote.Bytes) != fresh.Record.BaseSHA256 {
		return fmt.Errorf("remote catalog changed; review al sync --pull before pushing")
	}
	if err := svc.fetchCatalogBaseMetadata(ctx, runner, fresh.Record, remote); err != nil {
		return err
	}
	sourceCommit, _, err := createCatalogPathCommit(ctx, runner, fresh)
	if err != nil {
		return err
	}
	if err := svc.scanCatalogOutgoingHistory(ctx, runner, fresh.Record, sourceCommit); err != nil {
		return err
	}
	record := fresh.Record
	record.PushIntent = &catalogstore.CatalogPushIntent{LocalHEAD: fresh.HEAD, SourceCommit: sourceCommit, DestinationRef: record.RemoteRef, ExpectedRemote: remote.Revision, CatalogSHA256: hashBytes(fresh.Local)}
	intentPlan, err := svc.catalogPushStatePlan(record, fresh.Local, true)
	if err != nil {
		return err
	}
	if err := session.ApplyPlan(intentPlan, func() (workflowplan.OperationPlan, error) {
		return svc.catalogPushStatePlan(record, fresh.Local, true)
	}); err != nil {
		return err
	}
	return svc.reconcileCatalogPush(session, record, runner, ctx)
}

type catalogConflictMetadata struct {
	Version        int                     `json:"version"`
	BaseSHA256     string                  `json:"base_sha256"`
	LocalSHA256    string                  `json:"local_sha256"`
	RemoteSHA256   string                  `json:"remote_sha256"`
	RemoteRevision string                  `json:"remote_revision"`
	RemoteBlob     string                  `json:"remote_blob"`
	Conflicts      []catalog.MergeConflict `json:"conflicts"`
}
type CatalogConflictResolution struct {
	ID          string
	Base        catalog.Catalog
	Local       catalog.Catalog
	Remote      catalog.Catalog
	Conflicts   []catalog.MergeConflict
	Metadata    catalogConflictMetadata
	Inputs      []workflowplan.Input
	RemoteBytes []byte
}

func (svc *Services) loadCatalogConflictResolution(id string) (CatalogConflictResolution, error) {
	if !catalogstore.ValidHash(id) {
		return CatalogConflictResolution{}, fmt.Errorf("conflict ID must identify a saved catalog conflict")
	}
	root := filepath.Join(filepath.Dir(svc.catalogSyncPath()), "catalog-conflicts", id)
	var result CatalogConflictResolution
	result.ID = id
	data := [][]byte{}
	for _, name := range []string{"base.json", "local.json", "remote.json", "conflicts.json"} {
		path := filepath.Join(root, name)
		b, err := catalogstore.ReadPrivateBytes(path, catalogstore.MaxDocumentBytes)
		if err != nil {
			return result, err
		}
		data = append(data, b)
		result.Inputs = append(result.Inputs, svc.planInput("conflict_"+name, path, b))
	}
	if err := decodeUniqueJSON(data[3], &result.Metadata); err != nil {
		return result, fmt.Errorf("invalid saved conflict metadata")
	}
	canonical, _ := json.MarshalIndent(result.Metadata, "", "  ")
	canonical = append(canonical, '\n')
	m := result.Metadata
	if !bytes.Equal(canonical, data[3]) || m.Version != 1 || m.Conflicts == nil || len(m.Conflicts) == 0 || len(m.Conflicts) > 10000 || hashBytes(data[0]) != m.BaseSHA256 || hashBytes(data[1]) != m.LocalSHA256 || hashBytes(data[2]) != m.RemoteSHA256 || catalogstore.FramedHash(data[0], data[1], data[2]) != id {
		return result, fmt.Errorf("saved conflict proof is invalid")
	}
	var err error
	result.Base, err = decodeSyncCatalog(data[0])
	if err != nil {
		return result, err
	}
	result.Local, err = decodeSyncCatalog(data[1])
	if err != nil {
		return result, err
	}
	result.Remote, err = decodeSyncCatalog(data[2])
	if err != nil {
		return result, err
	}
	result.RemoteBytes = data[2]
	result.Conflicts = m.Conflicts
	return result, nil
}
func (svc *Services) setCatalogConflictField(target *catalog.Catalog, source catalog.Catalog, id, path string) error {
	var chosen *catalog.Entry
	for _, e := range source.Entries {
		if e.ID == id {
			copy := e
			chosen = &copy
			break
		}
	}
	found := -1
	for i, e := range target.Entries {
		if e.ID == id {
			found = i
			break
		}
	}
	if path == "entry" {
		if chosen == nil {
			if found >= 0 {
				target.Entries = append(target.Entries[:found], target.Entries[found+1:]...)
			}
			return nil
		}
		if found >= 0 {
			target.Entries[found] = *chosen
		} else {
			target.Entries = append(target.Entries, *chosen)
		}
		return nil
	}
	if chosen == nil || found < 0 {
		return fmt.Errorf("conflict needs a complete entry choice")
	}
	readMap := func(entry catalog.Entry) map[string]any {
		b, _ := json.Marshal(entry)
		var object map[string]any
		json.Unmarshal(b, &object)
		return object
	}
	dst, src := readMap(target.Entries[found]), readMap(*chosen)
	parts := strings.Split(path, ".")
	if len(parts) > 2 {
		return fmt.Errorf("unsupported conflict field")
	}
	key := parts[len(parts)-1]
	if len(parts) == 2 {
		parent := parts[0]
		a, _ := dst[parent].(map[string]any)
		if a == nil {
			a = map[string]any{}
			dst[parent] = a
		}
		b, _ := src[parent].(map[string]any)
		dst, src = a, b
	}
	if value, present := src[key]; present {
		dst[key] = value
	} else {
		delete(dst, key)
	}
	object := readMap(target.Entries[found])
	if len(parts) == 2 {
		object[parts[0]] = dst
	} else {
		object = dst
	}
	if w, ok := object["when"].(map[string]any); ok && len(w) == 0 {
		delete(object, "when")
	}
	encoded, _ := json.Marshal(object)
	var converted catalog.Entry
	if err := json.Unmarshal(encoded, &converted); err != nil {
		return fmt.Errorf("invalid resolved entry")
	}
	target.Entries[found] = converted
	return nil
}
func (svc *Services) buildCatalogConflictResolutionPlan(id string, choices map[string]string) (workflowplan.OperationPlan, error) {
	resolution, err := svc.loadCatalogConflictResolution(id)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	record, state, err := svc.readCatalogSyncRecord()
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	localBytes, err := readRegularFile(svc.localCatalogPath(), catalogstore.MaxDocumentBytes)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	if record.BaseSHA256 != resolution.Metadata.BaseSHA256 || hashBytes(localBytes) != resolution.Metadata.LocalSHA256 || record.PushIntent != nil {
		return workflowplan.OperationPlan{}, fmt.Errorf("conflict inputs changed; review a new al sync --pull plan")
	}
	runner, cleanup, err := managedgit.New()
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	defer cleanup()
	ctx, cancel := managedGitContext()
	defer cancel()
	remote, revision, blob, err := svc.catalogSyncRemoteSnapshot(ctx, runner, record)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	if hashBytes(remote) != resolution.Metadata.RemoteSHA256 || revision != resolution.Metadata.RemoteRevision || blob != resolution.Metadata.RemoteBlob {
		return workflowplan.OperationPlan{}, fmt.Errorf("remote conflict input changed; review again")
	}
	copyCatalog := func(value catalog.Catalog) catalog.Catalog {
		b, _ := json.Marshal(value)
		var output catalog.Catalog
		json.Unmarshal(b, &output)
		return output
	}
	local, right := copyCatalog(resolution.Local), copyCatalog(resolution.Remote)
	used := map[string]bool{}
	for _, conflict := range resolution.Conflicts {
		key := conflict.EntryID + ":" + conflict.Path
		choice, present := choices[key]
		if !present || (choice != "local" && choice != "remote") {
			return workflowplan.OperationPlan{}, fmt.Errorf("each semantic conflict needs a local or remote choice")
		}
		used[key] = true
		source := resolution.Local
		if choice == "remote" {
			source = resolution.Remote
		}
		if err := svc.setCatalogConflictField(&local, source, conflict.EntryID, conflict.Path); err != nil {
			return workflowplan.OperationPlan{}, err
		}
		if err := svc.setCatalogConflictField(&right, source, conflict.EntryID, conflict.Path); err != nil {
			return workflowplan.OperationPlan{}, err
		}
	}
	if len(used) != len(choices) {
		return workflowplan.OperationPlan{}, fmt.Errorf("conflict choices contain an unknown field")
	}
	merged := catalog.ThreeWayMerge(resolution.Base, local, right)
	if merged.Catalog == nil {
		return workflowplan.OperationPlan{}, fmt.Errorf("choices leave a name or entry conflict; revise the saved candidate before applying")
	}
	planned, d := catalog.Encode(*merged.Catalog)
	if len(d) > 0 {
		return workflowplan.OperationPlan{}, fmt.Errorf("resolved catalog is invalid")
	}
	record.BaseBytes = string(resolution.RemoteBytes)
	record.BaseSHA256 = resolution.Metadata.RemoteSHA256
	record.SourceSHA256 = hashBytes(planned)
	record.HEAD = resolution.Metadata.RemoteRevision
	record.Blob = resolution.Metadata.RemoteBlob
	next, err := catalogstore.Encode(record)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	actions := []workflowplan.Action{}
	if !bytes.Equal(localBytes, planned) {
		revision, err := svc.catalogSyncRevisionAction(localBytes)
		if err != nil {
			return workflowplan.OperationPlan{}, err
		}
		if revision != nil {
			revision.Sequence = 1
			actions = append(actions, *revision)
		}
	}
	for _, item := range []struct {
		path, role string
		data       []byte
	}{{svc.localCatalogPath(), "catalog", planned}, {svc.catalogSyncPath(), "catalog_sync", next}} {
		a, err := svc.catalogSyncAction(item.path, item.role, item.data)
		if err != nil {
			return workflowplan.OperationPlan{}, err
		}
		a.Sequence = len(actions) + 1
		actions = append(actions, a)
	}
	inputs := append(resolution.Inputs, svc.planInput("catalog", svc.localCatalogPath(), localBytes), svc.planInput("catalog_sync", svc.catalogSyncPath(), state))
	return workflowplan.Build("catalog.sync.resolve", inputs, actions, nil, nil), nil
}

func (svc *Services) observeCatalogSyncStatus(inputs *workflowstate.Inputs) {
	record, _, err := svc.readCatalogSyncRecord()
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	inputs.Sync.Configured = true
	if err != nil {
		inputs.Sync.Invalid = true
		return
	}
	inputs.Sync.BaseSHA256 = record.BaseSHA256
	inputs.Sync.RemoteSHA256 = record.BaseSHA256
	local, err := catalogstore.ReadPrivateBytes(svc.localCatalogPath(), catalogstore.MaxDocumentBytes)
	if err != nil {
		inputs.Sync.Invalid = true
		return
	}
	if _, err := decodeSyncCatalog(local); err != nil {
		inputs.Sync.Invalid = true
		return
	}
	inputs.Sync.LocalSHA256 = hashBytes(local)
	if record.RemoteURL == "" {
		path, e := repositoryFilePath(record.Repository, record.CatalogPath)
		if e != nil {
			inputs.Sync.Invalid = true
			return
		}
		remote, e := readRegularFile(path, catalogstore.MaxDocumentBytes)
		if e != nil {
			inputs.Sync.Invalid = true
			return
		}
		if _, e := decodeSyncCatalog(remote); e != nil {
			inputs.Sync.Invalid = true
			return
		}
		inputs.Sync.RemoteSHA256 = hashBytes(remote)
	}
	if record.PushIntent != nil {
		inputs.Sync.Conflict = true
	}
	paths, err := os.ReadDir(filepath.Join(filepath.Dir(svc.localCatalogPath()), "catalog-conflicts"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		inputs.Sync.Invalid = true
		return
	}
	if len(paths) > 1000 {
		inputs.Sync.Invalid = true
		return
	}
	for _, path := range paths {
		if !path.IsDir() {
			inputs.Sync.Invalid = true
			return
		}
		resolution, e := svc.loadCatalogConflictResolution(path.Name())
		if e != nil {
			inputs.Sync.Invalid = true
			return
		}
		if resolution.Metadata.BaseSHA256 == record.BaseSHA256 && resolution.Metadata.LocalSHA256 == inputs.Sync.LocalSHA256 {
			inputs.Sync.Conflict = true
		}
	}
}

func (svc *Services) reconcileCatalogSyncInSession(session *mutationSession) error {
	record, _, err := svc.readCatalogSyncRecord()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if record.PushIntent != nil {
		return fmt.Errorf("catalog push is uncertain; run al sync --catalog --push")
	}
	if record.RemoteURL == "" {
		return svc.inspectCatalogSync()
	}
	local, err := catalogstore.ReadPrivateBytes(svc.localCatalogPath(), catalogstore.MaxDocumentBytes)
	if err != nil {
		return err
	}
	if hashBytes(local) == record.BaseSHA256 {
		return svc.inspectCatalogSync()
	}
	preview, err := svc.prepareCatalogPush()
	if err != nil {
		return err
	}
	return svc.runCatalogSyncPushInSession(session, preview)
}

func (svc *Services) catalogSyncRevisionAction(before []byte) (*workflowplan.Action, error) {
	info, err := os.Lstat(svc.localCatalogPath())
	if err != nil {
		return nil, err
	}
	stamp := info.ModTime().UTC().Format("20060102T150405.000000000Z")
	path := filepath.Join(svc.homeDirectory(), ".local", "state", "alias-lens", "catalog-revisions", stamp+"-"+hashBytes(before)+".json")
	existing, err := catalogstore.ReadPrivateBytes(path, catalogstore.MaxDocumentBytes)
	if err == nil {
		if !bytes.Equal(existing, before) {
			return nil, fmt.Errorf("catalog revision collision")
		}
		return nil, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	action, err := svc.catalogSyncAction(path, "catalog_revision", before)
	return &action, err
}

func (svc *Services) verifyCatalogPushIntent(ctx context.Context, runner managedgit.Runner, record catalogstore.CatalogSyncRecord) error {
	intent := record.PushIntent
	if intent == nil || intent.ExpectedRemote != record.HEAD || intent.DestinationRef != record.RemoteRef {
		return fmt.Errorf("catalog push intent base differs from enrollment")
	}
	parents, err := runner.Run(ctx, record.Repository, nil, "rev-list", "--parents", "-n", "1", intent.SourceCommit)
	fields := strings.Fields(string(parents))
	if err != nil || len(fields) != 2 || fields[0] != intent.SourceCommit || fields[1] != intent.ExpectedRemote {
		return fmt.Errorf("catalog push intent source parent is unproven")
	}
	changes, err := runner.Run(ctx, record.Repository, nil, "diff-tree", "--no-commit-id", "--name-only", "-r", "-z", intent.ExpectedRemote, intent.SourceCommit)
	if err != nil {
		return err
	}
	for _, path := range bytes.Split(changes, []byte{0}) {
		if len(path) != 0 && string(path) != record.CatalogPath {
			return fmt.Errorf("catalog push intent changes an unenrolled path")
		}
	}
	data, err := runner.Run(ctx, record.Repository, nil, "show", intent.SourceCommit+":"+record.CatalogPath)
	if err != nil || hashBytes(data) != intent.CatalogSHA256 {
		return fmt.Errorf("catalog push intent source bytes are unproven")
	}
	value, err := decodeSyncCatalog(data)
	if err != nil {
		return err
	}
	if err := svc.scanCatalogForSync(value, data); err != nil {
		return err
	}
	return svc.scanCatalogOutgoingHistory(ctx, runner, record, intent.SourceCommit)
}
