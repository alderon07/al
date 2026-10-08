package catalogstore

import (
	"errors"
	"github.com/alderon07/al/internal/catalog"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var hashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var idPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
var namePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)
var profilePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
var routePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
var functionPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func ValidHash(s string) bool  { return hashPattern.MatchString(s) }
func validShell(s string) bool { return s == "bash" || s == "zsh" }
func absolute(s string) bool   { return filepath.IsAbs(s) && filepath.Clean(s) == s }
func validTime(s string) bool {
	t, e := time.Parse(time.RFC3339Nano, s)
	return e == nil && t.Format(time.RFC3339Nano) == s && strings.HasSuffix(s, "Z")
}
func keyValid(k ApprovalKey) bool {
	return idPattern.MatchString(k.EntryID) && validShell(k.Shell) && k.Renderer == k.Shell+"/v2" && ValidHash(k.ImplementationSHA256) && validName(k.Name, k.Kind)
}
func validName(n, k string) bool {
	return k == "command" && namePattern.MatchString(n) || k == "function" && functionPattern.MatchString(n)
}
func startupValid(s StartupRecord) bool {
	return len(s.Path) <= 4096 && (s.Route == "" || routePattern.MatchString(s.Route)) && absolute(s.Path) && ValidHash(s.SHA256) && ((s.OriginalPath == "" && s.OriginalSHA256 == "") || (absolute(s.OriginalPath) && ValidHash(s.OriginalSHA256)))
}
func Validate(value any) error {
	bad := errors.New("invalid catalog state record; run al catalog plan")
	switch v := value.(type) {
	case CatalogSyncRecord:
		return ValidateSyncRecord(v)
	case ApprovalKey:
		if !keyValid(v) {
			return bad
		}
	case ApprovalFile:
		if v.Version != 2 || v.Records == nil || len(v.Records) > 10000 {
			return bad
		}
		seen := map[ApprovalKey]bool{}
		for _, r := range v.Records {
			if !keyValid(r.Key) || !validTime(r.ApprovedAt) || seen[r.Key] {
				return bad
			}
			seen[r.Key] = true
		}
	case InstalledFile:
		if v.Version != 1 || v.Records == nil || len(v.Records) > 2 {
			return bad
		}
		seen := map[string]bool{}
		for _, r := range v.Records {
			if !validShell(r.Shell) || r.Renderer != r.Shell+"/v2" || !ValidHash(r.GenerationID) || seen[r.Shell] {
				return bad
			}
			seen[r.Shell] = true
			if r.NativeInputSHA256 != "" && !ValidHash(r.NativeInputSHA256) || r.StartupGraphSHA256 != "" && !ValidHash(r.StartupGraphSHA256) || r.RollbackID != "" && !ValidHash(r.RollbackID) || len(r.StartupRecords) > 64 {
				return bad
			}
			paths := map[string]bool{}
			for _, s := range r.StartupRecords {
				if !startupValid(s) || paths[s.Path] {
					return bad
				}
				paths[s.Path] = true
			}
		}
	case AdoptionsFile:
		if v.Version != 1 || v.Records == nil || len(v.Records) > 10000 {
			return bad
		}
		seen := map[string]bool{}
		for i, r := range v.Records {
			key := r.Shell + "/" + r.EntryID
			if seen[key] || !idPattern.MatchString(r.EntryID) || !validShell(r.Shell) || !validName(r.Name, r.Kind) || !absolute(r.Path) || !absolute(r.OriginalPath) || r.Start < 0 || r.End <= r.Start || r.End-r.Start != len(r.Definition) || Hash([]byte(r.Definition)) != r.DefinitionSHA256 || !ValidHash(r.FileSHA256) || !ValidHash(r.OriginalSHA256) {
				return bad
			}
			seen[key] = true
			for _, s := range v.Records[:i] {
				if s.Path == r.Path && r.Start < s.End && s.Start < r.End {
					return bad
				}
			}
		}
	case RollbackRecord:
		if v.Version != 1 || !ValidHash(v.ID) || !validShell(v.Shell) || !validTime(v.CreatedAt) || !absolute(v.NativePath) || !absolute(v.OriginalPath) || !ValidHash(v.OriginalSHA256) || v.StartupRecords == nil || len(v.StartupRecords) > 64 {
			return bad
		}
		seen := map[string]bool{}
		for _, s := range v.StartupRecords {
			if !startupValid(s) || seen[s.Path] {
				return bad
			}
			seen[s.Path] = true
		}
	case GenerationManifest:
		if v.NativeSourcePath != "" || v.NativeSourceIdentity != "" || v.NativeSourceLinkTarget != "" {
			if !absolute(v.NativeSourcePath) || !regexp.MustCompile(`^[0-9]{1,20}:[0-9]{1,20}$`).MatchString(v.NativeSourceIdentity) || len(v.NativeSourceLinkTarget) > 4096 || strings.ContainsAny(v.NativeSourceLinkTarget, "\x00\r\n") {
				return errors.New("invalid logical native source binding")
			}
		}
		if v.Version != 1 || !ValidHash(v.ID) || !validShell(v.Shell) || v.Renderer != v.Shell+"/v2" || !absolute(v.NativePath) || v.NativePolicy != "regular-user" || !ValidHash(v.SourceSHA256) || !ValidHash(v.NativeInputSHA256) || !ValidHash(v.FileSHA256) || v.Entries == nil || v.Profiles == nil || v.IncludedEntryIDs == nil || v.Confirmations == nil || v.ExecutableResolutions == nil || len(v.Entries) > 10000 || len(v.Profiles) > 32 || len(v.Confirmations) > 10000 || len(v.ExecutableResolutions) > 10000 || len(v.IncludedEntryIDs) > 10000 {
			return bad
		}
		if v.Platform != "linux" && v.Platform != "macos" && v.Platform != "wsl" && v.Platform != "windows" {
			return bad
		}
		entries := make([]catalog.Entry, 0, len(v.Entries))
		ids := map[string]bool{}
		for _, r := range v.Entries {
			entries = append(entries, r.Entry)
			ids[r.Entry.ID] = true
		}
		if len(catalog.Validate(catalog.Catalog{SchemaVersion: 2, Entries: entries})) != 0 {
			return bad
		}
		if len(v.IncludedEntryIDs) != len(v.Entries) {
			return bad
		}
		for i, id := range v.IncludedEntryIDs {
			if id != v.Entries[i].Entry.ID {
				return bad
			}
		}
		approvals := map[ApprovalKey]bool{}
		for _, k := range v.Confirmations {
			if !keyValid(k) || k.Shell != v.Shell || !ids[k.EntryID] || approvals[k] {
				return bad
			}
			approvals[k] = true
		}
		resolutions := map[string]bool{}
		for _, r := range v.ExecutableResolutions {
			if !ids[r.EntryID] || resolutions[r.EntryID] || !absolute(r.Path) || r.Program == "" {
				return bad
			}
			resolutions[r.EntryID] = true
		}
		seen := map[string]bool{}
		for _, p := range v.Profiles {
			if !profilePattern.MatchString(p) || seen[p] {
				return bad
			}
			seen[p] = true
		}
	default:
		return errors.New("unsupported catalog state type")
	}
	return nil
}
