package transaction

import "os"

type WorkflowRole string

const (
	WorkflowPrivate    WorkflowRole = "private"
	WorkflowUser       WorkflowRole = "user"
	WorkflowRepository WorkflowRole = "repository"
)

type WorkflowTarget struct {
	Path              string       `json:"path"`
	Role              WorkflowRole `json:"role"`
	Expected          FileIdentity `json:"expected"`
	Planned           []byte       `json:"-"`
	Mode              os.FileMode  `json:"mode"`
	Remove            bool         `json:"remove,omitempty"`
	RecoveryOrder     int          `json:"recovery_order"`
	ExpectedLink      string       `json:"expected_link,omitempty"`
	PreserveSymlink   bool         `json:"preserve_symlink,omitempty"`
	Directory         bool         `json:"directory,omitempty"`
	PromotionExpected FileIdentity `json:"promotion_expected"`
	PromotionSource   string       `json:"promotion_source,omitempty"`
}
type WorkflowSpec struct {
	OperationID  string           `json:"operation_id"`
	StateRoot    string           `json:"state_root"`
	PrivateRoots []string         `json:"private_roots"`
	Targets      []WorkflowTarget `json:"targets"`
}
type WorkflowAction struct {
	Target            WorkflowTarget `json:"target"`
	ResolvedPath      string         `json:"resolved_path"`
	Link              string         `json:"link,omitempty"`
	ParentIdentity    string         `json:"parent_identity"`
	BackupPath        string         `json:"backup_path,omitempty"`
	TemporaryName     string         `json:"temporary_name"`
	PlannedSHA256     string         `json:"planned_sha256"`
	PromotionIdentity FileIdentity   `json:"promotion_identity"`
}
type WorkflowManifest struct {
	Version int              `json:"version"`
	Spec    WorkflowSpec     `json:"spec"`
	Actions []WorkflowAction `json:"actions"`
}
type WorkflowProgress struct {
	Stage    string        `json:"stage"`
	Target   int           `json:"target"`
	Identity *FileIdentity `json:"identity,omitempty"`
}
type WorkflowBoundary = WorkflowProgress
