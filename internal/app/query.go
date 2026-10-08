package app

import (
	"sort"
	"strings"

	"unicode/utf8"
)

func suggestedAliases(aliases []Alias) []Alias {
	return suggestedAliasesForContext(aliases, nil)
}

func suggestedAliasesForContext(aliases []Alias, context ContextScorer) []Alias {
	type ranked struct {
		alias   Alias
		score   int
		context int
	}
	preferred := map[string]int{
		"git status -sb": 100,
		"eza -l --icons --git --group-directories-first": 95,
		"git add":                              90,
		"git commit":                           85,
		"git log --oneline --graph --decorate": 80,
		"docker ps":                            75,
	}
	var rankedAliases []ranked
	for _, alias := range aliases {
		lower := strings.ToLower(alias.Command)
		if strings.Contains(lower, "--force") || strings.Contains(lower, "reset --hard") || strings.Contains(lower, "clean -fd") || strings.Contains(lower, "branch -d") {
			continue
		}
		score := preferred[alias.Command]
		score += min(alias.Usage, 25) * 3
		if alias.Favorite {
			score += 150
		}
		if len(alias.Name) == 2 {
			score += 12
		} else if len(alias.Name) == 3 {
			score += 6
		}
		rankedAliases = append(rankedAliases, ranked{alias: alias, score: score, context: contextScore(context, alias)})
	}
	sort.SliceStable(rankedAliases, func(i, j int) bool {
		if rankedAliases[i].alias.Favorite != rankedAliases[j].alias.Favorite {
			return rankedAliases[i].alias.Favorite
		}
		if rankedAliases[i].context != rankedAliases[j].context {
			return rankedAliases[i].context > rankedAliases[j].context
		}
		if rankedAliases[i].score == rankedAliases[j].score {
			return rankedAliases[i].alias.Name < rankedAliases[j].alias.Name
		}
		return rankedAliases[i].score > rankedAliases[j].score
	})
	result := make([]Alias, len(rankedAliases))
	for index := range rankedAliases {
		result[index] = rankedAliases[index].alias
	}
	return result
}

func (svc *Services) filterAliases(aliases []Alias, query string) []Alias {
	return svc.filterAliasesForContext(aliases, query, nil)
}

func (svc *Services) filterAliasesForContext(aliases []Alias, query string, context ContextScorer) []Alias {
	needle := normalize(query)
	if needle == "" {
		return nil
	}
	if utf8.RuneCountInString(needle) == 1 {
		var prefixes []Alias
		for _, alias := range aliases {
			if strings.HasPrefix(normalize(alias.Name), needle) {
				prefixes = append(prefixes, alias)
			}
		}
		sort.SliceStable(prefixes, func(i, j int) bool {
			leftContext, rightContext := contextScore(context, prefixes[i]), contextScore(context, prefixes[j])
			if leftContext != rightContext {
				return leftContext > rightContext
			}
			if len(prefixes[i].Name) == len(prefixes[j].Name) {
				return prefixes[i].Name < prefixes[j].Name
			}
			return len(prefixes[i].Name) < len(prefixes[j].Name)
		})
		return prefixes
	}
	type rankedAlias struct {
		alias   Alias
		score   int
		context int
	}
	var ranked []rankedAlias
	for _, alias := range aliases {
		name := normalize(alias.Name)
		score := -1
		switch {
		case name == needle:
			score = 0
		case strings.HasPrefix(name, needle):
			score = 10
		case strings.Contains(name, needle):
			score = 20
		case wordsMatch(alias.Command, needle):
			score = 30
		case wordsMatch(alias.Description, needle):
			score = 40
		case wordsMatch(alias.Category, needle):
			score = 42
		case wordsMatch(strings.Join(alias.Tags, " "), needle):
			score = 45
		case wordsMatch(strings.Join(alias.Platforms, " "), needle):
			score = 50
		case alias.Type == needle:
			score = 55
		default:
			distance := levenshtein(name, needle)
			if distance <= max(1, len(needle)/3) {
				score = 60 + distance
			}
		}
		if score >= 0 {
			ranked = append(ranked, rankedAlias{alias: alias, score: score, context: contextScore(context, alias)})
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score < ranked[j].score
		}
		if ranked[i].context != ranked[j].context {
			return ranked[i].context > ranked[j].context
		}
		if ranked[i].score == ranked[j].score {
			return len(ranked[i].alias.Name) < len(ranked[j].alias.Name)
		}
		return false
	})
	matches := make([]Alias, len(ranked))
	for index := range ranked {
		matches[index] = ranked[index].alias
	}
	return matches
}

func wordsMatch(value, needle string) bool {
	for _, word := range strings.Fields(value) {
		if strings.HasPrefix(normalize(word), needle) {
			return true
		}
	}
	return false
}

func healthIssueCount(aliases []Alias) int {
	count := 0
	for _, alias := range aliases {
		count += len(alias.Issues)
	}
	return count
}

func closestAliases(aliases []Alias, query string) []string {
	needle := normalize(query)
	limit := max(1, len(needle)/3)
	var close []string
	for _, alias := range aliases {
		if levenshtein(normalize(alias.Name), needle) <= limit {
			close = append(close, alias.Name)
			if len(close) == 4 {
				break
			}
		}
	}
	return close
}

func normalize(value string) string {
	var result strings.Builder
	for _, char := range strings.ToLower(value) {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			result.WriteRune(char)
		}
	}
	return result.String()
}

func levenshtein(left, right string) int {
	row := make([]int, len(right)+1)
	for column := range row {
		row[column] = column
	}
	next := make([]int, len(right)+1)
	for i := 1; i <= len(left); i++ {
		next[0] = i
		for j := 1; j <= len(right); j++ {
			cost := 0
			if left[i-1] != right[j-1] {
				cost = 1
			}
			next[j] = min(min(next[j-1]+1, row[j]+1), row[j-1]+cost)
		}
		row, next = next, row
	}
	return row[len(right)]
}

type ContextScorer interface{ Match(Alias) int }

func contextScore(context ContextScorer, alias Alias) int {
	if context == nil {
		return 0
	}
	return context.Match(alias)
}
func (s *Services) SuggestedEntries(aliases []Alias, context ContextScorer) []Alias {
	return suggestedAliasesForContext(aliases, context)
}
func (s *Services) FilterEntries(aliases []Alias, query string, context ContextScorer) []Alias {
	return s.filterAliasesForContext(aliases, query, context)
}
func (s *Services) ClosestEntries(aliases []Alias, query string) []string {
	return closestAliases(aliases, query)
}
func (s *Services) EntryIssueCount(aliases []Alias) int { return healthIssueCount(aliases) }
