//go:build !windows

package transaction

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func workflowHook(h []func(WorkflowBoundary) error, b WorkflowBoundary) error {
	if len(h) > 0 && h[0] != nil {
		return h[0](b)
	}
	return nil
}
func workflowWithin(root, path string) bool {
	r, e := filepath.Rel(root, path)
	return e == nil && r != "." && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)) && !filepath.IsAbs(r)
}
func validateWorkflowSpec(s WorkflowSpec) error {
	if !operationIDPattern.MatchString(s.OperationID) || len(s.Targets) == 0 || len(s.Targets) > MaxActions {
		return ErrUnsafePath
	}
	if r, e := validatePrivateRoot(s.StateRoot); e != nil || r != s.StateRoot {
		return ErrUnsafePath
	}
	for _, r := range s.PrivateRoots {
		if clean, e := validatePrivateRoot(r); e != nil || clean != r {
			return ErrUnsafePath
		}
	}
	seen := map[string]bool{}
	for _, t := range s.Targets {
		if !filepath.IsAbs(t.Path) || filepath.Clean(t.Path) != t.Path || seen[t.Path] {
			return ErrUnsafePath
		}
		seen[t.Path] = true
		if t.Role != WorkflowPrivate && t.Role != WorkflowUser && t.Role != WorkflowRepository {
			return ErrUnsafePath
		}
		if t.Role == WorkflowPrivate {
			allowed := false
			for _, r := range append([]string{s.StateRoot}, s.PrivateRoots...) {
				if workflowWithin(r, t.Path) {
					if _, e := validatePath(r, t.Path, true); e != nil {
						if !workflowPlannedParents(s, t.Path) {
							return e
						}
					}
					allowed = true
					break
				}
			}
			if !allowed || t.PreserveSymlink || (!t.Directory && t.Mode != 0o600) || (t.Directory && t.Mode != 0o700) {
				return ErrUnsafePath
			}
		}
		if t.Mode.Perm() != t.Mode || t.Mode&0o022 != 0 || len(t.Planned) > MaxPrivateFileSize {
			return ErrUnsafePath
		}
		if t.Expected.Exists {
			if e := validateHash(t.Expected.SHA256, "expected"); e != nil {
				return e
			}
		}
		if t.Directory && (t.Role != WorkflowPrivate || t.Expected.Exists || t.Remove || t.PromotionSource != "" || t.Mode != 0o700) {
			return ErrUnsafePath
		}
		if t.PromotionSource != "" && (t.Role != WorkflowRepository || t.Mode != 0o700 || !t.PromotionExpected.Exists || validateHash(t.PromotionExpected.SHA256, "promotion") != nil || t.Expected.Exists || t.Remove || len(t.Planned) != 0 || t.PreserveSymlink) {
			return ErrUnsafePath
		}
	}
	return nil
}
func ApplyWorkflow(spec WorkflowSpec, hooks ...func(WorkflowBoundary) error) error {
	if err := validateWorkflowSpec(spec); err != nil {
		return err
	}
	dir := filepath.Join(spec.StateRoot, "workflows")
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	if _, err := validatePrivateRoot(dir); err != nil {
		return err
	}
	path := workflowJournalPath(spec.StateRoot, spec.OperationID)
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("workflow already exists: run al catalog recover")
	}
	if spec.PrivateRoots == nil {
		spec.PrivateRoots = []string{}
	}
	d := workflowDisk{Manifest: WorkflowManifest{Version: 2, Spec: spec, Actions: []WorkflowAction{}}, Progress: []WorkflowProgress{}}
	resolvedSeen := map[string]bool{}
	for i, t := range spec.Targets {
		a := WorkflowAction{Target: t, TemporaryName: fmt.Sprintf(".alias-lens-%s-%d.tmp", spec.OperationID, i), PlannedSHA256: hashBytes(t.Planned)}
		if t.Directory {
			a.ResolvedPath = t.Path
			a.PlannedSHA256 = hashBytes(nil)
			if _, e := os.Lstat(t.Path); !errors.Is(e, os.ErrNotExist) {
				return ErrIdentityChanged
			}
		} else if t.PromotionSource != "" {
			if _, e := validatePrivateRoot(t.PromotionSource); e != nil {
				return e
			}
			id, e := inspectWorkflowDirectory(t.PromotionSource)
			if e != nil {
				return e
			}
			if !sameIdentity(t.PromotionExpected, id) {
				return ErrIdentityChanged
			}
			a.PromotionIdentity = id
			a.ResolvedPath = t.Path
			a.PlannedSHA256 = id.SHA256
			if _, e := os.Lstat(t.Path); !errors.Is(e, os.ErrNotExist) {
				return ErrIdentityChanged
			}
		} else {
			resolved, link, e := resolveWorkflowTarget(t.Path, t.PreserveSymlink)
			if e != nil {
				if !workflowPlannedParents(spec, t.Path) || t.Expected.Exists {
					return e
				}
				resolved = t.Path
				link = ""
			}
			if t.ExpectedLink != "" && (!t.PreserveSymlink || t.ExpectedLink != link) {
				return ErrIdentityChanged
			}
			a.ResolvedPath = resolved
			a.Link = link
			id, _, e := inspectWorkflowFile(resolved, MaxPrivateFileSize)
			if e != nil && !(workflowPlannedParents(spec, t.Path) && !t.Expected.Exists) {
				return e
			}
			if !sameIdentity(t.Expected, id) {
				return ErrIdentityChanged
			}
			if id.Exists && uint32(t.Mode) != id.Mode {
				return fmt.Errorf("%w: replacement must preserve file mode", ErrUnsafePath)
			}
			if id.Exists {
				a.BackupPath = filepath.Join(dir, fmt.Sprintf("%s-%d.backup", spec.OperationID, i))
			}
		}
		if resolvedSeen[a.ResolvedPath] {
			return ErrUnsafePath
		}
		resolvedSeen[a.ResolvedPath] = true
		p, e := openWorkflowParent(a.ResolvedPath)
		if e != nil {
			if !workflowPlannedParents(spec, t.Path) {
				return e
			}
			d.Manifest.Actions = append(d.Manifest.Actions, a)
			continue
		}
		info, e := p.file.Stat()
		p.close()
		if e != nil {
			return e
		}
		pid, e := identityFromInfo(info)
		if e != nil {
			return e
		}
		a.ParentIdentity = pid.PlatformID
		d.Manifest.Actions = append(d.Manifest.Actions, a)
	}
	if err := saveWorkflow(path, d, true); err != nil {
		return err
	}
	if err := workflowHook(hooks, WorkflowBoundary{Stage: "manifest_synced", Target: -1}); err != nil {
		return err
	}
	for i, a := range d.Manifest.Actions {
		if a.Target.Expected.Exists {
			id, b, e := inspectWorkflowFile(a.ResolvedPath, MaxPrivateFileSize)
			if e != nil {
				return e
			}
			if !sameIdentity(a.Target.Expected, id) {
				return ErrIdentityChanged
			}
			if e := writeNewPrivateFile(spec.StateRoot, a.BackupPath, b); e != nil {
				return e
			}
			if e := workflowRecord(path, &d, "backup_synced", i, hooks); e != nil {
				return e
			}
		}
	}
	for i, a := range d.Manifest.Actions {
		if a.ParentIdentity == "" {
			p, e := openWorkflowParent(a.ResolvedPath)
			if e != nil {
				return e
			}
			info, e := p.file.Stat()
			p.close()
			if e != nil {
				return e
			}
			id, e := identityFromInfo(info)
			if e != nil {
				return e
			}
			a.ParentIdentity = id.PlatformID
			d.Manifest.Actions[i] = a
			if e = saveWorkflow(path, d, false); e != nil {
				return e
			}
		}
		if err := applyWorkflowAction(a, hooks, i); err != nil {
			return err
		}
		if err := workflowRecord(path, &d, "target_synced", i, hooks); err != nil {
			return err
		}
	}
	for _, a := range d.Manifest.Actions {
		id, e := workflowActionIdentity(a, false)
		if e != nil {
			return e
		}
		if a.Target.Directory {
			if !id.Exists || id.Mode != 0o700 {
				return ErrIdentityChanged
			}
			continue
		}
		if a.Target.PromotionSource != "" {
			if !sameIdentity(a.PromotionIdentity, id) {
				return ErrIdentityChanged
			}
			continue
		}
		if a.Target.Remove {
			if id.Exists {
				return ErrIdentityChanged
			}
		} else if !id.Exists || id.SHA256 != a.PlannedSHA256 || id.Mode != uint32(a.Target.Mode) {
			return ErrIdentityChanged
		}
	}
	if err := workflowRecord(path, &d, "committed", -1, hooks); err != nil {
		return err
	}
	return finishWorkflow(path, d)
}
func workflowPlannedParents(s WorkflowSpec, path string) bool {
	for p := filepath.Dir(path); ; p = filepath.Dir(p) {
		if info, e := os.Lstat(p); e == nil {
			return info.IsDir() && info.Mode()&os.ModeSymlink == 0
		}
		found := false
		for _, t := range s.Targets {
			if t.Path == p && t.Directory && !t.Expected.Exists {
				found = true
				break
			}
		}
		if !found || p == filepath.Dir(p) {
			return false
		}
	}
}
