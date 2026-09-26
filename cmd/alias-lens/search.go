package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func runSearchCommand(arguments []string) error {
	jsonOutput := false
	global := false
	for len(arguments) > 0 && strings.HasPrefix(arguments[0], "--") {
		switch arguments[0] {
		case "--json":
			jsonOutput = true
		case "--global":
			global = true
		default:
			return fmt.Errorf("usage: al search [--json] [--global] [QUERY]")
		}
		arguments = arguments[1:]
	}
	if len(arguments) > 1 {
		return fmt.Errorf("usage: al search [--json] [--global] [QUERY]")
	}
	aliases, err := loadAliases()
	if err != nil {
		return err
	}
	results := aliases
	if len(arguments) == 1 && strings.TrimSpace(arguments[0]) != "" {
		if global {
			results = filterAliases(aliases, arguments[0])
		} else {
			ranking, err := currentContextRanking()
			if err != nil {
				return fmt.Errorf("load context ranking: %w; use --global to search without it", err)
			}
			results = filterAliasesForContext(aliases, arguments[0], ranking)
		}
	}
	if jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(results)
	}
	for _, alias := range results {
		metadata := ""
		if len(alias.Tags) > 0 {
			safeTags := make([]string, len(alias.Tags))
			for index, tag := range alias.Tags {
				safeTags[index] = terminalSafeText(tag)
			}
			metadata = "  #" + strings.Join(safeTags, " #")
		}
		fmt.Printf("%s\t%s\t%s%s\n", terminalSafeText(alias.Name), terminalSafeText(alias.Command), terminalSafeText(alias.Description), metadata)
	}
	return nil
}
