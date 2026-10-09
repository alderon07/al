//go:build !windows

package transaction

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func RecoverWorkflows(stateRoot string, hooks ...func(WorkflowBoundary) error) error {
	return RecoverWorkflowsWithPolicy(stateRoot, nil, hooks...)
}

func RecoverWorkflowsWithPolicy(stateRoot string, policy WorkflowRecoveryPolicy, hooks ...func(WorkflowBoundary) error) error {
	if _, err := validatePrivateRoot(stateRoot); err != nil {
		return workflowRecoveryError(-1, err)
	}
	dir := filepath.Join(stateRoot, "workflows")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return workflowRecoveryError(-1, err)
	}
	if _, err = validatePrivateRoot(dir); err != nil {
		return workflowRecoveryError(-1, err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".workflow") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		m, p, err := ReadWorkflowJournal(stateRoot, path)
		if err != nil {
			return workflowRecoveryError(-1, err)
		}
		d := workflowDisk{m, p}
		committed := false
		recovered := false
		for _, r := range p {
			if r.Stage == "recovered" {
				recovered = true
			}
			if r.Stage == "committed" {
				committed = true
			}
		}
		if committed || recovered {
			if err = validateRetainedWorkflow(d, policy); err != nil {
				return workflowRecoveryError(0, err)
			}
			if err = finishWorkflow(path, d); err != nil {
				return workflowRecoveryError(-1, err)
			}
			continue
		}
		if len(p) == 0 || p[len(p)-1].Stage != "recovery_started" {
			if err = workflowRecord(path, &d, "recovery_started", -1, hooks); err != nil {
				return workflowRecoveryError(-1, err)
			}
		}
		retained, err := retainWorkflow(path, &d, policy, hooks)
		if err != nil {
			return workflowRecoveryError(0, err)
		}
		if retained {
			if err = workflowRecord(path, &d, "recovered", -1, hooks); err != nil {
				return workflowRecoveryError(-1, err)
			}
			if err = validateRetainedWorkflow(d, policy); err != nil {
				return workflowRecoveryError(0, err)
			}
			if err = finishWorkflow(path, d); err != nil {
				return workflowRecoveryError(-1, err)
			}
			continue
		}
		indices := make([]int, len(m.Actions))
		for i := range indices {
			indices[i] = i
		}
		sort.SliceStable(indices, func(i, j int) bool {
			return m.Actions[indices[i]].Target.RecoveryOrder < m.Actions[indices[j]].Target.RecoveryOrder
		})
		for _, i := range indices {
			for _, oldProgress := range p {
				if m.Actions[i].Target.Directory && oldProgress.Stage == "target_synced" && oldProgress.Target == i {
					if _, e := os.Lstat(m.Actions[i].Target.Path); e == nil {
						now, e := workflowActionIdentity(m.Actions[i], false)
						if e != nil || oldProgress.Identity == nil || !sameOpenIdentity(*oldProgress.Identity, now) {
							return workflowRecoveryError(i, ErrIdentityChanged)
						}
					}
				}
				if oldProgress.Stage == "recovery_target_synced" && oldProgress.Target == i {
					now, e := workflowActionIdentity(m.Actions[i], true)
					if e != nil || oldProgress.Identity == nil || !sameIdentity(*oldProgress.Identity, now) {
						return workflowRecoveryError(i, ErrIdentityChanged)
					}
				}
			}
			if err = recoverWorkflowAction(m.Actions[i], hooks, i); err != nil {
				return workflowRecoveryError(i, err)
			}
			if err = workflowRecord(path, &d, "recovery_target_synced", i, hooks); err != nil {
				return workflowRecoveryError(i, err)
			}
		}
		if err = workflowRecord(path, &d, "recovered", -1, hooks); err != nil {
			return workflowRecoveryError(-1, err)
		}
		if err = finishWorkflow(path, d); err != nil {
			return workflowRecoveryError(-1, err)
		}
	}
	return nil
}
func workflowContentMatches(a, b FileIdentity) bool {
	return a.Exists == b.Exists && (!a.Exists || a.Size == b.Size && a.SHA256 == b.SHA256 && a.Mode == b.Mode && a.Owner == b.Owner && a.Group == b.Group && b.Links == 1)
}
func recoverWorkflowAction(a WorkflowAction, h []func(WorkflowBoundary) error, i int) error {
	if !a.Target.Expected.Exists && a.Target.PromotionSource == "" {
		if _, e := os.Lstat(a.ResolvedPath); errors.Is(e, os.ErrNotExist) {
			return nil
		}
	}
	p, err := checkWorkflowAction(a)
	if err != nil {
		return err
	}
	defer p.close()
	if a.Target.Directory {
		info, e := os.Lstat(a.ResolvedPath)
		if errors.Is(e, os.ErrNotExist) {
			return nil
		}
		if e != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
			return ErrRecoveryBlocked
		}
		if e = workflowDirectoryMetadata(a.ResolvedPath); e != nil {
			return e
		}
		if e = removeWorkflowDirectory(p); e != nil {
			return e
		}
		return workflowHook(h, WorkflowBoundary{Stage: "recovery_after_rename", Target: i})
	}
	if a.Target.PromotionSource != "" {
		id, e := inspectWorkflowDirectory(a.Target.Path)
		if errors.Is(e, os.ErrNotExist) {
			source, se := inspectWorkflowDirectory(a.Target.PromotionSource)
			if se != nil || !sameIdentity(source, a.PromotionIdentity) {
				return ErrRecoveryBlocked
			}
			return nil
		}
		if e != nil || !sameIdentity(id, a.PromotionIdentity) {
			return ErrRecoveryBlocked
		}
		if _, e = os.Lstat(a.Target.PromotionSource); !errors.Is(e, os.ErrNotExist) {
			return ErrRecoveryBlocked
		}
		if err = promoteWorkflowDirectory(a.Target.Path, a.Target.PromotionSource); err != nil {
			return err
		}
		return workflowHook(h, WorkflowBoundary{Stage: "recovery_after_rename", Target: i})
	}
	id, _, err := inspectWorkflowFile(a.ResolvedPath, MaxPrivateFileSize)
	if err != nil {
		return err
	}
	if workflowContentMatches(a.Target.Expected, id) {
		if err = removeWorkflowTemp(p, a.TemporaryName); err != nil {
			return err
		}
		return p.file.Sync()
	}
	planned := !a.Target.Remove && id.Exists && id.SHA256 == a.PlannedSHA256 && id.Mode == uint32(a.Target.Mode)
	if planned && a.Target.Expected.Exists && (id.Owner != a.Target.Expected.Owner || id.Group != a.Target.Expected.Group) {
		return ErrRecoveryBlocked
	}
	if a.Target.Remove {
		planned = !id.Exists
	}
	if !planned {
		return errWorkflowContentChanged
	}
	if !a.Target.Expected.Exists {
		if err = p.remove(); err != nil {
			return err
		}
	} else {
		backup, b, e := inspectWorkflowFile(a.BackupPath, MaxPrivateFileSize)
		if e != nil || !backup.Exists || backup.SHA256 != a.Target.Expected.SHA256 {
			return errWorkflowBackupUnavailable
		}
		if err = removeWorkflowTemp(p, a.TemporaryName); err != nil {
			return err
		}
		old := a.Target.Expected
		if err = writeWorkflowTemp(p, a.TemporaryName, b, old.Mode, old.Owner, old.Group); err != nil {
			return err
		}
		if err = workflowHook(h, WorkflowBoundary{Stage: "recovery_temporary_synced", Target: i}); err != nil {
			return err
		}
		parentCheck, e := checkWorkflowAction(a)
		if e != nil {
			return e
		}
		parentCheck.close()
		current, _, e := inspectWorkflowFile(a.ResolvedPath, MaxPrivateFileSize)
		if e != nil || !sameIdentity(current, id) {
			return ErrIdentityChanged
		}
		if err = p.rename(a.TemporaryName); err != nil {
			return err
		}
	}
	if err = workflowHook(h, WorkflowBoundary{Stage: "recovery_after_rename", Target: i}); err != nil {
		return err
	}
	if err = p.file.Sync(); err != nil {
		return err
	}
	return removeWorkflowTemp(p, a.TemporaryName)
}
func finishWorkflow(path string, d workflowDisk) error {
	for _, a := range d.Manifest.Actions {
		if _, e := os.Lstat(filepath.Dir(a.ResolvedPath)); errors.Is(e, os.ErrNotExist) {
			continue
		}
		p, err := checkWorkflowAction(a)
		if err != nil {
			return err
		}
		err = removeWorkflowTemp(p, a.TemporaryName)
		p.close()
		if err != nil {
			return err
		}
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}
func workflowActionIdentity(a WorkflowAction, recovered bool) (FileIdentity, error) {
	if a.Target.PromotionSource != "" {
		path := a.Target.Path
		if recovered {
			path = a.Target.PromotionSource
		}
		return inspectWorkflowDirectory(path)
	}
	if a.Target.Directory {
		if _, e := os.Lstat(a.Target.Path); errors.Is(e, os.ErrNotExist) {
			return FileIdentity{}, nil
		}
		return inspectWorkflowDirectory(a.Target.Path)
	}
	if recovered && !a.Target.Expected.Exists {
		if _, e := os.Lstat(a.ResolvedPath); errors.Is(e, os.ErrNotExist) {
			return FileIdentity{}, nil
		}
	}
	id, _, e := inspectWorkflowFile(a.ResolvedPath, MaxPrivateFileSize)
	if recovered && !a.Target.Expected.Exists && errors.Is(e, os.ErrNotExist) {
		return FileIdentity{}, nil
	}
	return id, e
}

var errWorkflowContentChanged = errors.New("target content changed; original backup was preserved")
var errWorkflowBackupUnavailable = errors.New("original backup is unavailable or changed")
var errWorkflowRetentionChanged = errors.New("retained target identity or sync authority changed")

func workflowRecoveryError(index int, err error) error {
	reason := "target could not be safely restored"
	cause := err
	var pathError *os.PathError
	var linkError *os.LinkError
	var syscallError *os.SyscallError
	if errors.As(err, &pathError) || errors.As(err, &linkError) || errors.As(err, &syscallError) {
		cause = ErrRecoveryBlocked
	}
	switch {
	case errors.Is(err, errWorkflowContentChanged):
		reason = errWorkflowContentChanged.Error()
	case errors.Is(err, errWorkflowBackupUnavailable):
		reason = errWorkflowBackupUnavailable.Error()
	case errors.Is(err, errWorkflowRetentionChanged):
		reason = errWorkflowRetentionChanged.Error()
	case errors.Is(err, ErrIdentityChanged):
		reason = "target identity, parent, or link changed"
		cause = ErrIdentityChanged
	case errors.Is(err, ErrUnsafePath):
		reason = "target or backup metadata is unsafe"
		cause = ErrUnsafePath
	case errors.Is(err, ErrJournalCorrupt):
		reason = "workflow journal is invalid; preserve the original journal and backup"
		cause = ErrJournalCorrupt
	}
	return workflowRecoveryFailure{index, reason, cause}
}

type workflowRecoveryFailure struct {
	index  int
	reason string
	cause  error
}

func (e workflowRecoveryFailure) Error() string {
	return fmt.Sprintf("%s: target %d: %s; preserve the current file and original backup, then run al catalog recover", ErrRecoveryBlocked, e.index, e.reason)
}

func (e workflowRecoveryFailure) Unwrap() []error {
	return []error{ErrRecoveryBlocked, e.cause}
}

func workflowRetentionEligible(d workflowDisk) bool {
	if len(d.Manifest.Actions) != 1 {
		return false
	}
	a := d.Manifest.Actions[0]
	t := a.Target
	if t.Role != WorkflowPrivate || !t.Expected.Exists || t.Mode != 0o600 || t.Expected.Mode != 0o600 || t.Expected.Links != 1 || t.Remove || t.Directory || t.PromotionSource != "" || t.PreserveSymlink || t.ExpectedLink != "" || a.Link != "" || a.ParentIdentity == "" {
		return false
	}
	backup := false
	for _, p := range d.Progress {
		switch p.Stage {
		case "backup_synced":
			backup = true
		case "recovery_started", "recovery_target_retained", "recovered":
		default:
			return false
		}
	}
	return backup
}

func workflowRetentionSnapshot(a WorkflowAction) (FileIdentity, []byte, []byte, error) {
	p, err := checkWorkflowAction(a)
	if err != nil {
		return FileIdentity{}, nil, nil, err
	}
	p.close()
	id, current, err := inspectWorkflowFile(a.ResolvedPath, MaxPrivateFileSize)
	if err != nil {
		return id, nil, nil, err
	}
	expected := a.Target.Expected
	if !id.Exists || id.Mode != expected.Mode || id.Owner != expected.Owner || id.Group != expected.Group || id.Links != 1 {
		return id, nil, nil, ErrUnsafePath
	}
	backup, original, err := inspectWorkflowFile(a.BackupPath, MaxPrivateFileSize)
	if err != nil || !backup.Exists || backup.Mode != 0o600 || backup.Owner != expected.Owner || backup.Group != expected.Group || backup.Links != 1 || backup.SHA256 != expected.SHA256 || backup.Size != expected.Size {
		return id, nil, nil, errWorkflowBackupUnavailable
	}
	return id, original, current, nil
}

func workflowRetainedIdentity(d workflowDisk) *FileIdentity {
	for i := len(d.Progress) - 1; i >= 0; i-- {
		if d.Progress[i].Stage == "recovery_target_retained" {
			return d.Progress[i].Identity
		}
	}
	return nil
}

func validateRetainedWorkflow(d workflowDisk, policy WorkflowRecoveryPolicy) error {
	retained := workflowRetainedIdentity(d)
	if retained == nil {
		return nil
	}
	if policy == nil || !workflowRetentionEligible(d) {
		return errWorkflowRetentionChanged
	}
	id, original, current, err := workflowRetentionSnapshot(d.Manifest.Actions[0])
	if err != nil {
		return err
	}
	if !sameIdentity(*retained, id) || !policy(d.Manifest.Actions[0].Target, original, current) {
		return errWorkflowRetentionChanged
	}
	now, _, _, err := workflowRetentionSnapshot(d.Manifest.Actions[0])
	if err != nil {
		return err
	}
	if !sameIdentity(id, now) {
		return errWorkflowRetentionChanged
	}
	return nil
}

func retainWorkflow(path string, d *workflowDisk, policy WorkflowRecoveryPolicy, hooks []func(WorkflowBoundary) error) (bool, error) {
	retained := workflowRetainedIdentity(*d)
	if retained != nil {
		if err := validateRetainedWorkflow(*d, policy); err != nil {
			return false, err
		}
	} else if policy == nil || !workflowRetentionEligible(*d) {
		return false, nil
	}
	a := d.Manifest.Actions[0]
	if retained == nil {
		id, _, err := inspectWorkflowFile(a.ResolvedPath, MaxPrivateFileSize)
		if err != nil {
			return false, err
		}
		if workflowContentMatches(a.Target.Expected, id) || id.SHA256 == a.PlannedSHA256 {
			return false, nil
		}
	}
	id, original, current, err := workflowRetentionSnapshot(a)
	if err != nil {
		return false, err
	}
	if retained != nil && !sameIdentity(*retained, id) {
		return false, errWorkflowRetentionChanged
	}
	if retained == nil {
		if !policy(a.Target, original, current) {
			return false, errWorkflowContentChanged
		}
	}
	now, _, _, err := workflowRetentionSnapshot(a)
	if err != nil || !sameIdentity(id, now) {
		return false, errWorkflowRetentionChanged
	}
	b := WorkflowProgress{Stage: "recovery_target_retained", Target: 0, Identity: &id}
	d.Progress = append(d.Progress, b)
	if err = saveWorkflow(path, *d, false); err != nil {
		return false, err
	}
	if err = workflowHook(hooks, b); err != nil {
		return false, err
	}
	return true, nil
}
