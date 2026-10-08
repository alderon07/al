package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"strings"
	"unicode/utf8"

	neutralcatalog "github.com/alderon07/al/internal/catalog"
	workflowplan "github.com/alderon07/al/internal/plan"
)

const (
	completionSourceLimit = 8 << 20
	completionOutputLimit = 1 << 20
	completionMaxResults  = 1000
)

func (svc *Services) completionIntegrationState(adapter ShellAdapter) string {
	path, err := svc.aliasPathFor(adapter)
	if err != nil {
		return "missing"
	}
	contents, err := readRegularFile(path, completionSourceLimit)
	if err != nil {
		return "missing"
	}
	marker := "# Alias Lens " + adapter.DisplayName() + " integration"
	evalLine := `eval "$(command alias-lens shell-init ` + adapter.Name() + `)"`
	lines := strings.Split(string(contents), "\n")
	recognizableLines := 0
	healthyPairs := 0
	for index, line := range lines {
		switch {
		case line == marker:
			recognizableLines++
			if index+1 < len(lines) && lines[index+1] == evalLine {
				healthyPairs++
			}
		case line == evalLine:
			recognizableLines++
		case strings.Contains(line, marker), strings.Contains(line, "alias-lens shell-init "+adapter.Name()):
			recognizableLines++
		}
	}
	if healthyPairs == 1 && recognizableLines == 2 {
		return "ready"
	}
	if recognizableLines > 0 {
		return "edited"
	}
	return "missing"
}

func (svc *Services) previewCompletionPlan(action, shell string, script []byte) (workflowplan.OperationPlan, error) {
	if action != "install" && action != "remove" {
		return workflowplan.OperationPlan{}, fmt.Errorf("choose install or remove")
	}
	path, err := svc.completionInstallPath(shell)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	adapter, _ := svc.shellAdapter(shell)
	if svc.completionIntegrationState(adapter) == "edited" {
		return workflowplan.OperationPlan{}, fmt.Errorf("the Alias Lens section in your %s alias file was edited; enter al setup --repair %s before changing suggestions", shell, shell)
	}
	current, readErr := readRegularFile(path, completionOutputLimit)
	kind := workflowplan.ActionReplace
	backup := true
	if errors.Is(readErr, os.ErrNotExist) {
		current = nil
		kind = workflowplan.ActionCreate
		backup = false
	} else if readErr != nil {
		return workflowplan.OperationPlan{}, readErr
	}
	planned := []byte{}
	reason := "remove shell suggestions without changing aliases"
	operation := "completion.remove"
	if action == "install" {
		operation = "completion.install"
		reason = "install shell suggestions generated from the Alias Lens command list"
		if shell == "bash" {
			planned = script
		} else {
			planned = script
		}
	}
	expectedManaged := script
	if shell == "zsh" {
		expectedManaged = script
	}
	if current != nil && !bytes.Equal(current, expectedManaged) {
		return workflowplan.OperationPlan{}, fmt.Errorf("the saved %s suggestions were changed outside Alias Lens; move or remove %s yourself, then try again", shell, svc.displayPrivatePath(path))
	}
	inputs := []workflowplan.Input{}
	if current != nil {
		inputs = append(inputs, svc.planInput("completion_file", path, current))
	}
	if action == "remove" && current == nil {
		return workflowplan.Build(operation, inputs, nil, nil, []workflowplan.Diagnostic{{Code: "no_change", Message: "Shell suggestions are already in the requested state."}}), nil
	}
	if action == "install" && bytes.Equal(current, planned) {
		return workflowplan.Build(operation, inputs, nil, nil, []workflowplan.Diagnostic{{Code: "no_change", Message: "Shell suggestions are already in the requested state."}}), nil
	}
	if action == "remove" {
		kind = workflowplan.ActionRemove
		backup = true
	}
	return workflowplan.Build(operation, inputs, []workflowplan.Action{{Sequence: 1, Kind: kind, TargetRole: "completion_file", DisplayPath: svc.displayPrivatePath(path), Reason: reason, Risk: workflowplan.RiskLow, PlannedSHA256: hashBytes(planned), Backup: backup, Reversible: true, Target: plannedTarget(path, current, planned)}}, nil, nil), nil
}

func (svc *Services) completionInstallPath(shell string) (string, error) {
	if _, err := svc.shellAdapter(shell); err != nil {
		return "", err
	}
	path, err := svc.configPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "completion."+shell), nil
}

func (svc *Services) completionEntryCandidates(adapter ShellAdapter) ([]string, error) {
	if names, handled, err := svc.catalogInstalledCompletionNames(adapter.Name()); handled || err != nil {
		return names, err
	}
	active, err := svc.completionCatalogActive(adapter.Name())
	if err != nil {
		return nil, err
	}
	if !active {
		aliasPath, err := svc.aliasPathFor(adapter)
		if err != nil {
			return nil, err
		}
		contents, err := readCompletionSource(aliasPath)
		if err != nil {
			return nil, err
		}
		return completionLegacyEntries(contents, adapter)
	}
	value, err := svc.readInstalledCatalogSnapshot(adapter.Name())
	if err != nil {
		return nil, err
	}
	return svc.completionCatalogEntries(value, adapter.Name())
}

func (svc *Services) completionCatalogActive(shell string) (bool, error) {
	home, err := svc.dependencies.HomeDir()
	if err != nil {
		return false, err
	}
	path := filepath.Join(home, ".local", "state", "alias-lens", "catalog-state.json")
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("catalog state is not a regular file")
	}
	contents, err := svc.readObservedPrivateFile(path, completionSourceLimit)
	if err != nil {
		return false, err
	}
	var state struct {
		SchemaVersion   int                        `json:"schema_version"`
		InstalledShells map[string]json.RawMessage `json:"installed_shells"`
	}
	if err := json.Unmarshal(contents, &state); err != nil || state.SchemaVersion != 1 {
		return false, fmt.Errorf("catalog state is invalid")
	}
	_, active := state.InstalledShells[shell]
	return active, nil
}

func (svc *Services) completionCatalogEntries(catalog neutralcatalog.Catalog, shell string) ([]string, error) {
	profiles, err := svc.readCompletionProfiles()
	if err != nil {
		return nil, err
	}
	activeProfiles := make(map[string]bool, len(profiles))
	for _, profile := range profiles {
		activeProfiles[profile] = true
	}
	var names []string
	for _, entry := range catalog.Entries {
		if !svc.completionCatalogEntryActive(entry, shell, activeProfiles) {
			continue
		}
		names = append(names, entry.Name)
	}
	return uniqueCompletionNames(names), nil
}

func (svc *Services) completionCatalogEntryActive(entry neutralcatalog.Entry, shell string, profiles map[string]bool) bool {
	if entry.Portable == nil {
		if _, ok := entry.Native[shell]; !ok {
			return false
		}
	}
	if !svc.platformSupported(entry.Platforms) {
		return false
	}
	if entry.When == nil {
		return true
	}
	if len(entry.When.Shells) > 0 && !containsString(entry.When.Shells, shell) {
		return false
	}
	if len(entry.When.ProfilesAny) > 0 {
		matched := false
		for _, profile := range entry.When.ProfilesAny {
			matched = matched || profiles[profile]
		}
		if !matched {
			return false
		}
	}
	for _, profile := range entry.When.ProfilesNone {
		if profiles[profile] {
			return false
		}
	}
	return true
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func completionLegacyEntries(contents []byte, adapter ShellAdapter) ([]string, error) {
	var names []string
	inFunction := false
	seen := map[string]bool{}
	for index, line := range strings.Split(string(contents), "\n") {
		trimmed := strings.TrimSpace(line)
		if inFunction {
			if trimmed == "}" || strings.HasPrefix(trimmed, "};") {
				inFunction = false
			}
			continue
		}
		if name, _, ok := adapter.ParseAliasDefinition(line); ok {
			if checkAliasValueSyntax(line, index+1) != nil || seen[name] {
				return nil, fmt.Errorf("alias file contains an invalid or duplicate alias")
			}
			seen[name] = true
			names = append(names, name)
			continue
		}
		if strings.HasPrefix(trimmed, "alias ") {
			return nil, fmt.Errorf("alias file contains a malformed alias")
		}
		if match := functionStart.FindStringSubmatch(line); match != nil {
			if seen[match[1]] {
				return nil, fmt.Errorf("alias file contains a duplicate entry")
			}
			seen[match[1]] = true
			names = append(names, match[1])
			remainder := strings.TrimSpace(match[2])
			inFunction = !strings.Contains(remainder, "}")
		}
	}
	if inFunction {
		return nil, fmt.Errorf("alias file contains an incomplete function")
	}
	return uniqueCompletionNames(names), nil
}

func (svc *Services) completionProfileCandidates() ([]string, error) {
	profiles, err := svc.readCompletionProfiles()
	if err != nil {
		return nil, err
	}
	return uniqueCompletionNames(profiles), nil
}

func (svc *Services) readCompletionProfiles() ([]string, error) {
	observed, err := svc.observeConfig()
	if err != nil {
		return nil, err
	}
	return append([]string(nil), observed.Config.Profiles...), nil
}

func readCompletionSource(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > completionSourceLimit {
		return nil, fmt.Errorf("completion source is not a bounded regular file")
	}
	contents, err := io.ReadAll(io.LimitReader(file, completionSourceLimit+1))
	if err != nil || len(contents) > completionSourceLimit || !utf8.Valid(contents) {
		return nil, fmt.Errorf("completion source could not be read safely")
	}
	return contents, nil
}
