package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var aliasName = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

var atomicWriteBeforeRename func(string) error

func addAlias(name, command, description string) error {
	return addAliasWithMetadata(name, command, description, EntryMetadata{})
}

func addAliasWithMetadata(name, command, description string, metadata EntryMetadata) error {
	path, err := aliasesPath()
	if err != nil {
		return err
	}
	return addAliasToFileWithMetadata(path, name, command, description, metadata)
}

func addAliasToFile(path, name, command, description string) error {
	return addAliasToFileWithMetadata(path, name, command, description, EntryMetadata{})
}

func addAliasToFileWithMetadata(path, name, command, description string, metadata EntryMetadata) error {
	name, command, description, err := validateAliasInput(name, command, description)
	if err != nil {
		return err
	}
	metadata, err = validateEditableMetadata(metadata)
	if err != nil {
		return err
	}

	contents, err := readFileLimited(path, aliasFileLimit)
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
	lines = insertAliasLinesWithMetadata(lines, name, command, description, metadata)
	updated := []byte(strings.TrimLeft(strings.Join(lines, "\n"), "\n") + "\n")
	return writeAliasFile(path, contents, updated, mode)
}

type aliasAddition struct {
	Name        string
	Command     string
	Description string
	Metadata    EntryMetadata
}

func addAliasesToFile(path string, additions []aliasAddition) error {
	contents, mode, lines, err := readAliasFile(path)
	if err != nil {
		return err
	}
	existing := make(map[string]struct{})
	for _, line := range lines {
		name, _, ok := parseAliasDefinition(line)
		if ok {
			existing[name] = struct{}{}
		}
	}
	validated := make([]aliasAddition, 0, len(additions))
	for _, addition := range additions {
		addition.Name, addition.Command, addition.Description, err = validateAliasInput(addition.Name, addition.Command, addition.Description)
		if err != nil {
			return err
		}
		addition.Metadata, err = validateEditableMetadata(addition.Metadata)
		if err != nil {
			return err
		}
		if _, found := existing[addition.Name]; found {
			return fmt.Errorf("alias %q already exists", addition.Name)
		}
		existing[addition.Name] = struct{}{}
		validated = append(validated, addition)
	}
	for _, addition := range validated {
		lines = insertAliasLinesWithMetadata(lines, addition.Name, addition.Command, addition.Description, addition.Metadata)
	}
	updated := []byte(strings.TrimLeft(strings.Join(lines, "\n"), "\n") + "\n")
	if len(updated) > aliasFileLimit {
		return fmt.Errorf("alias file would exceed %d bytes", aliasFileLimit)
	}
	return writeAliasFile(path, contents, updated, mode)
}

func editAlias(originalName, name, command, description string) error {
	return editAliasWithMetadata(originalName, name, command, description, EntryMetadata{})
}

func editAliasWithMetadata(originalName, name, command, description string, metadata EntryMetadata) error {
	path, err := aliasesPath()
	if err != nil {
		return err
	}
	return editAliasInFileWithMetadata(path, originalName, name, command, description, metadata)
}

func editAliasInFile(path, originalName, name, command, description string) error {
	return editAliasInFileWithMetadata(path, originalName, name, command, description, EntryMetadata{})
}

func editAliasInFileWithMetadata(path, originalName, name, command, description string, metadata EntryMetadata) error {
	name, command, description, err := validateAliasInput(name, command, description)
	if err != nil {
		return err
	}
	metadata, err = validateEditableMetadata(metadata)
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
	for start > 0 {
		if _, ok := parseMetadataComment(lines[start-1]); ok || isDescriptionComment(lines[start-1]) {
			start--
			continue
		}
		break
	}
	lines = append(lines[:start], lines[aliasIndex+1:]...)
	lines = insertAliasLinesWithMetadata(lines, name, command, description, metadata)
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
		for start > 0 {
			if _, metadata := parseMetadataComment(lines[start-1]); metadata || isDescriptionComment(lines[start-1]) {
				start--
				continue
			}
			break
		}
		lines = append(lines[:start], lines[index+1:]...)
		updated := []byte(strings.TrimLeft(strings.Join(lines, "\n"), "\n") + "\n")
		return writeAliasFile(path, contents, updated, mode)
	}
	return fmt.Errorf("alias %q no longer exists", name)
}

func addAliasDescriptions() error {
	path, err := aliasesPath()
	if err != nil {
		return err
	}
	added, err := addAliasDescriptionsToFile(path)
	if err != nil {
		return err
	}
	if added == 0 {
		fmt.Println("Every alias already has a useful description.")
		return nil
	}
	fmt.Printf("Added or improved descriptions for %d aliases. Backup and private revision saved.\n", added)
	return nil
}

func addAliasDescriptionsToFile(path string) (int, error) {
	contents, mode, lines, err := readAliasFile(path)
	if err != nil {
		return 0, err
	}
	added := 0
	for index := 0; index < len(lines); index++ {
		name, command, ok := parseAliasDefinition(lines[index])
		if !ok {
			continue
		}
		description := describe(name, command)
		if descriptionIndex := aliasDescriptionIndex(lines, index); descriptionIndex >= 0 {
			existing := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[descriptionIndex]), "#"))
			if existing == legacyDescription(name, command) && existing != description {
				lines[descriptionIndex] = "# " + description
				added++
			}
			continue
		}
		insertAt := index
		for insertAt > 0 {
			if _, metadata := parseMetadataComment(lines[insertAt-1]); !metadata {
				break
			}
			insertAt--
		}
		lines = append(lines, "")
		copy(lines[insertAt+1:], lines[insertAt:])
		lines[insertAt] = "# " + description
		index++
		added++
	}
	if added == 0 {
		return 0, nil
	}
	updated := []byte(strings.Join(lines, "\n") + "\n")
	if err := writeAliasFile(path, contents, updated, mode); err != nil {
		return 0, err
	}
	return added, nil
}

func aliasDescriptionIndex(lines []string, aliasIndex int) int {
	for index := aliasIndex - 1; index >= 0; index-- {
		trimmed := strings.TrimSpace(lines[index])
		if trimmed == "" {
			continue
		}
		if _, metadata := parseMetadataComment(lines[index]); metadata {
			continue
		}
		if isDescriptionComment(lines[index]) {
			return index
		}
		return -1
	}
	return -1
}

func aliasesPath() (string, error) {
	return aliasPathFor(activeShellAdapter())
}

func readAliasFile(path string) ([]byte, os.FileMode, []string, error) {
	contents, err := readFileLimited(path, aliasFileLimit)
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

func validateEditableMetadata(metadata EntryMetadata) (EntryMetadata, error) {
	metadata.Tags = splitMetadataValues(strings.Join(metadata.Tags, ","))
	metadata.Category = normalizeCategory(metadata.Category)
	if metadata.Category != "" && !aliasName.MatchString(metadata.Category) {
		return EntryMetadata{}, fmt.Errorf("category may only use letters, numbers, dot, dash, and underscore")
	}
	return metadata, nil
}

func insertAliasLines(lines []string, name, command, description string) []string {
	return insertAliasLinesWithMetadata(lines, name, command, description, EntryMetadata{})
}

func insertAliasLinesWithMetadata(lines []string, name, command, description string, metadata EntryMetadata) []string {
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
	if line := metadataLine(metadata); line != "" {
		block = append(block, line)
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
	if err := aliasFileMatchesExpected(path, contents); err != nil {
		return err
	}
	if len(contents) > 0 {
		if err := saveRevision(path, contents); err != nil {
			return fmt.Errorf("save revision: %w", err)
		}
		if err := writePrivateBackup(path+".alias-lens.bak", contents); err != nil {
			return fmt.Errorf("create backup: %w", err)
		}
	}
	writePath := path
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		resolved, resolveErr := filepath.EvalSymlinks(path)
		if resolveErr != nil {
			return fmt.Errorf("resolve alias file symlink: %w", resolveErr)
		}
		writePath = resolved
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(writePath), ".alias-lens-write-*")
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
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if atomicWriteBeforeRename != nil {
		if err := atomicWriteBeforeRename(writePath); err != nil {
			return err
		}
	}
	if err := aliasFileMatchesExpected(writePath, contents); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, writePath); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(writePath))
}

func aliasFileMatchesExpected(path string, expected []byte) error {
	current, err := readFileLimited(path, aliasFileLimit)
	if os.IsNotExist(err) && len(expected) == 0 {
		return nil
	}
	if err != nil {
		return err
	}
	if !bytes.Equal(current, expected) {
		return fmt.Errorf("alias file changed since it was read; retry the edit")
	}
	return nil
}

func writePrivateBackup(path string, contents []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".alias-lens-backup-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if atomicWriteBeforeRename != nil {
		if err := atomicWriteBeforeRename(path); err != nil {
			return err
		}
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

func writeFileAtomically(path string, contents []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".alias-lens-atomic-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if atomicWriteBeforeRename != nil {
		if err := atomicWriteBeforeRename(path); err != nil {
			return err
		}
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
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
