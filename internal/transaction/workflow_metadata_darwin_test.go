package transaction

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func addTestProvenance(t *testing.T, path string) {
	t.Helper()
	if _, err := unix.Getxattr(path, "com.apple.provenance", nil); err == nil {
		return
	}
	if err := unix.Setxattr(path, "com.apple.provenance", []byte("synthetic provenance"), 0); err != nil {
		if errors.Is(err, unix.EPERM) || errors.Is(err, unix.ENOTSUP) {
			t.Skip("filesystem does not expose provenance for this fixture")
		}
		t.Fatal(err)
	}
}

func TestWorkflowProvenanceRecovery(t *testing.T) {
	for _, stage := range []string{"backup_synced", "temporary_synced", "after_rename", "committed"} {
		t.Run(stage, func(t *testing.T) {
			spec, paths := workflowFixture(t)
			for _, path := range paths {
				addTestProvenance(t, path)
			}
			stop := errors.New("synthetic interruption")
			if err := ApplyWorkflow(spec, func(boundary WorkflowBoundary) error {
				if boundary.Stage == stage {
					return stop
				}
				return nil
			}); !errors.Is(err, stop) {
				t.Fatalf("interrupt: %v", err)
			}
			if err := filepath.WalkDir(spec.StateRoot, func(path string, entry os.DirEntry, err error) error {
				if err == nil {
					addTestProvenance(t, path)
				}
				return err
			}); err != nil {
				t.Fatal(err)
			}
			for attempt := 0; attempt < 2; attempt++ {
				if err := RecoverWorkflows(spec.StateRoot); err != nil {
					t.Fatal(err)
				}
			}
			want := "before"
			if stage == "committed" {
				want = "after"
			}
			for _, path := range paths {
				if data, err := os.ReadFile(path); err != nil || string(data) != want {
					t.Fatalf("recovery contents: %q %v", data, err)
				}
			}
		})
	}
}

func TestWorkflowProvenanceDirectoryAndAppend(t *testing.T) {
	root := privateTestRoot(t)
	path := filepath.Join(root, "usage")
	if err := os.WriteFile(path, []byte("first\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	addTestProvenance(t, root)
	addTestProvenance(t, path)
	if err := AppendWorkflowPrivateFile(path, []byte("second\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectWorkflowDirectoryMetadata(root); err != nil {
		t.Fatal(err)
	}
	if err := workflowDirectoryMetadata(root); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectWorkflowDirectory(root); err != nil {
		t.Fatal(err)
	}
	if err := unix.Setxattr(path, "user.alias_lens_test", []byte("synthetic metadata"), 0); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectWorkflowDirectory(root); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("unknown metadata accepted: %v", err)
	}
	if err := AppendWorkflowPrivateFile(path, []byte("third\n")); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("unknown metadata append accepted: %v", err)
	}
}

func TestWorkflowDarwinSystemParents(t *testing.T) {
	for _, parent := range []string{"/tmp", "/var/tmp"} {
		t.Run(parent, func(t *testing.T) {
			root, err := os.MkdirTemp(parent, "alias-lens-test-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.RemoveAll(root) })
			path := filepath.Join(root, "state")
			spec := WorkflowSpec{OperationID: "system-parent", StateRoot: root, Targets: []WorkflowTarget{{Path: path, Role: WorkflowPrivate, Planned: []byte("synthetic"), Mode: 0o600}}}
			if err := ApplyWorkflow(spec); err != nil {
				t.Fatal(err)
			}
			if _, data, err := ReadWorkflowTarget(path, 1024); err != nil || string(data) != "synthetic" {
				t.Fatalf("read: %q %v", data, err)
			}
		})
	}
}
