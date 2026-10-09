//go:build !windows

package main

import (
	"fmt"
	"io"

	"github.com/alderon07/al/internal/app"
	workflowplan "github.com/alderon07/al/internal/plan"
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
	report, err := applicationServices().InspectCatalogPreview(shell)
	if err != nil {
		return 1, err
	}
	output, err := renderCatalogPreviewReport(report, jsonOutput)
	if err != nil {
		return 1, err
	}
	hasProblems := report.Summary.Unsupported+report.Summary.Different+report.Summary.Duplicate+report.Summary.Invalid+report.Summary.Blocked > 0 || len(report.Diagnostics) > 0
	if !jsonOutput {
		if hasProblems {
			output = append(output, []byte("Nothing was changed. Review the ranges above. al catalog import copies only proven equivalent definitions and leaves unsupported definitions native.\n")...)
		} else {
			output = append(output, []byte(fmt.Sprintf("Nothing was changed. Enter al catalog import --from %s when you are ready to create the catalog.\n", report.Shell))...)
		}
	}
	if len(output) > app.ShadowReportLimit {
		return 1, fmt.Errorf("catalog preview report exceeds 16 MiB")
	}
	written, err := catalogWorkflowStdout.Write(output)
	if err != nil {
		return 1, fmt.Errorf("write catalog preview: %w", err)
	}
	if written != len(output) {
		return 1, fmt.Errorf("write catalog preview: %w", io.ErrShortWrite)
	}
	if hasProblems {
		return 1, nil
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
