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
	if _, err := validatePrivateRoot(stateRoot); err != nil {
		return err
	}
	dir := filepath.Join(stateRoot, "workflows")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err = validatePrivateRoot(dir); err != nil {
		return err
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".workflow") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		m, p, err := ReadWorkflowJournal(stateRoot, path)
		if err != nil {
			return err
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
			if err = finishWorkflow(path, d); err != nil {
				return err
			}
			continue
		}
		if err = workflowRecord(path, &d, "recovery_started", -1, hooks); err != nil {
			return err
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
							return ErrRecoveryBlocked
						}
					}
				}
				if oldProgress.Stage == "recovery_target_synced" && oldProgress.Target == i {
					now, e := workflowActionIdentity(m.Actions[i], true)
					if e != nil || oldProgress.Identity == nil || !sameIdentity(*oldProgress.Identity, now) {
						return ErrRecoveryBlocked
					}
				}
			}
			if err = recoverWorkflowAction(m.Actions[i], hooks, i); err != nil {
				return fmt.Errorf("%w: target %d; run al catalog recover", ErrRecoveryBlocked, i)
			}
			if err = workflowRecord(path, &d, "recovery_target_synced", i, hooks); err != nil {
				return err
			}
		}
		if err = workflowRecord(path, &d, "recovered", -1, hooks); err != nil {
			return err
		}
		if err = finishWorkflow(path, d); err != nil {
			return err
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
		return ErrRecoveryBlocked
	}
	if !a.Target.Expected.Exists {
		if err = p.remove(); err != nil {
			return err
		}
	} else {
		backup, b, e := inspectWorkflowFile(a.BackupPath, MaxPrivateFileSize)
		if e != nil || !backup.Exists || backup.SHA256 != a.Target.Expected.SHA256 {
			return ErrRecoveryBlocked
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
