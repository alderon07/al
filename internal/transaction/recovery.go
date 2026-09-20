package transaction

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// AnalyzeRecovery classifies on-disk targets without changing them. It trusts
// only complete checksummed journal frames and descriptor-derived file hashes.
func AnalyzeRecovery(root, journalPath string) (RecoveryAnalysis, error) {
	manifest, records, truncated, err := ReadJournal(root, journalPath)
	if err != nil {
		return RecoveryAnalysis{}, err
	}
	analysis := RecoveryAnalysis{
		Actions:       make([]ActionRecovery, 0, len(manifest.Actions)),
		TruncatedTail: truncated,
	}
	for _, record := range records {
		if record.Kind == RecordCommitted {
			analysis.Committed = true
		}
	}

	oldCount, newCount := 0, 0
	for _, action := range manifest.Actions {
		result := ActionRecovery{Sequence: action.Sequence}
		identity, inspectErr := InspectPrivateFile(manifest.PrivateRoot, action.TargetPath, MaxPrivateFileSize)
		if inspectErr != nil {
			result.State = TargetAmbiguous
			result.Reason = "target could not be inspected safely"
			analysis.Actions = append(analysis.Actions, result)
			continue
		}
		switch {
		case sameIdentity(action.Expected, identity):
			result.State = TargetOld
			oldCount++
		case action.Remove && !identity.Exists:
			backup, backupErr := InspectPrivateFile(manifest.PrivateRoot, action.BackupPath, MaxPrivateFileSize)
			if backupErr != nil || !backup.Exists || backup.SHA256 != action.Expected.SHA256 || backup.Mode != 0o600 {
				result.State = TargetAmbiguous
				result.Reason = "removed target has no usable private backup"
				analysis.Actions = append(analysis.Actions, result)
				continue
			}
			result.State = TargetNew
			newCount++
		case !action.Remove && identity.Exists && identity.SHA256 == action.PlannedSHA256 && identity.Mode == action.Mode:
			if action.Expected.Exists {
				backup, backupErr := InspectPrivateFile(manifest.PrivateRoot, action.BackupPath, MaxPrivateFileSize)
				if backupErr != nil || !backup.Exists || backup.SHA256 != action.Expected.SHA256 || backup.Mode != 0o600 {
					result.State = TargetAmbiguous
					result.Reason = "planned target exists but its private backup is unavailable"
					analysis.Actions = append(analysis.Actions, result)
					continue
				}
			}
			result.State = TargetNew
			newCount++
		default:
			result.State = TargetAmbiguous
			result.Reason = "target matches neither the recorded old identity nor the planned content"
		}
		analysis.Actions = append(analysis.Actions, result)
	}

	if oldCount+newCount != len(manifest.Actions) {
		analysis.State = RecoveryAmbiguous
	} else if analysis.Committed && newCount != len(manifest.Actions) {
		analysis.State = RecoveryAmbiguous
		for index := range analysis.Actions {
			if analysis.Actions[index].State == TargetOld {
				analysis.Actions[index].Reason = "committed journal does not match the planned target"
			}
		}
	} else if oldCount == len(manifest.Actions) {
		analysis.State = RecoveryOld
	} else if newCount == len(manifest.Actions) {
		analysis.State = RecoveryNew
	} else {
		analysis.State = RecoveryMixed
	}
	return analysis, nil
}

// RecoverJournal completes mandatory recovery for one journal. A committed new
// state is finalized. Every other provable state is restored to the old state.
// Ambiguous files are left untouched.
func RecoverJournal(root, journalPath string) error {
	analysis, err := AnalyzeRecovery(root, journalPath)
	if err != nil {
		return err
	}
	if err := RequireRecoverable(analysis); err != nil {
		return err
	}
	journal, err := OpenJournal(root, journalPath)
	if err != nil {
		return err
	}
	if analysis.State == RecoveryNew && analysis.Committed {
		return journal.Finalize()
	}
	if analysis.State == RecoveryOld {
		manifest := journal.Manifest()
		if err := journal.Close(); err != nil {
			return err
		}
		for _, action := range manifest.Actions {
			identity, inspectErr := InspectPrivateFile(root, action.TemporaryPath, MaxPrivateFileSize)
			if inspectErr != nil {
				return inspectErr
			}
			if identity.Exists {
				if identity.SHA256 != action.PlannedSHA256 {
					return ErrRecoveryBlocked
				}
				if removeErr := os.Remove(action.TemporaryPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
					return removeErr
				}
			}
		}
		if err := os.Remove(journalPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return syncDirectory(filepath.Dir(journalPath))
	}
	return journal.Rollback()
}

// RequireRecoverable converts an ambiguous recovery result into the package's
// stable blocked error. Callers can use the full analysis for a private manual
// recovery report.
func RequireRecoverable(analysis RecoveryAnalysis) error {
	if analysis.State == RecoveryAmbiguous {
		return fmt.Errorf("%w: transaction targets cannot be proven old or new", ErrRecoveryBlocked)
	}
	return nil
}
