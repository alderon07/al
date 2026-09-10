package main

import (
	"fmt"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

type EntryMetadata struct {
	Tags      []string
	Platforms []string
	Favorite  bool
}

var functionStart = regexp.MustCompile(`^\s*(?:function\s+)?([A-Za-z_][A-Za-z0-9_]*)(?:\s*\(\s*\))?\s*\{(.*)$`)

func parseMetadataComment(line string) (EntryMetadata, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(strings.ToLower(trimmed), "# al:") {
		return EntryMetadata{}, false
	}
	metadata := EntryMetadata{}
	for _, field := range strings.Fields(strings.TrimSpace(trimmed[len("# al:"):])) {
		key, value, found := strings.Cut(field, "=")
		if !found {
			continue
		}
		values := splitMetadataValues(value)
		switch strings.ToLower(key) {
		case "tags", "collections":
			metadata.Tags = values
		case "platforms":
			metadata.Platforms = values
		case "favorite":
			metadata.Favorite = strings.EqualFold(value, "true") || value == "1" || strings.EqualFold(value, "yes")
		}
	}
	return metadata, true
}

func splitMetadataValues(value string) []string {
	seen := map[string]bool{}
	var values []string
	for _, item := range strings.Split(value, ",") {
		item = strings.ToLower(strings.TrimSpace(item))
		if item != "" && !seen[item] {
			seen[item] = true
			values = append(values, item)
		}
	}
	sort.Strings(values)
	return values
}

func applyMetadata(alias *Alias, metadata EntryMetadata) {
	alias.Tags = append([]string(nil), metadata.Tags...)
	alias.Platforms = append([]string(nil), metadata.Platforms...)
	alias.Favorite = metadata.Favorite
}

func parseFunctions(contents string) []Alias {
	lines := strings.Split(contents, "\n")
	var functions []Alias
	var notes []string
	metadata := EntryMetadata{}
	for index := 0; index < len(lines); index++ {
		trimmed := strings.TrimSpace(lines[index])
		if parsed, ok := parseMetadataComment(lines[index]); ok {
			metadata = parsed
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			note := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
			if note != "" && !isSectionHeading(note) {
				notes = append(notes, note)
			}
			continue
		}
		match := functionStart.FindStringSubmatch(lines[index])
		if match == nil {
			if trimmed != "" {
				notes = nil
				metadata = EntryMetadata{}
			}
			continue
		}
		name := match[1]
		body := strings.TrimSpace(match[2])
		if closeIndex := strings.LastIndex(body, "}"); closeIndex >= 0 {
			body = body[:closeIndex]
		} else {
			var bodyLines []string
			if body != "" {
				bodyLines = append(bodyLines, body)
			}
			for index++; index < len(lines); index++ {
				if strings.TrimSpace(lines[index]) == "}" {
					break
				}
				bodyLines = append(bodyLines, strings.TrimSpace(lines[index]))
			}
			body = strings.Join(bodyLines, " ")
		}
		body = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(body), ";"))
		description := "Shell function"
		if len(notes) > 0 {
			description = notes[len(notes)-1]
		}
		function := Alias{Name: name, Command: body, Description: description, Category: category(body), Type: "function"}
		applyMetadata(&function, metadata)
		functions = append(functions, function)
		notes = nil
		metadata = EntryMetadata{}
	}
	return functions
}

func currentPlatform() string {
	if runtime.GOOS == "linux" && (os.Getenv("WSL_DISTRO_NAME") != "" || os.Getenv("WSL_INTEROP") != "") {
		return "wsl"
	}
	return runtime.GOOS
}

func platformSupported(platforms []string) bool {
	if len(platforms) == 0 {
		return true
	}
	current := currentPlatform()
	for _, platform := range platforms {
		if platform == "all" || platform == current || (current == "wsl" && platform == "linux") {
			return true
		}
	}
	return false
}

func metadataLine(metadata EntryMetadata) string {
	var fields []string
	if len(metadata.Tags) > 0 {
		fields = append(fields, "tags="+strings.Join(metadata.Tags, ","))
	}
	if len(metadata.Platforms) > 0 {
		fields = append(fields, "platforms="+strings.Join(metadata.Platforms, ","))
	}
	if metadata.Favorite {
		fields = append(fields, "favorite=true")
	}
	if len(fields) == 0 {
		return ""
	}
	return "# al: " + strings.Join(fields, " ")
}

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
			metadata = EntryMetadata{Tags: alias.Tags, Platforms: alias.Platforms, Favorite: alias.Favorite}
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

func setEntryMetadata(path, name string, metadata EntryMetadata) error {
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
