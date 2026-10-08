package main

import (
	"fmt"
	"os"

	workflowstate "alias-lens/internal/state"
)

func runStatusCommand(arguments []string) (int, error) {
	jsonOutput := false
	if len(arguments) == 1 && arguments[0] == "--json" {
		jsonOutput = true
	} else if len(arguments) != 0 {
		return 2, fmt.Errorf("usage: al status [--json]")
	}
	report := applicationServices().InspectWorkflowStatus()
	if jsonOutput {
		output, err := workflowstate.EncodeJSON(report)
		if err != nil {
			return 1, fmt.Errorf("prepare status report: %w", err)
		}
		if _, err := os.Stdout.Write(output); err != nil {
			return 1, err
		}
	} else {
		fmt.Print(cliStatusText(workflowstate.RenderPlain(report)))
	}
	return workflowstate.ExitCode(report), nil
}
