package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	workflowplan "github.com/alderon07/al/internal/plan"
)

func privateTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if e := os.Chmod(home, 0o700); e != nil {
		t.Fatal(e)
	}
	return home
}
func TestConfigConcurrentFieldsMergeAndConflictsRefuse(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	if e := DefaultServices().saveConfig(DefaultServices().defaultConfig()); e != nil {
		t.Fatal(e)
	}
	first, e := DefaultServices().loadConfig()
	if e != nil {
		t.Fatal(e)
	}
	second, e := DefaultServices().loadConfig()
	if e != nil {
		t.Fatal(e)
	}
	first.Footer.Message = "updated"
	if e = DefaultServices().saveConfig(first); e != nil {
		t.Fatal(e)
	}
	second.ShortcutProfile = "linux"
	if e = DefaultServices().saveConfig(second); e != nil {
		t.Fatal(e)
	}
	actual, e := DefaultServices().loadConfig()
	if e != nil {
		t.Fatal(e)
	}
	if actual.Footer.Message != "updated" || actual.ShortcutProfile != "linux" {
		t.Fatal("independent config changes were overwritten")
	}
	first, e = DefaultServices().loadConfig()
	if e != nil {
		t.Fatal(e)
	}
	second, e = DefaultServices().loadConfig()
	if e != nil {
		t.Fatal(e)
	}
	first.Footer.Message = "first"
	second.Footer.Message = "second"
	if e = DefaultServices().saveConfig(first); e != nil {
		t.Fatal(e)
	}
	if e = DefaultServices().saveConfig(second); e == nil {
		t.Fatal("conflicting concurrent field was overwritten")
	}
}

func TestPlannedUserSymlinkPinsLeafAndReferent(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	first := filepath.Join(home, "first")
	second := filepath.Join(home, "second")
	leaf := filepath.Join(home, ".bash_aliases")
	for _, p := range []string{first, second} {
		if e := os.WriteFile(p, []byte("before"), 0o640); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.Symlink("first", leaf); e != nil {
		t.Skip(e)
	}
	target, e := plannedUserTarget(leaf, []byte("before"), []byte("after"))
	if e != nil {
		t.Fatal(e)
	}
	build := func() (workflowplan.OperationPlan, error) {
		return workflowplan.Build("fixture", nil, []workflowplan.Action{{Sequence: 1, Kind: workflowplan.ActionReplace, TargetRole: "native_alias_file", DisplayPath: "~/.bash_aliases", Reason: "fixture", Risk: workflowplan.RiskLow, Reversible: true, Target: target}}, nil, nil), nil
	}
	preview, _ := build()
	if e := os.Remove(leaf); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink("second", leaf); e != nil {
		t.Fatal(e)
	}
	if e := DefaultServices().applyMutationPlan(preview, build); !errors.Is(e, workflowplan.ErrStalePlan) {
		t.Fatal("retargeted leaf accepted")
	}
	for _, p := range []string{first, second} {
		b, e := os.ReadFile(p)
		if e != nil || string(b) != "before" {
			t.Fatal("referent changed", e)
		}
	}
	target, e = plannedUserTarget(leaf, []byte("before"), []byte("after"))
	if e != nil {
		t.Fatal(e)
	}
	preview, _ = build()
	if e := DefaultServices().applyMutationPlan(preview, build); e != nil {
		t.Fatal(e)
	}
	link, e := os.Readlink(leaf)
	if e != nil || link != "second" {
		t.Fatal("leaf not preserved", e)
	}
	b, e := os.ReadFile(second)
	if e != nil || string(b) != "after" {
		t.Fatal("referent not changed", e)
	}
}
