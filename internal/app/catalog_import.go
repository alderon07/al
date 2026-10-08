//go:build !windows

package app

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	neutralcatalog "github.com/alderon07/al/internal/catalog"
	workflowplan "github.com/alderon07/al/internal/plan"
)

func (svc *Services) buildCatalogImportPlan(shell string) (workflowplan.OperationPlan, error) {
	adapter, err := svc.shellAdapter(shell)
	if err != nil {
		return workflowplan.OperationPlan{}, fmt.Errorf("choose bash or zsh")
	}
	sourcePath := filepath.Join(svc.homeDirectory(), adapter.AliasFilename())
	source, err := readRegularFile(sourcePath, ShadowSourceLimit)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	report, err := svc.inspectCatalogShadow(shell)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	diagnostics := make([]workflowplan.Diagnostic, 0)
	entries := make([]neutralcatalog.Entry, 0, report.Summary.Equivalent)
	for _, result := range report.Results {
		if result.Status == "equivalent" && result.Entry != nil {
			entries = append(entries, *result.Entry)
			continue
		}
		diagnostics = append(diagnostics, workflowplan.Diagnostic{Code: "entry_skipped", Message: fmt.Sprintf("%s was left in the shell file because it could not be copied safely", shadowResultLabel(result))})
	}
	inputs := []workflowplan.Input{svc.planInput("native_alias_file", sourcePath, source)}
	if len(entries) == 0 {
		return workflowplan.Build("catalog.import", inputs, nil, nil, diagnostics), nil
	}
	path := svc.localCatalogPath()
	current, readErr := readRegularFile(path, neutralcatalog.MaxDocumentBytes)
	kind := workflowplan.ActionReplace
	backup := true
	catalog := neutralcatalog.Catalog{SchemaVersion: neutralcatalog.SchemaVersion, Entries: []neutralcatalog.Entry{}}
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
		inputs = append(inputs, svc.planInput("catalog", path, current))
	}
	if bytes.Equal(current, planned) {
		diagnostics = append(diagnostics, workflowplan.Diagnostic{Code: "no_change", Message: "The catalog already contains every safe entry."})
		return workflowplan.Build("catalog.import", inputs, nil, nil, diagnostics), nil
	}
	action := workflowplan.Action{Sequence: 1, Kind: kind, TargetRole: "catalog", DisplayPath: svc.displayPrivatePath(path), Reason: fmt.Sprintf("copy %d safe %s entries into the inactive catalog", len(entries), shell), Risk: workflowplan.RiskReview, PlannedSHA256: hashBytes(planned), Backup: backup, Reversible: true, Target: plannedTarget(path, current, planned)}
	return workflowplan.Build("catalog.import", inputs, []workflowplan.Action{action}, nil, diagnostics), nil
}

func shadowResultLabel(result shadowResult) string {
	if result.Name != "" {
		return result.Name
	}
	return fmt.Sprintf("lines %d through %d", result.StartLine, result.EndLine)
}

func (svc *Services) homeDirectory() string {
	home, _ := svc.dependencies.HomeDir()
	return home
}
