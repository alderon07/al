package app

import (
	"os"
	"strings"
)

type DefaultAliasOffer struct {
	Name        string
	Command     string
	Description string
}

var developerDefaultAliases = []DefaultAliasOffer{
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

func missingDefaultAliases(contents []byte) []DefaultAliasOffer {
	existing := aliasCommandMap(contents)
	existingCommands := make(map[string]bool, len(existing))
	for _, command := range existing {
		existingCommands[normalizeHistoryCommand(command)] = true
	}
	var missing []DefaultAliasOffer
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

func (svc *Services) addDefaultAliasesToFile(path string) (int, error) {
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
		lines = svc.insertAliasLines(lines, name, command, description)
	}
	if len(missing) == 0 {
		return 0, nil
	}
	updated := []byte(strings.TrimLeft(strings.Join(lines, "\n"), "\n") + "\n")
	if err := svc.writeAliasFile(path, contents, updated, mode); err != nil {
		return 0, err
	}
	return len(missing), nil
}
