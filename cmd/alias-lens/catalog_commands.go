//go:build !windows

package main

import "github.com/alderon07/al/internal/app"

import (
	"fmt"
	"strings"

	neutralcatalog "github.com/alderon07/al/internal/catalog"
	workflowplan "github.com/alderon07/al/internal/plan"
)

func runCatalogLifecycleCommand(arguments []string) (bool, int, error) {
	if len(arguments) == 0 {
		return false, 0, nil
	}
	operation := arguments[0]
	if operation == "recover" {
		if len(arguments) != 1 {
			return true, 2, fmt.Errorf("usage: al catalog recover")
		}
		err := applicationServices().RecoverWorkflows()
		if err != nil {
			return true, 1, err
		}
		fmt.Fprintln(catalogWorkflowStdout, "Incomplete workflows recovered.")
		return true, 0, nil
	}
	switch operation {
	case "enable", "plan", "review", "approve", "adopt", "rollback":
	default:
		return false, 0, nil
	}
	shell, name := "", ""
	explicit := []string{}
	apply := false
	jsonOutput := false
	for i := 1; i < len(arguments); i++ {
		switch arguments[i] {
		case "--startup-path":
			if i+1 >= len(arguments) {
				return true, 2, fmt.Errorf("--startup-path needs an absolute startup file")
			}
			i++
			explicit = append(explicit, arguments[i])
		case "--shell":
			if i+1 >= len(arguments) {
				return true, 2, fmt.Errorf("--shell needs bash or zsh")
			}
			i++
			shell = arguments[i]
		case "--apply":
			apply = true
		case "--json":
			jsonOutput = true
		default:
			if strings.HasPrefix(arguments[i], "--") || name != "" {
				return true, 2, fmt.Errorf("unknown catalog option")
			}
			name = arguments[i]
		}
	}
	adapter, err := applicationServices().RequestedShellAdapter(shell)
	if err != nil {
		return true, 2, err
	}
	shell = adapter.Name()
	if operation == "rollback" {
		code, err := runCatalogRollback(shell, apply, jsonOutput)
		return true, code, err
	}
	value, err := applicationServices().Catalog()
	if err != nil {
		return true, 1, err
	}
	sourceCanonical, _ := neutralcatalog.Encode(value)
	decisions := app.CatalogLifecycleDecisions{ConfirmedAt: applicationServices().LifecycleTimestamp()}
	if operation == "approve" || operation == "adopt" {
		if name == "" {
			return true, 2, fmt.Errorf("usage: al catalog %s NAME --shell bash|zsh", operation)
		}
		filtered := []neutralcatalog.Entry{}
		for _, entry := range value.Entries {
			if entry.Name == name {
				filtered = append(filtered, entry)
			}
		}
		if len(filtered) != 1 {
			return true, 1, fmt.Errorf("catalog name was not found")
		}
		value.Entries = filtered
	}
	if operation == "review" || operation == "approve" || operation == "adopt" {
		decisions, err = guidedCatalogLifecycleReview(value, shell, operation == "adopt")
		decisions.SourceSHA256 = applicationServices().HashBytes(sourceCanonical)
		if err != nil {
			return true, 1, err
		}
		preview, err := applicationServices().BuildCatalogDecisionPlan(decisions)
		if err != nil {
			return true, 1, err
		}
		if len(preview.Actions) == 0 {
			fmt.Fprintln(catalogWorkflowStdout, "No review decisions were saved.")
			return true, 0, nil
		}
		yes, err := confirmCatalogReview(newCatalogReviewReader(), "Save these exact review decisions?")
		if err != nil {
			return true, 1, err
		}
		if !yes {
			return true, 0, nil
		}
		err = applicationServices().ApplyCatalogDecisions(preview, decisions)
		if err == nil {
			fmt.Fprintln(catalogWorkflowStdout, "Review decisions saved. Enter al catalog enable --shell "+shell+" to install them.")
		}
		return true, func() int {
			if err != nil {
				return 1
			}
			return 0
		}(), err
	}
	if operation == "enable" && catalogReviewTerminal() && !apply {
		decisions, err = guidedCatalogLifecycleReview(value, shell, true)
		if err != nil {
			return true, 1, err
		}
	}
	decisions.StartupPaths = explicit
	preview, err := applicationServices().BuildCatalogEnablePlan(shell, decisions)
	if err != nil {
		return true, 1, err
	}
	if jsonOutput {
		data, e := workflowplan.EncodeJSON(preview)
		if e != nil {
			return true, 1, e
		}
		fmt.Fprint(catalogWorkflowStdout, string(data))
	} else {
		fmt.Fprint(catalogWorkflowStdout, workflowplan.RenderPlain(preview))
	}
	if operation == "plan" {
		return true, 0, nil
	}
	if !apply {
		if !catalogReviewTerminal() {
			fmt.Fprintln(catalogWorkflowStdout, "Enter al catalog enable --shell "+shell+" --apply after reviewing the plan.")
			return true, 0, nil
		}
		yes, e := confirmCatalogReview(newCatalogReviewReader(), "Apply this catalog installation?")
		if e != nil {
			return true, 1, e
		}
		if !yes {
			return true, 0, nil
		}
	}
	if err := applicationServices().ApplyCatalogInstallation(preview, shell, decisions); err != nil {
		return true, 1, err
	}
	fmt.Fprintln(catalogWorkflowStdout, "Catalog installed for new "+adapter.DisplayName()+" shells. Start a new shell to use it.")
	return true, 0, nil
}

func buildCatalogRequestedPlan(arguments []string) (workflowplan.OperationPlan, bool, error) {
	if len(arguments) > 0 && arguments[0] == "init" {
		options, err := parseCatalogInitOptions(arguments[1:])
		if err != nil {
			return workflowplan.OperationPlan{}, true, err
		}
		decisions := app.CatalogLifecycleDecisions{ConfirmedAt: applicationServices().LifecycleTimestamp(), StartupPaths: options.StartupPaths}
		if applicationServices().CatalogInitRemoteSource(options.Source) {
			config, e := loadConfig()
			if e != nil {
				return workflowplan.OperationPlan{}, true, e
			}
			ctx, cancel := applicationServices().InterruptContext()
			defer cancel()
			remote, e := applicationServices().DiscoverRemoteCatalog(ctx, config, options.Source, options.CatalogPath)
			if e != nil {
				return workflowplan.OperationPlan{}, true, e
			}
			plan, e := applicationServices().BuildRemoteCatalogInitPlan(options, remote, decisions, "")
			return plan, true, e
		}
		plan, err := applicationServices().BuildLocalCatalogInitPlan(options, decisions)
		return plan, true, err
	}
	if len(arguments) < 2 || arguments[0] != "catalog" {
		return workflowplan.OperationPlan{}, false, nil
	}
	operation := arguments[1]
	if operation != "enable" && operation != "rollback" {
		return workflowplan.OperationPlan{}, false, nil
	}
	shell := ""
	explicit := []string{}
	for i := 2; i < len(arguments); i++ {
		if i+1 >= len(arguments) {
			return workflowplan.OperationPlan{}, true, fmt.Errorf("catalog plan option needs a value")
		}
		switch arguments[i] {
		case "--shell":
			i++
			shell = arguments[i]
		case "--startup-path":
			i++
			explicit = append(explicit, arguments[i])
		default:
			return workflowplan.OperationPlan{}, true, fmt.Errorf("unknown catalog plan option")
		}
	}
	adapter, err := applicationServices().RequestedShellAdapter(shell)
	if err != nil {
		return workflowplan.OperationPlan{}, true, err
	}
	shell = adapter.Name()
	if operation == "rollback" {
		value, err := applicationServices().BuildCatalogRollbackPlan(shell)
		return value, true, err
	}
	value, err := applicationServices().BuildCatalogEnablePlan(shell, app.CatalogLifecycleDecisions{ConfirmedAt: applicationServices().LifecycleTimestamp(), StartupPaths: explicit})
	return value, true, err
}
