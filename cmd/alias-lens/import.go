package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type importIssue struct {
	Line    int
	Kind    string
	Message string
	Fatal   bool
}

type importPlan struct {
	Add    []Alias
	Skip   []string
	Issues []importIssue
}

func runImportCommand(arguments []string) error {
	apply := false
	if len(arguments) == 2 && arguments[1] == "--apply" {
		apply = true
	} else if len(arguments) != 1 {
		return fmt.Errorf("usage: al import FILE [--apply]")
	}
	sourcePath, err := filepath.Abs(arguments[0])
	if err != nil {
		return err
	}
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read import file: %w", err)
	}
	aliasPath, err := aliasesPath()
	if err != nil {
		return err
	}
	current, err := os.ReadFile(aliasPath)
	if err != nil {
		return err
	}
	plan := buildImportPlan(source, current, activeShellAdapter())
	if native := activeShellAdapter().CheckSyntax(sourcePath); native != nil {
		plan.Issues = append(plan.Issues, importIssue{Line: native.Line, Kind: strings.ToLower(string(native.Severity)), Message: native.Message, Fatal: native.Severity == checkError})
	}
	printImportPlan(sourcePath, plan)
	if !apply {
		fmt.Println("Preview only. Run the same command with --apply to write these changes.")
		return nil
	}
	for _, issue := range plan.Issues {
		if issue.Fatal {
			return fmt.Errorf("import has blocking problems; fix them and rerun al import %s", sourcePath)
		}
	}
	if len(plan.Add) == 0 {
		fmt.Println("No aliases need to be imported.")
		return nil
	}
	info, err := os.Stat(aliasPath)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimSuffix(string(current), "\n"), "\n")
	for _, alias := range plan.Add {
		metadata := EntryMetadata{Tags: alias.Tags, Platforms: alias.Platforms, Favorite: alias.Favorite}
		if alias.Category != category(alias.Command) {
			metadata.Category = alias.Category
		}
		lines = insertAliasLinesWithMetadata(lines, alias.Name, alias.Command, alias.Description, metadata)
	}
	updated := []byte(strings.TrimLeft(strings.Join(lines, "\n"), "\n") + "\n")
	if err := writeAliasFile(aliasPath, current, updated, info.Mode().Perm()); err != nil {
		return err
	}
	fmt.Printf("Imported %d aliases. Backup and private revision saved.\n", len(plan.Add))
	return nil
}

func buildImportPlan(source, current []byte, adapter ShellAdapter) importPlan {
	plan := importPlan{}
	for _, finding := range checkAliasContents(source) {
		fatal := finding.Severity == checkError
		plan.Issues = append(plan.Issues, importIssue{Line: finding.Line, Kind: strings.ToLower(string(finding.Severity)), Message: finding.Message, Fatal: fatal})
	}
	imports := parseImportAliases(source, adapter)
	if functions := adapter.ParseFunctions(string(source)); len(functions) > 0 {
		plan.Issues = append(plan.Issues, importIssue{Kind: "manual review", Message: fmt.Sprintf("%d shell functions are valid but alias import does not add functions", len(functions))})
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
				plan.Issues = append(plan.Issues, importIssue{Kind: "conflict", Message: fmt.Sprintf("alias %q already has a different command", alias.Name), Fatal: true})
			}
			continue
		}
		if names := currentByCommand[alias.Command]; len(names) > 0 {
			sort.Strings(names)
			plan.Issues = append(plan.Issues, importIssue{Kind: "duplicate command", Message: fmt.Sprintf("alias %q repeats the command used by %q", alias.Name, names[0])})
		}
		if first, found := seenCommands[alias.Command]; found {
			plan.Issues = append(plan.Issues, importIssue{Kind: "duplicate command", Message: fmt.Sprintf("aliases %q and %q use the same command", first, alias.Name)})
		} else {
			seenCommands[alias.Command] = alias.Name
		}
		plan.Add = append(plan.Add, alias)
	}
	sort.SliceStable(plan.Issues, func(i, j int) bool { return plan.Issues[i].Line < plan.Issues[j].Line })
	return plan
}

func parseImportAliases(contents []byte, adapter ShellAdapter) []Alias {
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
		alias := Alias{Name: name, Command: command, Description: description, Category: category(command), Type: "alias"}
		applyMetadata(&alias, metadata)
		aliases = append(aliases, alias)
		notes = nil
		metadata = EntryMetadata{}
	}
	return aliases
}

func printImportPlan(path string, plan importPlan) {
	fmt.Println("Import preview:", path)
	for _, issue := range plan.Issues {
		location := ""
		if issue.Line > 0 {
			location = fmt.Sprintf(" line %d", issue.Line)
		}
		fmt.Printf("ISSUE %-17s%s  %s\n", issue.Kind, location, issue.Message)
	}
	for _, skipped := range plan.Skip {
		fmt.Println("SKIP ", skipped)
	}
	for _, alias := range plan.Add {
		fmt.Printf("ADD    %-20s %s\n", alias.Name, alias.Command)
	}
	fmt.Printf("Planned: %d add, %d skip, %d issues.\n", len(plan.Add), len(plan.Skip), len(plan.Issues))
}
