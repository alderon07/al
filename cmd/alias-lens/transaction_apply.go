package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	workflowplan "alias-lens/internal/plan"
	"alias-lens/internal/transaction"
)

type planBuilder func() (workflowplan.OperationPlan, error)

func applyPrivatePlan(root string, preview workflowplan.OperationPlan, rebuild planBuilder) error {
	if preview.Summary.Blocked {
		return fmt.Errorf("this change is blocked; review the plan diagnostics")
	}
	if len(preview.Actions) == 0 {
		return nil
	}
	if err := ensurePrivateDirectory(root); err != nil {
		return err
	}
	transactionsPath := filepath.Join(root, "transactions")
	temporaryPath := filepath.Join(root, "temporary")
	backupsPath := filepath.Join(root, "backups")
	for _, path := range []string{transactionsPath, temporaryPath, backupsPath} {
		if err := ensurePrivateDirectory(path); err != nil {
			return err
		}
	}
	lock, err := transaction.AcquireLock(root, filepath.Join(root, "mutation.lock"))
	if err != nil {
		if errors.Is(err, transaction.ErrLocked) {
			return fmt.Errorf("another Alias Lens change is still running; try again when it finishes")
		}
		return err
	}
	defer lock.Close()
	if err := recoverPrivateTransactions(root, transactionsPath); err != nil {
		return fmt.Errorf("Alias Lens could not safely recover an earlier change: %w", err)
	}
	fresh, err := rebuild()
	if err != nil {
		return err
	}
	if err := workflowplan.CheckFresh(preview, fresh); err != nil {
		return fmt.Errorf("the files changed after the preview; review a new plan before trying again")
	}
	operationID, err := newOperationID()
	if err != nil {
		return err
	}
	manifest := transaction.Manifest{SchemaVersion: transaction.SchemaVersion, OperationID: operationID, PrivateRoot: root}
	for index, action := range fresh.Actions {
		if action.Kind != workflowplan.ActionCreate && action.Kind != workflowplan.ActionReplace && action.Kind != workflowplan.ActionRemove {
			return fmt.Errorf("the safe writer does not support action %q yet", action.Kind)
		}
		targetPath := action.Target.Path
		if filepath.Dir(targetPath) != root {
			return fmt.Errorf("planned target is outside the private settings directory")
		}
		identity, err := transaction.InspectPrivateFile(root, targetPath, transaction.MaxPrivateFileSize)
		if err != nil {
			return err
		}
		if !plannedIdentityMatches(action.Target.ExpectedIdentity, identity) || identity.Exists && identity.SHA256 != action.Target.ExpectedSHA256 {
			return fmt.Errorf("the files changed after the preview; review a new plan before trying again")
		}
		if !identity.Exists && action.Kind != workflowplan.ActionCreate {
			return fmt.Errorf("a file disappeared after the preview; no changes were made")
		}
		transactionAction := transaction.Action{
			Sequence: index + 1, TargetPath: targetPath,
			TemporaryPath: filepath.Join(temporaryPath, fmt.Sprintf("%s-%d.tmp", operationID, index+1)),
			Expected:      identity, PlannedSHA256: action.PlannedSHA256, Mode: 0o600, Remove: action.Kind == workflowplan.ActionRemove,
		}
		if identity.Exists {
			transactionAction.BackupPath = filepath.Join(backupsPath, fmt.Sprintf("%s-%d.bak", operationID, index+1))
		}
		manifest.Actions = append(manifest.Actions, transactionAction)
	}
	journalPath := filepath.Join(transactionsPath, operationID+".journal")
	journal, err := transaction.CreateJournal(root, journalPath, manifest)
	if err != nil {
		return err
	}
	defer journal.Close()
	rollback := func(cause error) error {
		if rollbackErr := journal.Rollback(); rollbackErr != nil {
			return fmt.Errorf("%v; automatic rollback also needs attention: %w", cause, rollbackErr)
		}
		return cause
	}
	for index, action := range fresh.Actions {
		var actionErr error
		if action.Kind == workflowplan.ActionRemove {
			actionErr = journal.RemovePrivate(index + 1)
		} else {
			actionErr = journal.ReplacePrivate(index+1, action.Target.PlannedBytes)
		}
		if actionErr != nil {
			return rollback(actionErr)
		}
	}
	if err := journal.Commit(); err != nil {
		return rollback(err)
	}
	if err := journal.Finalize(); err != nil {
		return fmt.Errorf("the change was saved, but cleanup needs attention: %w", err)
	}
	return nil
}

func plannedIdentityMatches(expected workflowplan.Identity, actual transaction.FileIdentity) bool {
	if expected.FileType == "missing" {
		return !actual.Exists
	}
	if expected.FileType != "regular" || !actual.Exists {
		return false
	}
	if expected.Device != 0 || expected.Inode != 0 {
		platformID := fmt.Sprintf("%d:%d", expected.Device, expected.Inode)
		return platformID == actual.PlatformID && expected.Mode == actual.Mode && expected.Owner == actual.Owner &&
			expected.Group == actual.Group && expected.LinkCount == actual.Links
	}
	return expected.Mode == actual.Mode && (expected.LinkCount == 0 || expected.LinkCount == actual.Links)
}

func ensurePrivateDirectory(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return err
		}
		info, err = os.Lstat(path)
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm() != 0o700 {
		return fmt.Errorf("Alias Lens cannot use %s because it is not a private directory with mode 0700", displayPrivatePath(path))
	}
	return nil
}

func recoverPrivateTransactions(root, directory string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".journal" {
			continue
		}
		if err := transaction.RecoverJournal(root, filepath.Join(directory, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func newOperationID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("create operation ID: %w", err)
	}
	return "op-" + hex.EncodeToString(buffer), nil
}
