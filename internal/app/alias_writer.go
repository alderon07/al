package app

import (
	"alias-lens/internal/transaction"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var aliasName = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

var atomicWriteBeforeRename func(string) error

func (svc *Services) addAlias(name, command, description string) error {
	return svc.addAliasWithMetadata(name, command, description, EntryMetadata{})
}

func (svc *Services) addAliasWithMetadata(name, command, description string, metadata EntryMetadata) error {
	if active, err := svc.catalogManagedEditing(); err != nil {
		return err
	} else if active {
		return svc.addCatalogAlias(name, command, description, metadata)
	}
	path, err := svc.aliasesPath()
	if err != nil {
		return err
	}
	return svc.addAliasToFileWithMetadata(path, name, command, description, metadata)
}

func (svc *Services) addAliasToFile(path, name, command, description string) error {
	return svc.addAliasToFileWithMetadata(path, name, command, description, EntryMetadata{})
}

func (svc *Services) addAliasToFileWithMetadata(path, name, command, description string, metadata EntryMetadata) error {
	name, command, description, err := validateAliasInput(name, command, description)
	if err != nil {
		return err
	}
	metadata, err = svc.validateEditableMetadata(metadata)
	if err != nil {
		return err
	}

	contents, err := readFileLimited(path, AliasFileLimit)
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
	lines = svc.insertAliasLinesWithMetadata(lines, name, command, description, metadata)
	updated := []byte(strings.TrimLeft(strings.Join(lines, "\n"), "\n") + "\n")
	return svc.writeAliasFile(path, contents, updated, mode)
}

type aliasAddition struct {
	Name        string
	Command     string
	Description string
	Metadata    EntryMetadata
}

func (svc *Services) addAliasesToFile(path string, additions []aliasAddition) error {
	return svc.withMutation(func(session *mutationSession) error {
		return svc.addAliasesToFileInSession(session, path, additions)
	})
}

func (svc *Services) addAliasesToFileInSession(session *mutationSession, path string, additions []aliasAddition) error {
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
		addition.Metadata, err = svc.validateEditableMetadata(addition.Metadata)
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
		lines = svc.insertAliasLinesWithMetadata(lines, addition.Name, addition.Command, addition.Description, addition.Metadata)
	}
	updated := []byte(strings.TrimLeft(strings.Join(lines, "\n"), "\n") + "\n")
	if len(updated) > AliasFileLimit {
		return fmt.Errorf("alias file would exceed %d bytes", AliasFileLimit)
	}
	return svc.writeAliasFileInSession(session, path, contents, updated, mode)
}

func (svc *Services) editAlias(originalName, name, command, description string) error {
	return svc.editAliasWithMetadata(originalName, name, command, description, EntryMetadata{})
}

func (svc *Services) editAliasWithMetadata(originalName, name, command, description string, metadata EntryMetadata) error {
	if active, err := svc.catalogManagedEditing(); err != nil {
		return err
	} else if active {
		return svc.editCatalogAlias(originalName, name, command, description, metadata)
	}
	path, err := svc.aliasesPath()
	if err != nil {
		return err
	}
	return svc.editAliasInFileWithMetadata(path, originalName, name, command, description, metadata)
}

func (svc *Services) editAliasInFile(path, originalName, name, command, description string) error {
	return svc.editAliasInFileWithMetadata(path, originalName, name, command, description, EntryMetadata{})
}

func (svc *Services) editAliasInFileWithMetadata(path, originalName, name, command, description string, metadata EntryMetadata) error {
	name, command, description, err := validateAliasInput(name, command, description)
	if err != nil {
		return err
	}
	metadata, err = svc.validateEditableMetadata(metadata)
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
	lines = svc.insertAliasLinesWithMetadata(lines, name, command, description, metadata)
	updated := []byte(strings.TrimLeft(strings.Join(lines, "\n"), "\n") + "\n")
	return svc.writeAliasFile(path, contents, updated, mode)
}

func (svc *Services) deleteAlias(name string) error {
	if active, err := svc.catalogManagedEditing(); err != nil {
		return err
	} else if active {
		return svc.deleteCatalogAlias(name)
	}
	path, err := svc.aliasesPath()
	if err != nil {
		return err
	}
	return svc.deleteAliasFromFile(path, name)
}

func (svc *Services) deleteAliasFromFile(path, name string) error {
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
		return svc.writeAliasFile(path, contents, updated, mode)
	}
	return fmt.Errorf("alias %q no longer exists", name)
}

type DescriptionUpdate struct {
	Catalog bool
	Count   int
}

func (svc *Services) describeEntries() (DescriptionUpdate, error) {
	if active, e := svc.catalogManagedEditing(); e != nil {
		return DescriptionUpdate{}, e
	} else if active {
		return DescriptionUpdate{Catalog: true}, svc.addCatalogDescriptions()
	}
	path, err := svc.aliasesPath()
	if err != nil {
		return DescriptionUpdate{}, err
	}
	added, err := svc.addAliasDescriptionsToFile(path)
	if err != nil {
		return DescriptionUpdate{}, err
	}
	return DescriptionUpdate{Count: added}, nil
}

func (svc *Services) addAliasDescriptionsToFile(path string) (int, error) {
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
	if err := svc.writeAliasFile(path, contents, updated, mode); err != nil {
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

func (svc *Services) aliasesPath() (string, error) {
	return svc.aliasPathFor(svc.activeShellAdapter())
}

func readAliasFile(path string) ([]byte, os.FileMode, []string, error) {
	contents, err := readFileLimited(path, AliasFileLimit)
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

func (svc *Services) validateEditableMetadata(metadata EntryMetadata) (EntryMetadata, error) {
	metadata.Tags = svc.splitMetadataValues(strings.Join(metadata.Tags, ","))
	metadata.Category = svc.normalizeCategory(metadata.Category)
	if metadata.Category != "" && !aliasName.MatchString(metadata.Category) {
		return EntryMetadata{}, fmt.Errorf("category may only use letters, numbers, dot, dash, and underscore")
	}
	return metadata, nil
}

func (svc *Services) insertAliasLines(lines []string, name, command, description string) []string {
	return svc.insertAliasLinesWithMetadata(lines, name, command, description, EntryMetadata{})
}

func (svc *Services) insertAliasLinesWithMetadata(lines []string, name, command, description string, metadata EntryMetadata) []string {
	insertAt := len(lines)
	wantedCategory := svc.category(command)
	wantedRelation := relationKey(command)
	lastCategory, lastRelated := -1, -1
	for index, line := range lines {
		_, existingCommand, ok := parseAliasDefinition(line)
		if !ok {
			continue
		}
		if svc.category(existingCommand) == wantedCategory {
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

func (svc *Services) writeAliasFile(path string, contents, updated []byte, mode os.FileMode) error {
	return svc.withMutation(func(session *mutationSession) error {
		return svc.writeAliasFileInSession(session, path, contents, updated, mode)
	})
}
func (svc *Services) writeAliasFileInSession(session *mutationSession, path string, contents, updated []byte, mode os.FileMode) error {
	id, e := transaction.InspectWorkflowTarget(path, AliasFileLimit, true)
	if e != nil {
		return e
	}
	if id.Exists && id.SHA256 != hashBytes(contents) || !id.Exists && len(contents) != 0 {
		return fmt.Errorf("alias file changed since it was read; retry the edit")
	}
	if id.Exists {
		if e = svc.saveRevisionInSession(session, path, contents); e != nil {
			return fmt.Errorf("save revision: %w", e)
		}
		if e = writePrivateBackupInSession(session, path+".alias-lens.bak", contents); e != nil {
			return fmt.Errorf("create backup: %w", e)
		}
	}
	if !id.Exists {
		mode = 0o600
	} else {
		mode = os.FileMode(id.Mode)
	}
	return session.writeUserFile(path, contents, updated, mode, true, 0, true)
}

func aliasFileMatchesExpected(path string, expected []byte) error {
	current, err := readFileLimited(path, AliasFileLimit)
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

func (svc *Services) writePrivateBackup(path string, contents []byte) error {
	return svc.withMutation(func(session *mutationSession) error { return writePrivateBackupInSession(session, path, contents) })
}
func writePrivateBackupInSession(session *mutationSession, path string, contents []byte) error {
	id, previous, e := transaction.ReadWorkflowTarget(path, AliasFileLimit)
	if e != nil {
		return e
	}
	if id.Exists && id.Mode != 0o600 {
		return transaction.ErrUnsafePath
	}
	return session.writeUserFile(path, previous, contents, 0o600, false, 3, false)
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
