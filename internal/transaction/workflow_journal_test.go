//go:build !windows

package transaction

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"os"
	"testing"
)

func TestWorkflowRejectsInvalidProgressAndEncoding(t *testing.T) {
	for _, corruption := range []string{"committed_target", "premature_commit", "backup_out_of_order", "recovery_without_start", "recovered_without_targets", "duplicate", "capitalized", "missing"} {
		t.Run(corruption, func(t *testing.T) {
			spec, _ := workflowFixture(t)
			stop := errors.New("stop")
			if e := ApplyWorkflow(spec, func(b WorkflowBoundary) error {
				if b.Stage == "manifest_synced" {
					return stop
				}
				return nil
			}); !errors.Is(e, stop) {
				t.Fatal(e)
			}
			path := workflowJournalPath(spec.StateRoot, spec.OperationID)
			m, p, e := ReadWorkflowJournal(spec.StateRoot, path)
			if e != nil {
				t.Fatal(e)
			}
			d := workflowDisk{m, p}
			switch corruption {
			case "committed_target":
				d.Progress = []WorkflowProgress{{Stage: "committed", Target: 0}}
			case "premature_commit":
				d.Progress = []WorkflowProgress{{Stage: "committed", Target: -1}}
			case "backup_out_of_order":
				d.Progress = []WorkflowProgress{{Stage: "backup_synced", Target: 1}}
			case "recovery_without_start":
				d.Progress = []WorkflowProgress{{Stage: "recovery_target_synced", Target: 0, Identity: &FileIdentity{}}}
			case "recovered_without_targets":
				d.Progress = []WorkflowProgress{{Stage: "recovery_started", Target: -1}, {Stage: "recovered", Target: -1}}
			}
			if e = saveWorkflow(path, d, false); e != nil {
				t.Fatal(e)
			}
			if corruption == "duplicate" || corruption == "capitalized" || corruption == "missing" {
				bytesOnDisk, e := os.ReadFile(path)
				if e != nil {
					t.Fatal(e)
				}
				payload := bytesOnDisk[8 : len(bytesOnDisk)-32]
				switch corruption {
				case "duplicate":
					payload = bytes.Replace(payload, []byte(`"version":2`), []byte(`"version":2,"version":2`), 1)
				case "capitalized":
					payload = bytes.Replace(payload, []byte(`"version":2`), []byte(`"Version":2`), 1)
				case "missing":
					payload = bytes.Replace(payload, []byte(`,"progress":[]`), nil, 1)
				}
				digest := sha256.Sum256(payload)
				var head [8]byte
				binary.BigEndian.PutUint64(head[:], uint64(len(payload)))
				result := append(head[:], payload...)
				result = append(result, digest[:]...)
				if e = os.WriteFile(path, result, 0o600); e != nil {
					t.Fatal(e)
				}
			}
			if _, _, e = ReadWorkflowJournal(spec.StateRoot, path); !errors.Is(e, ErrJournalCorrupt) {
				t.Fatalf("accepted corrupt workflow: %v", e)
			}
		})
	}
}
