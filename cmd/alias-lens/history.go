package main

import "fmt"

func runHistorySuggestions(arguments []string) error {
	aliases, err := applicationServices().Entries()
	if err != nil {
		return err
	}
	suggestions := applicationServices().HistorySuggestions(aliases, applicationServices().LoadHistoryCounts())
	if len(arguments) > 0 && arguments[0] == "add" {
		if len(arguments) < 2 || len(arguments) > 3 {
			return fmt.Errorf("usage: al suggest add NUMBER [NAME]")
		}
		var index int
		if _, err := fmt.Sscanf(arguments[1], "%d", &index); err != nil || index < 1 || index > len(suggestions) {
			return fmt.Errorf("suggestion number must be between 1 and %d", len(suggestions))
		}
		suggestion := suggestions[index-1]
		if len(arguments) == 3 {
			suggestion.Name = arguments[2]
		}
		return applicationServices().AddAlias(suggestion.Name, suggestion.Command, fmt.Sprintf("Used %d times in local %s history", suggestion.Count, applicationServices().ActiveShellAdapter().DisplayName()))
	}
	if len(arguments) != 0 {
		return fmt.Errorf("usage: al suggest [add NUMBER [NAME]]")
	}
	if len(suggestions) == 0 {
		fmt.Println("No repeated long commands need aliases yet.")
		return nil
	}
	cliHeading("Repeated commands that do not have aliases:")
	for index, suggestion := range suggestions {
		fmt.Printf("%2d  %-6s  %3d uses  %s\n", index+1, suggestion.Name, suggestion.Count, suggestion.Command)
	}
	fmt.Println("Add one with: al suggest add NUMBER [NAME]")
	return nil
}
