//go:build !windows

package transaction

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
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

func retainedWorkflowFixture(t *testing.T, boundary string) (WorkflowSpec, string) {
	t.Helper()
	spec, paths := workflowFixture(t)
	spec.Targets = spec.Targets[1:2]
	stop := errors.New("synthetic interruption")
	if err := ApplyWorkflow(spec, func(b WorkflowBoundary) error {
		if b.Stage == boundary {
			return stop
		}
		return nil
	}); !errors.Is(err, stop) {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths[1], []byte("current diagnostic"), 0o600); err != nil {
		t.Fatal(err)
	}
	return spec, paths[1]
}

func TestWorkflowRetainedIdentityReplayAndTerminalCleanup(t *testing.T) {
	for _, boundary := range []string{"recovery_target_retained", "recovered"} {
		for _, changed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/changed=%v", boundary, changed), func(t *testing.T) {
				spec, path := retainedWorkflowFixture(t, "backup_synced")
				policy := func(WorkflowTarget, []byte, []byte) bool { return true }
				stop := errors.New("synthetic interruption")
				if err := RecoverWorkflowsWithPolicy(spec.StateRoot, policy, func(b WorkflowBoundary) error {
					if b.Stage == boundary {
						return stop
					}
					return nil
				}); !errors.Is(err, stop) {
					t.Fatal(err)
				}
				journal := workflowJournalPath(spec.StateRoot, spec.OperationID)
				if _, _, err := ReadWorkflowJournal(spec.StateRoot, journal); err != nil {
					t.Fatal(err)
				}
				if err := RecoverWorkflows(spec.StateRoot); !errors.Is(err, ErrRecoveryBlocked) {
					t.Fatal("generic recovery accepted retention", err)
				}
				if changed {
					external := path + ".external"
					if err := os.WriteFile(external, []byte("current diagnostic"), 0o600); err != nil {
						t.Fatal(err)
					}
					if err := os.Rename(external, path); err != nil {
						t.Fatal(err)
					}
				}
				before, _, err := inspectWorkflowFile(path, MaxPrivateFileSize)
				if err != nil {
					t.Fatal(err)
				}
				err = RecoverWorkflowsWithPolicy(spec.StateRoot, policy)
				if changed {
					if !errors.Is(err, ErrRecoveryBlocked) {
						t.Fatal("changed retained identity accepted", err)
					}
					if _, err := os.Lstat(journal); err != nil {
						t.Fatal("blocked journal discarded", err)
					}
				} else if err != nil {
					t.Fatal(err)
				}
				after, actual, err := inspectWorkflowFile(path, MaxPrivateFileSize)
				if err != nil || !sameIdentity(before, after) || string(actual) != "current diagnostic" {
					t.Fatal("retention replay changed target", err)
				}
				backup := filepath.Join(spec.StateRoot, "workflows", spec.OperationID+"-0.backup")
				if contents, err := os.ReadFile(backup); err != nil || string(contents) != "before" {
					t.Fatal("retention changed backup", err)
				}
			})
		}
	}
}

func TestWorkflowRetentionRefusesUnprovenChanges(t *testing.T) {
	for _, name := range []string{"forward_progress", "mode", "hardlink", "symlink", "missing_backup", "corrupt_backup", "callback_change"} {
		t.Run(name, func(t *testing.T) {
			boundary := "backup_synced"
			if name == "forward_progress" {
				boundary = "target_synced"
			}
			spec, path := retainedWorkflowFixture(t, boundary)
			backup := filepath.Join(spec.StateRoot, "workflows", spec.OperationID+"-0.backup")
			switch name {
			case "mode":
				if err := os.Chmod(path, 0o640); err != nil {
					t.Fatal(err)
				}
			case "hardlink":
				if err := os.Link(path, path+".link"); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Rename(path, path+".referent"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Base(path)+".referent", path); err != nil {
					t.Fatal(err)
				}
			case "missing_backup":
				if err := os.Remove(backup); err != nil {
					t.Fatal(err)
				}
			case "corrupt_backup":
				if err := os.WriteFile(backup, []byte("corrupt"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			policy := func(WorkflowTarget, []byte, []byte) bool {
				if name == "callback_change" {
					if err := os.WriteFile(path, []byte("external callback change"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				return true
			}
			err := RecoverWorkflowsWithPolicy(spec.StateRoot, policy)
			if !errors.Is(err, ErrRecoveryBlocked) || strings.Contains(err.Error(), spec.StateRoot) {
				t.Fatal("unsafe change accepted or exposed", err)
			}
			actual, err := os.ReadFile(path)
			want := "current diagnostic"
			if name == "callback_change" {
				want = "external callback change"
			}
			if err != nil || string(actual) != want {
				t.Fatal("blocked recovery changed target", err)
			}
		})
	}
}

func TestWorkflowPolicyPreservesOrdinaryBaselineWithoutBackup(t *testing.T) {
	spec, path := retainedWorkflowFixture(t, "backup_synced")
	if err := os.WriteFile(path, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(spec.StateRoot, "workflows", spec.OperationID+"-0.backup")); err != nil {
		t.Fatal(err)
	}
	policy := func(WorkflowTarget, []byte, []byte) bool { t.Fatal("baseline recovery invoked policy"); return false }
	if err := RecoverWorkflowsWithPolicy(spec.StateRoot, policy); err != nil {
		t.Fatal(err)
	}
}

func TestWorkflowBlockedRetryPreservesRepeatedStarts(t *testing.T) {
	spec, _ := retainedWorkflowFixture(t, "backup_synced")
	journal := workflowJournalPath(spec.StateRoot, spec.OperationID)
	m, progress, err := ReadWorkflowJournal(spec.StateRoot, journal)
	if err != nil {
		t.Fatal(err)
	}
	for count := 0; count < 250; count++ {
		progress = append(progress, WorkflowProgress{Stage: "recovery_started", Target: -1})
	}
	if err := saveWorkflow(journal, workflowDisk{m, progress}, false); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(journal)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 3; attempt++ {
		if err := RecoverWorkflows(spec.StateRoot); !errors.Is(err, ErrRecoveryBlocked) {
			t.Fatal(err)
		}
		after, err := os.ReadFile(journal)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("blocked retry grew journal", err)
		}
	}
}

func TestWorkflowRetainedProcessDeath(t *testing.T) {
	if boundary := os.Getenv("AL_RETAINED_CRASH"); boundary != "" {
		root := os.Getenv("AL_TRANSACTION_ROOT")
		_ = RecoverWorkflowsWithPolicy(root, func(WorkflowTarget, []byte, []byte) bool { return true }, func(b WorkflowBoundary) error {
			if b.Stage == boundary {
				os.Exit(84)
			}
			return nil
		})
		os.Exit(85)
	}
	for _, boundary := range []string{"recovery_target_retained", "recovered"} {
		t.Run(boundary, func(t *testing.T) {
			spec, path := retainedWorkflowFixture(t, "backup_synced")
			cmd := exec.Command(os.Args[0], "-test.run=^TestWorkflowRetainedProcessDeath$")
			cmd.Env = append(os.Environ(), "AL_RETAINED_CRASH="+boundary, "AL_TRANSACTION_ROOT="+spec.StateRoot)
			var exitError *exec.ExitError
			if err := cmd.Run(); !errors.As(err, &exitError) || exitError.ExitCode() != 84 {
				t.Fatal("child missed durable crash boundary", err)
			}
			if err := RecoverWorkflowsWithPolicy(spec.StateRoot, func(WorkflowTarget, []byte, []byte) bool { return true }); err != nil {
				t.Fatal(err)
			}
			if actual, err := os.ReadFile(path); err != nil || string(actual) != "current diagnostic" {
				t.Fatal("crash recovery rolled retained target back", err)
			}
		})
	}
}

func TestWorkflowRetentionRefusesMultipleTargets(t *testing.T) {
	spec, paths := workflowFixture(t)
	spec.Targets = []WorkflowTarget{spec.Targets[1], spec.Targets[3]}
	stop := errors.New("synthetic interruption")
	if err := ApplyWorkflow(spec, func(b WorkflowBoundary) error {
		if b.Stage == "backup_synced" && b.Target == 0 {
			return stop
		}
		return nil
	}); !errors.Is(err, stop) {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths[1], []byte("current diagnostic"), 0o600); err != nil {
		t.Fatal(err)
	}
	policy := func(WorkflowTarget, []byte, []byte) bool {
		t.Fatal("multi-target workflow invoked retention policy")
		return true
	}
	if err := RecoverWorkflowsWithPolicy(spec.StateRoot, policy); !errors.Is(err, ErrRecoveryBlocked) {
		t.Fatal(err)
	}
	for _, expected := range []struct{ path, contents string }{{paths[1], "current diagnostic"}, {paths[3], "before"}} {
		if actual, err := os.ReadFile(expected.path); err != nil || string(actual) != expected.contents {
			t.Fatal("multi-target recovery changed files", err)
		}
	}
}

func TestWorkflowRetentionRefusesReplacedParent(t *testing.T) {
	spec, path := retainedWorkflowFixture(t, "backup_synced")
	originalRoot := spec.StateRoot + ".original"
	if err := os.Rename(spec.StateRoot, originalRoot); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(originalRoot) })
	if err := os.Mkdir(spec.StateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	workflows := filepath.Join(spec.StateRoot, "workflows")
	if err := os.Mkdir(workflows, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{spec.OperationID + ".workflow", spec.OperationID + "-0.backup"} {
		contents, err := os.ReadFile(filepath.Join(originalRoot, "workflows", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(workflows, name), contents, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(path, []byte("current diagnostic"), 0o600); err != nil {
		t.Fatal(err)
	}
	policy := func(WorkflowTarget, []byte, []byte) bool {
		t.Fatal("replaced parent invoked retention policy")
		return true
	}
	if err := RecoverWorkflowsWithPolicy(spec.StateRoot, policy); !errors.Is(err, ErrRecoveryBlocked) {
		t.Fatal(err)
	}
	if actual, err := os.ReadFile(path); err != nil || string(actual) != "current diagnostic" {
		t.Fatal("replaced parent recovery changed target", err)
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
