package main

import (
	"fmt"
	"strings"
)

type healthBreakdown struct {
	missing int
	broken  int
	risky   int
}

func summarizeHealth(aliases []Alias) healthBreakdown {
	var summary healthBreakdown
	for _, alias := range aliases {
		missing, broken, risky := false, false, false
		for _, issue := range alias.Issues {
			switch {
			case strings.HasPrefix(issue, "missing executable:"):
				missing = true
			case issue == "review before running":
				risky = true
			default:
				broken = true
			}
		}
		if missing {
			summary.missing++
		}
		if broken {
			summary.broken++
		}
		if risky {
			summary.risky++
		}
	}
	return summary
}

func healthHeaderSummary(aliases []Alias) string {
	summary := summarizeHealth(aliases)
	var parts []string
	if summary.missing > 0 {
		parts = append(parts, fmt.Sprintf("%d missing", summary.missing))
	}
	if summary.broken > 0 {
		parts = append(parts, fmt.Sprintf("%d broken", summary.broken))
	}
	if summary.risky > 0 {
		parts = append(parts, fmt.Sprintf("%d risky", summary.risky))
	}
	if len(parts) == 0 {
		return "healthy"
	}
	return strings.Join(parts, ", ") + " ^h"
}
