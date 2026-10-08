package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	workflowplan "alias-lens/internal/plan"
	"alias-lens/internal/transaction"
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
	if e := saveConfig(defaultConfig()); e != nil {
		t.Fatal(e)
	}
	first, e := loadConfig()
	if e != nil {
		t.Fatal(e)
	}
	second, e := loadConfig()
	if e != nil {
		t.Fatal(e)
	}
	first.Footer.Message = "updated"
	if e = saveConfig(first); e != nil {
		t.Fatal(e)
	}
	second.ShortcutProfile = "linux"
	if e = saveConfig(second); e != nil {
		t.Fatal(e)
	}
	actual, e := loadConfig()
	if e != nil {
		t.Fatal(e)
	}
	if actual.Footer.Message != "updated" || actual.ShortcutProfile != "linux" {
		t.Fatal("independent config changes were overwritten")
	}
	first, e = loadConfig()
	if e != nil {
		t.Fatal(e)
	}
	second, e = loadConfig()
	if e != nil {
		t.Fatal(e)
	}
	first.Footer.Message = "first"
	second.Footer.Message = "second"
	if e = saveConfig(first); e != nil {
		t.Fatal(e)
	}
	if e = saveConfig(second); e == nil {
		t.Fatal("conflicting concurrent field was overwritten")
	}
}
func TestManagedWritersShareStateLock(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	if e := withMutation(func(session *mutationSession) error { return nil }); e != nil {
		t.Fatal(e)
	}
	state := filepath.Join(home, ".local", "state", "alias-lens")
	lock, e := transaction.AcquireLock(state, filepath.Join(state, "mutation.lock"))
	if e != nil {
		t.Fatal(e)
	}
	defer lock.Close()
	for name, work := range map[string]func() error{"config": func() error { return saveConfig(defaultConfig()) }, "theme": func() error { return saveTheme(defaultTheme()) }, "tour": scheduleTour, "usage": func() error { return recordAliasUse("fixture") }, "context": func() error { return updateContextFile(func(*contextFile) (bool, error) { return false, nil }) }, "clear": func() error { return runDataCommand([]string{"clear-usage"}) }} {
		if e := work(); !errors.Is(e, transaction.ErrLocked) {
			t.Fatalf("%s ignored state lock", name)
		}
	}
}
func TestRevisionClearRefusesLinks(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	outside := filepath.Join(home, "outside")
	os.WriteFile(outside, []byte("retained"), 0o600)
	directory := filepath.Join(home, ".local", "share", "alias-lens", "revisions")
	os.MkdirAll(directory, 0o700)
	os.Symlink(outside, filepath.Join(directory, "revision"))
	if e := runDataCommand([]string{"clear-revisions"}); e == nil {
		t.Fatal("revision symlink accepted")
	}
	if b, e := os.ReadFile(outside); e != nil || string(b) != "retained" {
		t.Fatal("revision clear changed external file")
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
	if e := applyMutationPlan(preview, build); !errors.Is(e, workflowplan.ErrStalePlan) {
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
	if e := applyMutationPlan(preview, build); e != nil {
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
