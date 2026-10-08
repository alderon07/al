//go:build !windows

package transaction

import (
	"errors"
	"os"
	"path/filepath"
)

func InspectWorkflowTarget(path string, limit int64, preserveSymlink bool) (FileIdentity, error) {
	if limit <= 0 {
		limit = MaxPrivateFileSize
	}
	resolved, _, err := resolveWorkflowTarget(path, preserveSymlink)
	if err != nil {
		return FileIdentity{}, err
	}
	id, _, err := inspectWorkflowFile(resolved, limit)
	return id, err
}
func resolveWorkflowTarget(path string, preserve bool) (string, string, error) {
	p, err := openWorkflowParent(path)
	if err != nil {
		return "", "", err
	}
	defer p.close()
	link, err := p.readLink()
	if errors.Is(err, os.ErrNotExist) || err != nil && link == "" {
		if _, _, check := inspectWorkflowFile(path, MaxPrivateFileSize); check != nil {
			return "", "", check
		}
		return path, "", nil
	}
	if err != nil || !preserve {
		return "", "", ErrUnsafePath
	}
	resolved := link
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(filepath.Dir(path), resolved)
	}
	resolved = filepath.Clean(resolved)
	if _, _, err := inspectWorkflowFile(resolved, MaxPrivateFileSize); err != nil {
		return "", "", err
	}
	return resolved, link, nil
}
func checkWorkflowAction(a WorkflowAction) (*workflowParent, error) {
	resolved, link, err := resolveWorkflowTarget(a.Target.Path, a.Target.PreserveSymlink)
	if a.Target.PromotionSource != "" || a.Target.Directory {
		resolved = a.Target.Path
		link = ""
		err = nil
	}
	if err != nil || resolved != a.ResolvedPath || link != a.Link {
		return nil, ErrIdentityChanged
	}
	p, err := openWorkflowParent(a.ResolvedPath)
	if err != nil {
		return nil, err
	}
	info, err := p.file.Stat()
	if err == nil {
		var id FileIdentity
		id, err = identityFromInfo(info)
		if a.ParentIdentity != "" && id.PlatformID != a.ParentIdentity {
			err = ErrIdentityChanged
		}
	}
	if err != nil {
		p.close()
		return nil, err
	}
	return p, nil
}
func applyWorkflowAction(a WorkflowAction, h []func(WorkflowBoundary) error, i int) error {
	p, err := checkWorkflowAction(a)
	if err != nil {
		return err
	}
	defer p.close()
	if a.Target.Directory {
		if e := workflowHook(h, WorkflowBoundary{Stage: "before_rename", Target: i}); e != nil {
			return e
		}
		if e := createWorkflowDirectory(p); e != nil {
			return e
		}
		return workflowHook(h, WorkflowBoundary{Stage: "after_rename", Target: i})
	}
	if a.Target.PromotionSource != "" {
		id, e := inspectWorkflowDirectory(a.Target.PromotionSource)
		if e != nil || !sameIdentity(a.PromotionIdentity, id) {
			return ErrIdentityChanged
		}
		if _, e = os.Lstat(a.Target.Path); !errors.Is(e, os.ErrNotExist) {
			return ErrIdentityChanged
		}
		if e = workflowHook(h, WorkflowBoundary{Stage: "before_rename", Target: i}); e != nil {
			return e
		}
		current, e := inspectWorkflowDirectory(a.Target.PromotionSource)
		if e != nil || !sameIdentity(current, a.PromotionIdentity) {
			return ErrIdentityChanged
		}
		if _, e = os.Lstat(a.Target.Path); !errors.Is(e, os.ErrNotExist) {
			return ErrIdentityChanged
		}
		if e = promoteWorkflowDirectory(a.Target.PromotionSource, a.Target.Path); e != nil {
			return e
		}
		return workflowHook(h, WorkflowBoundary{Stage: "after_rename", Target: i})
	}
	id, _, err := inspectWorkflowFile(a.ResolvedPath, MaxPrivateFileSize)
	if err != nil {
		return err
	}
	if !sameIdentity(a.Target.Expected, id) {
		return ErrIdentityChanged
	}
	if !a.Target.Remove {
		owner, group := workflowOwnerGroup()
		if id.Exists {
			owner = id.Owner
			group = id.Group
		}
		if err = writeWorkflowTemp(p, a.TemporaryName, a.Target.Planned, uint32(a.Target.Mode), owner, group); err != nil {
			return err
		}
		if err = p.file.Sync(); err != nil {
			return err
		}
		if err = workflowHook(h, WorkflowBoundary{Stage: "temporary_synced", Target: i}); err != nil {
			return err
		}
	}
	if err = workflowHook(h, WorkflowBoundary{Stage: "before_rename", Target: i}); err != nil {
		return err
	}
	parentCheck, e := checkWorkflowAction(a)
	if e != nil {
		return e
	}
	parentCheck.close()
	id, _, err = inspectWorkflowFile(a.ResolvedPath, MaxPrivateFileSize)
	if err != nil || !sameIdentity(a.Target.Expected, id) {
		return ErrIdentityChanged
	}
	if a.Target.Remove {
		if id.Exists {
			err = p.remove()
		}
	} else {
		err = p.rename(a.TemporaryName)
	}
	if err != nil {
		return err
	}
	if err = workflowHook(h, WorkflowBoundary{Stage: "after_rename", Target: i}); err != nil {
		return err
	}
	return p.file.Sync()
}
func InspectWorkflowDirectory(path string) (FileIdentity, error) {
	return inspectWorkflowDirectory(path)
}
