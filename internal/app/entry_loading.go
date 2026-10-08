package app

import (
	"alias-lens/internal/entry"
	"alias-lens/internal/shell"

	"fmt"

	"os"
	"os/exec"

	"sort"

	"strings"
)

func (svc *Services) loadAliases() ([]Alias, error) {
	adapter := svc.activeShellAdapter()
	path, err := svc.aliasesPath()
	if err != nil {
		return nil, fmt.Errorf("find alias file: %w", err)
	}
	contents, err := readFileLimited(path, AliasFileLimit)
	if os.IsNotExist(err) {
		if createErr := svc.ensureAliasFileExists(path); createErr != nil {
			return nil, createErr
		}
		contents, err = readFileLimited(path, AliasFileLimit)
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var aliases []Alias
	var notes []string
	metadata := EntryMetadata{}
	for _, line := range strings.Split(string(contents), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			if parsed, ok := parseMetadataComment(line); ok {
				metadata = parsed
				continue
			}
			note := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
			if strings.Contains(note, "===") || strings.Contains(note, "---") || isSectionHeading(note) {
				notes = nil
				continue
			}
			if note != "" {
				notes = append(notes, note)
			}
			continue
		}

		name, command, ok := adapter.ParseAliasDefinition(line)
		if !ok {
			notes = nil
			metadata = EntryMetadata{}
			continue
		}
		description := describe(name, command)
		if len(notes) > 0 {
			description = notes[len(notes)-1]
		}
		alias := Alias{
			Name:        name,
			Command:     command,
			Description: description,
			Category:    svc.category(command),
			Type:        "alias",
		}
		applyMetadata(&alias, metadata)
		aliases = append(aliases, alias)
		notes = nil
		metadata = EntryMetadata{}
	}
	aliases = append(aliases, adapter.ParseFunctions(string(contents))...)

	sort.Slice(aliases, func(i, j int) bool { return strings.ToLower(aliases[i].Name) < strings.ToLower(aliases[j].Name) })
	aliases, err = svc.catalogEntryFacade(adapter.Name(), aliases)
	if err != nil {
		return nil, err
	}
	annotateUsage(aliases, svc.loadHistoryCounts())
	svc.annotateHealth(aliases)
	return aliases, nil
}

func (svc *Services) annotateHealth(aliases []Alias) {
	counts := make(map[string]int, len(aliases))
	for _, alias := range aliases {
		counts[alias.Name]++
	}
	for index := range aliases {
		alias := &aliases[index]
		if counts[alias.Name] > 1 {
			alias.Issues = append(alias.Issues, "duplicate definition")
		}
		if isDangerousCommand(alias.Command) {
			alias.Issues = append(alias.Issues, "review before running")
		}
		if !svc.platformSupported(alias.Platforms) {
			alias.Issues = append(alias.Issues, "not for "+svc.currentPlatform())
		}
		fields := strings.Fields(alias.Command)
		if len(fields) > 0 && shouldCheckExecutable(fields[0]) {
			if _, err := exec.LookPath(fields[0]); err != nil {
				alias.Issues = append(alias.Issues, "missing executable: "+fields[0])
			}
		}
	}
}

func isDangerousCommand(command string) bool {
	return len(dangerousCommandReasons(command)) > 0
}

func dangerousCommandReasons(command string) []string {
	lower := strings.ToLower(command)
	patterns := []struct {
		needle string
		reason string
	}{
		{"sudo ", "runs with elevated privileges"},
		{"--force", "uses a force option"},
		{"reset --hard", "discards uncommitted Git changes"},
		{"clean -fd", "deletes untracked Git files"},
		{"rm -rf", "recursively deletes files"},
		{"chmod -r", "recursively changes file permissions"},
		{"chown -r", "recursively changes file ownership"},
		{"docker system prune", "deletes unused Docker data"},
	}
	var reasons []string
	for _, pattern := range patterns {
		if strings.Contains(lower, pattern.needle) {
			reasons = append(reasons, pattern.reason)
		}
	}
	if strings.Contains(command, "branch -D") {
		reasons = append(reasons, "force-deletes a Git branch")
	}
	if strings.TrimSpace(lower) == "sudo" {
		reasons = append(reasons, "runs with elevated privileges")
	}
	return reasons
}

func shouldCheckExecutable(command string) bool {
	builtins := map[string]bool{
		".": true, "alias": true, "cd": true, "command": true, "echo": true, "export": true,
		"for": true, "if": true, "local": true, "printf": true, "pwd": true, "read": true,
		"return": true, "source": true, "test": true, "type": true,
	}
	return !builtins[command] && !strings.ContainsAny(command, "$()`")
}

func parseAliasDefinition(line string) (string, string, bool) {
	return parseLegacyAliasDefinition(line)
}

func parseLegacyAliasDefinition(line string) (string, string, bool) {
	return shell.ParseLegacyAliasDefinition(line)
}

func isSectionHeading(note string) bool { return entry.IsSectionHeading(note) }

func firstNonEmpty(values ...string) string { return entry.FirstNonEmpty(values...) }

func (svc *Services) category(command string) string { return entry.Category(command) }

func describe(name, command string) string { return entry.Describe(name, command) }

func legacyDescription(name, command string) string { return entry.LegacyDescription(name, command) }
