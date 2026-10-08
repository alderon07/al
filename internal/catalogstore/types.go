package catalogstore

import "github.com/alderon07/al/internal/catalog"

type ApprovalKey struct {
	EntryID              string `json:"entry_id"`
	Shell                string `json:"shell"`
	Name                 string `json:"name"`
	Kind                 string `json:"kind"`
	ImplementationSHA256 string `json:"implementation_sha256"`
	Renderer             string `json:"renderer"`
}
type ApprovalRecord struct {
	Key        ApprovalKey `json:"key"`
	ApprovedAt string      `json:"approved_at"`
}
type ApprovalFile struct {
	Version int              `json:"version"`
	Records []ApprovalRecord `json:"records"`
}
type StartupRecord struct {
	OriginalExists bool   `json:"original_exists"`
	Path           string `json:"path"`
	SHA256         string `json:"sha256"`
	OriginalPath   string `json:"original_path,omitempty"`
	OriginalSHA256 string `json:"original_sha256,omitempty"`
	Route          string `json:"route,omitempty"`
}
type InstalledState struct {
	Shell              string          `json:"shell"`
	GenerationID       string          `json:"generation_id"`
	Renderer           string          `json:"renderer"`
	NativeInputSHA256  string          `json:"native_input_sha256,omitempty"`
	StartupRecords     []StartupRecord `json:"startup_records,omitempty"`
	StartupGraphSHA256 string          `json:"startup_graph_sha256,omitempty"`
	RollbackID         string          `json:"rollback_id,omitempty"`
}
type InstalledFile struct {
	Version int              `json:"version"`
	Records []InstalledState `json:"records"`
}
type Adoption struct {
	EntryID          string `json:"entry_id"`
	Shell            string `json:"shell"`
	Name             string `json:"name"`
	Kind             string `json:"kind"`
	Path             string `json:"path"`
	Start            int    `json:"start"`
	End              int    `json:"end"`
	Definition       string `json:"definition"`
	DefinitionSHA256 string `json:"definition_sha256"`
	FileSHA256       string `json:"file_sha256"`
	OriginalPath     string `json:"original_path"`
	OriginalSHA256   string `json:"original_sha256"`
}
type AdoptionsFile struct {
	Version int        `json:"version"`
	Records []Adoption `json:"records"`
}
type AdoptionFile = AdoptionsFile
type RollbackRecord struct {
	NativeExists   bool            `json:"native_exists"`
	Version        int             `json:"version"`
	ID             string          `json:"id"`
	Shell          string          `json:"shell"`
	CreatedAt      string          `json:"created_at"`
	NativePath     string          `json:"native_path"`
	OriginalPath   string          `json:"original_path"`
	OriginalSHA256 string          `json:"original_sha256"`
	StartupRecords []StartupRecord `json:"startup_records"`
}
type ExecutableResolution struct {
	EntryID string `json:"entry_id"`
	Program string `json:"program"`
	Path    string `json:"path"`
}
type GenerationEntry struct {
	Entry       catalog.Entry `json:"entry"`
	Declaration string        `json:"declaration"`
}
type GenerationManifest struct {
	Version                int                    `json:"version"`
	ID                     string                 `json:"id"`
	Shell                  string                 `json:"shell"`
	Renderer               string                 `json:"renderer"`
	Platform               string                 `json:"platform"`
	NativePath             string                 `json:"native_path"`
	NativePolicy           string                 `json:"native_policy"`
	NativeSourcePath       string                 `json:"native_source_path,omitempty"`
	NativeSourceLinkTarget string                 `json:"native_source_link_target,omitempty"`
	NativeSourceIdentity   string                 `json:"native_source_identity,omitempty"`
	SourceSHA256           string                 `json:"source_sha256"`
	NativeInputSHA256      string                 `json:"native_input_sha256"`
	FileSHA256             string                 `json:"file_sha256"`
	Profiles               []string               `json:"profiles"`
	Entries                []GenerationEntry      `json:"entries"`
	IncludedEntryIDs       []string               `json:"included_entry_ids"`
	Confirmations          []ApprovalKey          `json:"confirmations"`
	ExecutableResolutions  []ExecutableResolution `json:"executable_resolutions"`
}
