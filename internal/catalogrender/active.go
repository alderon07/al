package catalogrender

import (
	"alias-lens/internal/catalog"
	"alias-lens/internal/catalogstore"
	"path/filepath"
)

type RenderV2Context struct {
	Shell             string
	Platform          string
	Profiles          []string
	Approvals         map[catalogstore.ApprovalKey]bool
	ValidateNative    func(shell string, declaration []byte) error
	ResolveExecutable func(program string) (string, error)
}
type RenderV2Result struct {
	Body                  []byte
	Entries               []catalogstore.GenerationEntry
	IncludedEntryIDs      []string
	PendingApprovals      []catalogstore.ApprovalKey
	Confirmations         []catalogstore.ApprovalKey
	ExecutableResolutions []catalogstore.ExecutableResolution
	Unavailable           []catalog.ResolvedEntry
}

func RendererV2ID(shell string) string {
	if shell == "bash" || shell == "zsh" {
		return shell + "/v2"
	}
	return ""
}
func RenderV2(value catalog.Catalog, context RenderV2Context) (RenderV2Result, []catalog.Diagnostic) {
	resolved, diagnostics := catalog.Resolve(value, catalog.ResolveContext{Shell: context.Shell, Platform: context.Platform, Profiles: context.Profiles})
	if len(diagnostics) > 0 {
		return RenderV2Result{}, diagnostics
	}
	result := RenderV2Result{Body: []byte{}, Entries: []catalogstore.GenerationEntry{}, IncludedEntryIDs: []string{}, PendingApprovals: []catalogstore.ApprovalKey{}, Confirmations: []catalogstore.ApprovalKey{}, ExecutableResolutions: []catalogstore.ExecutableResolution{}, Unavailable: []catalog.ResolvedEntry{}}
	fail := func(code string) (RenderV2Result, []catalog.Diagnostic) {
		return RenderV2Result{}, []catalog.Diagnostic{{Code: code, Field: "native", Message: "catalog declaration validation failed; run al catalog review"}}
	}
	for _, item := range resolved {
		entry := item.Entry
		path := ""
		native, hasNative := entry.Native[context.Shell]
		if hasNative {
			declaration, err := catalogstore.Declaration(entry, context.Shell, "")
			if err != nil || context.ValidateNative == nil {
				return fail("native_validator_required")
			}
			if err := context.ValidateNative(context.Shell, []byte(declaration)); err != nil {
				return fail("unsafe_native_declaration")
			}
		}
		if !item.Available {
			result.Unavailable = append(result.Unavailable, item)
			continue
		}
		if hasNative {
			key := catalogstore.NativeApproval(entry, context.Shell, native)
			if !context.Approvals[key] {
				result.PendingApprovals = append(result.PendingApprovals, key)
				continue
			}
			result.Confirmations = append(result.Confirmations, key)
		} else {
			if context.ResolveExecutable == nil {
				return fail("executable_resolver_required")
			}
			var err error
			path, err = context.ResolveExecutable(entry.Portable.Program)
			if err != nil {
				item.Available = false
				item.UnavailableReasons = append(item.UnavailableReasons, "external executable is unavailable")
				result.Unavailable = append(result.Unavailable, item)
				continue
			}
			if !filepath.IsAbs(path) || filepath.Clean(path) != path {
				return fail("invalid_executable_resolution")
			}
			result.ExecutableResolutions = append(result.ExecutableResolutions, catalogstore.ExecutableResolution{EntryID: entry.ID, Program: entry.Portable.Program, Path: path})
		}
		declaration, err := catalogstore.Declaration(entry, context.Shell, path)
		if err != nil {
			return fail("invalid_declaration")
		}
		result.Body = append(result.Body, []byte(declaration)...)
		result.Entries = append(result.Entries, catalogstore.GenerationEntry{Entry: entry, Declaration: declaration})
		result.IncludedEntryIDs = append(result.IncludedEntryIDs, entry.ID)
	}
	return result, nil
}
