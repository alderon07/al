//go:build !windows

package transaction

import (
	"bytes"
	"errors"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkflowForwardBoundaryRecovery(t *testing.T) {
	for _, stage := range []string{"manifest_synced", "backup_synced", "temporary_synced", "before_rename", "after_rename", "target_synced", "committed"} {
		t.Run(stage, func(t *testing.T) {
			spec, paths := workflowFixture(t)
			stop := errors.New("stop")
			err := ApplyWorkflow(spec, func(b WorkflowBoundary) error {
				if b.Stage == stage {
					return stop
				}
				return nil
			})
			if !errors.Is(err, stop) {
				t.Fatalf("boundary: %v", err)
			}
			if err := RecoverWorkflows(spec.StateRoot); err != nil {
				t.Fatal(err)
			}
			if err := RecoverWorkflows(spec.StateRoot); err != nil {
				t.Fatal(err)
			}
			want := "before"
			if stage == "committed" {
				want = "after"
			}
			for _, p := range paths {
				b, e := os.ReadFile(p)
				if e != nil || string(b) != want {
					t.Fatalf("target=%q error=%v", b, e)
				}
			}
		})
	}
}
func TestWorkflowLeafSymlinkAndStaleInput(t *testing.T) {
	spec, paths := workflowFixture(t)
	link := filepath.Join(filepath.Dir(paths[0]), "link")
	if e := os.Symlink("native", link); e != nil {
		t.Fatal(e)
	}
	spec.Targets = spec.Targets[:1]
	spec.Targets[0].Path = link
	spec.Targets[0].PreserveSymlink = true
	if e := ApplyWorkflow(spec); e != nil {
		t.Fatal(e)
	}
	if got, e := os.Readlink(link); e != nil || got != "native" {
		t.Fatalf("link=%q error=%v", got, e)
	}
	spec.OperationID = "stale"
	if e := ApplyWorkflow(spec); !errors.Is(e, ErrIdentityChanged) {
		t.Fatalf("stale=%v", e)
	}
}
func TestWorkflowRejectsHardlinksAndParentLinks(t *testing.T) {
	spec, paths := workflowFixture(t)
	if e := os.Link(paths[0], paths[0]+".hard"); e != nil {
		t.Fatal(e)
	}
	if _, e := InspectWorkflowTarget(paths[0], 1024, false); !errors.Is(e, ErrUnsafePath) {
		t.Fatal(e)
	}
	parent := filepath.Join(spec.StateRoot, "parent-link")
	if e := os.Symlink(filepath.Dir(paths[0]), parent); e != nil {
		t.Fatal(e)
	}
	if _, e := InspectWorkflowTarget(filepath.Join(parent, "native"), 1024, false); !errors.Is(e, ErrUnsafePath) {
		t.Fatal(e)
	}
}
func TestWorkflowPromotionRejectsChangedReviewedTree(t *testing.T) {
	root := privateTestRoot(t)
	source := filepath.Join(root, "stage")
	os.Mkdir(source, 0o700)
	file := filepath.Join(source, "catalog")
	os.WriteFile(file, []byte("old"), 0o600)
	expected, e := InspectWorkflowDirectory(source)
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(file, []byte("changed"), 0o600)
	spec := WorkflowSpec{OperationID: "stale-promotion", StateRoot: root, Targets: []WorkflowTarget{{Path: filepath.Join(root, "repository"), Role: WorkflowRepository, Mode: 0o700, PromotionSource: source, PromotionExpected: expected}}}
	if e = ApplyWorkflow(spec); !errors.Is(e, ErrIdentityChanged) {
		t.Fatalf("stale promotion: %v", e)
	}
}
func TestWorkflowExtendedMetadataIsRefused(t *testing.T) {
	spec, paths := workflowFixture(t)
	if e := unix.Setxattr(paths[0], "user.alias_lens_test", []byte("metadata"), 0); e != nil {
		if errors.Is(e, unix.ENOTSUP) || errors.Is(e, unix.EPERM) {
			t.Skip("filesystem does not expose test extended attributes")
		}
		t.Fatal(e)
	}
	if _, e := InspectWorkflowTarget(paths[0], 1024, false); !errors.Is(e, ErrUnsafePath) {
		t.Fatalf("metadata accepted: %v", e)
	}
	if e := ApplyWorkflow(spec); !errors.Is(e, ErrUnsafePath) {
		t.Fatalf("metadata mutation accepted: %v", e)
	}
}
func TestWorkflowTreeAllowsBoundedGitPacks(t *testing.T) {
	root := privateTestRoot(t)
	stage := filepath.Join(root, "stage")
	for _, p := range []string{stage, filepath.Join(stage, ".git"), filepath.Join(stage, ".git", "objects"), filepath.Join(stage, ".git", "objects", "pack")} {
		if e := os.Mkdir(p, 0o700); e != nil {
			t.Fatal(e)
		}
	}
	pack := filepath.Join(stage, ".git", "objects", "pack", "fixture.pack")
	if e := os.WriteFile(pack, bytes.Repeat([]byte("p"), 9<<20), 0o600); e != nil {
		t.Fatal(e)
	}
	if _, e := InspectWorkflowDirectory(stage); e != nil {
		t.Fatalf("bounded pack rejected: %v", e)
	}
	if e := os.Rename(pack, filepath.Join(stage, "catalog")); e != nil {
		t.Fatal(e)
	}
	if _, e := InspectWorkflowDirectory(stage); !errors.Is(e, ErrUnsafePath) {
		t.Fatalf("oversized enrolled file accepted: %v", e)
	}
}
