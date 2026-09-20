// Package transaction provides crash-aware primitives for Alias Lens-owned
// private-file mutations. It does not choose which product operation to run.
package transaction

import (
	"errors"
	"fmt"
	"regexp"
)

const (
	SchemaVersion      = 1
	MaxActions         = 20_000
	MaxFrameBytes      = 8 << 20
	MaxJournalBytes    = 32 << 20
	MaxPrivateFileSize = 8 << 20
)

var (
	ErrLocked           = errors.New("another Alias Lens mutation holds the lock")
	ErrUnsafePath       = errors.New("private path is unsafe")
	ErrIdentityChanged  = errors.New("file identity changed")
	ErrJournalCorrupt   = errors.New("transaction journal is corrupt")
	ErrRecoveryBlocked  = errors.New("transaction recovery is blocked")
	operationIDPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)
	lowerHexHashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// FileIdentity records the facts needed to detect a replaced private file.
// PlatformID is device and inode on Unix. Platforms without a stable file ID
// leave PlatformID empty and use the remaining conservative checks.
type FileIdentity struct {
	Exists     bool   `json:"exists"`
	PlatformID string `json:"platform_id,omitempty"`
	Size       int64  `json:"size,omitempty"`
	Mode       uint32 `json:"mode,omitempty"`
	Owner      uint64 `json:"owner,omitempty"`
	Group      uint64 `json:"group,omitempty"`
	Links      uint64 `json:"links,omitempty"`
	SHA256     string `json:"sha256,omitempty"`
}

// Action describes one Alias Lens-owned private-file replacement. All paths
// must be below Manifest.PrivateRoot. TemporaryPath and BackupPath identify
// operation-owned artifacts before the first target mutation.
type Action struct {
	Sequence      int          `json:"sequence"`
	TargetPath    string       `json:"target_path"`
	TemporaryPath string       `json:"temporary_path"`
	BackupPath    string       `json:"backup_path,omitempty"`
	Expected      FileIdentity `json:"expected"`
	PlannedSHA256 string       `json:"planned_sha256"`
	Mode          uint32       `json:"mode"`
	Remove        bool         `json:"remove,omitempty"`
}

// Manifest is the immutable first journal frame.
type Manifest struct {
	SchemaVersion int      `json:"schema_version"`
	OperationID   string   `json:"operation_id"`
	PrivateRoot   string   `json:"private_root"`
	Actions       []Action `json:"actions"`
}

type RecordKind string

const (
	RecordBackupSynced  RecordKind = "backup_synced"
	RecordTempSynced    RecordKind = "temporary_synced"
	RecordTargetRenamed RecordKind = "target_renamed"
	RecordTargetSynced  RecordKind = "target_directory_synced"
	RecordCommitted     RecordKind = "committed"
)

// Record describes a completed write or fsync boundary. Sequence is zero only
// for a transaction-wide record such as committed.
type Record struct {
	Ordinal      int        `json:"ordinal"`
	Sequence     int        `json:"sequence"`
	Kind         RecordKind `json:"kind"`
	TargetSHA256 string     `json:"target_sha256,omitempty"`
}

type TargetState string

const (
	TargetOld       TargetState = "old"
	TargetNew       TargetState = "new"
	TargetAmbiguous TargetState = "ambiguous"
)

type RecoveryState string

const (
	RecoveryOld       RecoveryState = "old"
	RecoveryNew       RecoveryState = "new"
	RecoveryMixed     RecoveryState = "mixed"
	RecoveryAmbiguous RecoveryState = "ambiguous"
)

type ActionRecovery struct {
	Sequence int         `json:"sequence"`
	State    TargetState `json:"state"`
	Reason   string      `json:"reason,omitempty"`
}

// RecoveryAnalysis is observational. AnalyzeRecovery does not remove an
// artifact, restore a backup, acquire a lock, or append to the journal.
type RecoveryAnalysis struct {
	State         RecoveryState    `json:"state"`
	Actions       []ActionRecovery `json:"actions"`
	Committed     bool             `json:"committed"`
	TruncatedTail bool             `json:"truncated_tail"`
}

type Boundary string

const (
	BoundaryManifestSynced Boundary = "manifest_synced"
	BoundaryBackupSynced   Boundary = "backup_synced"
	BoundaryTempSynced     Boundary = "temporary_synced"
	BoundaryBeforeRename   Boundary = "before_rename"
	BoundaryAfterRename    Boundary = "after_rename"
	BoundaryTargetSynced   Boundary = "target_directory_synced"
	BoundaryCommitted      Boundary = "committed"
)

func validateHash(value, field string) error {
	if !lowerHexHashPattern.MatchString(value) {
		return fmt.Errorf("%s must be a lowercase SHA-256 digest", field)
	}
	return nil
}
