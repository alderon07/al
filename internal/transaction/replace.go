package transaction

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ReplacePrivate applies one manifest action. It never creates parent
// directories. The caller must hold the shared mutation lock for the complete
// operation and must call Commit only after every action succeeds.
func (journal *Journal) ReplacePrivate(sequence int, contents []byte) error {
	action, err := journal.action(sequence)
	if err != nil {
		return err
	}
	if len(contents) > MaxPrivateFileSize {
		return fmt.Errorf("planned private file exceeds %d bytes", MaxPrivateFileSize)
	}
	if hashBytes(contents) != action.PlannedSHA256 {
		return fmt.Errorf("planned bytes do not match action %d hash", sequence)
	}

	current, err := InspectPrivateFile(journal.root, action.TargetPath, MaxPrivateFileSize)
	if err != nil {
		return err
	}
	if current.Exists && current.SHA256 == action.PlannedSHA256 && current.Mode == action.Mode {
		return fmt.Errorf("action %d target already contains planned bytes; inspect journal recovery state", sequence)
	}
	if !sameIdentity(action.Expected, current) {
		return fmt.Errorf("action %d: %w", sequence, ErrIdentityChanged)
	}

	if action.Expected.Exists {
		if err := journal.ensureBackup(action); err != nil {
			return err
		}
		if err := journal.appendRecord(Record{Sequence: sequence, Kind: RecordBackupSynced}); err != nil {
			return err
		}
		if err := journal.callBoundary(BoundaryBackupSynced); err != nil {
			return err
		}
	}
	if err := writeNewPrivateFile(journal.root, action.TemporaryPath, contents); err != nil {
		return fmt.Errorf("write action %d temporary file: %w", sequence, err)
	}
	if err := journal.appendRecord(Record{Sequence: sequence, Kind: RecordTempSynced}); err != nil {
		return err
	}
	if err := journal.callBoundary(BoundaryTempSynced); err != nil {
		return err
	}

	current, err = InspectPrivateFile(journal.root, action.TargetPath, MaxPrivateFileSize)
	if err != nil {
		return err
	}
	if !sameIdentity(action.Expected, current) {
		return fmt.Errorf("action %d before rename: %w", sequence, ErrIdentityChanged)
	}
	if err := journal.callBoundary(BoundaryBeforeRename); err != nil {
		return err
	}
	if err := os.Rename(action.TemporaryPath, action.TargetPath); err != nil {
		return fmt.Errorf("replace action %d target: %w", sequence, err)
	}
	if err := journal.callBoundary(BoundaryAfterRename); err != nil {
		return err
	}
	if err := journal.appendRecord(Record{Sequence: sequence, Kind: RecordTargetRenamed, TargetSHA256: action.PlannedSHA256}); err != nil {
		return err
	}
	if err := syncDirectory(filepath.Dir(action.TargetPath)); err != nil {
		return fmt.Errorf("sync action %d target directory: %w", sequence, err)
	}
	if err := journal.appendRecord(Record{Sequence: sequence, Kind: RecordTargetSynced, TargetSHA256: action.PlannedSHA256}); err != nil {
		return err
	}
	if err := journal.callBoundary(BoundaryTargetSynced); err != nil {
		return err
	}
	return nil
}

// RemovePrivate removes one proven private file after saving and syncing its
// exact bytes. Rollback restores the backup if the transaction does not commit.
func (journal *Journal) RemovePrivate(sequence int) error {
	action, err := journal.action(sequence)
	if err != nil {
		return err
	}
	if !action.Remove || !action.Expected.Exists {
		return fmt.Errorf("action %d is not a removal", sequence)
	}
	current, err := InspectPrivateFile(journal.root, action.TargetPath, MaxPrivateFileSize)
	if err != nil {
		return err
	}
	if !sameIdentity(action.Expected, current) {
		return fmt.Errorf("action %d: %w", sequence, ErrIdentityChanged)
	}
	if err := journal.ensureBackup(action); err != nil {
		return err
	}
	if err := journal.appendRecord(Record{Sequence: sequence, Kind: RecordBackupSynced}); err != nil {
		return err
	}
	if err := journal.callBoundary(BoundaryBackupSynced); err != nil {
		return err
	}
	current, err = InspectPrivateFile(journal.root, action.TargetPath, MaxPrivateFileSize)
	if err != nil {
		return err
	}
	if !sameIdentity(action.Expected, current) {
		return fmt.Errorf("action %d before removal: %w", sequence, ErrIdentityChanged)
	}
	if err := journal.callBoundary(BoundaryBeforeRename); err != nil {
		return err
	}
	if err := os.Remove(action.TargetPath); err != nil {
		return fmt.Errorf("remove action %d target: %w", sequence, err)
	}
	if err := journal.callBoundary(BoundaryAfterRename); err != nil {
		return err
	}
	if err := journal.appendRecord(Record{Sequence: sequence, Kind: RecordTargetRenamed, TargetSHA256: action.PlannedSHA256}); err != nil {
		return err
	}
	if err := syncDirectory(filepath.Dir(action.TargetPath)); err != nil {
		return err
	}
	if err := journal.appendRecord(Record{Sequence: sequence, Kind: RecordTargetSynced, TargetSHA256: action.PlannedSHA256}); err != nil {
		return err
	}
	return journal.callBoundary(BoundaryTargetSynced)
}

func (journal *Journal) ensureBackup(action Action) error {
	backup, err := InspectPrivateFile(journal.root, action.BackupPath, MaxPrivateFileSize)
	if err != nil {
		return err
	}
	if backup.Exists {
		if backup.SHA256 == action.Expected.SHA256 && backup.Mode == 0o600 {
			return nil
		}
		return fmt.Errorf("action %d backup already exists with unexpected contents", action.Sequence)
	}

	source, err := openRegularNoFollow(action.TargetPath)
	if err != nil {
		return fmt.Errorf("open action %d backup source: %w", action.Sequence, err)
	}
	defer source.Close()
	before, err := source.Stat()
	if err != nil {
		return err
	}
	beforeIdentity, err := identityFromInfo(before)
	if err != nil {
		return err
	}
	if err := validatePrivateFileInfo(before, beforeIdentity, MaxPrivateFileSize); err != nil {
		return err
	}
	if beforeIdentity.PlatformID != action.Expected.PlatformID || before.Size() != action.Expected.Size {
		return ErrIdentityChanged
	}

	backupFile, err := createPrivateNoFollow(action.BackupPath)
	if err != nil {
		return err
	}
	copyErr := func() error {
		if _, err := io.Copy(backupFile, io.LimitReader(source, MaxPrivateFileSize+1)); err != nil {
			return err
		}
		if err := backupFile.Sync(); err != nil {
			return err
		}
		return backupFile.Close()
	}()
	if copyErr != nil {
		_ = backupFile.Close()
		return copyErr
	}
	if err := syncDirectory(filepath.Dir(action.BackupPath)); err != nil {
		return err
	}
	afterSource, err := source.Stat()
	if err != nil {
		return err
	}
	afterIdentity, err := identityFromInfo(afterSource)
	if err != nil {
		return err
	}
	if !sameOpenIdentity(beforeIdentity, afterIdentity) || before.Size() != afterSource.Size() {
		return ErrIdentityChanged
	}
	backup, err = InspectPrivateFile(journal.root, action.BackupPath, MaxPrivateFileSize)
	if err != nil {
		return err
	}
	if backup.SHA256 != action.Expected.SHA256 {
		return fmt.Errorf("action %d backup hash mismatch", action.Sequence)
	}
	return nil
}

func writeNewPrivateFile(root, path string, contents []byte) error {
	path, err := validatePath(root, path, true)
	if err != nil {
		return err
	}
	file, err := createPrivateNoFollow(path)
	if err != nil {
		return err
	}
	writeErr := writeAll(file, contents)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	identity, err := InspectPrivateFile(root, path, MaxPrivateFileSize)
	if err != nil {
		return err
	}
	if !identity.Exists || identity.SHA256 != hashBytes(contents) {
		return fmt.Errorf("private file verification failed")
	}
	return nil
}

func (journal *Journal) Commit() error {
	if journal == nil || journal.file == nil {
		return fmt.Errorf("transaction journal is closed")
	}
	for _, action := range journal.manifest.Actions {
		identity, err := InspectPrivateFile(journal.root, action.TargetPath, MaxPrivateFileSize)
		if err != nil {
			return err
		}
		if action.Remove {
			if identity.Exists {
				return fmt.Errorf("action %d target was not removed", action.Sequence)
			}
			continue
		}
		if !identity.Exists || identity.SHA256 != action.PlannedSHA256 || identity.Mode != action.Mode {
			return fmt.Errorf("action %d target does not match the plan", action.Sequence)
		}
	}
	if err := journal.appendRecord(Record{Sequence: 0, Kind: RecordCommitted}); err != nil {
		return err
	}
	if err := journal.callBoundary(BoundaryCommitted); err != nil {
		return err
	}
	return nil
}

// Rollback restores the exact pre-transaction private files when every target
// can be proven old or planned-new. It refuses an ambiguous target.
func (journal *Journal) Rollback() error {
	if journal == nil {
		return nil
	}
	for index := len(journal.manifest.Actions) - 1; index >= 0; index-- {
		action := journal.manifest.Actions[index]
		identity, err := InspectPrivateFile(journal.root, action.TargetPath, MaxPrivateFileSize)
		if err != nil {
			return err
		}
		if sameIdentity(action.Expected, identity) {
			continue
		}
		plannedState := action.Remove && !identity.Exists || !action.Remove && identity.Exists && identity.SHA256 == action.PlannedSHA256 && identity.Mode == action.Mode
		if !plannedState {
			return fmt.Errorf("action %d: %w", action.Sequence, ErrRecoveryBlocked)
		}
		if !action.Expected.Exists {
			if err := os.Remove(action.TargetPath); err != nil {
				return err
			}
			if err := syncDirectory(filepath.Dir(action.TargetPath)); err != nil {
				return err
			}
			continue
		}
		backup, err := InspectPrivateFile(journal.root, action.BackupPath, MaxPrivateFileSize)
		if err != nil || !backup.Exists || backup.SHA256 != action.Expected.SHA256 {
			return fmt.Errorf("action %d backup: %w", action.Sequence, ErrRecoveryBlocked)
		}
		contents, err := readPrivateBytes(action.BackupPath, MaxPrivateFileSize)
		if err != nil {
			return err
		}
		if err := writeNewPrivateFile(journal.root, action.TemporaryPath, contents); err != nil {
			return err
		}
		if err := os.Rename(action.TemporaryPath, action.TargetPath); err != nil {
			return err
		}
		if err := syncDirectory(filepath.Dir(action.TargetPath)); err != nil {
			return err
		}
	}
	if err := journal.Close(); err != nil {
		return err
	}
	if err := os.Remove(journal.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return syncDirectory(filepath.Dir(journal.path))
}

func readPrivateBytes(path string, limit int64) ([]byte, error) {
	file, err := openRegularNoFollow(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	contents, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(contents)) > limit {
		return nil, fmt.Errorf("private file exceeds %d bytes", limit)
	}
	return contents, nil
}

func removeOwnedTemporary(root, path, expectedHash string) error {
	identity, err := InspectPrivateFile(root, path, MaxPrivateFileSize)
	if err != nil {
		return err
	}
	if !identity.Exists {
		return nil
	}
	if expectedHash != "" && identity.SHA256 != expectedHash {
		return ErrRecoveryBlocked
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}
