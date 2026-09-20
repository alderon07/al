package catalog

import (
	"encoding/json"
	"fmt"
	"sort"
)

type MergeConflict struct {
	EntryID string `json:"entry_id,omitempty"`
	Name    string `json:"name,omitempty"`
	Path    string `json:"path"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

type MergeResult struct {
	Catalog   *Catalog        `json:"catalog,omitempty"`
	Conflicts []MergeConflict `json:"conflicts"`
}

// ThreeWayMerge combines only non-overlapping semantic changes. A conflict
// returns no catalog, which lets callers leave the live file unchanged.
func ThreeWayMerge(base, local, remote Catalog) MergeResult {
	result := MergeResult{Conflicts: []MergeConflict{}}
	if base.SchemaVersion != local.SchemaVersion || base.SchemaVersion != remote.SchemaVersion {
		result.Conflicts = append(result.Conflicts, MergeConflict{Path: "schema_version", Kind: "version", Message: "Catalog data formats differ. Both catalogs must use the current format."})
		return result
	}
	for _, item := range []struct {
		label string
		value Catalog
	}{{"base", base}, {"local", local}, {"remote", remote}} {
		if diagnostics := Validate(item.value); len(diagnostics) > 0 {
			result.Conflicts = append(result.Conflicts, MergeConflict{Path: "catalog", Kind: "invalid", Message: fmt.Sprintf("The %s catalog is not valid, so no files were changed.", item.label)})
		}
	}
	if len(result.Conflicts) > 0 {
		return result
	}
	base, local, remote = Normalize(base), Normalize(local), Normalize(remote)
	bm, lm, rm := entriesByID(base.Entries), entriesByID(local.Entries), entriesByID(remote.Entries)
	ids := unionIDs(bm, lm, rm)
	merged := Catalog{SchemaVersion: base.SchemaVersion, Entries: []Entry{}}
	for _, id := range ids {
		baseEntry, inBase := bm[id]
		localEntry, inLocal := lm[id]
		remoteEntry, inRemote := rm[id]
		switch {
		case !inBase:
			switch {
			case inLocal && inRemote && equalJSON(localEntry, remoteEntry):
				merged.Entries = append(merged.Entries, localEntry)
			case inLocal && !inRemote:
				merged.Entries = append(merged.Entries, localEntry)
			case !inLocal && inRemote:
				merged.Entries = append(merged.Entries, remoteEntry)
			case inLocal && inRemote:
				result.Conflicts = append(result.Conflicts, entryConflict(id, localEntry.Name, "addition", "This entry was added differently on both sides."))
			}
		case !inLocal && !inRemote:
			// Both sides deleted the entry.
		case !inLocal:
			if equalJSON(baseEntry, remoteEntry) {
				continue
			}
			result.Conflicts = append(result.Conflicts, entryConflict(id, baseEntry.Name, "delete_edit", "This entry was deleted locally and changed remotely."))
		case !inRemote:
			if equalJSON(baseEntry, localEntry) {
				continue
			}
			result.Conflicts = append(result.Conflicts, entryConflict(id, baseEntry.Name, "delete_edit", "This entry was changed locally and deleted remotely."))
		default:
			entry := baseEntry
			entryConflictFound := false
			for _, path := range semanticFieldOrder {
				baseValue := semanticField(baseEntry, path)
				localValue := semanticField(localEntry, path)
				remoteValue := semanticField(remoteEntry, path)
				localChanged := !equalJSON(baseValue, localValue)
				remoteChanged := !equalJSON(baseValue, remoteValue)
				switch {
				case localChanged && remoteChanged && !equalJSON(localValue, remoteValue):
					result.Conflicts = append(result.Conflicts, MergeConflict{EntryID: id, Name: localEntry.Name, Path: path, Kind: "field", Message: "This field changed differently on both sides."})
					entryConflictFound = true
				case localChanged:
					setSemanticField(&entry, path, localValue)
				case remoteChanged:
					setSemanticField(&entry, path, remoteValue)
				}
			}
			if !entryConflictFound {
				merged.Entries = append(merged.Entries, entry)
			}
		}
	}
	if len(result.Conflicts) > 0 {
		return result
	}
	merged = Normalize(merged)
	names := map[string]string{}
	for _, entry := range merged.Entries {
		if otherID, exists := names[entry.Name]; exists && otherID != entry.ID {
			result.Conflicts = append(result.Conflicts, MergeConflict{EntryID: entry.ID, Name: entry.Name, Path: "name", Kind: "name", Message: "Different entries would have the same name."})
			return result
		}
		names[entry.Name] = entry.ID
	}
	if diagnostics := Validate(merged); len(diagnostics) > 0 {
		result.Conflicts = append(result.Conflicts, MergeConflict{Path: "catalog", Kind: "invalid_result", Message: "The combined catalog is not valid, so no files were changed."})
		return result
	}
	result.Catalog = &merged
	return result
}

func unionIDs(values ...map[string]Entry) []string {
	seen := map[string]bool{}
	for _, entries := range values {
		for id := range entries {
			seen[id] = true
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func entryConflict(id, name, kind, message string) MergeConflict {
	return MergeConflict{EntryID: id, Name: name, Path: "entry", Kind: kind, Message: message}
}

func setSemanticField(entry *Entry, path string, value any) {
	// Values originate from typed catalogs. JSON conversion here provides a
	// compact, type-safe copy without retaining slices or pointers from inputs.
	assign := func(target any) {
		encoded, _ := json.Marshal(value)
		_ = json.Unmarshal(encoded, target)
	}
	switch path {
	case "name":
		assign(&entry.Name)
	case "kind":
		assign(&entry.Kind)
	case "description":
		assign(&entry.Description)
	case "category":
		assign(&entry.Category)
	case "tags":
		assign(&entry.Tags)
	case "platforms":
		assign(&entry.Platforms)
	case "favorite":
		assign(&entry.Favorite)
	case "portable":
		entry.Portable = nil
		if value != nil {
			assign(&entry.Portable)
		}
	case "native.bash", "native.zsh":
		shell := path[len("native."):]
		if entry.Native == nil {
			entry.Native = map[string]NativeImplementation{}
		}
		if value == nil {
			delete(entry.Native, shell)
		} else {
			var implementation NativeImplementation
			assign(&implementation)
			entry.Native[shell] = implementation
		}
	case "when.profiles_any", "when.profiles_none", "when.shells":
		if entry.When == nil {
			entry.When = &Conditions{}
		}
		switch path {
		case "when.profiles_any":
			assign(&entry.When.ProfilesAny)
		case "when.profiles_none":
			assign(&entry.When.ProfilesNone)
		case "when.shells":
			assign(&entry.When.Shells)
		}
		if len(entry.When.ProfilesAny) == 0 && len(entry.When.ProfilesNone) == 0 && len(entry.When.Shells) == 0 {
			entry.When = nil
		}
	}
}
