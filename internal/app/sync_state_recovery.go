package app

import (
	"bytes"
	"encoding/json"
	"io"
	"path/filepath"
	"unicode/utf8"

	"github.com/alderon07/al/internal/transaction"
)

func (s *mutationSession) recoverWorkflows() error {
	return transaction.RecoverWorkflowsWithPolicy(s.stateRoot, s.retainSyncDiagnosticState)
}

func (s *mutationSession) retainSyncDiagnosticState(target transaction.WorkflowTarget, original, current []byte) bool {
	if target.Path != filepath.Join(s.stateRoot, "sync-state.json") {
		return false
	}
	before, ok := decodeRecoverySyncState(original)
	if !ok {
		return false
	}
	after, ok := decodeRecoverySyncState(current)
	return ok && before.LocalHash == after.LocalHash && before.RemoteHash == after.RemoteHash && (before.Status == "conflict") == (after.Status == "conflict")
}

func decodeRecoverySyncState(contents []byte) (SyncState, bool) {
	var state SyncState
	if len(contents) > 1<<20 || !utf8.Valid(contents) {
		return state, false
	}
	d := json.NewDecoder(bytes.NewReader(contents))
	start, err := d.Token()
	if err != nil || start != json.Delim('{') {
		return state, false
	}
	seen := map[string]bool{}
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return state, false
		}
		key, ok := token.(string)
		if !ok || seen[key] {
			return state, false
		}
		seen[key] = true
		switch key {
		case "local_hash", "remote_hash", "status", "message", "updated_at":
		default:
			return state, false
		}
		var raw json.RawMessage
		if d.Decode(&raw) != nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return state, false
		}
		var value string
		if json.Unmarshal(raw, &value) != nil {
			return state, false
		}
		if key == "updated_at" {
			if json.Unmarshal(raw, &state.UpdatedAt) != nil {
				return state, false
			}
			canonical, err := state.UpdatedAt.MarshalJSON()
			if err != nil || !bytes.Equal(raw, canonical) {
				return state, false
			}
		}
	}
	end, err := d.Token()
	if err != nil || end != json.Delim('}') || d.Decode(&struct{}{}) != io.EOF || !seen["status"] || !seen["updated_at"] {
		return state, false
	}
	if json.Unmarshal(contents, &state) != nil || !validRecoverySyncHash(state.LocalHash) || !validRecoverySyncHash(state.RemoteHash) {
		return SyncState{}, false
	}
	switch state.Status {
	case "waiting", "synced", "pushed", "pulled", "conflict", "offline", "stopped":
		return state, true
	default:
		return SyncState{}, false
	}
}

func validRecoverySyncHash(value string) bool {
	if value == "" {
		return true
	}
	if len(value) != 64 {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
