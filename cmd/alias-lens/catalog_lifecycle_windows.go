//go:build windows

package main

import (
	workflowplan "github.com/alderon07/al/internal/plan"

	"fmt"
)

func runCatalogLoader([]string) error {
	return fmt.Errorf("catalog activation needs Bash or Zsh on Linux, WSL, or macOS")
}
func runCatalogLifecycleCommand(arguments []string) (bool, int, error) {
	if len(arguments) > 0 {
		switch arguments[0] {
		case "enable", "plan", "review", "approve", "adopt", "rollback":
			return true, 2, runCatalogLoader(nil)
		}
	}
	return false, 0, nil
}

func runCatalogCheck(bool) (int, error) { return 2, runCatalogLoader(nil) }

func buildCatalogRequestedPlan(arguments []string) (workflowplan.OperationPlan, bool, error) {
	if len(arguments) > 0 && (arguments[0] == "init" || arguments[0] == "catalog") {
		return workflowplan.OperationPlan{}, true, runCatalogLoader(nil)
	}
	return workflowplan.OperationPlan{}, false, nil
}

func runCatalogConflictReview(string) error { return runCatalogLoader(nil) }
