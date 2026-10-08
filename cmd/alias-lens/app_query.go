package main

import "github.com/alderon07/al/internal/app"

func suggestedAliases(aliases []Alias) []Alias {
	return applicationServices().SuggestedEntries(aliases, nil)
}
func suggestedAliasesForContext(aliases []Alias, context app.ContextRanking) []Alias {
	return applicationServices().SuggestedEntries(aliases, context)
}
func filterAliases(aliases []Alias, query string) []Alias {
	return applicationServices().FilterEntries(aliases, query, nil)
}
func filterAliasesForContext(aliases []Alias, query string, context app.ContextRanking) []Alias {
	return applicationServices().FilterEntries(aliases, query, context)
}
func closestAliases(aliases []Alias, query string) []string {
	return applicationServices().ClosestEntries(aliases, query)
}
func healthIssueCount(aliases []Alias) int { return applicationServices().EntryIssueCount(aliases) }
