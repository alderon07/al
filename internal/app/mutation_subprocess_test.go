//go:build !windows

package app

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	workflowplan "github.com/alderon07/al/internal/plan"
	"github.com/alderon07/al/internal/transaction"
	"os"

	"path/filepath"

	"strings"
	"testing"
)

func mutationInventory(t *testing.T, home string) string {
	t.Helper()
	h := sha256.New()
	e := filepath.WalkDir(home, func(path string, entry os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		info, e := os.Lstat(path)
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(home, path)
		if e != nil {
			return e
		}
		fmt.Fprintf(h, "%q %d %d\n", rel, info.Mode(), info.Size())
		if info.Mode().IsRegular() {
			b, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			h.Write(b)
		} else if info.Mode()&os.ModeSymlink != 0 {
			link, e := os.Readlink(path)
			if e != nil {
				return e
			}
			h.Write([]byte(link))
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func TestRecoveryObservationPreservesWorkflowAndStageInventory(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	if e := DefaultServices().withMutation(func(*mutationSession) error { return nil }); e != nil {
		t.Fatal(e)
	}
	root := filepath.Join(home, ".local", "state", "alias-lens")
	target := filepath.Join(home, ".config", "alias-lens", "fixture")
	interrupted := errors.New("boundary")
	spec := transaction.WorkflowSpec{OperationID: "op-11111111111111111111111111111111", StateRoot: root, PrivateRoots: []string{filepath.Dir(target)}, Targets: []transaction.WorkflowTarget{{Path: target, Role: transaction.WorkflowPrivate, Mode: 0o600, Planned: []byte("synthetic")}}}
	e := transaction.ApplyWorkflow(spec, func(b transaction.WorkflowBoundary) error {
		if b.Stage == "target_synced" {
			return interrupted
		}
		return nil
	})
	if !errors.Is(e, interrupted) {
		t.Fatal(e)
	}
	stageDir := filepath.Join(root, "catalog-stages")
	if e := os.Mkdir(stageDir, 0o700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(stageDir, "bad.json"), []byte("{}\n"), 0o600); e != nil {
		t.Fatal(e)
	}
	parent := filepath.Join(home, ".local", "share", "alias-lens", "catalog-repos")
	if e := os.MkdirAll(parent, 0o700); e != nil {
		t.Fatal(e)
	}
	stageRoot := filepath.Join(parent, ".stage-synthetic")
	if e := os.Mkdir(stageRoot, 0o700); e != nil {
		t.Fatal(e)
	}
	stageID := "op-22222222222222222222222222222222"
	stageIdentity := observePlanIdentity(stageRoot)
	intent := catalogStageIntent{Version: 1, ID: stageID, Root: stageRoot, Device: stageIdentity.Device, Inode: stageIdentity.Inode, Revision: strings.Repeat("a", 40), Blob: strings.Repeat("b", 40), Destination: filepath.Join(parent, strings.Repeat("c", 64)), CreatedParents: []string{}, ParentIdentities: []workflowplan.Identity{}}
	if e := os.WriteFile(filepath.Join(stageRoot, ".operation"), []byte(stageID+"\n"), 0o600); e != nil {
		t.Fatal(e)
	}
	data, _ := json.MarshalIndent(intent, "", "  ")
	data = append(data, '\n')
	if e := os.WriteFile(filepath.Join(stageDir, stageID+".json"), data, 0o600); e != nil {
		t.Fatal(e)
	}
	if e := os.Remove(filepath.Join(stageDir, "bad.json")); e != nil {
		t.Fatal(e)
	}
	valid := inspectStageRecovery(home)
	if !valid.Required || valid.Blocked {
		t.Fatalf("valid stage rejected: %#v", valid)
	}
	if e := os.WriteFile(filepath.Join(stageDir, "bad.json"), []byte("{}\n"), 0o600); e != nil {
		t.Fatal(e)
	}
	before := mutationInventory(t, home)
	workflow := inspectWorkflowRecovery(root)
	stage := inspectStageRecovery(home)
	if !workflow.Required || workflow.Blocked || !stage.Required || !stage.Blocked {
		t.Fatalf("recovery observations: %#v %#v", workflow, stage)
	}
	if after := mutationInventory(t, home); after != before {
		t.Fatal("observational recovery changed files")
	}
}
