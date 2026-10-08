package app

import (
	"fmt"
	"os"

	"sort"
	"strings"
)

const ImportFileLimit = 8 << 20

type ImportIssue struct {
	Line    int
	Kind    string
	Message string
	Fatal   bool
}

type ImportPlan struct {
	Add    []Alias
	Skip   []string
	Issues []ImportIssue
}

func checkImportSyntax(contents []byte, adapter ShellAdapter) (*aliasCheckFinding, error) {
	file, err := os.CreateTemp("", ".alias-lens-import-*")
	if err != nil {
		return nil, fmt.Errorf("prepare import syntax check: %w", err)
	}
	path := file.Name()
	defer os.Remove(path)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return nil, fmt.Errorf("prepare import syntax check: %w", err)
	}
	if _, err := file.Write(contents); err != nil {
		file.Close()
		return nil, fmt.Errorf("prepare import syntax check: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("prepare import syntax check: %w", err)
	}
	return adapter.CheckSyntax(path), nil
}

func (svc *Services) buildImportPlan(source, current []byte, adapter ShellAdapter) ImportPlan {
	plan := ImportPlan{}
	for _, finding := range checkAliasContents(source) {
		fatal := finding.Severity == CheckError
		plan.Issues = append(plan.Issues, ImportIssue{Line: finding.Line, Kind: strings.ToLower(string(finding.Severity)), Message: finding.Message, Fatal: fatal})
	}
	imports := svc.parseImportAliases(source, adapter)
	if functions := adapter.ParseFunctions(string(source)); len(functions) > 0 {
		plan.Issues = append(plan.Issues, ImportIssue{Kind: "manual review", Message: fmt.Sprintf("%d shell functions are valid but alias import does not add functions", len(functions))})
	}
	currentByName := aliasCommandMap(current)
	currentByCommand := make(map[string][]string)
	for name, command := range currentByName {
		currentByCommand[command] = append(currentByCommand[command], name)
	}
	seenCommands := make(map[string]string)
	seenNames := make(map[string]bool)
	for _, alias := range imports {
		if seenNames[alias.Name] {
			continue
		}
		seenNames[alias.Name] = true
		if existing, found := currentByName[alias.Name]; found {
			if existing == alias.Command {
				plan.Skip = append(plan.Skip, alias.Name+" (already exists)")
			} else {
				plan.Issues = append(plan.Issues, ImportIssue{Kind: "conflict", Message: fmt.Sprintf("alias %q already has a different command", alias.Name), Fatal: true})
			}
			continue
		}
		if names := currentByCommand[alias.Command]; len(names) > 0 {
			sort.Strings(names)
			plan.Issues = append(plan.Issues, ImportIssue{Kind: "duplicate command", Message: fmt.Sprintf("alias %q repeats the command used by %q", alias.Name, names[0])})
		}
		if first, found := seenCommands[alias.Command]; found {
			plan.Issues = append(plan.Issues, ImportIssue{Kind: "duplicate command", Message: fmt.Sprintf("aliases %q and %q use the same command", first, alias.Name)})
		} else {
			seenCommands[alias.Command] = alias.Name
		}
		plan.Add = append(plan.Add, alias)
	}
	sort.SliceStable(plan.Issues, func(i, j int) bool { return plan.Issues[i].Line < plan.Issues[j].Line })
	return plan
}

func (svc *Services) parseImportAliases(contents []byte, adapter ShellAdapter) []Alias {
	lines := strings.Split(string(contents), "\n")
	var aliases []Alias
	var notes []string
	metadata := EntryMetadata{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			if parsed, ok := parseMetadataComment(line); ok {
				metadata = parsed
				continue
			}
			note := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
			if note != "" {
				notes = append(notes, note)
			}
			continue
		}
		name, command, ok := adapter.ParseAliasDefinition(line)
		if !ok {
			if trimmed != "" {
				notes = nil
				metadata = EntryMetadata{}
			}
			continue
		}
		description := "Imported from " + adapter.DisplayName()
		if len(notes) > 0 {
			description = notes[len(notes)-1]
		}
		alias := Alias{Name: name, Command: command, Description: description, Category: svc.category(command), Type: "alias"}
		applyMetadata(&alias, metadata)
		aliases = append(aliases, alias)
		notes = nil
		metadata = EntryMetadata{}
	}
	return aliases
}
