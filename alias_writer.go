package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var aliasName = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

func addAlias(name, command, description string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return addAliasToFile(filepath.Join(home, ".bash_aliases"), name, command, description)
}

func addAliasToFile(path, name, command, description string) error {
	name, command, description, err := validateAliasInput(name, command, description)
	if err != nil {
		return err
	}

	contents, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	}

	lines := strings.Split(strings.TrimSuffix(string(contents), "\n"), "\n")
	for _, line := range lines {
		existingName, _, ok := parseAliasDefinition(line)
		if ok && existingName == name {
			return fmt.Errorf("alias %q already exists", name)
		}
	}
	lines = insertAliasLines(lines, name, command, description)
	updated := []byte(strings.TrimLeft(strings.Join(lines, "\n"), "\n") + "\n")
	return writeAliasFile(path, contents, updated, mode)
}

func editAlias(originalName, name, command, description string) error {
	path, err := aliasesPath()
	if err != nil {
		return err
	}
	return editAliasInFile(path, originalName, name, command, description)
}

func editAliasInFile(path, originalName, name, command, description string) error {
	name, command, description, err := validateAliasInput(name, command, description)
	if err != nil {
		return err
	}
	contents, mode, lines, err := readAliasFile(path)
	if err != nil {
		return err
	}
	aliasIndex := -1
	for index, line := range lines {
		existingName, _, ok := parseAliasDefinition(line)
		if !ok {
			continue
		}
		if existingName == name && existingName != originalName {
			return fmt.Errorf("alias %q already exists", name)
		}
		if existingName == originalName {
			aliasIndex = index
		}
	}
	if aliasIndex < 0 {
		return fmt.Errorf("alias %q no longer exists", originalName)
	}
	start := aliasIndex
	if aliasIndex > 0 && isDescriptionComment(lines[aliasIndex-1]) {
		start--
	}
	lines = append(lines[:start], lines[aliasIndex+1:]...)
	lines = insertAliasLines(lines, name, command, description)
	updated := []byte(strings.TrimLeft(strings.Join(lines, "\n"), "\n") + "\n")
	return writeAliasFile(path, contents, updated, mode)
}

func deleteAlias(name string) error {
	path, err := aliasesPath()
	if err != nil {
		return err
	}
	return deleteAliasFromFile(path, name)
}

func deleteAliasFromFile(path, name string) error {
	contents, mode, lines, err := readAliasFile(path)
	if err != nil {
		return err
	}
	for index, line := range lines {
		existingName, _, ok := parseAliasDefinition(line)
		if !ok || existingName != name {
			continue
		}
		start := index
		if index > 0 && isDescriptionComment(lines[index-1]) {
			start--
		}
		lines = append(lines[:start], lines[index+1:]...)
		updated := []byte(strings.TrimLeft(strings.Join(lines, "\n"), "\n") + "\n")
		return writeAliasFile(path, contents, updated, mode)
	}
	return fmt.Errorf("alias %q no longer exists", name)
}

func aliasesPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".bash_aliases"), nil
}

func readAliasFile(path string) ([]byte, os.FileMode, []string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, nil, err
	}
	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	}
	lines := strings.Split(strings.TrimSuffix(string(contents), "\n"), "\n")
	return contents, mode, lines, nil
}

func validateAliasInput(name, command, description string) (string, string, string, error) {
	name = strings.TrimSpace(name)
	command = strings.TrimSpace(command)
	description = strings.TrimSpace(description)
	if name == "" || !aliasName.MatchString(name) {
		return "", "", "", fmt.Errorf("name may only use letters, numbers, dot, dash, and underscore")
	}
	if command == "" {
		return "", "", "", fmt.Errorf("command cannot be empty")
	}
	if strings.ContainsAny(command, "\r\n") || strings.ContainsAny(description, "\r\n") {
		return "", "", "", fmt.Errorf("aliases must fit on one line")
	}
	return name, command, description, nil
}

func insertAliasLines(lines []string, name, command, description string) []string {
	insertAt := len(lines)
	wantedCategory := category(command)
	wantedRelation := relationKey(command)
	lastCategory, lastRelated := -1, -1
	for index, line := range lines {
		_, existingCommand, ok := parseAliasDefinition(line)
		if !ok {
			continue
		}
		if category(existingCommand) == wantedCategory {
			lastCategory = index
		}
		if relationKey(existingCommand) == wantedRelation {
			lastRelated = index
		}
	}
	if lastRelated >= 0 {
		insertAt = lastRelated + 1
	} else if lastCategory >= 0 {
		insertAt = lastCategory + 1
	}
	block := []string{""}
	if description != "" {
		block = append(block, "# "+description)
	}
	block = append(block, fmt.Sprintf("alias %s=%s", name, shellQuote(command)))
	lines = append(lines, make([]string, len(block))...)
	copy(lines[insertAt+len(block):], lines[insertAt:])
	copy(lines[insertAt:], block)
	return lines
}

func isDescriptionComment(line string) bool {
	note := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "#"))
	return strings.HasPrefix(strings.TrimSpace(line), "#") && note != "" && !strings.Contains(note, "===") && !strings.Contains(note, "---") && !isSectionHeading(note)
}

func writeAliasFile(path string, contents, updated []byte, mode os.FileMode) error {

	if len(contents) > 0 {
		if err := saveRevision(path, contents); err != nil {
			return fmt.Errorf("save revision: %w", err)
		}
		if err := os.WriteFile(path+".alias-lens.bak", contents, mode); err != nil {
			return fmt.Errorf("create backup: %w", err)
		}
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".bash_aliases.alias-lens-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(updated); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return nil
}

func relationKey(command string) string {
	fields := strings.Fields(strings.ToLower(command))
	if len(fields) == 0 {
		return "custom"
	}
	if len(fields) > 1 && (fields[0] == "git" || fields[0] == "docker") {
		return fields[0] + " " + fields[1]
	}
	return fields[0]
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
