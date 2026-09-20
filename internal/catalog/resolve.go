package catalog

import (
	"fmt"
	"sort"
)

// ResolveContext contains only explicit, deterministic inputs. Resolution does
// not inspect the environment, hostname, filesystem, clock, or network.
type ResolveContext struct {
	Shell    string
	Platform string
	Profiles []string
}

type ResolvedEntry struct {
	Entry              Entry
	Available          bool
	UnavailableReasons []string
}

// Resolve reports every catalog entry. Unavailable entries remain visible so
// callers can explain why they cannot be used on this machine.
func Resolve(value Catalog, context ResolveContext) ([]ResolvedEntry, []Diagnostic) {
	if diagnostics := Validate(value); len(diagnostics) > 0 {
		return nil, diagnostics
	}
	if context.Shell != "bash" && context.Shell != "zsh" {
		return nil, []Diagnostic{diagnostic("invalid_resolve_shell", -1, "root", "shell must be bash or zsh")}
	}
	if !makeSet([]string{"linux", "macos", "wsl", "windows"})[context.Platform] {
		return nil, []Diagnostic{diagnostic("invalid_resolve_platform", -1, "root", "platform is not supported")}
	}
	profiles := makeSet(context.Profiles)
	normalized := Normalize(value)
	result := make([]ResolvedEntry, 0, len(normalized.Entries))
	for _, entry := range normalized.Entries {
		reasons := availabilityReasons(entry, context, profiles)
		result = append(result, ResolvedEntry{
			Entry:              entry,
			Available:          len(reasons) == 0,
			UnavailableReasons: reasons,
		})
	}
	return result, nil
}

func availabilityReasons(entry Entry, context ResolveContext, profiles map[string]bool) []string {
	var reasons []string
	if len(entry.Platforms) > 0 && !makeSet(entry.Platforms)[context.Platform] {
		reasons = append(reasons, fmt.Sprintf("not available on %s", context.Platform))
	}
	if entry.When != nil {
		if len(entry.When.ProfilesAny) > 0 {
			matched := false
			for _, profile := range entry.When.ProfilesAny {
				matched = matched || profiles[profile]
			}
			if !matched {
				reasons = append(reasons, "needs one of these profiles: "+joinNames(entry.When.ProfilesAny))
			}
		}
		for _, profile := range entry.When.ProfilesNone {
			if profiles[profile] {
				reasons = append(reasons, "not available with profile "+profile)
			}
		}
		if len(entry.When.Shells) > 0 && !makeSet(entry.When.Shells)[context.Shell] {
			reasons = append(reasons, "not available in "+context.Shell)
		}
	}
	if _, native := entry.Native[context.Shell]; !native && entry.Portable == nil {
		reasons = append(reasons, "no "+context.Shell+" version is available")
	}
	sort.Strings(reasons)
	return reasons
}

func joinNames(values []string) string {
	result := ""
	for index, value := range values {
		if index > 0 {
			result += ", "
		}
		result += value
	}
	return result
}
