package main

import (
	"alias-lens/internal/catalogstore"
	"alias-lens/internal/managedgit"
	workflowplan "alias-lens/internal/plan"
	"alias-lens/internal/transaction"
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

const managedCloneTreeLimit int64 = 256 << 20

type catalogStageIntent struct {
	Version          int                     `json:"version"`
	ID               string                  `json:"id"`
	Root             string                  `json:"root"`
	Device           uint64                  `json:"device"`
	Inode            uint64                  `json:"inode"`
	Revision         string                  `json:"revision"`
	Blob             string                  `json:"blob"`
	Destination      string                  `json:"destination"`
	CreatedParents   []string                `json:"created_parents"`
	ParentIdentities []workflowplan.Identity `json:"parent_identities"`
}

func managedCatalogDestination(preview remoteCatalogPreview) string {
	id := catalogstore.FramedHash([]byte(preview.Provider), []byte(preview.Host), []byte(preview.Repository))
	return filepath.Join(homeDirectory(), ".local", "share", "alias-lens", "catalog-repos", id)
}
func managedStageIntentPath(id string) string {
	return filepath.Join(homeDirectory(), ".local", "state", "alias-lens", "catalog-stages", id+".json")
}
func beginManagedCatalogStage(session *mutationSession, preview remoteCatalogPreview) (catalogStageIntent, error) {
	if observePlanIdentity(managedCatalogDestination(preview)).FileType != "missing" {
		return catalogStageIntent{}, errors.New("managed destination already exists; enroll its local path")
	}
	id, err := newOperationID()
	if err != nil {
		return catalogStageIntent{}, err
	}
	created, err := prepareManagedStageParents(session, preview)
	if err != nil {
		return catalogStageIntent{}, err
	}
	parent := filepath.Dir(managedCatalogDestination(preview))
	root, err := os.MkdirTemp(parent, ".stage-")
	if err != nil {
		return catalogStageIntent{}, err
	}
	if err := os.Chmod(root, 0700); err != nil {
		os.Remove(root)
		return catalogStageIntent{}, err
	}
	identity := observePlanIdentity(root)
	intent := catalogStageIntent{Version: 1, ID: id, Root: root, Device: identity.Device, Inode: identity.Inode, Revision: preview.Revision, Blob: preview.Blob, Destination: managedCatalogDestination(preview), CreatedParents: created}
	intent.ParentIdentities = make([]workflowplan.Identity, len(created))
	for i, parent := range created {
		intent.ParentIdentities[i] = observePlanIdentity(parent)
	}
	marker := filepath.Join(root, ".operation")
	if err := os.WriteFile(marker, []byte(id+"\n"), 0600); err != nil {
		os.Remove(root)
		return catalogStageIntent{}, err
	}
	data, err := json.MarshalIndent(intent, "", "  ")
	if err != nil {
		return catalogStageIntent{}, err
	}
	data = append(data, '\n')
	if err := session.writePrivate(managedStageIntentPath(id), data); err != nil {
		os.Remove(marker)
		os.Remove(root)
		return catalogStageIntent{}, err
	}
	return intent, nil
}
func cleanupManagedCatalogStage(intent catalogStageIntent) error {
	if err := inspectManagedStageParents(intent); err != nil {
		return err
	}
	identity := observePlanIdentity(intent.Root)
	if identity.FileType != "missing" {
		if identity.FileType != "directory" || identity.Device != intent.Device || identity.Inode != intent.Inode || identity.Owner != uint64(os.Geteuid()) || identity.Mode != 0700 {
			return errors.New("catalog staging identity changed; private stage was preserved")
		}
		marker, err := catalogstore.ReadPrivateBytes(filepath.Join(intent.Root, ".operation"), 256)
		if err != nil || string(marker) != intent.ID+"\n" {
			return errors.New("catalog staging ownership proof changed")
		}
		if err := validateManagedStageTree(intent.Root); err != nil {
			return err
		}
		if err := os.RemoveAll(intent.Root); err != nil {
			return err
		}
	}
	_, err := os.Lstat(intent.Destination)
	if err == nil {
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for i := len(intent.CreatedParents) - 1; i >= 0; i-- {
		if i >= len(intent.ParentIdentities) {
			return errors.New("catalog parent ownership proof missing")
		}
		expected := intent.ParentIdentities[i]
		current := observePlanIdentity(intent.CreatedParents[i])
		if current.FileType == "missing" {
			continue
		}
		if current.FileType != "directory" || current.Device != expected.Device || current.Inode != expected.Inode || current.Owner != expected.Owner || current.Mode != expected.Mode {
			return errors.New("catalog parent ownership changed; directory preserved")
		}
		if err := os.Remove(intent.CreatedParents[i]); err != nil && !errors.Is(err, os.ErrNotExist) {
			break
		}
	}
	return nil
}

func validateManagedStageTree(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return errors.New("cannot inspect managed staging tree")
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return errors.New("managed staging tree contains unsupported links or files")
		}
		identity := observePlanIdentity(path)
		if identity.Owner != uint64(os.Geteuid()) || identity.FileType == "regular" && identity.LinkCount != 1 {
			return errors.New("managed staging tree ownership changed")
		}
		return nil
	})
}
func managedTreeSize(root string) (int64, error) {
	return managedTreeSizeLimit(root, managedCloneTreeLimit)
}
func managedTreeSizeLimit(root string, limit int64) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("managed staging links refused")
		}
		if !entry.IsDir() {
			info, err := entry.Info()
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			if err != nil {
				return err
			}
			total += info.Size()
			if total > limit {
				return errors.New("managed clone exceeds its byte limit")
			}
		}
		return nil
	})
	return total, err
}
func managedFilterProof(trace []byte) bool {
	advertised, requested := false, false
	for _, line := range bytes.Split(trace, []byte{'\n'}) {
		text := string(line)
		if !strings.Contains(text, "packet:") {
			continue
		}
		if i := strings.Index(text, "< "); i >= 0 {
			payload := text[i+2:]
			for _, word := range strings.Fields(payload) {
				if word == "filter" {
					advertised = true
				}
			}
		}
		if i := strings.Index(text, "> "); i >= 0 && strings.TrimSpace(text[i+2:]) == "filter blob:none" {
			requested = true
		}
	}
	return advertised && requested
}

type managedStageLimits struct {
	Deadline               time.Duration
	TreeBytes, PacketBytes int64
}

func stageRemoteCatalog(ctx context.Context, session *mutationSession, preview remoteCatalogPreview) (catalogStageIntent, string, error) {
	return stageRemoteCatalogWithLimits(ctx, session, preview, managedStageLimits{5 * time.Minute, managedCloneTreeLimit, catalogstore.MaxDocumentBytes})
}
func stageRemoteCatalogWithLimits(ctx context.Context, session *mutationSession, preview remoteCatalogPreview, limits managedStageLimits) (catalogStageIntent, string, error) {
	if limits.Deadline <= 0 || limits.Deadline > 5*time.Minute || limits.TreeBytes <= 0 || limits.TreeBytes > managedCloneTreeLimit || limits.PacketBytes <= 0 || limits.PacketBytes > catalogstore.MaxDocumentBytes {
		return catalogStageIntent{}, "", errors.New("invalid managed staging bounds")
	}

	intent, err := beginManagedCatalogStage(session, preview)
	if err != nil {
		return intent, "", err
	}
	repository := filepath.Join(intent.Root, "checkout")
	fail := func(err error) (catalogStageIntent, string, error) {
		cleanupErr := cleanupManagedCatalogStage(intent)
		if cleanupErr != nil {
			return intent, "", errors.Join(err, cleanupErr)
		}
		return intent, "", err
	}
	runner, cleanup, err := managedgit.New()
	if err != nil {
		return fail(err)
	}
	defer cleanup()
	token, err := catalogProviderToken(ctx, preview.Provider, preview.Host)
	if err != nil {
		return fail(err)
	}
	runner.Username = map[string]string{"github": "x-access-token", "gitlab": "oauth2", "bitbucket": "x-token-auth"}[preview.Provider]
	runner.Password = token
	runner.Authority = preview.Host
	runner.TracePath = filepath.Join(runner.Home, "packets")
	if err := os.WriteFile(runner.TracePath, nil, 0600); err != nil {
		return fail(err)
	}
	bounded, cancel := context.WithTimeout(ctx, limits.Deadline)
	defer cancel()
	monitorDone := make(chan struct{})
	monitorExited := make(chan struct{})
	monitorError := make(chan error, 1)
	tracePath := runner.TracePath
	go func() {
		defer close(monitorExited)
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-monitorDone:
				return
			case <-bounded.Done():
				return
			case <-ticker.C:
				_, err := managedTreeSizeLimit(intent.Root, limits.TreeBytes)
				if err == nil {
					if info, e := os.Stat(tracePath); e != nil || info.Size() > limits.PacketBytes {
						err = errors.New("managed protocol evidence exceeds limit")
					}
				}
				if err != nil {
					select {
					case monitorError <- err:
					default:
					}
					cancel()
					return
				}
			}
		}
	}()
	_, err = runner.RunTransport(bounded, "", preview.managedTransport(), nil, "clone", "--no-checkout", "--no-recurse-submodules", "--single-branch", "--depth=1", "--filter=blob:none", "--branch", preview.Branch, "--template="+runner.Hooks, "--", preview.CloneURL, repository)
	close(monitorDone)
	<-monitorExited
	if err != nil {
		return fail(err)
	}
	select {
	case err := <-monitorError:
		return fail(err)
	default:
	}
	if bounded.Err() != nil {
		return fail(bounded.Err())
	}
	if _, err := managedTreeSizeLimit(intent.Root, limits.TreeBytes); err != nil {
		return fail(err)
	}
	trace, err := catalogstore.ReadPrivateBytes(runner.TracePath, limits.PacketBytes)
	if err != nil || !managedFilterProof(trace) {
		return fail(errors.New("server did not negotiate blob filtering; use al init with a local checkout"))
	}
	runner.TracePath = ""
	runner.Password = ""
	head, err := runner.Run(bounded, repository, nil, "rev-parse", "HEAD")
	if err != nil || managedGitText(head) != preview.Revision {
		return fail(errors.New("remote revision changed after review"))
	}
	tree, err := runner.Run(bounded, repository, nil, "ls-tree", "-z", "HEAD", "--", preview.CatalogPath)
	if err != nil {
		return fail(err)
	}
	expected := "100644 blob " + preview.Blob + "\t" + preview.CatalogPath + "\x00"
	expectedExecutable := "100755 blob " + preview.Blob + "\t" + preview.CatalogPath + "\x00"
	if string(tree) != expected && string(tree) != expectedExecutable {
		return fail(errors.New("remote catalog tree proof differs from preview"))
	}
	objects, err := runner.Run(bounded, repository, nil, "cat-file", "--batch-all-objects", "--batch-check=%(objecttype)")
	if err != nil {
		return fail(err)
	}
	for _, kind := range strings.Fields(string(objects)) {
		if kind == "blob" {
			return fail(errors.New("clone materialized blobs despite blob:none; local enrollment is required"))
		}
	}
	blob, err := runner.Run(bounded, repository, preview.Bytes, "hash-object", "-w", "--stdin")
	if err != nil || managedGitText(blob) != preview.Blob {
		return fail(errors.New("catalog blob materialization proof failed"))
	}
	if err := initializeManagedCatalogIndex(bounded, runner, repository, preview.CatalogPath); err != nil {
		return fail(err)
	}
	target := filepath.Join(repository, filepath.FromSlash(preview.CatalogPath))
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return fail(err)
	}
	if err := os.WriteFile(target, preview.Bytes, 0600); err != nil {
		return fail(err)
	}
	if err := normalizeManagedStageModes(intent.Root); err != nil {
		return fail(err)
	}
	if err := validateManagedStageTree(intent.Root); err != nil {
		return fail(err)
	}
	if bounded.Err() != nil {
		return fail(bounded.Err())
	}
	return intent, repository, nil
}
func managedPromotionAction(repository, destination string) (workflowplan.Action, error) {
	identity, err := transaction.InspectWorkflowDirectory(repository)
	if err != nil {
		return workflowplan.Action{}, err
	}
	target := workflowplan.Target{Path: destination, Scope: workflowplan.TargetScopeRepository, Mode: 0700, RecoveryOrder: 4, ExpectedIdentity: observePlanIdentity(destination), ExpectedSHA256: hashBytes(nil), PromotionSource: repository, PromotionIdentity: observePlanIdentity(repository), PromotionSHA256: identity.SHA256}
	if target.ExpectedIdentity.FileType != "missing" {
		return workflowplan.Action{}, fmt.Errorf("managed destination already exists; enroll its local path")
	}
	return workflowplan.Action{Kind: workflowplan.ActionClone, TargetRole: "catalog_repository", DisplayPath: displayPrivatePath(destination), Reason: "Install only the API-pinned catalog from a filtered managed repository", Risk: workflowplan.RiskReview, Reversible: true, Target: target}, nil
}
func recoverManagedCatalogStages(session *mutationSession) error {
	directory := filepath.Join(homeDirectory(), ".local", "state", "alias-lens", "catalog-stages")
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) > 1000 {
		return errors.New("catalog staging intent limit exceeded")
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			return errors.New("unexpected catalog staging intent")
		}
		path := filepath.Join(directory, entry.Name())
		data, err := catalogstore.ReadPrivateBytes(path, 1<<20)
		if err != nil {
			return err
		}
		intent, err := decodeCatalogStageIntent(data, entry.Name(), homeDirectory())
		if err != nil {
			return err
		}
		if err := cleanupManagedCatalogStage(intent); err != nil {
			return err
		}
		if err := session.removePrivate(path); err != nil {
			return err
		}
	}
	return nil
}

func prepareManagedStageParents(session *mutationSession, preview remoteCatalogPreview) ([]string, error) {
	paths := []string{filepath.Join(homeDirectory(), ".local", "share"), filepath.Join(homeDirectory(), ".local", "share", "alias-lens"), filepath.Dir(managedCatalogDestination(preview))}
	created := []string{}
	for _, path := range paths {
		if observePlanIdentity(path).FileType == "missing" {
			created = append(created, path)
		}
	}
	if err := session.EnsurePrivateDataDirectories(paths...); err != nil {
		return nil, err
	}
	return created, nil
}

func fetchCatalogBaseMetadata(ctx context.Context, runner managedgit.Runner, record catalogstore.CatalogSyncRecord, preview remoteCatalogPreview) error {
	preview.Transport = record.Transport
	preview.KnownHostsSHA = record.KnownHostsSHA
	preview.KnownHostsIdentity = record.KnownHostsIdentity
	if _, err := runner.Run(ctx, record.Repository, nil, "cat-file", "-e", record.HEAD+"^{commit}"); err == nil {
		blob, e := runner.Run(ctx, record.Repository, []byte(record.BaseBytes), "hash-object", "-w", "--stdin")
		if e != nil || managedGitText(blob) != record.Blob {
			return errors.New("saved catalog blob proof failed")
		}
		return nil
	}
	token, err := catalogProviderToken(ctx, record.Provider, preview.Host)
	if err != nil {
		return err
	}
	runner.Username = map[string]string{"github": "x-access-token", "gitlab": "oauth2", "bitbucket": "x-token-auth"}[record.Provider]
	runner.Password, runner.Authority = token, preview.Host
	runner.TracePath = filepath.Join(runner.Home, "fetch-packets")
	if err := os.WriteFile(runner.TracePath, nil, 0600); err != nil {
		return err
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	done := make(chan struct{})
	exited := make(chan struct{})
	defer func() {
		close(done)
		<-exited
	}()
	tracePath := runner.TracePath
	go func() {
		defer close(exited)
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-bounded.Done():
				return
			case <-ticker.C:
				_, e := managedTreeSize(filepath.Join(record.Repository, ".git"))
				info, traceErr := os.Stat(tracePath)
				if e != nil || traceErr != nil || info.Size() > catalogstore.MaxDocumentBytes {
					cancel()
					return
				}
			}
		}
	}()
	beforeObjects, err := runner.Run(bounded, record.Repository, nil, "cat-file", "--batch-all-objects", "--batch-check=%(objectname) %(objecttype)")
	if err != nil {
		return err
	}
	knownBlobs := map[string]bool{}
	for _, line := range strings.Split(string(beforeObjects), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == "blob" {
			knownBlobs[fields[0]] = true
		}
	}
	if _, err := runner.RunTransport(bounded, record.Repository, preview.managedTransport(), nil, "fetch", "--no-write-fetch-head", "--no-tags", "--no-recurse-submodules", "--depth=1", "--filter=blob:none", "--", record.RemoteURL, record.HEAD); err != nil {
		return err
	}
	afterObjects, err := runner.Run(bounded, record.Repository, nil, "cat-file", "--batch-all-objects", "--batch-check=%(objectname) %(objecttype)")
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(afterObjects), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == "blob" && !knownBlobs[fields[0]] {
			return errors.New("metadata fetch materialized an unapproved blob")
		}
	}
	trace, err := readRegularFile(runner.TracePath, catalogstore.MaxDocumentBytes)
	if err != nil || !managedFilterProof(trace) || bounded.Err() != nil {
		return errors.New("remote metadata fetch lacks proven filtering")
	}
	if _, err := managedTreeSize(filepath.Join(record.Repository, ".git")); err != nil {
		return err
	}
	tree, err := runner.Run(bounded, record.Repository, nil, "ls-tree", "-z", record.HEAD, "--", record.CatalogPath)
	if err != nil || (string(tree) != "100644 blob "+record.Blob+"\t"+record.CatalogPath+"\x00" && string(tree) != "100755 blob "+record.Blob+"\t"+record.CatalogPath+"\x00") {
		return errors.New("remote base tree differs from API proof")
	}
	blob, err := runner.Run(bounded, record.Repository, []byte(record.BaseBytes), "hash-object", "-w", "--stdin")
	if err != nil || managedGitText(blob) != record.Blob {
		return errors.New("saved catalog blob proof failed")
	}
	return nil
}

func initializeManagedCatalogIndex(ctx context.Context, runner managedgit.Runner, repository, catalogPath string) error {
	if _, err := runner.Run(ctx, repository, nil, "config", "core.sparseCheckout", "true"); err != nil {
		return err
	}
	if _, err := runner.Run(ctx, repository, nil, "config", "core.sparseCheckoutCone", "false"); err != nil {
		return err
	}
	pattern := strings.NewReplacer("\\", "\\\\", "*", "\\*", "?", "\\?", "[", "\\[", "]", "\\]", "!", "\\!", "#", "\\#").Replace(catalogPath)
	if err := os.MkdirAll(filepath.Join(repository, ".git", "info"), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(repository, ".git", "info", "sparse-checkout"), []byte("/"+pattern+"\n"), 0600); err != nil {
		return err
	}

	if _, err := runner.Run(ctx, repository, nil, "read-tree", "HEAD"); err != nil {
		return err
	}
	paths, err := runner.Run(ctx, repository, nil, "ls-files", "-z")
	if err != nil {
		return err
	}
	if _, err := runner.Run(ctx, repository, paths, "update-index", "--skip-worktree", "-z", "--stdin"); err != nil {
		return err
	}
	if _, err := runner.Run(ctx, repository, nil, "update-index", "--no-skip-worktree", "--", catalogPath); err != nil {
		return err
	}
	diff, err := runner.Run(ctx, repository, nil, "diff", "--cached", "--name-only", "--no-ext-diff", "--no-textconv")
	if err != nil || len(diff) != 0 {
		return errors.New("managed catalog index differs from pinned HEAD")
	}
	return nil
}

func normalizeManagedStageModes(root string) error {
	if err := validateManagedStageTree(root); err != nil {
		return err
	}
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		mode := os.FileMode(0600)
		if entry.IsDir() {
			mode = 0700
		}
		return os.Chmod(path, mode)
	})
}

func decodeCatalogStageIntent(data []byte, name, home string) (catalogStageIntent, error) {
	var intent catalogStageIntent
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&intent); err != nil {
		return intent, errors.New("invalid catalog staging intent")
	}
	canonical, _ := json.MarshalIndent(intent, "", "  ")
	canonical = append(canonical, '\n')
	parent := filepath.Join(home, ".local", "share", "alias-lens", "catalog-repos")
	if !bytes.Equal(data, canonical) || intent.Version != 1 || name != intent.ID+".json" || len(intent.ID) != 35 || !strings.HasPrefix(intent.ID, "op-") || !catalogstore.ValidHash(intent.ID[3:]+intent.ID[3:]) || !catalogGitObject.MatchString(intent.Revision) || !catalogGitObject.MatchString(intent.Blob) || filepath.Dir(intent.Root) != parent || !strings.HasPrefix(filepath.Base(intent.Root), ".stage-") || filepath.Clean(intent.Root) != intent.Root || filepath.Dir(intent.Destination) != parent || !catalogstore.ValidHash(filepath.Base(intent.Destination)) {
		return intent, errors.New("catalog staging intent is not canonical and owned")
	}
	if intent.CreatedParents == nil || intent.ParentIdentities == nil || len(intent.CreatedParents) != len(intent.ParentIdentities) || len(intent.CreatedParents) > 3 {
		return intent, errors.New("invalid staging parent proof")
	}
	expectedParents := []string{filepath.Dir(filepath.Dir(parent)), filepath.Dir(parent), parent}
	previous := -1
	for i, path := range intent.CreatedParents {
		matched := -1
		for j, expected := range expectedParents {
			if path == expected {
				matched = j
			}
		}
		if matched <= previous || intent.ParentIdentities[i].FileType != "directory" || intent.ParentIdentities[i].Inode == 0 || intent.ParentIdentities[i].Mode > 0777 {
			return intent, errors.New("invalid staging parent ownership")
		}
		previous = matched
	}
	return intent, nil
}
func inspectManagedStageParents(intent catalogStageIntent) error {
	if len(intent.CreatedParents) != len(intent.ParentIdentities) {
		return errors.New("catalog parent ownership proof missing")
	}
	for i, path := range intent.CreatedParents {
		current := observePlanIdentity(path)
		if current.FileType == "missing" {
			continue
		}
		expected := intent.ParentIdentities[i]
		if current.FileType != "directory" || current.Device != expected.Device || current.Inode != expected.Inode || current.Owner != expected.Owner || current.Mode != expected.Mode {
			return errors.New("catalog parent ownership changed; directory preserved")
		}
		if _, err := transaction.InspectWorkflowDirectoryMetadata(path); err != nil {
			return errors.New("catalog parent path changed; directory preserved")
		}
	}
	return nil
}
