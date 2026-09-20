//go:build !windows

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	neutralcatalog "alias-lens/internal/catalog"
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
	report, err := inspectCatalogShadow(shell)
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
	preview, err := buildCatalogImportPlan(shell)
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
	root := filepath.Dir(localCatalogPath())
	if err := applyPrivatePlan(root, preview, func() (workflowplan.OperationPlan, error) { return buildCatalogImportPlan(shell) }); err != nil {
		return 1, err
	}
	fmt.Fprintln(catalogWorkflowStdout, "The catalog is ready but inactive. Your current aliases were left unchanged.")
	fmt.Fprintf(catalogWorkflowStdout, "The catalog is not active yet. Enter al catalog diff --from installed --shell %s to review it.\n", shell)
	return 0, nil
}

func buildCatalogImportPlan(shell string) (workflowplan.OperationPlan, error) {
	adapter, err := shellAdapter(shell)
	if err != nil {
		return workflowplan.OperationPlan{}, fmt.Errorf("choose bash or zsh")
	}
	sourcePath := filepath.Join(homeDirectory(), adapter.AliasFilename())
	source, err := readRegularFile(sourcePath, shadowSourceLimit)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	report, err := inspectCatalogShadow(shell)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	diagnostics := make([]workflowplan.Diagnostic, 0)
	entries := make([]neutralcatalog.Entry, 0, report.Summary.Equivalent)
	for _, result := range report.Results {
		if result.Status == "equivalent" && result.entry != nil {
			entries = append(entries, *result.entry)
			continue
		}
		diagnostics = append(diagnostics, workflowplan.Diagnostic{Code: "entry_skipped", Message: fmt.Sprintf("%s was left in the shell file because it could not be copied safely", shadowResultLabel(result))})
	}
	inputs := []workflowplan.Input{planInput("native_alias_file", sourcePath, source)}
	if len(entries) == 0 {
		return workflowplan.Build("catalog.import", inputs, nil, nil, diagnostics), nil
	}
	path := localCatalogPath()
	current, readErr := readRegularFile(path, neutralcatalog.MaxDocumentBytes)
	kind := workflowplan.ActionReplace
	backup := true
	catalog := neutralcatalog.Catalog{SchemaVersion: neutralcatalog.SchemaVersion2, Entries: []neutralcatalog.Entry{}}
	if errors.Is(readErr, os.ErrNotExist) {
		current = nil
		kind = workflowplan.ActionCreate
		backup = false
	} else if readErr != nil {
		return workflowplan.OperationPlan{}, readErr
	} else {
		catalog, _ = neutralcatalog.Decode(current)
		if problems := neutralcatalog.Validate(catalog); len(problems) > 0 {
			return workflowplan.OperationPlan{}, fmt.Errorf("Alias Lens cannot use the existing catalog because its data format is not valid")
		}
		if catalog.SchemaVersion == neutralcatalog.SchemaVersion {
			return workflowplan.OperationPlan{}, fmt.Errorf("the catalog uses data format 1; enter al catalog migrate --to 2 first")
		}
	}
	byID := make(map[string]neutralcatalog.Entry, len(catalog.Entries)+len(entries))
	nameOwner := make(map[string]string, len(catalog.Entries)+len(entries))
	for _, entry := range catalog.Entries {
		byID[entry.ID] = entry
		nameOwner[entry.Name] = entry.ID
	}
	for _, entry := range entries {
		if owner, exists := nameOwner[entry.Name]; exists {
			existing := byID[owner]
			if existing.Kind != entry.Kind {
				diagnostics = append(diagnostics, workflowplan.Diagnostic{Code: "kind_collision", Message: fmt.Sprintf("%s has a different type in the catalog", entry.Name), Blocked: true})
				continue
			}
			if existing.Native == nil {
				existing.Native = map[string]neutralcatalog.NativeImplementation{}
			}
			existing.Native[shell] = entry.Native[shell]
			byID[owner] = existing
			continue
		}
		byID[entry.ID] = entry
		nameOwner[entry.Name] = entry.ID
	}
	catalog.Entries = catalog.Entries[:0]
	for _, entry := range byID {
		catalog.Entries = append(catalog.Entries, entry)
	}
	sort.Slice(catalog.Entries, func(i, j int) bool { return catalog.Entries[i].ID < catalog.Entries[j].ID })
	planned, problems := neutralcatalog.Encode(catalog)
	if len(problems) > 0 {
		return workflowplan.OperationPlan{}, fmt.Errorf("Alias Lens could not prepare a valid catalog")
	}
	if current != nil {
		inputs = append(inputs, planInput("catalog", path, current))
	}
	if bytes.Equal(current, planned) {
		diagnostics = append(diagnostics, workflowplan.Diagnostic{Code: "no_change", Message: "The catalog already contains every safe entry."})
		return workflowplan.Build("catalog.import", inputs, nil, nil, diagnostics), nil
	}
	action := workflowplan.Action{Sequence: 1, Kind: kind, TargetRole: "catalog", DisplayPath: displayPrivatePath(path), Reason: fmt.Sprintf("copy %d safe %s entries into the inactive catalog", len(entries), shell), Risk: workflowplan.RiskReview, PlannedSHA256: hashBytes(planned), Backup: backup, Reversible: true, Target: plannedTarget(path, current, planned)}
	return workflowplan.Build("catalog.import", inputs, []workflowplan.Action{action}, nil, diagnostics), nil
}

func shadowResultLabel(result shadowResult) string {
	if result.Name != "" {
		return result.Name
	}
	return fmt.Sprintf("lines %d through %d", result.StartLine, result.EndLine)
}

func homeDirectory() string {
	home, _ := os.UserHomeDir()
	return home
}
