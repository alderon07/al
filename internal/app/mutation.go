package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	workflowplan "github.com/alderon07/al/internal/plan"
	"github.com/alderon07/al/internal/transaction"
)

type mutationSession struct {
	stateRoot  string
	configRoot string
	services   *Services
}

func (svc *Services) withMutation(work func(*mutationSession) error) error {
	home, err := svc.dependencies.HomeDir()
	if err != nil {
		return err
	}
	configRoot := filepath.Join(home, ".config", "alias-lens")
	stateRoot := filepath.Join(home, ".local", "state", "alias-lens")
	for _, path := range []string{filepath.Join(home, ".config"), filepath.Join(home, ".local"), filepath.Join(home, ".local", "state")} {
		if err := transaction.EnsureWorkflowDirectory(path, false); err != nil {
			return fmt.Errorf("cannot prepare Alias Lens directories: %w", err)
		}
	}
	for _, path := range []string{configRoot, stateRoot} {
		if err := transaction.EnsureWorkflowDirectory(path, true); err != nil {
			return fmt.Errorf("cannot prepare private Alias Lens directory: %w", err)
		}
	}
	lock, err := transaction.AcquireLock(stateRoot, filepath.Join(stateRoot, "mutation.lock"))
	if err != nil {
		if errors.Is(err, transaction.ErrLocked) {
			return fmt.Errorf("%w; try again when the change finishes", err)
		}
		return err
	}
	defer lock.Close()
	session := &mutationSession{stateRoot: stateRoot, configRoot: configRoot, services: svc}
	if err := session.recover(); err != nil {
		return fmt.Errorf("Alias Lens could not safely recover an earlier change; run al catalog recover: %w", err)
	}
	return work(session)
}
func (s *mutationSession) recover() error {
	svc := s.services
	if svc == nil {
		svc = DefaultServices()
	}

	if err := transaction.RecoverWorkflows(s.stateRoot); err != nil {
		return err
	}
	for _, root := range []string{s.configRoot, s.stateRoot} {
		dir := filepath.Join(root, "transactions")
		if _, err := os.Lstat(dir); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		if err := recoverPrivateTransactions(root, dir); err != nil {
			return err
		}
	}
	return svc.recoverManagedCatalogStages(s)
}
func (svc *Services) applyMutationPlan(preview workflowplan.OperationPlan, rebuild planBuilder) error {
	if preview.Summary.Blocked {
		return fmt.Errorf("this change is blocked; review the plan diagnostics")
	}
	if len(preview.Actions) == 0 {
		return nil
	}
	return svc.withMutation(func(s *mutationSession) error { return s.ApplyPlan(preview, rebuild) })
}
func (s *mutationSession) ApplyPlan(preview workflowplan.OperationPlan, rebuild planBuilder) error {
	svc := s.services
	if svc == nil {
		svc = DefaultServices()
	}

	fresh, err := rebuild()
	if err != nil {
		return err
	}
	if err := workflowplan.CheckFresh(preview, fresh); err != nil {
		return fmt.Errorf("the files changed after the preview; review a new plan before trying again")
	}
	if fresh.Summary.Blocked {
		return fmt.Errorf("this change is blocked; review the plan diagnostics")
	}
	for _, a := range fresh.Approvals {
		if a.State == workflowplan.ApprovalRequired {
			return fmt.Errorf("native shell code needs approval; run al catalog review")
		}
	}
	if len(fresh.Actions) == 0 {
		return nil
	}
	id, err := newOperationID()
	if err != nil {
		return err
	}
	spec := transaction.WorkflowSpec{OperationID: id, StateRoot: s.stateRoot, PrivateRoots: []string{s.configRoot}}
	for _, action := range fresh.Actions {
		switch action.Kind {
		case workflowplan.ActionCreate, workflowplan.ActionReplace, workflowplan.ActionRemove, workflowplan.ActionEditOwnedRange, workflowplan.ActionRemoveOwnedRange, workflowplan.ActionActivate, workflowplan.ActionDeactivate, workflowplan.ActionRecordApproval, workflowplan.ActionConfigure, workflowplan.ActionClone:
		default:
			return fmt.Errorf("the safe writer does not support action %q yet", action.Kind)
		}
		role, order, err := mutationTargetRole(action.TargetRole)
		if action.Target.Scope != "" {
			role = transaction.WorkflowRole(action.Target.Scope)
			order = action.Target.RecoveryOrder
			err = nil
			if role != transaction.WorkflowPrivate && role != transaction.WorkflowUser && role != transaction.WorkflowRepository {
				err = transaction.ErrUnsafePath
			}
		}
		if err != nil {
			return err
		}
		path := action.Target.Path
		if role == transaction.WorkflowPrivate {
			allowed := false
			for _, root := range []string{s.configRoot, s.stateRoot} {
				relative, e := filepath.Rel(root, path)
				if e == nil && relative != "." && relative != ".." && !filepath.IsAbs(relative) && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
					allowed = true
				}
			}
			parentSession := s
			if !allowed {
				home, e := svc.dependencies.HomeDir()
				if e != nil {
					return e
				}
				data := filepath.Join(home, ".local", "share", "alias-lens")
				r, e := filepath.Rel(data, path)
				if e != nil || r == "." || r == ".." || filepath.IsAbs(r) || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
					return fmt.Errorf("planned target is outside the named private directories")
				}
				if _, e = s.dataRoot(); e != nil {
					return e
				}
				found := false
				for _, root := range spec.PrivateRoots {
					if root == data {
						found = true
					}
				}
				if !found {
					spec.PrivateRoots = append(spec.PrivateRoots, data)
				}
				parentSession = &mutationSession{stateRoot: s.stateRoot, configRoot: data, services: svc}
			}
			directories, e := prepareMutationParents(parentSession, path)
			if e != nil {
				return e
			}
			for _, directory := range directories {
				found := false
				for _, existing := range spec.Targets {
					if existing.Path == directory.Path {
						found = true
						break
					}
				}
				if !found {
					spec.Targets = append(spec.Targets, directory)
				}
			}
		}
		preserve := action.Target.PreserveSymlink || role == transaction.WorkflowUser
		if action.Target.PromotionSource != "" {
			if action.Kind != workflowplan.ActionClone || role != transaction.WorkflowRepository || action.Target.ExpectedIdentity.FileType != "missing" {
				return transaction.ErrUnsafePath
			}
			if _, e := os.Lstat(path); !errors.Is(e, os.ErrNotExist) {
				return workflowplan.ErrStalePlan
			}
			source, e := transaction.InspectWorkflowDirectory(action.Target.PromotionSource)
			if e != nil {
				return e
			}
			expected := action.Target.PromotionIdentity
			if expected.FileType != "directory" || source.PlatformID != fmt.Sprintf("%d:%d", expected.Device, expected.Inode) || source.Mode != expected.Mode || source.Owner != expected.Owner || source.Group != expected.Group || source.Links != expected.LinkCount || source.SHA256 != action.Target.PromotionSHA256 {
				return workflowplan.ErrStalePlan
			}
			spec.Targets = append(spec.Targets, transaction.WorkflowTarget{Path: path, Role: role, Expected: transaction.FileIdentity{}, Mode: 0o700, RecoveryOrder: order, PromotionSource: action.Target.PromotionSource, PromotionExpected: source})
			continue
		}
		if action.Kind == workflowplan.ActionClone {
			return transaction.ErrUnsafePath
		}
		if action.Target.Directory {
			spec.Targets = append(spec.Targets, transaction.WorkflowTarget{Path: path, Role: role, Mode: 0o700, Directory: true, RecoveryOrder: 1000})
			continue
		}

		identity, err := transaction.InspectWorkflowTarget(path, transaction.MaxPrivateFileSize, preserve)
		if err != nil {
			if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) || action.Target.ExpectedIdentity.FileType != "missing" {
				return err
			}
			identity = transaction.FileIdentity{}
		}
		expected := action.Target.ExpectedIdentity
		if expected.FileType == "symlink" && preserve {
			if !reflect.DeepEqual(expected, observePlanIdentity(path)) || action.Target.ReferentIdentity.FileType != "regular" {
				return workflowplan.ErrStalePlan
			}
			expected = action.Target.ReferentIdentity
		}
		if !plannedIdentityMatches(expected, identity) || identity.Exists && identity.SHA256 != action.Target.ExpectedSHA256 {
			return fmt.Errorf("the files changed after the preview; review a new plan before trying again")
		}
		mode := os.FileMode(0o600)
		if action.Target.Mode != 0 {
			mode = os.FileMode(action.Target.Mode)
		}
		if identity.Exists {
			mode = os.FileMode(identity.Mode)
		}
		remove := action.Kind == workflowplan.ActionRemove || action.Kind == workflowplan.ActionRemoveOwnedRange && len(action.Target.PlannedBytes) == 0
		spec.Targets = append(spec.Targets, transaction.WorkflowTarget{Path: path, Role: role, Expected: identity, Planned: action.Target.PlannedBytes, Mode: mode, Remove: remove, RecoveryOrder: order, PreserveSymlink: preserve, ExpectedLink: action.Target.ExpectedIdentity.LinkTarget})
	}
	if err := transaction.ApplyWorkflow(spec); err != nil {
		if recoveryErr := transaction.RecoverWorkflows(s.stateRoot); recoveryErr != nil {
			return fmt.Errorf("%v; automatic rollback needs attention; run al catalog recover: %w", err, recoveryErr)
		}
		return err
	}
	return nil
}
func mutationTargetRole(role string) (transaction.WorkflowRole, int, error) {
	switch role {
	case "native", "native_aliases", "native_alias_file", "ownership_native":
		return transaction.WorkflowUser, 0, nil
	case "pointer", "active_pointer", "activation_pointer":
		return transaction.WorkflowPrivate, 1, nil
	case "startup", "startup_file", "startup_block":
		return transaction.WorkflowUser, 2, nil
	case "repository", "repository_catalog", "catalog_repository":
		return transaction.WorkflowRepository, 3, nil
	case "catalog", "config", "state", "installed_state", "catalog_state", "approval", "approvals", "ownership", "snapshot", "generation", "generated", "revision", "completion", "completion_file", "catalog_snapshot", "generation_manifest", "rollback_native", "rollback_startup", "rollback_record", "adoptions", "installed", "catalog_revision":
		return transaction.WorkflowPrivate, 3, nil
	default:
		return "", 0, fmt.Errorf("planned target has unsupported role %q", role)
	}
}
func prepareMutationParents(s *mutationSession, path string) ([]transaction.WorkflowTarget, error) {
	root := ""
	for _, candidate := range []string{s.configRoot, s.stateRoot} {
		relative, e := filepath.Rel(candidate, path)
		if e == nil && relative != "." && relative != ".." && !filepath.IsAbs(relative) && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			root = candidate
			break
		}
	}
	if root == "" {
		return nil, transaction.ErrUnsafePath
	}
	var parents []string
	for parent := filepath.Dir(path); parent != root; parent = filepath.Dir(parent) {
		if parent == filepath.Dir(parent) {
			return nil, transaction.ErrUnsafePath
		}
		parents = append(parents, parent)
	}
	var targets []transaction.WorkflowTarget
	for i := len(parents) - 1; i >= 0; i-- {
		path := parents[i]
		info, e := os.Lstat(path)
		if errors.Is(e, os.ErrNotExist) {
			targets = append(targets, transaction.WorkflowTarget{Path: path, Role: transaction.WorkflowPrivate, Mode: 0o700, Directory: true, RecoveryOrder: 1000 - (len(parents) - i)})
			continue
		}
		if e != nil {
			return nil, e
		}
		if !info.IsDir() || info.Mode().Perm() != 0o700 || info.Mode()&os.ModeSymlink != 0 {
			return nil, transaction.ErrUnsafePath
		}
	}
	return targets, nil
}

func readManagedPrivateFile(path string, limit int64) ([]byte, error) {
	if _, e := os.Lstat(path); errors.Is(e, os.ErrNotExist) {
		return nil, os.ErrNotExist
	} else if e != nil {
		return nil, e
	}
	id, b, e := transaction.ReadWorkflowTarget(path, limit)
	if e != nil {
		return nil, e
	}
	if !id.Exists {
		return nil, os.ErrNotExist
	}
	if id.Mode != 0o600 {
		return nil, transaction.ErrUnsafePath
	}
	return b, nil
}
func (s *mutationSession) dataRoot() (string, error) {
	svc := s.services
	if svc == nil {
		svc = DefaultServices()
	}

	home, e := svc.dependencies.HomeDir()
	if e != nil {
		return "", e
	}
	share := filepath.Join(home, ".local", "share")
	if e = transaction.EnsureWorkflowDirectory(share, false); e != nil {
		return "", e
	}
	root := filepath.Join(share, "alias-lens")
	if e = transaction.EnsureWorkflowDirectory(root, true); e != nil {
		return "", e
	}
	return root, nil
}
func (s *mutationSession) privateSpec(path string) (transaction.WorkflowSpec, error) {
	svc := s.services
	if svc == nil {
		svc = DefaultServices()
	}

	id, e := newOperationID()
	if e != nil {
		return transaction.WorkflowSpec{}, e
	}
	spec := transaction.WorkflowSpec{OperationID: id, StateRoot: s.stateRoot, PrivateRoots: []string{s.configRoot}}
	root := ""
	for _, candidate := range []string{s.configRoot, s.stateRoot} {
		r, e := filepath.Rel(candidate, path)
		if e == nil && r != "." && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)) && !filepath.IsAbs(r) {
			root = candidate
			break
		}
	}
	if root == "" {
		data, e := s.dataRoot()
		if e != nil {
			return spec, e
		}
		r, e := filepath.Rel(data, path)
		if e != nil || r == "." || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) || filepath.IsAbs(r) {
			return spec, transaction.ErrUnsafePath
		}
		root = data
		spec.PrivateRoots = append(spec.PrivateRoots, data)
	}
	directories, e := prepareMutationParents(&mutationSession{stateRoot: s.stateRoot, configRoot: root, services: svc}, path)
	if e != nil {
		return spec, e
	}
	spec.Targets = directories
	return spec, nil
}
func (s *mutationSession) WritePrivate(path string, contents []byte) error {
	svc := s.services
	if svc == nil {
		svc = DefaultServices()
	}

	spec, e := s.privateSpec(path)
	if e != nil {
		return e
	}
	id, e := transaction.InspectWorkflowTarget(path, transaction.MaxPrivateFileSize, false)
	if e != nil {
		if _, se := os.Lstat(path); !errors.Is(se, os.ErrNotExist) {
			return e
		}
		id = transaction.FileIdentity{}
	}
	if id.Exists && id.Mode != 0o600 {
		return transaction.ErrUnsafePath
	}
	spec.Targets = append(spec.Targets, transaction.WorkflowTarget{Path: path, Role: transaction.WorkflowPrivate, Expected: id, Planned: contents, Mode: 0o600, RecoveryOrder: 3})
	return s.applySpec(spec)
}
func (s *mutationSession) RemovePrivate(path string) error {
	svc := s.services
	if svc == nil {
		svc = DefaultServices()
	}

	id, e := transaction.InspectWorkflowTarget(path, transaction.MaxPrivateFileSize, false)
	if e != nil {
		if _, se := os.Lstat(path); errors.Is(se, os.ErrNotExist) {
			return nil
		}
		return e
	}
	if !id.Exists {
		return nil
	}
	if id.Mode != 0o600 {
		return transaction.ErrUnsafePath
	}
	spec, e := s.privateSpec(path)
	if e != nil {
		return e
	}
	spec.Targets = append(spec.Targets, transaction.WorkflowTarget{Path: path, Role: transaction.WorkflowPrivate, Expected: id, Remove: true, Mode: 0o600, RecoveryOrder: 3})
	return s.applySpec(spec)
}
func (s *mutationSession) applySpec(spec transaction.WorkflowSpec) error {
	svc := s.services
	if svc == nil {
		svc = DefaultServices()
	}

	if e := transaction.ApplyWorkflow(spec); e != nil {
		if recovery := transaction.RecoverWorkflows(s.stateRoot); recovery != nil {
			return fmt.Errorf("change failed; run al catalog recover: %w", recovery)
		}
		return e
	}
	return nil
}

func (s *mutationSession) writeUserFile(path string, expected, contents []byte, mode os.FileMode, preserve bool, order int, hook bool) error {
	svc := s.services
	if svc == nil {
		svc = DefaultServices()
	}

	id, e := transaction.InspectWorkflowTarget(path, transaction.MaxPrivateFileSize, preserve)
	if e != nil {
		return e
	}
	if id.Exists && id.SHA256 != hashBytes(expected) || !id.Exists && len(expected) > 0 {
		return transaction.ErrIdentityChanged
	}
	if id.Exists {
		if id.Mode != uint32(mode) {
			return transaction.ErrUnsafePath
		}
	}
	operation, e := newOperationID()
	if e != nil {
		return e
	}
	spec := transaction.WorkflowSpec{OperationID: operation, StateRoot: s.stateRoot, Targets: []transaction.WorkflowTarget{{Path: path, Role: transaction.WorkflowUser, Expected: id, Planned: contents, Mode: mode, RecoveryOrder: order, PreserveSymlink: preserve}}}
	boundary := func(b transaction.WorkflowBoundary) error {
		if hook && b.Stage == "before_rename" && atomicWriteBeforeRename != nil {
			target := path
			if preserve {
				if info, e := os.Lstat(path); e == nil && info.Mode()&os.ModeSymlink != 0 {
					target, e = filepath.EvalSymlinks(path)
					if e != nil {
						return e
					}
				}
			}
			return atomicWriteBeforeRename(target)
		}
		return nil
	}
	if e = transaction.ApplyWorkflow(spec, boundary); e != nil {
		if recoverErr := transaction.RecoverWorkflows(s.stateRoot); recoverErr != nil {
			return fmt.Errorf("change failed; run al catalog recover: %w", recoverErr)
		}
		return e
	}
	return nil
}
func (s *mutationSession) EnsurePrivateDataDirectories(paths ...string) error {
	svc := s.services
	if svc == nil {
		svc = DefaultServices()
	}

	root, e := s.dataRoot()
	if e != nil {
		return e
	}
	for _, path := range paths {
		if path == root || path == filepath.Dir(root) {
			continue
		}
		r, e := filepath.Rel(root, path)
		if e != nil || r == "." || r == ".." || filepath.IsAbs(r) || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
			return transaction.ErrUnsafePath
		}
		spec, e := s.privateSpec(filepath.Join(path, ".directory-target"))
		if e != nil {
			return e
		}
		if len(spec.Targets) > 0 {
			if e = s.applySpec(spec); e != nil {
				return e
			}
		}
	}
	return nil
}

func plannedUserTarget(path string, current, planned []byte) (workflowplan.Target, error) {
	target := plannedTarget(path, current, planned)
	target.Scope = workflowplan.TargetScopeUser
	target.PreserveSymlink = true
	referent, e := transaction.InspectWorkflowTarget(path, transaction.MaxPrivateFileSize, true)
	if e != nil {
		return target, e
	}
	if referent.Exists && referent.SHA256 != hashBytes(current) {
		return target, workflowplan.ErrStalePlan
	}
	identity := workflowplan.Identity{FileType: "missing"}
	if referent.Exists {
		identity.FileType = "regular"
		identity.Mode = referent.Mode
		identity.Owner = referent.Owner
		identity.Group = referent.Group
		identity.LinkCount = referent.Links
		if referent.PlatformID != "" {
			if _, e := fmt.Sscanf(referent.PlatformID, "%d:%d", &identity.Device, &identity.Inode); e != nil {
				return target, e
			}
		}
	}
	target.ReferentIdentity = identity
	return target, nil
}
