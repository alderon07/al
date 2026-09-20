package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	neutralcatalog "alias-lens/internal/catalog"
	workflowplan "alias-lens/internal/plan"
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
		fmt.Print(workflowplan.RenderPlain(value))
	}
	if value.Summary.Blocked {
		return 1, nil
	}
	return 0, nil
}

func buildRequestedPlan(arguments []string) (workflowplan.OperationPlan, error) {
	switch {
	case len(arguments) == 4 && arguments[0] == "config" && arguments[1] == "profile" && (arguments[2] == "add" || arguments[2] == "remove"):
		return buildProfilePlan(arguments[2], arguments[3])
	case len(arguments) == 2 && arguments[0] == "config" && arguments[1] == "migrate":
		return buildConfigMigrationPlan()
	case len(arguments) == 3 && arguments[0] == "catalog" && arguments[1] == "migrate" && arguments[2] == "--to=2":
		return buildCatalogMigrationPlan()
	case len(arguments) == 4 && arguments[0] == "catalog" && arguments[1] == "migrate" && arguments[2] == "--to" && arguments[3] == "2":
		return buildCatalogMigrationPlan()
	case len(arguments) == 3 && arguments[0] == "completion" && (arguments[1] == "install" || arguments[1] == "remove"):
		return buildCompletionPlan(arguments[1], arguments[2])
	default:
		return workflowplan.OperationPlan{}, fmt.Errorf("this plan is not available; enter al help plan to see available previews")
	}
}

func buildProfilePlan(action, name string) (workflowplan.OperationPlan, error) {
	if !profileNamePattern.MatchString(name) {
		return workflowplan.OperationPlan{}, fmt.Errorf("profile names start with a lowercase letter and use only lowercase letters, numbers, _ or -")
	}
	observed, err := observeConfig()
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	config := observed.Config
	profiles := append([]string(nil), config.Profiles...)
	exists := false
	for _, profile := range profiles {
		exists = exists || profile == name
	}
	if action == "add" && !exists {
		profiles = append(profiles, name)
	}
	if action == "remove" {
		kept := profiles[:0]
		for _, profile := range profiles {
			if profile != name {
				kept = append(kept, profile)
			}
		}
		profiles = kept
	}
	sort.Strings(profiles)
	config.Profiles = profiles
	config.Version = currentConfigVersion
	if err := validateAppConfig(config); err != nil {
		return workflowplan.OperationPlan{}, fmt.Errorf("Alias Lens cannot prepare this settings change: %w", err)
	}
	planned, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	planned = append(planned, '\n')
	path, err := configPath()
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	current, readErr := readRegularFile(path, 1<<20)
	kind := workflowplan.ActionReplace
	backup := true
	if errors.Is(readErr, os.ErrNotExist) {
		current = nil
		kind = workflowplan.ActionCreate
		backup = false
	} else if readErr != nil {
		return workflowplan.OperationPlan{}, readErr
	}
	operation := "config.profile." + action
	if (action == "add" && exists) || (action == "remove" && !exists) {
		return workflowplan.Build(operation, []workflowplan.Input{planInput("config", path, current)}, nil, nil, []workflowplan.Diagnostic{{Code: "no_change", Message: "That machine profile is already in the requested state."}}), nil
	}
	affected, err := affectedProfileEntries(name, action == "add")
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	reason := "change the active machine profiles"
	if len(affected) == 0 {
		reason += "; no catalog entries change availability"
	} else {
		reason += "; affected entries: " + strings.Join(affected, ", ")
	}
	inputs := []workflowplan.Input{planInput("config", path, current)}
	actions := []workflowplan.Action{}
	sequence := 1
	if observed.MigrationRequired {
		backupPath := path + ".alias-lens.bak"
		backupCurrent, backupErr := readRegularFile(backupPath, 1<<20)
		backupKind := workflowplan.ActionReplace
		backupExists := true
		if errors.Is(backupErr, os.ErrNotExist) {
			backupCurrent = nil
			backupKind = workflowplan.ActionCreate
			backupExists = false
		} else if backupErr != nil {
			return workflowplan.OperationPlan{}, backupErr
		}
		if backupExists {
			inputs = append(inputs, planInput("config_backup", backupPath, backupCurrent))
		}
		actions = append(actions, workflowplan.Action{
			Sequence: sequence, Kind: backupKind, TargetRole: "config_backup", DisplayPath: displayPrivatePath(backupPath),
			Reason: "save the current settings before updating their data format", Risk: workflowplan.RiskLow,
			PlannedSHA256: hashBytes(current), Backup: backupExists, Reversible: true,
			Target: plannedTarget(backupPath, backupCurrent, current),
		})
		sequence++
		reason = "update the settings data format and " + reason
	}
	actions = append(actions, workflowplan.Action{
		Sequence: sequence, Kind: kind, TargetRole: "config", DisplayPath: displayPrivatePath(path),
		Reason: reason, Risk: workflowplan.RiskReview, PlannedSHA256: hashBytes(planned), Backup: backup, Reversible: true,
		Target: plannedTarget(path, current, planned),
	})
	return workflowplan.Build(operation, inputs, actions, nil, nil), nil
}

func affectedProfileEntries(name string, adding bool) ([]string, error) {
	value, _, err := inspectLocalCatalog()
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("Alias Lens cannot check which aliases use this profile: %w", err)
	}
	observed, err := observeConfig()
	if err != nil {
		return nil, err
	}
	beforeProfiles := append([]string(nil), observed.Config.Profiles...)
	afterProfiles := append([]string(nil), beforeProfiles...)
	if adding {
		afterProfiles = append(afterProfiles, name)
	} else {
		kept := afterProfiles[:0]
		for _, profile := range afterProfiles {
			if profile != name {
				kept = append(kept, profile)
			}
		}
		afterProfiles = kept
	}
	affectedShells := map[string][]string{}
	for _, shell := range []string{"bash", "zsh"} {
		before, beforeDiagnostics := neutralcatalog.Resolve(value, neutralcatalog.ResolveContext{Shell: shell, Platform: currentPlatform(), Profiles: beforeProfiles})
		after, afterDiagnostics := neutralcatalog.Resolve(value, neutralcatalog.ResolveContext{Shell: shell, Platform: currentPlatform(), Profiles: afterProfiles})
		if len(beforeDiagnostics) > 0 || len(afterDiagnostics) > 0 {
			return nil, fmt.Errorf("Alias Lens cannot resolve the catalog for %s", friendlyShellName(shell))
		}
		for index := range before {
			if before[index].Available != after[index].Available {
				affectedShells[before[index].Entry.Name] = append(affectedShells[before[index].Entry.Name], shell)
			}
		}
	}
	names := make([]string, 0, len(affectedShells))
	for entryName := range affectedShells {
		names = append(names, entryName)
	}
	sort.Strings(names)
	result := make([]string, 0, len(names))
	for _, entryName := range names {
		result = append(result, fmt.Sprintf("%s (%s)", entryName, strings.Join(affectedShells[entryName], " and ")))
	}
	return result, nil
}

func buildConfigMigrationPlan() (workflowplan.OperationPlan, error) {
	observed, err := observeConfig()
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	path, err := configPath()
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	if !observed.Present || !observed.MigrationRequired {
		return workflowplan.Build("config.migrate", nil, nil, nil, []workflowplan.Diagnostic{{Code: "no_change", Message: "Settings already use the current data format."}}), nil
	}
	current, err := readRegularFile(path, 1<<20)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	config := observed.Config
	config.Version = currentConfigVersion
	config = ensureConfigDefaults(config)
	if err := validateAppConfig(config); err != nil {
		return workflowplan.OperationPlan{}, fmt.Errorf("Alias Lens cannot prepare the settings update: %w", err)
	}
	planned, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	planned = append(planned, '\n')
	backupPath := path + ".alias-lens.bak"
	backupCurrent, backupErr := readRegularFile(backupPath, 1<<20)
	backupKind := workflowplan.ActionReplace
	backupExists := true
	if errors.Is(backupErr, os.ErrNotExist) {
		backupCurrent = nil
		backupKind = workflowplan.ActionCreate
		backupExists = false
	} else if backupErr != nil {
		return workflowplan.OperationPlan{}, backupErr
	}
	inputs := []workflowplan.Input{planInput("config", path, current)}
	if backupExists {
		inputs = append(inputs, planInput("config_backup", backupPath, backupCurrent))
	}
	actions := []workflowplan.Action{
		{
			Sequence: 1, Kind: backupKind, TargetRole: "config_backup", DisplayPath: displayPrivatePath(backupPath),
			Reason: "save the exact current settings before updating their data format", Risk: workflowplan.RiskLow,
			PlannedSHA256: hashBytes(current), Backup: backupExists, Reversible: true,
			Target: plannedTarget(backupPath, backupCurrent, current),
		},
		{
			Sequence: 2, Kind: workflowplan.ActionReplace, TargetRole: "config", DisplayPath: displayPrivatePath(path),
			Reason: "update the settings data format without changing your choices", Risk: workflowplan.RiskReview,
			PlannedSHA256: hashBytes(planned), Backup: true, Reversible: true,
			Target: plannedTarget(path, current, planned),
		},
	}
	return workflowplan.Build("config.migrate", inputs, actions, nil, nil), nil
}

func buildCatalogMigrationPlan() (workflowplan.OperationPlan, error) {
	path := localCatalogPath()
	current, err := readRegularFile(path, neutralcatalog.MaxDocumentBytes)
	if err != nil {
		return workflowplan.OperationPlan{}, fmt.Errorf("read catalog: %w", err)
	}
	value, diagnostics := neutralcatalog.Decode(current)
	if len(diagnostics) > 0 {
		return workflowplan.OperationPlan{}, fmt.Errorf("Alias Lens cannot use this catalog because its data format is not valid")
	}
	if value.SchemaVersion == neutralcatalog.SchemaVersion2 {
		return workflowplan.Build("catalog.migrate", []workflowplan.Input{planInput("catalog", path, current)}, nil, nil, []workflowplan.Diagnostic{{Code: "no_change", Message: "The catalog already uses data format 2."}}), nil
	}
	value.SchemaVersion = neutralcatalog.SchemaVersion2
	planned, diagnostics := neutralcatalog.Encode(value)
	if len(diagnostics) > 0 {
		return workflowplan.OperationPlan{}, fmt.Errorf("Alias Lens could not prepare data format 2")
	}
	info, err := os.Stat(path)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	revisionName := fmt.Sprintf("catalog.revision-%s-%s.json", info.ModTime().UTC().Format("20060102T150405.000000000Z"), hashBytes(current)[:12])
	revisionPath := filepath.Join(filepath.Dir(path), revisionName)
	revisionCurrent, revisionErr := readRegularFile(revisionPath, neutralcatalog.MaxDocumentBytes)
	inputs := []workflowplan.Input{planInput("catalog", path, current)}
	actions := []workflowplan.Action{}
	sequence := 1
	switch {
	case errors.Is(revisionErr, os.ErrNotExist):
		actions = append(actions, workflowplan.Action{
			Sequence: sequence, Kind: workflowplan.ActionCreate, TargetRole: "catalog_revision", DisplayPath: displayPrivatePath(revisionPath),
			Reason: "save an exact dated catalog revision before updating its data format", Risk: workflowplan.RiskLow,
			PlannedSHA256: hashBytes(current), Backup: false, Reversible: true,
			Target: plannedTarget(revisionPath, nil, current),
		})
		sequence++
	case revisionErr != nil:
		return workflowplan.OperationPlan{}, revisionErr
	case !bytes.Equal(revisionCurrent, current):
		return workflowplan.OperationPlan{}, fmt.Errorf("the planned catalog revision name is already used by different data")
	default:
		inputs = append(inputs, planInput("catalog_revision", revisionPath, revisionCurrent))
	}
	actions = append(actions, workflowplan.Action{
		Sequence: sequence, Kind: workflowplan.ActionReplace, TargetRole: "catalog", DisplayPath: displayPrivatePath(path),
		Reason: "update the catalog data format without changing any entries", Risk: workflowplan.RiskReview,
		PlannedSHA256: hashBytes(planned), Backup: true, Reversible: true,
		Target: plannedTarget(path, current, planned),
	})
	return workflowplan.Build("catalog.migrate", inputs, actions, nil, nil), nil
}

func planInput(role, path string, contents []byte) workflowplan.Input {
	return workflowplan.Input{Role: role, DisplayPath: displayPrivatePath(path), SHA256: hashBytes(contents), Internal: workflowplan.InternalInput{Path: path, Identity: observePlanIdentity(path)}}
}

func plannedTarget(path string, current, planned []byte) workflowplan.Target {
	return workflowplan.Target{Path: path, ExpectedIdentity: observePlanIdentity(path), ExpectedSHA256: hashBytes(current), PlannedBytes: planned}
}

func hashBytes(contents []byte) string {
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:])
}

func displayPrivatePath(path string) string {
	home, err := os.UserHomeDir()
	if err == nil {
		if relative, relativeErr := filepath.Rel(home, path); relativeErr == nil && relative != "." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return filepath.Join("~", relative)
		}
	}
	return filepath.Base(path)
}
