//go:build !windows

package transaction

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWorkflowSecondRecoveryAndOrder(t *testing.T) {
	spec, paths := workflowFixture(t)
	spec.Targets = []WorkflowTarget{spec.Targets[3], spec.Targets[2], spec.Targets[1], spec.Targets[0]}
	stop := errors.New("stop")
	if e := ApplyWorkflow(spec, func(b WorkflowBoundary) error {
		if b.Stage == "target_synced" && b.Target == 3 {
			return stop
		}
		return nil
	}); !errors.Is(e, stop) {
		t.Fatal(e)
	}
	var restored []int
	if e := RecoverWorkflows(spec.StateRoot, func(b WorkflowBoundary) error {
		if b.Stage == "recovery_after_rename" {
			restored = append(restored, b.Target)
			if len(restored) == 2 {
				return stop
			}
		}
		return nil
	}); !errors.Is(e, ErrRecoveryBlocked) {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(restored, []int{3, 2}) {
		t.Fatalf("order=%v", restored)
	}
	if e := RecoverWorkflows(spec.StateRoot); e != nil {
		t.Fatal(e)
	}
	if e := RecoverWorkflows(spec.StateRoot); e != nil {
		t.Fatal(e)
	}
	for _, p := range paths {
		b, e := os.ReadFile(p)
		if e != nil || string(b) != "before" {
			t.Fatalf("restored=%q error=%v", b, e)
		}
	}
}
func TestWorkflowDirectoryPromotionRecovery(t *testing.T) {
	root := privateTestRoot(t)
	source := filepath.Join(root, "stage")
	destination := filepath.Join(root, "repository")
	os.Mkdir(source, 0o700)
	os.WriteFile(filepath.Join(source, "catalog.json"), []byte("catalog"), 0o600)
	promotionIdentity, e := InspectWorkflowDirectory(source)
	if e != nil {
		t.Fatal(e)
	}
	spec := WorkflowSpec{OperationID: "promotion", StateRoot: root, Targets: []WorkflowTarget{{Path: destination, Role: WorkflowRepository, Mode: 0o700, PromotionSource: source, PromotionExpected: promotionIdentity}}}
	stop := errors.New("stop")
	if e := ApplyWorkflow(spec, func(b WorkflowBoundary) error {
		if b.Stage == "after_rename" {
			return stop
		}
		return nil
	}); !errors.Is(e, stop) {
		t.Fatal(e)
	}
	if e := RecoverWorkflows(root, func(b WorkflowBoundary) error {
		if b.Stage == "recovery_after_rename" {
			return stop
		}
		return nil
	}); !errors.Is(e, ErrRecoveryBlocked) {
		t.Fatal(e)
	}
	if e := RecoverWorkflows(root); e != nil {
		t.Fatal(e)
	}
	if e := RecoverWorkflows(root); e != nil {
		t.Fatal(e)
	}
	if b, e := os.ReadFile(filepath.Join(source, "catalog.json")); e != nil || string(b) != "catalog" {
		t.Fatalf("promotion=%q %v", b, e)
	}
}
func TestWorkflowProcessDeathTwice(t *testing.T) {
	if phase := os.Getenv("AL_TRANSACTION_CRASH"); phase != "" {
		root := os.Getenv("AL_TRANSACTION_ROOT")
		path := filepath.Join(root, "native")
		if phase == "forward" {
			id, e := InspectWorkflowTarget(path, 1024, false)
			if e != nil {
				os.Exit(80)
			}
			spec := WorkflowSpec{OperationID: "death", StateRoot: root, Targets: []WorkflowTarget{{Path: path, Role: WorkflowPrivate, Expected: id, Planned: []byte("after"), Mode: 0o600}}}
			_ = ApplyWorkflow(spec, func(b WorkflowBoundary) error {
				if b.Stage == "after_rename" {
					os.Exit(81)
				}
				return nil
			})
		} else {
			_ = RecoverWorkflows(root, func(b WorkflowBoundary) error {
				if b.Stage == "recovery_after_rename" {
					os.Exit(82)
				}
				return nil
			})
		}
		os.Exit(83)
	}
	root := privateTestRoot(t)
	path := filepath.Join(root, "native")
	os.WriteFile(path, []byte("before"), 0o600)
	for _, phase := range []string{"forward", "recovery"} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestWorkflowProcessDeathTwice$")
		cmd.Env = append(os.Environ(), "AL_TRANSACTION_CRASH="+phase, "AL_TRANSACTION_ROOT="+root)
		if e := cmd.Run(); e == nil {
			t.Fatal("child survived crash boundary")
		}
	}
	if e := RecoverWorkflows(root); e != nil {
		t.Fatal(e)
	}
	if e := RecoverWorkflows(root); e != nil {
		t.Fatal(e)
	}
	if b, e := os.ReadFile(path); e != nil || string(b) != "before" {
		t.Fatalf("restored=%q %v", b, e)
	}
}
func TestWorkflowNamedDirectoriesRecoverWithoutOrphans(t *testing.T) {
	for _, stage := range []string{"manifest_synced", "after_rename", "target_synced"} {
		t.Run(stage, func(t *testing.T) {
			root := privateTestRoot(t)
			parent := filepath.Join(root, "generated")
			child := filepath.Join(parent, "shell")
			file := filepath.Join(child, "package")
			spec := WorkflowSpec{OperationID: "directories", StateRoot: root, Targets: []WorkflowTarget{{Path: parent, Role: WorkflowPrivate, Mode: 0o700, Directory: true, RecoveryOrder: 100}, {Path: child, Role: WorkflowPrivate, Mode: 0o700, Directory: true, RecoveryOrder: 99}, {Path: file, Role: WorkflowPrivate, Mode: 0o600, Planned: []byte("package"), RecoveryOrder: 3}}}
			stop := errors.New("stop")
			if e := ApplyWorkflow(spec, func(b WorkflowBoundary) error {
				if b.Stage == stage && (stage == "manifest_synced" || b.Target == 2) {
					return stop
				}
				return nil
			}); !errors.Is(e, stop) {
				t.Fatal(e)
			}
			if e := RecoverWorkflows(root); e != nil {
				t.Fatal(e)
			}
			if e := RecoverWorkflows(root); e != nil {
				t.Fatal(e)
			}
			if _, e := os.Lstat(parent); !errors.Is(e, os.ErrNotExist) {
				t.Fatalf("orphan directory: %v", e)
			}
		})
	}
}
func TestWorkflowRecordedInverseIdentityRefusesExternalReplacement(t *testing.T) {
	spec, paths := workflowFixture(t)
	stop := errors.New("stop")
	if e := ApplyWorkflow(spec, func(b WorkflowBoundary) error {
		if b.Stage == "after_rename" {
			return stop
		}
		return nil
	}); !errors.Is(e, stop) {
		t.Fatal(e)
	}
	if e := RecoverWorkflows(spec.StateRoot, func(b WorkflowBoundary) error {
		if b.Stage == "recovery_target_synced" && b.Target == 0 {
			return stop
		}
		return nil
	}); !errors.Is(e, stop) {
		t.Fatal(e)
	}
	temp := paths[0] + ".external"
	os.WriteFile(temp, []byte("before"), 0o644)
	os.Rename(temp, paths[0])
	if e := RecoverWorkflows(spec.StateRoot); !errors.Is(e, ErrRecoveryBlocked) {
		t.Fatalf("external inverse replacement accepted: %v", e)
	}
}
func TestWorkflowRecordedAbsentDirectoryRefusesRecreation(t *testing.T) {
	root := privateTestRoot(t)
	path := filepath.Join(root, "generated")
	spec := WorkflowSpec{OperationID: "recreated", StateRoot: root, Targets: []WorkflowTarget{{Path: path, Role: WorkflowPrivate, Mode: 0o700, Directory: true}}}
	stop := errors.New("stop")
	if e := ApplyWorkflow(spec, func(b WorkflowBoundary) error {
		if b.Stage == "target_synced" {
			return stop
		}
		return nil
	}); !errors.Is(e, stop) {
		t.Fatal(e)
	}
	if e := RecoverWorkflows(root, func(b WorkflowBoundary) error {
		if b.Stage == "recovery_target_synced" {
			return stop
		}
		return nil
	}); !errors.Is(e, stop) {
		t.Fatal(e)
	}
	if e := os.Mkdir(path, 0o700); e != nil {
		t.Fatal(e)
	}
	if e := RecoverWorkflows(root); !errors.Is(e, ErrRecoveryBlocked) {
		t.Fatalf("recreated directory accepted: %v", e)
	}
	if _, e := os.Stat(path); e != nil {
		t.Fatalf("external directory removed: %v", e)
	}
}
