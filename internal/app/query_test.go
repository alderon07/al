package app

import "testing"

func TestRankedSearchFindsMeaningAndTypos(t *testing.T) {
	aliases := []Alias{
		{Name: "dps", Command: "docker ps", Description: "List containers"},
		{Name: "gs", Command: "git status -sb", Description: "Show status"},
		{Name: "gb", Command: "git branch", Description: "List branches"},
	}
	if result := DefaultServices().filterAliases(aliases, "status"); len(result) == 0 || result[0].Name != "gs" {
		t.Fatalf("semantic search did not rank gs first: %+v", result)
	}
	if result := DefaultServices().filterAliases(aliases, "gss"); len(result) == 0 || result[0].Name != "gs" {
		t.Fatalf("fuzzy search did not correct gss: %+v", result)
	}
}

func TestSingleLetterSearchReturnsEveryMatchingPrefix(t *testing.T) {
	aliases := []Alias{
		{Name: "ll", Command: "eza -l"},
		{Name: "gs", Command: "git status"},
		{Name: "gco", Command: "git checkout"},
		{Name: "ag", Command: "silver searcher"},
	}
	result := DefaultServices().filterAliases(aliases, "g")
	if len(result) != 2 || result[0].Name != "gs" || result[1].Name != "gco" {
		t.Fatalf("single-letter prefix search returned the wrong aliases: %+v", result)
	}
}

func TestSearchMatchesCustomCategory(t *testing.T) {
	aliases := []Alias{{Name: "cl", Command: "clear", Description: "Clear the screen", Category: "utility"}}
	results := DefaultServices().filterAliases(aliases, "utility")
	if len(results) != 1 || results[0].Name != "cl" {
		t.Fatalf("category search returned %#v", results)
	}
}
