package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func runSearchCommand(arguments []string) error {
	jsonOutput := false
	if len(arguments) > 0 && arguments[0] == "--json" {
		jsonOutput = true
		arguments = arguments[1:]
	}
	if len(arguments) > 1 {
		return fmt.Errorf("usage: al search [--json] [QUERY]")
	}
	aliases, err := loadAliases()
	if err != nil {
		return err
	}
	results := aliases
	if len(arguments) == 1 && strings.TrimSpace(arguments[0]) != "" {
		results = filterAliases(aliases, arguments[0])
	}
	if jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(results)
	}
	for _, alias := range results {
		metadata := ""
		if len(alias.Tags) > 0 {
			metadata = "  #" + strings.Join(alias.Tags, " #")
		}
		fmt.Printf("%s\t%s\t%s%s\n", alias.Name, alias.Command, alias.Description, metadata)
	}
	return nil
}
