package main

import (
	"alias-lens/internal/transaction"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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

func runAutoSyncCommand(arguments []string) error {
	config, err := loadConfig()
	if err != nil {
		return err
	}
	if len(arguments) != 1 || (arguments[0] != "enable" && arguments[0] != "disable" && arguments[0] != "status") {
		return fmt.Errorf("usage: al autosync enable|disable|status")
	}
	if arguments[0] == "status" {
		statePath, err := syncDataPath("sync-state.json")
		if err != nil {
			return fmt.Errorf("locate automatic sync status: %w", err)
		}
		state, err := loadSyncStateAt(statePath)
		if err != nil {
			return fmt.Errorf("read automatic sync status %s: %w; run al data paths to locate it", statePath, err)
		}
		type trackedStatus struct {
			file  TrackedFileConfig
			state SyncState
		}
		trackedStatuses := make([]trackedStatus, 0, len(config.TrackedFiles))
		for _, tracked := range config.TrackedFiles {
			trackedPath, pathErr := trackedStatePath(tracked)
			if pathErr != nil {
				return fmt.Errorf("locate tracked sync status for %s: %w", tracked.Source, pathErr)
			}
			trackedState, readErr := loadSyncStateAt(trackedPath)
			if readErr != nil {
				return fmt.Errorf("read tracked sync status for %s at %s: %w", tracked.Source, trackedPath, readErr)
			}
			trackedStatuses = append(trackedStatuses, trackedStatus{file: tracked, state: trackedState})
		}
		if cliStyled() {
			cliHeading("Automatic sync")
		}
		cliKeyValue("enabled", fmt.Sprint(config.AutoSync.Enabled))
		cliKeyValue("status", defaultString(state.Status, "not started"))
		if state.Message != "" {
			cliKeyValue("message", state.Message)
		}
		if !state.UpdatedAt.IsZero() {
			cliKeyValue("updated", state.UpdatedAt.Local().Format(time.RFC3339))
		}
		for _, tracked := range trackedStatuses {
			fmt.Printf("%s: %s -> %s [%s]\n", cliAccent("tracked"), tracked.file.Source, tracked.file.RepositoryPath, defaultString(tracked.state.Status, "waiting"))
		}
		return nil
	}

	return withMutation(func(session *mutationSession) error {
		fresh, e := loadConfig()
		if e != nil {
			return e
		}
		if arguments[0] == "enable" {
			if _, _, e = autosyncUnits(fresh); e != nil {
				return e
			}
		}
		fresh.AutoSync.Enabled = arguments[0] == "enable"
		if e = saveConfigInSession(session, fresh); e != nil {
			return e
		}
		if fresh.AutoSync.Enabled {
			if e = watchProcessLaunch(session); e != nil {
				return e
			}
			cliResult("Automatic sync enabled.")
		} else {
			cliResult("Automatic sync disabled. The current worker will stop on its next check.")
		}
		return nil
	})
}

var watchProcessLaunch = ensureWatchProcessInSession

func autosyncUnits(config AppConfig) (bool, bool, error) {
	_, _, e := readCatalogSyncRecord()
	catalogUnit := e == nil
	if e != nil && !errors.Is(e, os.ErrNotExist) {
		return false, false, e
	}
	installed, e := catalogSyncShellInstalled(config.Shell)
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
		if e := validateTrackedFileConfig(tracked); e != nil {
			return false, false, fmt.Errorf("automatic sync refused a tracked file; run al untrack %s: %w", tracked.Source, e)
		}
	}
	return catalogUnit, nativeUnit, nil
}
func runWatch(daemon bool) error {
	ctx, cancel := interruptContext()
	defer cancel()

	if daemon {
		var worker *transaction.Lock
		for {
			duplicate := false
			e := withMutation(func(session *mutationSession) error {
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
		cycleErr := withMutation(func(session *mutationSession) error {
			config, e := loadConfig()
			if e != nil {
				return e
			}
			interval = config.AutoSync.IntervalSeconds
			if interval < 5 {
				interval = 15
			}
			if !config.AutoSync.Enabled {
				disabled = true
				return writeSyncStatusInSession(session, "stopped", "automatic sync is disabled", "", "")
			}
			catalogUnit, nativeUnit, e := autosyncUnits(config)
			if e != nil {
				return e
			}

			var unitErrors []error
			if catalogUnit {
				if e = reconcileCatalogSyncInSession(session); e != nil {
					unitErrors = append(unitErrors, fmt.Errorf("catalog sync: %w", e))
				}
			}
			if nativeUnit {
				if e = reconcileAliasesInSession(session, config); e != nil {
					unitErrors = append(unitErrors, fmt.Errorf("native alias sync: %w", e))
				}
			}
			for _, tracked := range config.TrackedFiles {
				if e = reconcileTrackedFileInSession(session, config, tracked); e != nil {
					unitErrors = append(unitErrors, e)
				}
			}
			if len(unitErrors) > 0 {
				return errors.Join(unitErrors...)
			}
			if !nativeUnit {
				return writeSyncStatusInSession(session, "synced", "configured sync units checked", "", "")
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
			state, _ := loadSyncState()
			if state.Status != "conflict" {
				_ = writeSyncStatus("offline", cycleErr.Error(), state.LocalHash, state.RemoteHash)
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

func reconcileTrackedFile(config AppConfig, tracked TrackedFileConfig) error {
	return withMutation(func(session *mutationSession) error { return reconcileTrackedFileInSession(session, config, tracked) })
}
func reconcileTrackedFileInSession(session *mutationSession, config AppConfig, tracked TrackedFileConfig) error {
	if err := rejectCatalogFallbackTracking(tracked.Source); err != nil {
		return err
	}
	if err := validateTrackedFileConfig(tracked); err != nil {
		return err
	}
	target, err := repositoryFilePath(config.Repository, tracked.RepositoryPath)
	if err != nil {
		return err
	}
	local, localErr := readFileLimited(tracked.Source, trackedFileLimit)
	remote, remoteErr := readFileLimited(target, trackedFileLimit)
	localMissing, remoteMissing := os.IsNotExist(localErr), os.IsNotExist(remoteErr)
	if localErr != nil && !localMissing {
		return localErr
	}
	if remoteErr != nil && !remoteMissing {
		return remoteErr
	}
	statePath, err := trackedStatePath(tracked)
	if err != nil {
		return err
	}
	state, _ := loadSyncStateAt(statePath)
	if localMissing && remoteMissing {
		return writeSyncStateAtInSession(session, statePath, SyncState{Status: "waiting", Message: "source and repository file do not exist", UpdatedAt: time.Now()})
	}
	if localMissing {
		return saveTrackedConflictInSession(session, tracked, nil, remote, statePath)
	}
	if remoteMissing {
		return pushTrackedFileInSession(session, config, tracked, local, statePath)
	}
	localHash, remoteHash := contentHash(local), contentHash(remote)
	switch decideSyncAction(state, localHash, remoteHash) {
	case actionNoop:
		return writeSyncStateAtInSession(session, statePath, SyncState{LocalHash: localHash, RemoteHash: remoteHash, Status: "synced", Message: "files match", UpdatedAt: time.Now()})
	case actionPush:
		return pushTrackedFileInSession(session, config, tracked, local, statePath)
	case actionPull:
		return saveTrackedConflictInSession(session, tracked, local, remote, statePath)
	case actionConflict:
		return saveTrackedConflictInSession(session, tracked, local, remote, statePath)
	}
	return nil
}

func pushTrackedFile(config AppConfig, tracked TrackedFileConfig, contents []byte, statePath string) error {
	return withMutation(func(session *mutationSession) error {
		return pushTrackedFileInSession(session, config, tracked, contents, statePath)
	})
}
func pushTrackedFileInSession(session *mutationSession, config AppConfig, tracked TrackedFileConfig, contents []byte, statePath string) error {
	if err := validateTrackedFileConfig(tracked); err != nil {
		return err
	}
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
	return writeSyncStateAtInSession(session, statePath, SyncState{LocalHash: hash, RemoteHash: hash, Status: "pushed", Message: "committed and pushed local update", UpdatedAt: time.Now()})
}

func replaceTrackedFile(path string, contents []byte) error {
	return withMutation(func(session *mutationSession) error { return replaceTrackedFileInSession(session, path, contents) })
}
func replaceTrackedFileInSession(session *mutationSession, path string, contents []byte) error {
	current, err := readFileLimited(path, trackedFileLimit)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if err := writePrivateBackupInSession(session, path+".alias-lens.bak", current); err != nil {
		return err
	}
	return session.writeUserFile(path, current, contents, info.Mode().Perm(), true, 0, true)
}

func writePrivateFile(path string, contents []byte) error {
	return withMutation(func(session *mutationSession) error { return session.writePrivate(path, contents) })
}

func trackedStatePath(tracked TrackedFileConfig) (string, error) {
	return syncDataPath("file-" + contentHash([]byte(tracked.Source + "\x00" + tracked.RepositoryPath))[:16] + ".json")
}

func saveTrackedConflict(tracked TrackedFileConfig, local, remote []byte, statePath string) error {
	return withMutation(func(session *mutationSession) error {
		return saveTrackedConflictInSession(session, tracked, local, remote, statePath)
	})
}
func saveTrackedConflictInSession(session *mutationSession, tracked TrackedFileConfig, local, remote []byte, statePath string) error {
	id := contentHash([]byte(tracked.Source + "\x00" + tracked.RepositoryPath))[:16]
	directory, err := syncDataPath(filepath.Join("conflicts", id))
	if err != nil {
		return err
	}
	localCopy := filepath.Join(directory, "local")
	remoteCopy := filepath.Join(directory, "remote")
	if err := session.writePrivate(localCopy, local); err != nil {
		return err
	}
	if err := session.writePrivate(remoteCopy, remote); err != nil {
		return err
	}
	message := fmt.Sprintf("files were not overwritten; compare private copies at %s and %s", localCopy, remoteCopy)
	state := SyncState{LocalHash: contentHash(local), RemoteHash: contentHash(remote), Status: "conflict", Message: message, UpdatedAt: time.Now()}
	if err := writeSyncStateAtInSession(session, statePath, state); err != nil {
		return err
	}
	return fmt.Errorf("tracked file conflict: %s; %s", tracked.Source, message)
}

func ensureWatchProcess() error { return withMutation(ensureWatchProcessInSession) }
func ensureWatchProcessInSession(session *mutationSession) error {
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
	executable, e := os.Executable()
	if e != nil {
		return e
	}
	command := exec.Command(executable, "watch", "--daemon")
	if e = command.Start(); e != nil {
		return e
	}
	return command.Process.Release()
}

func reconcileAliases(config AppConfig) error {
	return withMutation(func(session *mutationSession) error { return reconcileAliasesInSession(session, config) })
}
func reconcileAliasesInSession(session *mutationSession, config AppConfig) error {
	installed, err := catalogSyncShellInstalled(config.Shell)
	if err != nil {
		return err
	}
	if installed {
		return nil
	}
	adapter, err := shellAdapter(config.Shell)
	if err != nil {
		return err
	}
	aliasPath, err := aliasPathFor(adapter)
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
	local, localErr := readFileLimited(aliasPath, aliasFileLimit)
	remote, remoteErr := readFileLimited(target, aliasFileLimit)
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
			if err := writeNewAliasFileInSession(session, aliasPath, nil); err != nil {
				return err
			}
			local = nil
		} else {
			return saveSyncConflictInSession(session, nil, remote, "remote aliases require approval; review them and run al sync --pull")
		}
	}
	state, _ := loadSyncState()
	localHash, remoteHash := contentHash(local), contentHash(remote)
	if remoteMissing {
		return pushAliasSnapshotInSession(session, config, aliasPath, local)
	}
	switch decideSyncAction(state, localHash, remoteHash) {
	case actionNoop:
		return writeSyncStatusInSession(session, "synced", "files match", localHash, remoteHash)
	case actionPush:
		return pushAliasSnapshotInSession(session, config, aliasPath, local)
	case actionPull:
		return saveSyncConflictInSession(session, local, remote, "remote aliases require approval; review them and run al sync --pull")
	case actionConflict:
		return saveSyncConflictInSession(session, local, remote, "both local and remote aliases changed")
	}
	return nil
}

func pushAliasSnapshot(config AppConfig, aliasPath string, contents []byte) error {
	return withMutation(func(session *mutationSession) error {
		return pushAliasSnapshotInSession(session, config, aliasPath, contents)
	})
}
func pushAliasSnapshotInSession(session *mutationSession, config AppConfig, aliasPath string, contents []byte) error {
	if err := secretFindingsError(findSecretFindings(contents)); err != nil {
		return err
	}
	if _, err := syncRepositoryFilesInSession(session, config, aliasPath, true); err != nil {
		return err
	}
	hash := contentHash(contents)
	return writeSyncStatusInSession(session, "pushed", "committed and pushed local alias update", hash, hash)
}

func saveSyncConflict(local, remote []byte, message string) error {
	return withMutation(func(session *mutationSession) error {
		return saveSyncConflictInSession(session, local, remote, message)
	})
}
func saveSyncConflictInSession(session *mutationSession, local, remote []byte, message string) error {
	directory, err := syncDataPath("conflicts")
	if err != nil {
		return err
	}
	suffix := strings.TrimPrefix(activeShellAdapter().AliasFilename(), ".")
	localCopy := filepath.Join(directory, "local."+suffix)
	remoteCopy := filepath.Join(directory, "remote."+suffix)
	if err := session.writePrivate(localCopy, local); err != nil {
		return err
	}
	if err := session.writePrivate(remoteCopy, remote); err != nil {
		return err
	}
	message += fmt.Sprintf("; live aliases were not overwritten; private copies: %s and %s; run al diff", localCopy, remoteCopy)
	if err := writeSyncStatusInSession(session, "conflict", message, contentHash(local), contentHash(remote)); err != nil {
		return fmt.Errorf("save conflict copies, but record sync status: %w", err)
	}
	return errors.New(message)
}

func replaceAliasFile(path string, contents []byte) error {
	return withMutation(func(session *mutationSession) error { return replaceAliasFileInSession(session, path, contents) })
}
func replaceAliasFileInSession(session *mutationSession, path string, contents []byte) error {
	current, mode, _, err := readAliasFile(path)
	if err != nil {
		return err
	}
	return writeAliasFileInSession(session, path, current, contents, mode)
}

func writeNewAliasFile(path string, contents []byte) error {
	return withMutation(func(session *mutationSession) error { return writeNewAliasFileInSession(session, path, contents) })
}
func writeNewAliasFileInSession(session *mutationSession, path string, contents []byte) error {
	return writeAliasFileInSession(session, path, nil, contents, 0600)
}

func contentHash(contents []byte) string {
	digest := sha256.Sum256(contents)
	return hex.EncodeToString(digest[:])
}

func syncDataPath(name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	directory := filepath.Join(home, ".local", "state", "alias-lens")
	return filepath.Join(directory, name), nil
}

func loadSyncState() (SyncState, error) {
	path, err := syncDataPath("sync-state.json")
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

func writeSyncStatus(status, message, localHash, remoteHash string) error {
	return withMutation(func(session *mutationSession) error {
		return writeSyncStatusInSession(session, status, message, localHash, remoteHash)
	})
}
func writeSyncStatusInSession(session *mutationSession, status, message, localHash, remoteHash string) error {
	path, err := syncDataPath("sync-state.json")
	if err != nil {
		return err
	}
	state := SyncState{LocalHash: localHash, RemoteHash: remoteHash, Status: status, Message: message, UpdatedAt: time.Now()}
	return writeSyncStateAtInSession(session, path, state)
}

func writeSyncStateAt(path string, state SyncState) error {
	return withMutation(func(session *mutationSession) error { return writeSyncStateAtInSession(session, path, state) })
}
func writeSyncStateAtInSession(session *mutationSession, path string, state SyncState) error {
	contents, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return session.writePrivate(path, append(contents, '\n'))
}

func syncStatusLabel() string {
	config, configErr := loadConfig()
	if configErr != nil || !config.AutoSync.Enabled {
		return "sync off"
	}
	state, stateErr := loadSyncState()
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
