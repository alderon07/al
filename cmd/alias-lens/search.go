package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/x/ansi"
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
	if cliStyled() {
		fmt.Println(cliReportHeading("search"))
		if len(results) == 0 {
			fmt.Println(cliMuted("No aliases found."))
			return nil
		}
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
		if !cliStyled() {
			fmt.Printf("%s\t%s\t%s%s\n", terminalSafeText(alias.Name), terminalSafeText(alias.Command), terminalSafeText(alias.Description), metadata)
			continue
		}
		fmt.Println(cliAccent(terminalSafeText(alias.Name)) + cliMuted(metadata))
		for _, line := range strings.Split(ansi.Wrap(terminalSafeText(alias.Command), cliColumns()-2, ""), "\n") {
			fmt.Println("  " + line)
		}
		if alias.Description != "" {
			for _, line := range strings.Split(ansi.Wrap(terminalSafeText(alias.Description), cliColumns()-2, ""), "\n") {
				fmt.Println("  " + cliMuted(line))
			}
		}
		fmt.Println()
	}
	return nil
}
