package tui

import "github.com/alderon07/al/internal/app"

func suggestedAliases(services *app.Services, aliases []aliasEntry) []aliasEntry {
	return services.SuggestedEntries(aliases, nil)
}
func suggestedAliasesForContext(services *app.Services, aliases []aliasEntry, context app.ContextRanking) []aliasEntry {
	return services.SuggestedEntries(aliases, context)
}
func filterAliases(services *app.Services, aliases []aliasEntry, query string) []aliasEntry {
	return services.FilterEntries(aliases, query, nil)
}
func filterAliasesForContext(services *app.Services, aliases []aliasEntry, query string, context app.ContextRanking) []aliasEntry {
	return services.FilterEntries(aliases, query, context)
}
func closestAliases(services *app.Services, aliases []aliasEntry, query string) []string {
	return services.ClosestEntries(aliases, query)
}
func healthIssueCount(services *app.Services, aliases []aliasEntry) int {
	return services.EntryIssueCount(aliases)
}
