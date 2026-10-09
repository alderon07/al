package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alderon07/al/internal/transaction"
)

func TestRecoverySyncStateStrictCodec(t *testing.T) {
	valid := `{"local_hash":"` + strings.Repeat("a", 64) + `","remote_hash":"","status":"offline","message":"synthetic private diagnostic","updated_at":"2026-10-09T00:00:00Z"}`
	for _, contents := range []string{valid, `{"status":"waiting","updated_at":"2026-10-09T00:00:00Z"}`, `{"status":"offline","updated_at":"2026-10-09T12:34:56.123456789-04:00"}`, `{"status":"waiting","updated_at":"0001-01-01T00:00:00Z"}`} {
		if _, ok := decodeRecoverySyncState([]byte(contents)); !ok {
			t.Fatal("valid record rejected")
		}
	}
	cases := map[string]string{
		"unknown":             strings.Replace(valid, `"message"`, `"unknown"`, 1),
		"case_variant":        strings.Replace(valid, `"status"`, `"Status"`, 1),
		"duplicate":           strings.Replace(valid, `"status":"offline"`, `"status":"conflict","status":"offline"`, 1),
		"null_field":          strings.Replace(valid, `"offline"`, `null`, 1),
		"null_timestamp":      strings.Replace(valid, `"2026-10-09T00:00:00Z"`, `null`, 1),
		"null_hash":           strings.Replace(valid, `"remote_hash":""`, `"remote_hash":null`, 1),
		"null_message":        strings.Replace(valid, `"synthetic private diagnostic"`, `null`, 1),
		"numeric_message":     strings.Replace(valid, `"synthetic private diagnostic"`, `42`, 1),
		"null_object":         `null`,
		"array":               `[]`,
		"mistyped":            strings.Replace(valid, `"offline"`, `false`, 1),
		"missing_status":      `{"updated_at":"2026-10-09T00:00:00Z"}`,
		"missing_timestamp":   `{"status":"offline"}`,
		"trailing_document":   valid + `{}`,
		"unsupported_status":  strings.Replace(valid, `"offline"`, `"unsupported"`, 1),
		"bad_timestamp":       strings.Replace(valid, `2026-10-09T00:00:00Z`, `yesterday`, 1),
		"short_hour":          strings.Replace(valid, `2026-10-09T00:00:00Z`, `2026-10-09T0:00:00Z`, 1),
		"comma_fraction":      strings.Replace(valid, `2026-10-09T00:00:00Z`, `2026-10-09T00:00:00,1Z`, 1),
		"invalid_zone_hour":   strings.Replace(valid, `2026-10-09T00:00:00Z`, `2026-10-09T00:00:00+24:00`, 1),
		"invalid_zone_minute": strings.Replace(valid, `2026-10-09T00:00:00Z`, `2026-10-09T00:00:00+00:60`, 1),
		"short_hash":          strings.Replace(valid, strings.Repeat("a", 64), "abc", 1),
		"uppercase_hash":      strings.Replace(valid, strings.Repeat("a", 64), strings.Repeat("A", 64), 1),
		"malformed":           valid[:len(valid)-1],
		"oversized":           strings.Repeat(" ", (1<<20)+1),
		"invalid_utf8":        strings.Replace(valid, "synthetic", string([]byte{0xff}), 1),
	}
	for name, contents := range cases {
		t.Run(name, func(t *testing.T) {
			if _, ok := decodeRecoverySyncState([]byte(contents)); ok {
				t.Fatal("unsafe record accepted")
			}
		})
	}
}

func TestSyncDiagnosticRecoveryConflictAuthority(t *testing.T) {
	for _, status := range []string{"conflict", "offline"} {
		t.Run(status, func(t *testing.T) {
			original := SyncState{Status: "conflict", UpdatedAt: time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)}
			current := original
			current.Status = status
			current.Message = "synthetic updated diagnostic"
			_, path, _, contents := syncRecoveryFixture(t, "sync-state.json", original, current)
			err := DefaultServices().RecoverWorkflows()
			if status == "conflict" && err != nil {
				t.Fatal(err)
			}
			if status != "conflict" && !errors.Is(err, transaction.ErrRecoveryBlocked) {
				t.Fatal("conflict authority cleared", err)
			}
			actual, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(actual, contents) {
				t.Fatal("conflict recovery changed current bytes", err)
			}
		})
	}
}

func TestSyncDiagnosticPolicyPreservesOrdinaryBaselineWithoutBackup(t *testing.T) {
	original := SyncState{Status: "waiting", UpdatedAt: time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)}
	_, _, backup, _ := syncRecoveryFixture(t, "other-state.json", original, original)
	if err := os.Remove(backup); err != nil {
		t.Fatal(err)
	}
	if err := DefaultServices().RecoverWorkflows(); err != nil {
		t.Fatal(err)
	}
}

func syncRecoveryFixture(t *testing.T, name string, original, current SyncState) (transaction.WorkflowSpec, string, string, []byte) {
	t.Helper()
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	if err := DefaultServices().withMutation(func(*mutationSession) error { return nil }); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(home, ".local", "state", "alias-lens")
	path := filepath.Join(root, name)
	before, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	id, err := transaction.InspectWorkflowTarget(path, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	planned := original
	planned.Message = "planned diagnostic"
	contents, err := json.Marshal(planned)
	if err != nil {
		t.Fatal(err)
	}
	spec := transaction.WorkflowSpec{OperationID: "sync-diagnostic", StateRoot: root, Targets: []transaction.WorkflowTarget{{Path: path, Role: transaction.WorkflowPrivate, Expected: id, Planned: contents, Mode: 0o600}}}
	stop := errors.New("synthetic interruption")
	if err = transaction.ApplyWorkflow(spec, func(b transaction.WorkflowBoundary) error {
		if b.Stage == "backup_synced" {
			return stop
		}
		return nil
	}); !errors.Is(err, stop) {
		t.Fatal(err)
	}
	after, err := json.Marshal(current)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, after, 0o600); err != nil {
		t.Fatal(err)
	}
	return spec, path, filepath.Join(root, "workflows", "sync-diagnostic-0.backup"), after
}

func TestSyncDiagnosticRecoveryPreservesCurrentAndOriginalBackup(t *testing.T) {
	original := SyncState{LocalHash: strings.Repeat("a", 64), RemoteHash: strings.Repeat("b", 64), Status: "synced", Message: "original diagnostic", UpdatedAt: time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)}
	current := original
	current.Status = "offline"
	current.Message = "synthetic private current diagnostic"
	current.UpdatedAt = current.UpdatedAt.Add(time.Minute)
	spec, path, backup, contents := syncRecoveryFixture(t, "sync-state.json", original, current)
	before, err := transaction.InspectWorkflowTarget(path, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	backupBefore, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := DefaultServices().RecoverWorkflows(); err != nil {
			t.Fatal(err)
		}
	}
	after, err := transaction.InspectWorkflowTarget(path, 0, false)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("retention changed file identity", err)
	}
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, contents) {
		t.Fatal("retention changed current bytes", err)
	}
	actual, err = os.ReadFile(backup)
	if err != nil || !bytes.Equal(actual, backupBefore) {
		t.Fatal("retention changed backup", err)
	}
	if _, err := os.Lstat(filepath.Join(spec.StateRoot, "workflows", spec.OperationID+".workflow")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("completed journal remains", err)
	}
}

func TestSyncDiagnosticRecoveryRefusesAuthorityAndOtherTargets(t *testing.T) {
	original := SyncState{LocalHash: strings.Repeat("a", 64), RemoteHash: strings.Repeat("b", 64), Status: "synced", UpdatedAt: time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)}
	for _, name := range []string{"local_hash", "remote_hash", "conflict", "other_path", "bad_backup", "bad_current"} {
		t.Run(name, func(t *testing.T) {
			current := original
			current.Message = "synthetic private diagnostic"
			target := "sync-state.json"
			switch name {
			case "local_hash":
				current.LocalHash = strings.Repeat("c", 64)
			case "remote_hash":
				current.RemoteHash = strings.Repeat("c", 64)
			case "conflict":
				current.Status = "conflict"
			case "other_path":
				target = "other-state.json"
			}
			spec, path, backup, contents := syncRecoveryFixture(t, target, original, current)
			if name == "bad_backup" {
				if err := os.WriteFile(backup, []byte("corrupt synthetic backup"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if name == "bad_current" {
				contents = []byte(`{"status":"offline","updated_at":null}`)
				if err := os.WriteFile(path, contents, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			backupBefore, err := os.ReadFile(backup)
			if err != nil {
				t.Fatal(err)
			}
			journal := filepath.Join(spec.StateRoot, "workflows", spec.OperationID+".workflow")
			var first []byte
			for attempt := 0; attempt < 2; attempt++ {
				err := DefaultServices().RecoverWorkflows()
				if !errors.Is(err, transaction.ErrRecoveryBlocked) || strings.Contains(err.Error(), current.Message) || strings.Contains(err.Error(), spec.StateRoot) {
					t.Fatal("unsafe recovery diagnostic", err)
				}
				progress, err := os.ReadFile(journal)
				if err != nil {
					t.Fatal(err)
				}
				if attempt == 0 {
					first = progress
				} else if !bytes.Equal(first, progress) {
					t.Fatal("blocked retry grew journal")
				}
			}
			actual, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(actual, contents) {
				t.Fatal("blocked recovery changed current bytes", err)
			}
			actual, err = os.ReadFile(backup)
			if err != nil || !bytes.Equal(actual, backupBefore) {
				t.Fatal("blocked recovery changed backup", err)
			}
		})
	}
}
