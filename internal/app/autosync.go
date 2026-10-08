package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/alderon07/al/internal/transaction"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type SyncState struct {
	LocalHash  string    `json:"local_hash,omitempty"`
	RemoteHash string    `json:"remote_hash,omitempty"`
	Status     string    `json:"status"`
	Message    string    `json:"message,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type syncAction string

const (
	actionNoop     syncAction = "noop"
	actionPush     syncAction = "push"
	actionPull     syncAction = "pull"
	actionConflict syncAction = "conflict"
)

func decideSyncAction(state SyncState, localHash, remoteHash string) syncAction {
	if localHash == remoteHash {
		return actionNoop
	}
	if state.Status == "conflict" || (state.LocalHash == "" && state.RemoteHash == "") {
		return actionConflict
	}
	localChanged := localHash != state.LocalHash
	remoteChanged := remoteHash != state.RemoteHash
	if localChanged && !remoteChanged {
		return actionPush
	}
	if !localChanged && remoteChanged {
		return actionPull
	}
	return actionConflict
}

var watchProcessLaunch = func(session *mutationSession) error { return session.services.ensureWatchProcessInSession(session) }

func (svc *Services) autosyncUnits(config AppConfig) (bool, bool, error) {
	_, _, e := svc.readCatalogSyncRecord()
	catalogUnit := e == nil
	if e != nil && !errors.Is(e, os.ErrNotExist) {
		return false, false, e
	}
	installed, e := svc.catalogSyncShellInstalled(config.Shell)
	if e != nil {
		return false, false, e
	}
	nativeUnit := config.Repository != "" && !installed
	if !catalogUnit && !nativeUnit && len(config.TrackedFiles) == 0 {
		return false, false, fmt.Errorf("configure a repository with al repo or enroll a catalog with al init")
	}
	if len(config.TrackedFiles) > 0 && config.Repository == "" {
		return false, false, fmt.Errorf("tracked files need a repository; run al repo or al untrack FILE")
	}
	for _, tracked := range config.TrackedFiles {
		if e := svc.validateTrackedFileConfig(tracked); e != nil {
			return false, false, fmt.Errorf("automatic sync refused a tracked file; run al untrack %s: %w", tracked.Source, e)
		}
	}
	return catalogUnit, nativeUnit, nil
}
func (svc *Services) runWatch(daemon bool) error {
	ctx, cancel := interruptContext()
	defer cancel()

	if daemon {
		var worker *transaction.Lock
		for {
			duplicate := false
			e := svc.withMutation(func(session *mutationSession) error {
				var e error
				worker, e = transaction.AcquireLock(session.stateRoot, filepath.Join(session.stateRoot, "watch-worker.lock"))
				duplicate = errors.Is(e, transaction.ErrLocked)
				return e
			})
			if duplicate {
				return nil
			}
			if e == nil {
				break
			}
			if !errors.Is(e, transaction.ErrLocked) {
				return e
			}
			timer := time.NewTimer(100 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil
			case <-timer.C:
			}
		}
		defer worker.Close()
	}

	for {
		if ctx.Err() != nil {
			return nil
		}
		interval := 15
		disabled := false
		cycleErr := svc.withMutation(func(session *mutationSession) error {
			config, e := svc.loadConfig()
			if e != nil {
				return e
			}
			interval = config.AutoSync.IntervalSeconds
			if interval < 5 {
				interval = 15
			}
			if !config.AutoSync.Enabled {
				disabled = true
				return svc.writeSyncStatusInSession(session, "stopped", "automatic sync is disabled", "", "")
			}
			catalogUnit, nativeUnit, e := svc.autosyncUnits(config)
			if e != nil {
				return e
			}

			var unitErrors []error
			if catalogUnit {
				if e = svc.reconcileCatalogSyncInSession(session); e != nil {
					unitErrors = append(unitErrors, fmt.Errorf("catalog sync: %w", e))
				}
			}
			if nativeUnit {
				if e = svc.reconcileAliasesInSession(session, config); e != nil {
					unitErrors = append(unitErrors, fmt.Errorf("native alias sync: %w", e))
				}
			}
			for _, tracked := range config.TrackedFiles {
				if e = svc.reconcileTrackedFileInSession(session, config, tracked); e != nil {
					unitErrors = append(unitErrors, e)
				}
			}
			if len(unitErrors) > 0 {
				return errors.Join(unitErrors...)
			}
			if !nativeUnit {
				return svc.writeSyncStatusInSession(session, "synced", "configured sync units checked", "", "")
			}
			return nil

		})
		if disabled {
			return cycleErr
		}
		if !daemon {
			return cycleErr
		}
		if cycleErr != nil && !errors.Is(cycleErr, transaction.ErrLocked) {
			state, _ := svc.loadSyncState()
			if state.Status != "conflict" {
				_ = svc.writeSyncStatus("offline", cycleErr.Error(), state.LocalHash, state.RemoteHash)
			}
		}
		timer := time.NewTimer(time.Duration(interval) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

func (svc *Services) reconcileTrackedFile(config AppConfig, tracked TrackedFileConfig) error {
	return svc.withMutation(func(session *mutationSession) error {
		return svc.reconcileTrackedFileInSession(session, config, tracked)
	})
}
func (svc *Services) reconcileTrackedFileInSession(session *mutationSession, config AppConfig, tracked TrackedFileConfig) error {
	if err := svc.rejectCatalogFallbackTracking(tracked.Source); err != nil {
		return err
	}
	if err := svc.validateTrackedFileConfig(tracked); err != nil {
		return err
	}
	target, err := repositoryFilePath(config.Repository, tracked.RepositoryPath)
	if err != nil {
		return err
	}
	observed, localErr := svc.internalSettings().observeTrackedSource(tracked.Source)
	local := observed.contents
	remote, remoteErr := readFileLimited(target, trackedFileLimit)
	localMissing, remoteMissing := os.IsNotExist(localErr), os.IsNotExist(remoteErr)
	if localErr != nil && !localMissing {
		return localErr
	}
	if remoteErr != nil && !remoteMissing {
		return remoteErr
	}
	statePath, err := svc.trackedStatePath(tracked)
	if err != nil {
		return err
	}
	state, _ := loadSyncStateAt(statePath)
	if localMissing && remoteMissing {
		return writeSyncStateAtInSession(session, statePath, SyncState{Status: "waiting", Message: "source and repository file do not exist", UpdatedAt: svc.dependencies.Now()})
	}
	if localMissing {
		return svc.saveTrackedConflictInSession(session, tracked, nil, remote, statePath)
	}
	if remoteMissing {
		return svc.pushObservedTrackedFileInSession(session, config, tracked, observed, statePath)
	}
	localHash, remoteHash := contentHash(local), contentHash(remote)
	switch decideSyncAction(state, localHash, remoteHash) {
	case actionNoop:
		return writeSyncStateAtInSession(session, statePath, SyncState{LocalHash: localHash, RemoteHash: remoteHash, Status: "synced", Message: "files match", UpdatedAt: svc.dependencies.Now()})
	case actionPush:
		return svc.pushObservedTrackedFileInSession(session, config, tracked, observed, statePath)
	case actionPull:
		return svc.saveTrackedConflictInSession(session, tracked, local, remote, statePath)
	case actionConflict:
		return svc.saveTrackedConflictInSession(session, tracked, local, remote, statePath)
	}
	return nil
}

func (svc *Services) pushTrackedFile(config AppConfig, tracked TrackedFileConfig, contents []byte, statePath string) error {
	return svc.withMutation(func(session *mutationSession) error {
		return svc.pushTrackedFileInSession(session, config, tracked, contents, statePath)
	})
}
func (svc *Services) pushTrackedFileInSession(session *mutationSession, config AppConfig, tracked TrackedFileConfig, contents []byte, statePath string) error {
	observed, err := svc.internalSettings().observeTrackedSource(tracked.Source)
	if err != nil {
		return err
	}
	if contentHash(contents) != contentHash(observed.contents) {
		return fmt.Errorf("tracked source changed before publication; run al watch to retry")
	}
	return svc.pushObservedTrackedFileInSession(session, config, tracked, observed, statePath)
}

func (svc *Services) pushObservedTrackedFileInSession(session *mutationSession, config AppConfig, tracked TrackedFileConfig, observed trackedSourceObservation, statePath string) error {
	if err := svc.validateTrackedFileConfig(tracked); err != nil {
		return err
	}
	if err := svc.internalSettings().verifyTrackedSource(tracked.Source, observed); err != nil {
		return err
	}
	contents := observed.contents
	if err := secretFindingsErrorFor(tracked.Source, findSecretFindings(contents)); err != nil {
		return fmt.Errorf("%s: %w", tracked.Source, err)
	}
	if err := writeRepositoryFile(config.Repository, tracked.RepositoryPath, contents, 0o600); err != nil {
		return err
	}
	if output, err := gitOutput("-C", config.Repository, "add", "--", tracked.RepositoryPath); err != nil {
		return fmt.Errorf("git add failed: %s", cleanCommandOutput(output))
	}
	message := "Update " + filepath.Base(tracked.RepositoryPath)
	if output, err := gitOutput("-C", config.Repository, "commit", "--only", "-m", message, "--", tracked.RepositoryPath); err != nil {
		return fmt.Errorf("git commit failed: %s", cleanCommandOutput(output))
	}
	if err := pushRepository(config); err != nil {
		return fmt.Errorf("push tracked file: %w; run al watch to retry", err)
	}
	hash := contentHash(contents)
	return writeSyncStateAtInSession(session, statePath, SyncState{LocalHash: hash, RemoteHash: hash, Status: "pushed", Message: "committed and pushed local update", UpdatedAt: svc.dependencies.Now()})
}

func (svc *Services) replaceTrackedFile(path string, contents []byte) error {
	return svc.withMutation(func(session *mutationSession) error { return replaceTrackedFileInSession(session, path, contents) })
}
func replaceTrackedFileInSession(session *mutationSession, path string, contents []byte) error {
	svc := session.services
	if svc == nil {
		svc = DefaultServices()
	}
	observed, err := svc.internalSettings().observeTrackedSource(path)
	if err != nil {
		return err
	}
	if err := svc.internalSettings().verifyTrackedSource(path, observed); err != nil {
		return err
	}
	current, info := observed.contents, observed.info
	if err := writePrivateBackupInSession(session, path+".alias-lens.bak", current); err != nil {
		return err
	}
	return session.writeUserFile(path, current, contents, info.Mode().Perm(), true, 0, true)
}

func (svc *Services) writePrivateFile(path string, contents []byte) error {
	return svc.withMutation(func(session *mutationSession) error { return session.WritePrivate(path, contents) })
}

func (svc *Services) trackedStatePath(tracked TrackedFileConfig) (string, error) {
	return svc.syncDataPath("file-" + contentHash([]byte(tracked.Source + "\x00" + tracked.RepositoryPath))[:16] + ".json")
}

func (svc *Services) saveTrackedConflict(tracked TrackedFileConfig, local, remote []byte, statePath string) error {
	return svc.withMutation(func(session *mutationSession) error {
		return svc.saveTrackedConflictInSession(session, tracked, local, remote, statePath)
	})
}
func (svc *Services) saveTrackedConflictInSession(session *mutationSession, tracked TrackedFileConfig, local, remote []byte, statePath string) error {
	id := contentHash([]byte(tracked.Source + "\x00" + tracked.RepositoryPath))[:16]
	directory, err := svc.syncDataPath(filepath.Join("conflicts", id))
	if err != nil {
		return err
	}
	localCopy := filepath.Join(directory, "local")
	remoteCopy := filepath.Join(directory, "remote")
	if err := session.WritePrivate(localCopy, local); err != nil {
		return err
	}
	if err := session.WritePrivate(remoteCopy, remote); err != nil {
		return err
	}
	message := fmt.Sprintf("files were not overwritten; compare private copies at %s and %s", localCopy, remoteCopy)
	state := SyncState{LocalHash: contentHash(local), RemoteHash: contentHash(remote), Status: "conflict", Message: message, UpdatedAt: svc.dependencies.Now()}
	if err := writeSyncStateAtInSession(session, statePath, state); err != nil {
		return err
	}
	return fmt.Errorf("tracked file conflict: %s; %s", tracked.Source, message)
}

func (svc *Services) ensureWatchProcess() error {
	return svc.withMutation(svc.ensureWatchProcessInSession)
}
func (svc *Services) ensureWatchProcessInSession(session *mutationSession) error {
	if svc.dependencies.StartWatcher != nil {
		return svc.dependencies.StartWatcher()
	}
	worker, e := transaction.AcquireLock(session.stateRoot, filepath.Join(session.stateRoot, "watch-worker.lock"))
	if errors.Is(e, transaction.ErrLocked) {
		return nil
	}
	if e != nil {
		return e
	}
	if e = worker.Close(); e != nil {
		return e
	}
	executable, e := svc.dependencies.Executable()
	if e != nil {
		return e
	}
	home, err := svc.dependencies.HomeDir()
	if err != nil {
		return err
	}
	command := exec.Command(executable, "watch", "--daemon")
	command.Env = append(os.Environ(), "HOME="+home)
	if e = command.Start(); e != nil {
		return e
	}
	return command.Process.Release()
}

func (svc *Services) reconcileAliases(config AppConfig) error {
	return svc.withMutation(func(session *mutationSession) error { return svc.reconcileAliasesInSession(session, config) })
}
func (svc *Services) reconcileAliasesInSession(session *mutationSession, config AppConfig) error {
	installed, err := svc.catalogSyncShellInstalled(config.Shell)
	if err != nil {
		return err
	}
	if installed {
		return nil
	}
	adapter, err := svc.shellAdapter(config.Shell)
	if err != nil {
		return err
	}
	aliasPath, err := svc.aliasPathFor(adapter)
	if err != nil {
		return err
	}
	if output, err := gitOutput("-C", config.Repository, "pull", "--ff-only"); err != nil {
		return fmt.Errorf("pull deferred: %s; run al watch to retry", cleanCommandOutput(output))
	}
	target, err := repositoryFilePath(config.Repository, config.AliasFile)
	if err != nil {
		return err
	}
	local, localErr := readFileLimited(aliasPath, AliasFileLimit)
	remote, remoteErr := readFileLimited(target, AliasFileLimit)
	localMissing, remoteMissing := os.IsNotExist(localErr), os.IsNotExist(remoteErr)
	if localErr != nil && !localMissing {
		return localErr
	}
	if remoteErr != nil && !remoteMissing {
		return remoteErr
	}
	if !remoteMissing {
		if err := protectRepositoryAliasCopy(config.Repository, config.AliasFile, target, remote); err != nil {
			return err
		}
	}
	if localMissing {
		if remoteMissing {
			if err := svc.writeNewAliasFileInSession(session, aliasPath, nil); err != nil {
				return err
			}
			local = nil
		} else {
			return svc.saveSyncConflictInSession(session, nil, remote, "remote aliases require approval; review them and run al sync --pull")
		}
	}
	state, _ := svc.loadSyncState()
	localHash, remoteHash := contentHash(local), contentHash(remote)
	if remoteMissing {
		return svc.pushAliasSnapshotInSession(session, config, aliasPath, local)
	}
	switch decideSyncAction(state, localHash, remoteHash) {
	case actionNoop:
		return svc.writeSyncStatusInSession(session, "synced", "files match", localHash, remoteHash)
	case actionPush:
		return svc.pushAliasSnapshotInSession(session, config, aliasPath, local)
	case actionPull:
		return svc.saveSyncConflictInSession(session, local, remote, "remote aliases require approval; review them and run al sync --pull")
	case actionConflict:
		return svc.saveSyncConflictInSession(session, local, remote, "both local and remote aliases changed")
	}
	return nil
}

func (svc *Services) pushAliasSnapshot(config AppConfig, aliasPath string, contents []byte) error {
	return svc.withMutation(func(session *mutationSession) error {
		return svc.pushAliasSnapshotInSession(session, config, aliasPath, contents)
	})
}
func (svc *Services) pushAliasSnapshotInSession(session *mutationSession, config AppConfig, aliasPath string, contents []byte) error {
	if err := svc.secretFindingsError(findSecretFindings(contents)); err != nil {
		return err
	}
	if _, err := svc.syncRepositoryFilesInSession(session, config, aliasPath, true); err != nil {
		return err
	}
	hash := contentHash(contents)
	return svc.writeSyncStatusInSession(session, "pushed", "committed and pushed local alias update", hash, hash)
}

func (svc *Services) saveSyncConflict(local, remote []byte, message string) error {
	return svc.withMutation(func(session *mutationSession) error {
		return svc.saveSyncConflictInSession(session, local, remote, message)
	})
}
func (svc *Services) saveSyncConflictInSession(session *mutationSession, local, remote []byte, message string) error {
	directory, err := svc.syncDataPath("conflicts")
	if err != nil {
		return err
	}
	suffix := strings.TrimPrefix(svc.activeShellAdapter().AliasFilename(), ".")
	localCopy := filepath.Join(directory, "local."+suffix)
	remoteCopy := filepath.Join(directory, "remote."+suffix)
	if err := session.WritePrivate(localCopy, local); err != nil {
		return err
	}
	if err := session.WritePrivate(remoteCopy, remote); err != nil {
		return err
	}
	message += fmt.Sprintf("; live aliases were not overwritten; private copies: %s and %s; run al diff", localCopy, remoteCopy)
	if err := svc.writeSyncStatusInSession(session, "conflict", message, contentHash(local), contentHash(remote)); err != nil {
		return fmt.Errorf("save conflict copies, but record sync status: %w", err)
	}
	return errors.New(message)
}

func (svc *Services) replaceAliasFile(path string, contents []byte) error {
	return svc.withMutation(func(session *mutationSession) error {
		return svc.replaceAliasFileInSession(session, path, contents)
	})
}
func (svc *Services) replaceAliasFileInSession(session *mutationSession, path string, contents []byte) error {
	current, mode, _, err := readAliasFile(path)
	if err != nil {
		return err
	}
	return svc.writeAliasFileInSession(session, path, current, contents, mode)
}

func (svc *Services) writeNewAliasFile(path string, contents []byte) error {
	return svc.withMutation(func(session *mutationSession) error {
		return svc.writeNewAliasFileInSession(session, path, contents)
	})
}
func (svc *Services) writeNewAliasFileInSession(session *mutationSession, path string, contents []byte) error {
	return svc.writeAliasFileInSession(session, path, nil, contents, 0600)
}

func contentHash(contents []byte) string {
	digest := sha256.Sum256(contents)
	return hex.EncodeToString(digest[:])
}

func (svc *Services) syncDataPath(name string) (string, error) {
	home, err := svc.dependencies.HomeDir()
	if err != nil {
		return "", err
	}
	directory := filepath.Join(home, ".local", "state", "alias-lens")
	return filepath.Join(directory, name), nil
}

func (svc *Services) loadSyncState() (SyncState, error) {
	path, err := svc.syncDataPath("sync-state.json")
	if err != nil {
		return SyncState{}, err
	}
	return loadSyncStateAt(path)
}

func loadSyncStateAt(path string) (SyncState, error) {
	contents, err := readManagedPrivateFile(path, 1<<20)
	if os.IsNotExist(err) {
		return SyncState{}, nil
	}
	if err != nil {
		return SyncState{}, err
	}
	var state SyncState
	return state, json.Unmarshal(contents, &state)
}

func (svc *Services) writeSyncStatus(status, message, localHash, remoteHash string) error {
	return svc.withMutation(func(session *mutationSession) error {
		return svc.writeSyncStatusInSession(session, status, message, localHash, remoteHash)
	})
}
func (svc *Services) writeSyncStatusInSession(session *mutationSession, status, message, localHash, remoteHash string) error {
	path, err := svc.syncDataPath("sync-state.json")
	if err != nil {
		return err
	}
	state := SyncState{LocalHash: localHash, RemoteHash: remoteHash, Status: status, Message: message, UpdatedAt: svc.dependencies.Now()}
	return writeSyncStateAtInSession(session, path, state)
}

func (svc *Services) writeSyncStateAt(path string, state SyncState) error {
	return svc.withMutation(func(session *mutationSession) error { return writeSyncStateAtInSession(session, path, state) })
}
func writeSyncStateAtInSession(session *mutationSession, path string, state SyncState) error {
	contents, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return session.WritePrivate(path, append(contents, '\n'))
}

func (svc *Services) syncStatusLabel() string {
	config, configErr := svc.loadConfig()
	if configErr != nil || !config.AutoSync.Enabled {
		return "sync off"
	}
	state, stateErr := svc.loadSyncState()
	if stateErr != nil || state.Status == "" {
		return "sync waiting"
	}
	switch state.Status {
	case "synced", "pushed", "pulled":
		return "● " + state.Status
	case "conflict":
		return "! conflict"
	case "offline":
		return "○ offline"
	default:
		return state.Status
	}
}
