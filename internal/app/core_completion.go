package app

import (
	"regexp"
	"sort"
)

var completionCandidateName = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,254}$`)

func uniqueCompletionNames(names []string) []string {
	sort.Strings(names)
	result := names[:0]
	for _, name := range names {
		if !completionCandidateName.MatchString(name) {
			continue
		}
		if len(result) == 0 || result[len(result)-1] != name {
			result = append(result, name)
		}
	}
	return result
}
