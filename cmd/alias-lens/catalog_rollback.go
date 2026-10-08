//go:build !windows

package main

import "github.com/alderon07/al/internal/presentation"

import (
	"github.com/alderon07/al/internal/app"

	"fmt"

	workflowplan "github.com/alderon07/al/internal/plan"
)

func runCatalogRollback(shell string, apply, jsonOutput bool) (int, error) {
	preview, err := applicationServices().BuildCatalogRollbackPlan(shell)
	if err != nil {
		return 1, err
	}
	if jsonOutput {
		data, e := workflowplan.EncodeJSON(preview)
		if e != nil {
			return 1, e
		}
		fmt.Fprint(catalogWorkflowStdout, string(data))
	} else {
		fmt.Fprint(catalogWorkflowStdout, workflowplan.RenderPlain(preview))
	}
	if !apply {
		if !catalogReviewTerminal() {
			return 0, nil
		}
		yes, e := confirmCatalogReview(newCatalogReviewReader(), app.CatalogRollbackConfirmation)
		if e != nil {
			return 1, e
		}
		if !yes {
			return 0, nil
		}
	}
	if err := applicationServices().ApplyCatalogRollback(preview, shell); err != nil {
		return 1, err
	}
	fmt.Fprintln(catalogWorkflowStdout, "Native files restored. Start a new "+presentation.FriendlyShellName(shell)+" shell.")
	return 0, nil
}
