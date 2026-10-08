package main

import (
	"alias-lens/internal/entry"
	"alias-lens/internal/shell"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"
)

type EntryMetadata = entry.EntryMetadata

var functionStart = regexp.MustCompile(`^\s*(?:function\s+)?([A-Za-z_][A-Za-z0-9_]*)(?:\s*\(\s*\))?\s*\{(.*)$`)
var functionName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func parseMetadataComment(line string) (EntryMetadata, bool) { return entry.ParseMetadataComment(line) }

func splitMetadataValues(value string) []string { return entry.SplitMetadataValues(value) }

func applyMetadata(alias *Alias, metadata EntryMetadata) { entry.ApplyMetadata(alias, metadata) }

func normalizeCategory(value string) string { return entry.NormalizeCategory(value) }

func parseFunctions(contents string) []Alias {
	return parseLegacyFunctions(contents)
}

func parseLegacyFunctions(contents string) []Alias { return shell.ParseLegacyFunctions(contents) }

func currentPlatform() string {
	return platformName(runtime.GOOS, os.Getenv)
}

func platformName(goos string, getenv func(string) string) string {
	return entry.PlatformName(goos, getenv)
}

func platformSupported(platforms []string) bool {
	return entry.PlatformSupported(platforms, currentPlatform())
}

func metadataLine(metadata EntryMetadata) string { return entry.MetadataLine(metadata) }

func runMetadataCommand(arguments []string) error {
	if len(arguments) < 2 {
		return fmt.Errorf("usage: al meta ALIAS key=value [key=value]")
	}
	aliases, err := loadAliases()
	if err != nil {
		return err
	}
	metadata := EntryMetadata{}
	foundEntry := false
	for _, alias := range aliases {
		if alias.Name == arguments[0] {
			metadata = metadataForAlias(alias)
			foundEntry = true
			break
		}
	}
	if !foundEntry {
		return fmt.Errorf("entry %q was not found", arguments[0])
	}
	for _, argument := range arguments[1:] {
		key, value, found := strings.Cut(argument, "=")
		if !found {
			return fmt.Errorf("metadata must use key=value")
		}
		switch strings.ToLower(key) {
		case "tags", "collections":
			metadata.Tags = splitMetadataValues(value)
		case "platforms":
			metadata.Platforms = splitMetadataValues(value)
		case "favorite":
			metadata.Favorite = strings.EqualFold(value, "true") || value == "1" || strings.EqualFold(value, "yes")
		case "category":
			metadata.Category = normalizeCategory(value)
			if metadata.Category != "" && !aliasName.MatchString(metadata.Category) {
				return fmt.Errorf("category may only use letters, numbers, dot, dash, and underscore")
			}
		default:
			return fmt.Errorf("unknown metadata field %q", key)
		}
	}
	path, err := aliasesPath()
	if err != nil {
		return err
	}
	return setEntryMetadata(path, arguments[0], metadata)
}

func metadataForAlias(alias Alias) EntryMetadata { return entry.MetadataForAlias(alias) }

func setEntryMetadata(path, name string, metadata EntryMetadata) error {
	if active, err := catalogManagedEditing(); err != nil {
		return err
	} else if active {
		return setCatalogMetadata(name, metadata)
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
	return writeAliasFile(path, contents, updated, mode)
}
