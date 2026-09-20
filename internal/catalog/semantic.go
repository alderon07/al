package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

const SemanticDiffSchemaVersion = 1

type SemanticDiffReport struct {
	SchemaVersion int                  `json:"schema_version"`
	Source        string               `json:"source"`
	Shell         string               `json:"shell,omitempty"`
	Changes       []SemanticChange     `json:"changes"`
	Diagnostics   []SemanticDiagnostic `json:"diagnostics"`
	Summary       SemanticSummary      `json:"summary"`
}

type SemanticChange struct {
	Scope        string `json:"scope"`
	EntryID      string `json:"entry_id,omitempty"`
	Name         string `json:"name,omitempty"`
	Kind         string `json:"kind"`
	Path         string `json:"path"`
	BeforeSHA256 string `json:"before_sha256,omitempty"`
	BeforeBytes  int    `json:"before_bytes,omitempty"`
	AfterSHA256  string `json:"after_sha256,omitempty"`
	AfterBytes   int    `json:"after_bytes,omitempty"`
}

type SemanticDiagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type SemanticSummary struct {
	Added   int `json:"added"`
	Deleted int `json:"deleted"`
	Changed int `json:"changed"`
}

var semanticFieldOrder = []string{
	"name", "kind", "description", "category", "tags", "platforms", "favorite",
	"portable", "native.bash", "native.zsh", "when.profiles_any", "when.profiles_none", "when.shells",
}

// SemanticDiff compares catalog intent by stable entry identity. It never
// renders, validates with a shell, or executes catalog content.
func SemanticDiff(before, after Catalog, source, shell string) SemanticDiffReport {
	report := SemanticDiffReport{SchemaVersion: SemanticDiffSchemaVersion, Source: source, Shell: shell, Changes: []SemanticChange{}, Diagnostics: []SemanticDiagnostic{}}
	if source != "repository" && source != "installed" {
		report.Diagnostics = append(report.Diagnostics, SemanticDiagnostic{Code: "invalid_source", Message: "Choose repository or installed as the comparison source."})
		return report
	}
	if source == "installed" && shell != "bash" && shell != "zsh" {
		report.Diagnostics = append(report.Diagnostics, SemanticDiagnostic{Code: "invalid_shell", Message: "Choose Bash or Zsh for an installed comparison."})
		return report
	}
	if diagnostics := Validate(before); len(diagnostics) > 0 {
		report.Diagnostics = append(report.Diagnostics, SemanticDiagnostic{Code: "invalid_before", Message: "Alias Lens cannot compare the first catalog because it is not valid."})
	}
	if diagnostics := Validate(after); len(diagnostics) > 0 {
		report.Diagnostics = append(report.Diagnostics, SemanticDiagnostic{Code: "invalid_after", Message: "Alias Lens cannot compare the second catalog because it is not valid."})
	}
	if len(report.Diagnostics) > 0 {
		return report
	}
	before, after = Normalize(before), Normalize(after)
	if before.SchemaVersion != after.SchemaVersion {
		report.Changes = append(report.Changes, semanticChange("catalog", "", "", "changed", "schema_version", before.SchemaVersion, after.SchemaVersion))
		report.Summary.Changed++
	}
	left, right := entriesByID(before.Entries), entriesByID(after.Entries)
	ids := make([]string, 0, len(left)+len(right))
	seen := map[string]bool{}
	for id := range left {
		ids = append(ids, id)
		seen[id] = true
	}
	for id := range right {
		if !seen[id] {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		oldEntry, oldOK := left[id]
		newEntry, newOK := right[id]
		switch {
		case !oldOK:
			report.Changes = append(report.Changes, semanticChange("entry", id, newEntry.Name, "added", "entry", nil, newEntry))
			report.Summary.Added++
		case !newOK:
			report.Changes = append(report.Changes, semanticChange("entry", id, oldEntry.Name, "deleted", "entry", oldEntry, nil))
			report.Summary.Deleted++
		default:
			changed := false
			for _, path := range semanticFieldOrder {
				oldValue, newValue := semanticField(oldEntry, path), semanticField(newEntry, path)
				if equalJSON(oldValue, newValue) {
					continue
				}
				name := newEntry.Name
				if name == "" {
					name = oldEntry.Name
				}
				report.Changes = append(report.Changes, semanticChange("entry", id, name, "changed", path, oldValue, newValue))
				changed = true
			}
			if changed {
				report.Summary.Changed++
			}
		}
	}
	return report
}

func entriesByID(entries []Entry) map[string]Entry {
	result := make(map[string]Entry, len(entries))
	for _, entry := range entries {
		result[entry.ID] = entry
	}
	return result
}

func semanticField(entry Entry, path string) any {
	switch path {
	case "name":
		return entry.Name
	case "kind":
		return entry.Kind
	case "description":
		return entry.Description
	case "category":
		return entry.Category
	case "tags":
		return entry.Tags
	case "platforms":
		return entry.Platforms
	case "favorite":
		return entry.Favorite
	case "portable":
		return entry.Portable
	case "native.bash":
		value, ok := entry.Native["bash"]
		if !ok {
			return nil
		}
		return value
	case "native.zsh":
		value, ok := entry.Native["zsh"]
		if !ok {
			return nil
		}
		return value
	case "when.profiles_any":
		if entry.When != nil {
			return entry.When.ProfilesAny
		}
	case "when.profiles_none":
		if entry.When != nil {
			return entry.When.ProfilesNone
		}
	case "when.shells":
		if entry.When != nil {
			return entry.When.Shells
		}
	}
	return nil
}

func semanticChange(scope, id, name, kind, path string, before, after any) SemanticChange {
	change := SemanticChange{Scope: scope, EntryID: id, Name: name, Kind: kind, Path: path}
	if before != nil {
		encoded, _ := json.Marshal(before)
		sum := sha256.Sum256(encoded)
		change.BeforeSHA256 = hex.EncodeToString(sum[:])
		change.BeforeBytes = len(encoded)
	}
	if after != nil {
		encoded, _ := json.Marshal(after)
		sum := sha256.Sum256(encoded)
		change.AfterSHA256 = hex.EncodeToString(sum[:])
		change.AfterBytes = len(encoded)
	}
	return change
}

func equalJSON(left, right any) bool {
	a, _ := json.Marshal(left)
	b, _ := json.Marshal(right)
	return string(a) == string(b)
}
