package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

type defaultAlias struct {
	Name        string
	Command     string
	Description string
}

var developerDefaultAliases = []defaultAlias{
	{Name: "gs", Command: "git status --short --branch", Description: "Show changed files and the current branch"},
	{Name: "ga", Command: "git add", Description: "Stage files for the next commit"},
	{Name: "gc", Command: "git commit", Description: "Create a commit from staged changes"},
	{Name: "gd", Command: "git diff", Description: "Show unstaged changes"},
	{Name: "gl", Command: "git log --oneline --graph --decorate", Description: "Show compact branch history"},
	{Name: "ll", Command: "ls -alF", Description: "Show all files with details"},
}

func interactiveInput(input *os.File) bool {
	info, err := input.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func missingDefaultAliases(contents []byte) []defaultAlias {
	existing := aliasCommandMap(contents)
	existingCommands := make(map[string]bool, len(existing))
	for _, command := range existing {
		existingCommands[normalizeHistoryCommand(command)] = true
	}
	var missing []defaultAlias
	for _, candidate := range developerDefaultAliases {
		if _, nameTaken := existing[candidate.Name]; nameTaken {
			continue
		}
		if existingCommands[normalizeHistoryCommand(candidate.Command)] {
			continue
		}
		missing = append(missing, candidate)
	}
	return missing
}

func offerDefaultAliases(path string, input io.Reader, output io.Writer) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	missing := missingDefaultAliases(contents)
	if len(missing) == 0 {
		fmt.Fprintln(output, "No optional developer aliases are missing.")
		return nil
	}

	fmt.Fprintln(output)
	fmt.Fprintln(output, "Alias Lens can add these optional shortcuts to ~/.bash_aliases:")
	for _, candidate := range missing {
		fmt.Fprintf(output, "  %-3s  %-43s %s\n", candidate.Name, candidate.Command, candidate.Description)
	}
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Nothing above runs during setup. The aliases only run when you type their names later.")
	fmt.Fprintln(output, "Existing alias names and equivalent commands will be skipped. Before writing, Alias Lens creates a backup and private revision.")
	fmt.Fprint(output, "Add these optional aliases? [y/N] ")

	answer, err := bufio.NewReader(input).ReadString('\n')
	if err != nil && err != io.EOF {
		return err
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer != "y" && answer != "yes" {
		fmt.Fprintln(output, "Skipped optional developer aliases.")
		return nil
	}

	added, err := addDefaultAliasesToFile(path)
	if err != nil {
		return err
	}
	fmt.Fprintf(output, "Added %d optional developer aliases. Start a new Bash shell before using them.\n", added)
	return nil
}

func addDefaultAliasesToFile(path string) (int, error) {
	contents, mode, lines, err := readAliasFile(path)
	if err != nil {
		return 0, err
	}
	missing := missingDefaultAliases(contents)
	for _, candidate := range missing {
		name, command, description, validationErr := validateAliasInput(candidate.Name, candidate.Command, candidate.Description)
		if validationErr != nil {
			return 0, validationErr
		}
		lines = insertAliasLines(lines, name, command, description)
	}
	if len(missing) == 0 {
		return 0, nil
	}
	updated := []byte(strings.TrimLeft(strings.Join(lines, "\n"), "\n") + "\n")
	if err := writeAliasFile(path, contents, updated, mode); err != nil {
		return 0, err
	}
	return len(missing), nil
}
