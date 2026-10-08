package main

import "alias-lens/internal/presentation"

import (
	"alias-lens/internal/app"

	"fmt"
	"path/filepath"
)

func runImportCommand(arguments []string) error {
	apply := false
	if len(arguments) == 2 && arguments[1] == "--apply" {
		apply = true
	} else if len(arguments) != 1 {
		return fmt.Errorf("usage: al import FILE [--apply]")
	}
	sourcePath, err := filepath.Abs(arguments[0])
	if err != nil {
		return err
	}
	preview, err := applicationServices().PrepareImport(sourcePath)
	if err != nil {
		return err
	}
	plan := preview.Plan
	printImportPlan(sourcePath, plan)
	if !apply {
		fmt.Println("Preview only. Run the same command with --apply to write these changes.")
		return nil
	}
	result, err := applicationServices().ApplyImport(preview)
	if err != nil {
		return err
	}
	if result.Added == 0 {
		fmt.Println("No aliases need to be imported.")
		return nil
	}
	if result.Catalog {
		cliResult(fmt.Sprintf("Imported %d catalog entries. Review with al catalog enable.", result.Added))
		return nil
	}
	cliResult(fmt.Sprintf("Imported %d aliases. Backup and private revision saved.", result.Added))
	return nil
}

func printImportPlan(path string, plan app.ImportPlan) {
	fmt.Println(cliAccent("Import preview:"), presentation.TerminalSafeText(path))
	for _, issue := range plan.Issues {
		location := ""
		if issue.Line > 0 {
			location = fmt.Sprintf(" line %d", issue.Line)
		}
		fmt.Printf("%s %-17s%s  %s\n", cliAttention("ISSUE"), issue.Kind, location, issue.Message)
	}
	for _, skipped := range plan.Skip {
		fmt.Println(cliAccent("SKIP "), skipped)
	}
	for _, alias := range plan.Add {
		fmt.Printf("%s    %-20s %s\n", cliPositive("ADD"), alias.Name, presentation.TerminalSafeText(alias.Command))
	}
	fmt.Printf("Planned: %d add, %d skip, %d issues.\n", len(plan.Add), len(plan.Skip), len(plan.Issues))
}
