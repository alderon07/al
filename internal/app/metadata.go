package app

import (
	"alias-lens/internal/entry"
	"alias-lens/internal/shell"
	"fmt"

	"regexp"
	"runtime"
	"strings"
)

type EntryMetadata = entry.EntryMetadata

var functionStart = regexp.MustCompile(`^\s*(?:function\s+)?([A-Za-z_][A-Za-z0-9_]*)(?:\s*\(\s*\))?\s*\{(.*)$`)
var functionName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func parseMetadataComment(line string) (EntryMetadata, bool) { return entry.ParseMetadataComment(line) }

func (svc *Services) splitMetadataValues(value string) []string {
	return entry.SplitMetadataValues(value)
}

func applyMetadata(alias *Alias, metadata EntryMetadata) { entry.ApplyMetadata(alias, metadata) }

func (svc *Services) normalizeCategory(value string) string {
	return entry.NormalizeCategory(value)
}

func parseFunctions(contents string) []Alias {
	return parseLegacyFunctions(contents)
}

func parseLegacyFunctions(contents string) []Alias { return shell.ParseLegacyFunctions(contents) }

func (svc *Services) currentPlatform() string {
	return platformName(runtime.GOOS, svc.dependencies.Environment)
}

func platformName(goos string, getenv func(string) string) string {
	return entry.PlatformName(goos, getenv)
}

func (svc *Services) platformSupported(platforms []string) bool {
	return entry.PlatformSupported(platforms, svc.currentPlatform())
}

func metadataLine(metadata EntryMetadata) string { return entry.MetadataLine(metadata) }

func (svc *Services) metadataForAlias(alias Alias) EntryMetadata {
	return entry.MetadataForAlias(alias)
}

func (svc *Services) setEntryMetadata(path, name string, metadata EntryMetadata) error {
	if active, err := svc.catalogManagedEditing(); err != nil {
		return err
	} else if active {
		return svc.setCatalogMetadata(name, metadata)
	}
	contents, mode, lines, err := readAliasFile(path)
	if err != nil {
		return err
	}
	entryIndex := -1
	for index, line := range lines {
		aliasName, _, aliasOK := parseAliasDefinition(line)
		functionMatch := functionStart.FindStringSubmatch(line)
		if (aliasOK && aliasName == name) || (functionMatch != nil && functionMatch[1] == name) {
			entryIndex = index
			break
		}
	}
	if entryIndex < 0 {
		return fmt.Errorf("entry %q was not found", name)
	}
	if entryIndex > 0 {
		if _, ok := parseMetadataComment(lines[entryIndex-1]); ok {
			lines = append(lines[:entryIndex-1], lines[entryIndex:]...)
			entryIndex--
		}
	}
	if line := metadataLine(metadata); line != "" {
		lines = append(lines, "")
		copy(lines[entryIndex+1:], lines[entryIndex:])
		lines[entryIndex] = line
	}
	updated := []byte(strings.TrimLeft(strings.Join(lines, "\n"), "\n") + "\n")
	return svc.writeAliasFile(path, contents, updated, mode)
}
