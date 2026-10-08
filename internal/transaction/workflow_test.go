//go:build !windows

package transaction

import (
	"os"
	"path/filepath"
	"testing"
)

func workflowFixture(t *testing.T) (WorkflowSpec, []string) {
	t.Helper()
	root := privateTestRoot(t)
	user := t.TempDir()
	os.Chmod(user, 0o700)
	paths := []string{filepath.Join(user, "native"), filepath.Join(root, "pointer"), filepath.Join(user, "startup"), filepath.Join(root, "state")}
	spec := WorkflowSpec{OperationID: "workflow-test", StateRoot: root}
	for i, p := range paths {
		mode := os.FileMode(0o600)
		role := WorkflowPrivate
		if i == 0 || i == 2 {
			mode = 0o644
			role = WorkflowUser
		}
		if err := os.WriteFile(p, []byte("before"), mode); err != nil {
			t.Fatal(err)
		}
		id, e := InspectWorkflowTarget(p, 1024, false)
		if e != nil {
			t.Fatal(e)
		}
		spec.Targets = append(spec.Targets, WorkflowTarget{Path: p, Role: role, Expected: id, Planned: []byte("after"), Mode: mode, RecoveryOrder: i})
	}
	return spec, paths
}
