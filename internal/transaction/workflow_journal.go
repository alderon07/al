//go:build !windows

package transaction

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
)

type workflowDisk struct {
	Manifest WorkflowManifest   `json:"manifest"`
	Progress []WorkflowProgress `json:"progress"`
}

func workflowJournalPath(root, id string) string {
	return filepath.Join(root, "workflows", id+".workflow")
}
func workflowRecord(path string, d *workflowDisk, stage string, i int, h []func(WorkflowBoundary) error) error {
	b := WorkflowBoundary{Stage: stage, Target: i}
	if stage == "target_synced" || stage == "recovery_target_synced" {
		identity, e := workflowActionIdentity(d.Manifest.Actions[i], stage == "recovery_target_synced")
		if e != nil {
			return e
		}
		b.Identity = &identity
	}
	d.Progress = append(d.Progress, b)
	if err := saveWorkflow(path, *d, false); err != nil {
		return err
	}
	return workflowHook(h, b)
}
func saveWorkflow(path string, d workflowDisk, initial bool) error {
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	if len(b) > MaxJournalBytes {
		return ErrUnsafePath
	}
	digest := sha256.Sum256(b)
	var head [8]byte
	binary.BigEndian.PutUint64(head[:], uint64(len(b)))
	payload := append(head[:], b...)
	payload = append(payload, digest[:]...)
	if initial {
		p, e := openWorkflowParent(path)
		if e != nil {
			return e
		}
		defer p.close()
		owner, group := workflowOwnerGroup()
		if e = writeWorkflowTemp(p, p.name, payload, 0o600, owner, group); e != nil {
			return e
		}
		return p.file.Sync()
	}
	p, err := openWorkflowParent(path)
	if err != nil {
		return err
	}
	defer p.close()
	name := p.name + ".next"
	if err = removeWorkflowTemp(p, name); err != nil {
		return err
	}
	owner, group := workflowOwnerGroup()
	if err = writeWorkflowTemp(p, name, payload, 0o600, owner, group); err != nil {
		return err
	}
	if err = p.rename(name); err != nil {
		return err
	}
	return p.file.Sync()
}
func ReadWorkflowJournal(stateRoot, path string) (WorkflowManifest, []WorkflowProgress, error) {
	if _, err := validatePrivateRoot(stateRoot); err != nil {
		return WorkflowManifest{}, nil, err
	}
	if filepath.Dir(path) != filepath.Join(stateRoot, "workflows") || !strings.HasSuffix(path, ".workflow") {
		return WorkflowManifest{}, nil, ErrUnsafePath
	}
	id, err := InspectPrivateFile(stateRoot, path, MaxJournalBytes+40)
	if err != nil || !id.Exists {
		return WorkflowManifest{}, nil, ErrJournalCorrupt
	}
	b, err := readPrivateBytes(path, MaxJournalBytes+40)
	if err != nil {
		return WorkflowManifest{}, nil, err
	}
	if len(b) < 40 || binary.BigEndian.Uint64(b[:8]) != uint64(len(b)-40) {
		return WorkflowManifest{}, nil, ErrJournalCorrupt
	}
	digest := sha256.Sum256(b[8 : len(b)-32])
	if !bytes.Equal(digest[:], b[len(b)-32:]) {
		return WorkflowManifest{}, nil, ErrJournalCorrupt
	}
	var d workflowDisk
	dec := json.NewDecoder(bytes.NewReader(b[8 : len(b)-32]))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&d); err != nil || dec.Decode(&struct{}{}) != io.EOF {
		return WorkflowManifest{}, nil, ErrJournalCorrupt
	}
	canonical, e := json.Marshal(d)
	if e != nil || !bytes.Equal(canonical, b[8:len(b)-32]) || d.Progress == nil || d.Manifest.Spec.PrivateRoots == nil || d.Manifest.Actions == nil || d.Manifest.Spec.Targets == nil {
		return WorkflowManifest{}, nil, ErrJournalCorrupt
	}
	if d.Manifest.Version != 2 || d.Manifest.Spec.StateRoot != stateRoot || workflowJournalPath(stateRoot, d.Manifest.Spec.OperationID) != path || len(d.Manifest.Actions) != len(d.Manifest.Spec.Targets) {
		return WorkflowManifest{}, nil, ErrJournalCorrupt
	}
	if err = validateWorkflowSpec(d.Manifest.Spec); err != nil {
		return WorkflowManifest{}, nil, ErrJournalCorrupt
	}
	for i, a := range d.Manifest.Actions {
		t := d.Manifest.Spec.Targets[i]
		tb, _ := json.Marshal(t)
		ab, _ := json.Marshal(a.Target)
		if !bytes.Equal(tb, ab) {
			return WorkflowManifest{}, nil, ErrJournalCorrupt
		}
		if a.Link != "" {
			resolved := a.Link
			if !filepath.IsAbs(resolved) {
				resolved = filepath.Join(filepath.Dir(t.Path), resolved)
			}
			if filepath.Clean(resolved) != a.ResolvedPath || !t.PreserveSymlink {
				return WorkflowManifest{}, nil, ErrJournalCorrupt
			}
		}
		if t.ExpectedLink != "" && (!t.PreserveSymlink || a.Link != t.ExpectedLink) {
			return WorkflowManifest{}, nil, ErrJournalCorrupt
		}
		if a.ResolvedPath != t.Path && (!t.PreserveSymlink || a.Link == "") {
			return WorkflowManifest{}, nil, ErrJournalCorrupt
		}
		if a.TemporaryName != fmt.Sprintf(".alias-lens-%s-%d.tmp", d.Manifest.Spec.OperationID, i) {
			return WorkflowManifest{}, nil, ErrJournalCorrupt
		}
		if t.Expected.Exists && a.BackupPath != filepath.Join(stateRoot, "workflows", fmt.Sprintf("%s-%d.backup", d.Manifest.Spec.OperationID, i)) {
			return WorkflowManifest{}, nil, ErrJournalCorrupt
		}
		if !t.Expected.Exists && a.BackupPath != "" || validateHash(a.PlannedSHA256, "planned") != nil {
			return WorkflowManifest{}, nil, ErrJournalCorrupt
		}
	}
	if err = validateWorkflowProgress(d); err != nil {
		return WorkflowManifest{}, nil, err
	}
	return d.Manifest, d.Progress, nil
}
func validateWorkflowProgress(d workflowDisk) error {
	nextBackup := 0
	nextTarget := 0
	recovering := false
	finished := false
	recoveryIndex := 0
	indices := make([]int, len(d.Manifest.Actions))
	for i := range indices {
		indices[i] = i
	}
	sort.SliceStable(indices, func(i, j int) bool {
		return d.Manifest.Actions[indices[i]].Target.RecoveryOrder < d.Manifest.Actions[indices[j]].Target.RecoveryOrder
	})
	skipBackups := func() {
		for nextBackup < len(d.Manifest.Actions) && !d.Manifest.Actions[nextBackup].Target.Expected.Exists {
			nextBackup++
		}
	}
	skipBackups()
	for _, p := range d.Progress {
		if p.Identity != nil {
			if p.Target < 0 || p.Target >= len(d.Manifest.Actions) {
				return ErrJournalCorrupt
			}
			a := d.Manifest.Actions[p.Target]
			id := *p.Identity
			if p.Stage == "target_synced" {
				if a.Target.PromotionSource != "" {
					if !sameIdentity(a.PromotionIdentity, id) {
						return ErrJournalCorrupt
					}
				} else if a.Target.Directory {
					if !id.Exists || id.Mode != 0o700 || validateHash(id.SHA256, "directory") != nil {
						return ErrJournalCorrupt
					}
				} else if a.Target.Remove {
					if id != (FileIdentity{}) {
						return ErrJournalCorrupt
					}
				} else if !id.Exists || id.SHA256 != a.PlannedSHA256 || id.Mode != uint32(a.Target.Mode) || id.Links != 1 {
					return ErrJournalCorrupt
				}
			} else if p.Stage == "recovery_target_synced" {
				if a.Target.PromotionSource != "" {
					if !sameIdentity(a.PromotionIdentity, id) {
						return ErrJournalCorrupt
					}
				} else if !workflowContentMatches(a.Target.Expected, id) {
					return ErrJournalCorrupt
				}
			}
		}
		if finished {
			return ErrJournalCorrupt
		}
		switch p.Stage {
		case "backup_synced":
			if recovering || nextTarget != 0 || p.Target != nextBackup || nextBackup == len(d.Manifest.Actions) || p.Identity != nil {
				return ErrJournalCorrupt
			}
			nextBackup++
			skipBackups()
		case "target_synced":
			if recovering || nextBackup != len(d.Manifest.Actions) || p.Target != nextTarget || nextTarget == len(d.Manifest.Actions) || p.Identity == nil {
				return ErrJournalCorrupt
			}
			nextTarget++
		case "committed":
			if recovering || nextTarget != len(d.Manifest.Actions) || p.Target != -1 || p.Identity != nil {
				return ErrJournalCorrupt
			}
			finished = true
		case "recovery_started":
			if p.Target != -1 || p.Identity != nil {
				return ErrJournalCorrupt
			}
			recovering = true
			recoveryIndex = 0
		case "recovery_target_synced":
			if !recovering || recoveryIndex == len(indices) || p.Target != indices[recoveryIndex] || p.Identity == nil {
				return ErrJournalCorrupt
			}
			recoveryIndex++
		case "recovered":
			if !recovering || recoveryIndex != len(indices) || p.Target != -1 || p.Identity != nil {
				return ErrJournalCorrupt
			}
			finished = true
		default:
			return ErrJournalCorrupt
		}
	}
	return nil
}
