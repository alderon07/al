package main

import (
	"fmt"
	"os"

	workflowplan "github.com/alderon07/al/internal/plan"
)

func runPlanCommand(arguments []string) (int, error) {
	jsonOutput := false
	if len(arguments) > 0 && arguments[0] == "--json" {
		jsonOutput = true
		arguments = arguments[1:]
	}
	if len(arguments) == 0 {
		return 2, fmt.Errorf("usage: al plan [--json] COMMAND [ARGUMENTS]")
	}
	value, err := buildRequestedPlan(arguments)
	if err != nil {
		return 2, err
	}
	if jsonOutput {
		output, err := workflowplan.EncodeJSON(value)
		if err != nil {
			return 1, err
		}
		if _, err := os.Stdout.Write(output); err != nil {
			return 1, err
		}
	} else {
		fmt.Print(cliPlanText(workflowplan.RenderPlain(value)))
	}
	if value.Summary.Blocked {
		return 1, nil
	}
	return 0, nil
}

func buildRequestedPlan(arguments []string) (workflowplan.OperationPlan, error) {
	if value, handled, err := buildCatalogRequestedPlan(arguments); handled || err != nil {
		return value, err
	}
	switch {
	case len(arguments) == 4 && arguments[0] == "config" && arguments[1] == "profile" && (arguments[2] == "add" || arguments[2] == "remove"):
		return applicationServices().BuildProfilePlan(arguments[2], arguments[3])
	case len(arguments) == 3 && arguments[0] == "completion" && (arguments[1] == "install" || arguments[1] == "remove"):
		return buildCompletionPlan(arguments[1], arguments[2])
	default:
		return workflowplan.OperationPlan{}, fmt.Errorf("this plan is not available; enter al help plan to see available previews")
	}
}
