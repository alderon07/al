package main

import (
	workflowplan "github.com/alderon07/al/internal/plan"

	"fmt"
)

func runCatalogSyncPull(apply bool) error {
	preview, err := applicationServices().BuildCatalogSyncPullPlan()
	if err != nil {
		return err
	}
	fmt.Fprint(catalogWorkflowStdout, workflowplan.RenderPlain(preview))
	if !apply {
		fmt.Fprintln(catalogWorkflowStdout, "Enter al sync --pull --apply after reviewing this plan.")
		return nil
	}
	return applicationServices().ApplyCatalogPull(preview)
}

func runCatalogSyncCommand(arguments []string) (bool, error) {
	forceCatalog := false
	filtered := []string{}
	for _, arg := range arguments {
		if arg == "--catalog" {
			forceCatalog = true
		} else {
			filtered = append(filtered, arg)
		}
	}
	arguments = filtered
	applicable, err := applicationServices().CatalogSyncApplicable(forceCatalog)
	if err != nil {
		return true, err
	}
	if !applicable {
		return false, nil
	}
	if len(arguments) == 2 && arguments[0] == "--resolve" {
		return true, runCatalogConflictReview(arguments[1])
	}
	operation := "inspect"
	apply := false
	for _, a := range arguments {
		switch a {
		case "--pull":
			if operation != "inspect" {
				return true, fmt.Errorf("choose --pull or --push")
			}
			operation = "pull"
		case "--push":
			if operation != "inspect" {
				return true, fmt.Errorf("choose --pull or --push")
			}
			operation = "push"
		case "--apply":
			apply = true
		default:
			return true, fmt.Errorf("usage: al sync [--pull|--push] [--apply]")
		}
	}
	switch operation {
	case "pull":
		return true, runCatalogSyncPull(apply)
	case "push":
		return true, runCatalogSyncPush(apply)
	default:
		return true, applicationServices().InspectCatalogSync()
	}
}

func runCatalogSyncPush(apply bool) error {
	preview, err := applicationServices().PrepareCatalogPush()
	if err != nil {
		return err
	}
	if !apply {
		fmt.Fprintln(catalogWorkflowStdout, "Push only the enrolled catalog after the full decoded-entry and outgoing-history secret scan. Enter al sync --push --apply to continue.")
		return nil
	}
	return applicationServices().ApplyCatalogPush(preview)
}
