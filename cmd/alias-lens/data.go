package main

import (
	"fmt"
	"os"
)

func runDataCommand(arguments []string) error {
	if len(arguments) != 1 {
		return fmt.Errorf("usage: al data paths|clear-usage|clear-revisions")
	}
	_, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	switch arguments[0] {
	case "paths":
		paths, err := applicationServices().DataPaths()
		if err != nil {
			return err
		}
		for _, item := range paths {
			fmt.Printf("%-18s %s\n", item.Kind, item.Path)
		}
		return nil
	case "clear-usage":
		if err := applicationServices().ClearUsageData(); err != nil {
			return fmt.Errorf("clear usage data: %w", err)
		}
		fmt.Println("Alias Lens usage data cleared.")
		return nil
	case "clear-revisions":
		if err := applicationServices().ClearRevisionData(); err != nil {
			return fmt.Errorf("clear revisions: %w", err)
		}
		fmt.Println("Alias Lens revisions cleared. Alias-file backups were kept.")
		return nil
	default:
		return fmt.Errorf("usage: al data paths|clear-usage|clear-revisions")
	}
}
