package app

import (
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

func (svc *Services) buildProfilePlan(action, name string) (workflowplan.OperationPlan, error) {
	if !profileNamePattern.MatchString(name) {
		return workflowplan.OperationPlan{}, fmt.Errorf("profile names start with a lowercase letter and use only lowercase letters, numbers, _ or -")
	}
	observed, err := svc.observeConfig()
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
	if err := svc.validateAppConfig(config); err != nil {
		return workflowplan.OperationPlan{}, fmt.Errorf("Alias Lens cannot prepare this settings change: %w", err)
	}
	planned, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	planned = append(planned, '\n')
	path, err := svc.configPath()
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
		return workflowplan.Build(operation, []workflowplan.Input{svc.planInput("config", path, current)}, nil, nil, []workflowplan.Diagnostic{{Code: "no_change", Message: "That machine profile is already in the requested state."}}), nil
	}
	affected, err := svc.affectedProfileEntries(name, action == "add")
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	reason := "change the active machine profiles"
	if len(affected) == 0 {
		reason += "; no catalog entries change availability"
	} else {
		reason += "; affected entries: " + strings.Join(affected, ", ")
	}
	inputs := []workflowplan.Input{svc.planInput("config", path, current)}
	actions := []workflowplan.Action{}
	actions = append(actions, workflowplan.Action{
		Sequence: 1, Kind: kind, TargetRole: "config", DisplayPath: svc.displayPrivatePath(path),
		Reason: reason, Risk: workflowplan.RiskReview, PlannedSHA256: hashBytes(planned), Backup: backup, Reversible: true,
		Target: plannedTarget(path, current, planned),
	})
	return workflowplan.Build(operation, inputs, actions, nil, nil), nil
}

func (svc *Services) affectedProfileEntries(name string, adding bool) ([]string, error) {
	value, _, err := svc.inspectLocalCatalog()
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("Alias Lens cannot check which aliases use this profile: %w", err)
	}
	observed, err := svc.observeConfig()
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
		before, beforeDiagnostics := neutralcatalog.Resolve(value, neutralcatalog.ResolveContext{Shell: shell, Platform: svc.currentPlatform(), Profiles: beforeProfiles})
		after, afterDiagnostics := neutralcatalog.Resolve(value, neutralcatalog.ResolveContext{Shell: shell, Platform: svc.currentPlatform(), Profiles: afterProfiles})
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

func (svc *Services) planInput(role, path string, contents []byte) workflowplan.Input {
	return workflowplan.Input{Role: role, DisplayPath: svc.displayPrivatePath(path), SHA256: hashBytes(contents), Internal: workflowplan.InternalInput{Path: path, Identity: observePlanIdentity(path)}}
}

func plannedTarget(path string, current, planned []byte) workflowplan.Target {
	return workflowplan.Target{Path: path, ExpectedIdentity: observePlanIdentity(path), ExpectedSHA256: hashBytes(current), PlannedBytes: planned}
}

func hashBytes(contents []byte) string {
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:])
}

func (svc *Services) displayPrivatePath(path string) string {
	home, err := svc.dependencies.HomeDir()
	if err == nil {
		if relative, relativeErr := filepath.Rel(home, path); relativeErr == nil && relative != "." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return filepath.Join("~", relative)
		}
	}
	return filepath.Base(path)
}
