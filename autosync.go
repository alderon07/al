package main

import (
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
		state, _ := loadSyncState()
		fmt.Printf("enabled: %t\nstatus: %s\n", config.AutoSync.Enabled, defaultString(state.Status, "not started"))
		if state.Message != "" {
			fmt.Println("message:", state.Message)
		}
		if !state.UpdatedAt.IsZero() {
			fmt.Println("updated:", state.UpdatedAt.Local().Format(time.RFC3339))
		}
		for _, tracked := range config.TrackedFiles {
			statePath, pathErr := trackedStatePath(tracked)
			if pathErr != nil {
				continue
			}
			trackedState, _ := loadSyncStateAt(statePath)
			fmt.Printf("tracked: %s -> %s [%s]\n", tracked.Source, tracked.RepositoryPath, defaultString(trackedState.Status, "waiting"))
		}
		return nil
	}
	if arguments[0] == "enable" && config.Repository == "" {
		return fmt.Errorf("configure a repository first with al repo")
	}
	config.AutoSync.Enabled = arguments[0] == "enable"
	if err := saveConfig(config); err != nil {
		return err
	}
	if config.AutoSync.Enabled {
		if err := ensureWatchProcess(); err != nil {
			return err
		}
		fmt.Println("Automatic sync enabled.")
	} else {
		fmt.Println("Automatic sync disabled. The current worker will stop on its next check.")
	}
	return nil
}

func runWatch(daemon bool) error {
	lock, err := acquireSyncLock()
	if errors.Is(err, os.ErrExist) {
		if !daemon {
			fmt.Println("Alias Lens automatic sync is already running.")
		}
		return nil
	}
	if err != nil {
		return err
	}
	defer func() {
		lock.Close()
		os.Remove(lock.Name())
	}()
	for {
		config, err := loadConfig()
		if err != nil {
			writeSyncStatus("error", err.Error(), "", "")
			return err
		}
		if !config.AutoSync.Enabled || config.Repository == "" {
			writeSyncStatus("stopped", "automatic sync is disabled", "", "")
			return nil
		}
		cycleErr := reconcileAliases(config)
		if cycleErr == nil {
			for _, tracked := range config.TrackedFiles {
				if trackedErr := reconcileTrackedFile(config, tracked); trackedErr != nil {
					cycleErr = trackedErr
					break
				}
			}
		}
		if cycleErr != nil {
			state, _ := loadSyncState()
			if state.Status != "conflict" {
				writeSyncStatus("offline", cycleErr.Error(), state.LocalHash, state.RemoteHash)
			}
		}
		if !daemon {
			return cycleErr
		}
		os.Chtimes(lock.Name(), time.Now(), time.Now())
		time.Sleep(time.Duration(config.AutoSync.IntervalSeconds) * time.Second)
	}
}

func reconcileTrackedFile(config AppConfig, tracked TrackedFileConfig) error {
	if sensitiveConfigPath(tracked.Source) {
		return fmt.Errorf("refusing to sync credential-shaped file %s", tracked.Source)
	}
	target := filepath.Join(config.Repository, filepath.Clean(tracked.RepositoryPath))
	local, localErr := os.ReadFile(tracked.Source)
	remote, remoteErr := os.ReadFile(target)
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
		return writeSyncStateAt(statePath, SyncState{Status: "waiting", Message: "source and repository file do not exist", UpdatedAt: time.Now()})
	}
	if localMissing {
		if err := writePrivateFile(tracked.Source, remote); err != nil {
			return err
		}
		hash := contentHash(remote)
		return writeSyncStateAt(statePath, SyncState{LocalHash: hash, RemoteHash: hash, Status: "pulled", Message: "restored missing local file", UpdatedAt: time.Now()})
	}
	if remoteMissing {
		return pushTrackedFile(config, tracked, local, statePath)
	}
	localHash, remoteHash := contentHash(local), contentHash(remote)
	switch decideSyncAction(state, localHash, remoteHash) {
	case actionNoop:
		return writeSyncStateAt(statePath, SyncState{LocalHash: localHash, RemoteHash: remoteHash, Status: "synced", Message: "files match", UpdatedAt: time.Now()})
	case actionPush:
		return pushTrackedFile(config, tracked, local, statePath)
	case actionPull:
		if err := replaceTrackedFile(tracked.Source, remote); err != nil {
			return err
		}
		return writeSyncStateAt(statePath, SyncState{LocalHash: remoteHash, RemoteHash: remoteHash, Status: "pulled", Message: "applied remote update", UpdatedAt: time.Now()})
	case actionConflict:
		return saveTrackedConflict(tracked, local, remote, statePath)
	}
	return nil
}

func pushTrackedFile(config AppConfig, tracked TrackedFileConfig, contents []byte, statePath string) error {
	if err := secretFindingsError(findSecretFindings(contents)); err != nil {
		return fmt.Errorf("%s: %w", tracked.Source, err)
	}
	target := filepath.Join(config.Repository, filepath.Clean(tracked.RepositoryPath))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(target, contents, 0o600); err != nil {
		return err
	}
	if output, err := exec.Command("git", "-C", config.Repository, "add", "--", tracked.RepositoryPath).CombinedOutput(); err != nil {
		return fmt.Errorf("git add failed: %s", cleanCommandOutput(output))
	}
	message := "Update " + filepath.Base(tracked.RepositoryPath)
	if output, err := exec.Command("git", "-C", config.Repository, "commit", "--only", "-m", message, "--", tracked.RepositoryPath).CombinedOutput(); err != nil {
		return fmt.Errorf("git commit failed: %s", cleanCommandOutput(output))
	}
	if output, err := exec.Command("git", "-C", config.Repository, "push").CombinedOutput(); err != nil {
		return fmt.Errorf("git push failed: %s", cleanCommandOutput(output))
	}
	hash := contentHash(contents)
	return writeSyncStateAt(statePath, SyncState{LocalHash: hash, RemoteHash: hash, Status: "pushed", Message: "committed and pushed local update", UpdatedAt: time.Now()})
}

func replaceTrackedFile(path string, contents []byte) error {
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path+".alias-lens.bak", current, 0o600); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".alias-lens-sync-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(info.Mode().Perm()); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func writePrivateFile(path string, contents []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, contents, 0o600)
}

func trackedStatePath(tracked TrackedFileConfig) (string, error) {
	return syncDataPath("file-" + contentHash([]byte(tracked.Source + "\x00" + tracked.RepositoryPath))[:16] + ".json")
}

func saveTrackedConflict(tracked TrackedFileConfig, local, remote []byte, statePath string) error {
	id := contentHash([]byte(tracked.Source + "\x00" + tracked.RepositoryPath))[:16]
	directory, err := syncDataPath(filepath.Join("conflicts", id))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(directory, "local"), local, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(directory, "remote"), remote, 0o600); err != nil {
		return err
	}
	state := SyncState{LocalHash: contentHash(local), RemoteHash: contentHash(remote), Status: "conflict", Message: "both copies changed; files were not overwritten", UpdatedAt: time.Now()}
	if err := writeSyncStateAt(statePath, state); err != nil {
		return err
	}
	return fmt.Errorf("tracked file conflict: %s", tracked.Source)
}

func ensureWatchProcess() error {
	lockPath, err := syncDataPath("sync.lock")
	if err != nil {
		return err
	}
	if info, err := os.Stat(lockPath); err == nil && time.Since(info.ModTime()) < 2*time.Minute {
		return nil
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	logPath, err := syncDataPath("watch.log")
	if err != nil {
		return err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	command := exec.Command(executable, "watch", "--daemon")
	command.Stdin = nil
	command.Stdout = logFile
	command.Stderr = logFile
	if err := command.Start(); err != nil {
		logFile.Close()
		return err
	}
	logFile.Close()
	return command.Process.Release()
}

func reconcileAliases(config AppConfig) error {
	aliasPath, err := aliasesPath()
	if err != nil {
		return err
	}
	target := filepath.Join(config.Repository, filepath.Clean(config.AliasFile))
	if output, err := exec.Command("git", "-C", config.Repository, "pull", "--ff-only").CombinedOutput(); err != nil {
		return fmt.Errorf("pull deferred: %s", cleanCommandOutput(output))
	}
	local, localErr := os.ReadFile(aliasPath)
	remote, remoteErr := os.ReadFile(target)
	localMissing, remoteMissing := os.IsNotExist(localErr), os.IsNotExist(remoteErr)
	if localErr != nil && !localMissing {
		return localErr
	}
	if remoteErr != nil && !remoteMissing {
		return remoteErr
	}
	if localMissing {
		if remoteMissing {
			if err := os.WriteFile(aliasPath, nil, 0o600); err != nil {
				return err
			}
			local = nil
		} else {
			if err := writeNewAliasFile(aliasPath, remote); err != nil {
				return err
			}
			local = remote
		}
	}
	state, _ := loadSyncState()
	localHash, remoteHash := contentHash(local), contentHash(remote)
	if remoteMissing {
		return pushAliasSnapshot(config, aliasPath, local)
	}
	switch decideSyncAction(state, localHash, remoteHash) {
	case actionNoop:
		return writeSyncStatus("synced", "files match", localHash, remoteHash)
	case actionPush:
		return pushAliasSnapshot(config, aliasPath, local)
	case actionPull:
		if err := replaceAliasFile(aliasPath, remote); err != nil {
			return err
		}
		return writeSyncStatus("pulled", "applied remote alias update", remoteHash, remoteHash)
	case actionConflict:
		return saveSyncConflict(local, remote, "both local and remote aliases changed")
	}
	return nil
}

func pushAliasSnapshot(config AppConfig, aliasPath string, contents []byte) error {
	if err := secretFindingsError(findSecretFindings(contents)); err != nil {
		return err
	}
	if _, err := syncRepositoryFiles(config, aliasPath, true); err != nil {
		return err
	}
	hash := contentHash(contents)
	return writeSyncStatus("pushed", "committed and pushed local alias update", hash, hash)
}

func saveSyncConflict(local, remote []byte, message string) error {
	directory, err := syncDataPath("conflicts")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	suffix := strings.TrimPrefix(activeShellAdapter().AliasFilename(), ".")
	if err := os.WriteFile(filepath.Join(directory, "local."+suffix), local, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(directory, "remote."+suffix), remote, 0o600); err != nil {
		return err
	}
	writeSyncStatus("conflict", message+"; run al diff", contentHash(local), contentHash(remote))
	return fmt.Errorf("%s; live aliases were not overwritten", message)
}

func replaceAliasFile(path string, contents []byte) error {
	current, mode, _, err := readAliasFile(path)
	if err != nil {
		return err
	}
	return writeAliasFile(path, current, contents, mode)
}

func writeNewAliasFile(path string, contents []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, contents, 0o600)
}

func contentHash(contents []byte) string {
	digest := sha256.Sum256(contents)
	return hex.EncodeToString(digest[:])
}

func acquireSyncLock() (*os.File, error) {
	path, err := syncDataPath("sync.lock")
	if err != nil {
		return nil, err
	}
	lock, openErr := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if !errors.Is(openErr, os.ErrExist) {
		return lock, openErr
	}
	if info, statErr := os.Stat(path); statErr == nil && time.Since(info.ModTime()) > 2*time.Minute {
		if removeErr := os.Remove(path); removeErr == nil {
			return os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		}
	}
	return nil, openErr
}

func syncDataPath(name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	directory := filepath.Join(home, ".local", "state", "alias-lens")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", err
	}
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
	contents, err := os.ReadFile(path)
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
	path, err := syncDataPath("sync-state.json")
	if err != nil {
		return err
	}
	state := SyncState{LocalHash: localHash, RemoteHash: remoteHash, Status: status, Message: message, UpdatedAt: time.Now()}
	return writeSyncStateAt(path, state)
}

func writeSyncStateAt(path string, state SyncState) error {
	contents, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(contents, '\n'), 0o600)
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
