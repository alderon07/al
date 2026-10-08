//go:build !windows

package main

import (
	"fmt"

	workflowplan "alias-lens/internal/plan"
)

func runCatalogPreviewCommand(arguments []string) (int, error) {
	shell := ""
	jsonOutput := false
	for index := 0; index < len(arguments); index++ {
		switch arguments[index] {
		case "--from", "--shell":
			if index+1 >= len(arguments) {
				return 2, fmt.Errorf("%s needs bash or zsh", arguments[index])
			}
			index++
			shell = arguments[index]
		case "--json":
			jsonOutput = true
		default:
			return 2, fmt.Errorf("unknown catalog preview option %q", arguments[index])
		}
	}
	report, err := applicationServices().InspectCatalogShadow(shell)
	if err != nil {
		return 1, err
	}
	output, err := renderShadowReport(report, jsonOutput)
	if err != nil {
		return 1, err
	}
	if _, err := catalogWorkflowStdout.Write(output); err != nil {
		return 1, err
	}
	if report.Summary.Unsupported+report.Summary.Different+report.Summary.Duplicate+report.Summary.Invalid+report.Summary.Blocked > 0 {
		if !jsonOutput {
			fmt.Fprintln(catalogWorkflowStdout, "Nothing was changed. Fix the items that need attention, then enter this command again.")
		}
		return 1, nil
	}
	if !jsonOutput {
		fmt.Fprintf(catalogWorkflowStdout, "Nothing was changed. Enter al catalog import --from %s when you are ready to create the catalog.\n", report.Shell)
	}
	return 0, nil
}

func runCatalogImportCommand(arguments []string) (int, error) {
	if len(arguments) != 2 || arguments[0] != "--from" {
		return 2, fmt.Errorf("usage: al catalog import --from bash|zsh")
	}
	shell := arguments[1]
	preview, err := applicationServices().BuildCatalogImportPlan(shell)
	if err != nil {
		return 1, err
	}
	fmt.Fprint(catalogWorkflowStdout, workflowplan.RenderPlain(preview))
	if preview.Summary.Blocked {
		fmt.Fprintln(catalogWorkflowStdout, "The catalog was left unchanged. Fix the items that need attention, then enter the import command again.")
		return 1, nil
	}
	if len(preview.Actions) == 0 {
		message := "The catalog already contains these entries. Nothing changed."
		for _, diagnostic := range preview.Diagnostics {
			if diagnostic.Code == "entry_skipped" {
				message = "No entries could be copied safely. Your catalog and aliases were left unchanged."
				break
			}
		}
		fmt.Fprintln(catalogWorkflowStdout, message)
		return 0, nil
	}
	if err := applicationServices().ApplyCatalogImport(preview, shell); err != nil {
		return 1, err
	}
	fmt.Fprintln(catalogWorkflowStdout, "The catalog is ready but inactive. Your current aliases were left unchanged.")
	fmt.Fprintf(catalogWorkflowStdout, "The catalog is not active yet. Enter al catalog diff --from installed --shell %s to review it.\n", shell)
	return 0, nil
}
