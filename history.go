package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type HistorySuggestion struct {
	Command string
	Count   int
	Name    string
}

func loadHistoryCounts() map[string]int {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	counts, _ := historyCountsFrom(filepath.Join(home, ".bash_history"))
	return counts
}

func historyCountsFrom(path string) (map[string]int, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]int{}, nil
		}
		return nil, err
	}
	defer file.Close()
	counts := make(map[string]int)
	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 1024*1024)
	for scanner.Scan() {
		command := normalizeHistoryCommand(scanner.Text())
		if command != "" {
			counts[command]++
		}
	}
	return counts, scanner.Err()
}

func normalizeHistoryCommand(command string) string {
	command = strings.TrimSpace(command)
	if command == "" || (strings.HasPrefix(command, "#") && strings.Trim(command[1:], "0123456789") == "") {
		return ""
	}
	return strings.Join(strings.Fields(command), " ")
}

func historySuggestions(aliases []Alias, counts map[string]int) []HistorySuggestion {
	existingCommands := make(map[string]bool, len(aliases))
	existingNames := make(map[string]bool, len(aliases))
	for _, alias := range aliases {
		existingCommands[normalizeHistoryCommand(alias.Command)] = true
		existingNames[alias.Name] = true
	}
	var suggestions []HistorySuggestion
	for command, count := range counts {
		if count < 3 || len(command) < 12 || existingCommands[command] || unsuitableHistoryCommand(command) {
			continue
		}
		name := suggestAliasName(command, existingNames)
		suggestions = append(suggestions, HistorySuggestion{Command: command, Count: count, Name: name})
	}
	sort.SliceStable(suggestions, func(i, j int) bool {
		if suggestions[i].Count == suggestions[j].Count {
			return len(suggestions[i].Command) > len(suggestions[j].Command)
		}
		return suggestions[i].Count > suggestions[j].Count
	})
	if len(suggestions) > 20 {
		suggestions = suggestions[:20]
	}
	return suggestions
}

func unsuitableHistoryCommand(command string) bool {
	lower := strings.ToLower(command)
	prefixes := []string{"al ", "alias ", "history", "export ", "read ", "sudo ", "ssh ", "scp ", "curl ", "wget "}
	for _, prefix := range prefixes {
		if lower == strings.TrimSpace(prefix) || strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return len(findSecretFindings([]byte(command))) > 0
}

func suggestAliasName(command string, existing map[string]bool) string {
	var initials []rune
	for _, field := range strings.Fields(command) {
		field = strings.TrimLeftFunc(field, func(character rune) bool { return !unicode.IsLetter(character) && !unicode.IsDigit(character) })
		if field != "" {
			initials = append(initials, []rune(strings.ToLower(field))[0])
		}
		if len(initials) == 4 {
			break
		}
	}
	if len(initials) < 2 {
		initials = []rune("cmd")
	}
	base := string(initials)
	name := base
	for suffix := 2; existing[name]; suffix++ {
		name = fmt.Sprintf("%s%d", base, suffix)
	}
	return name
}

func annotateUsage(aliases []Alias, counts map[string]int) {
	for index := range aliases {
		aliases[index].Usage = counts[normalizeHistoryCommand(aliases[index].Command)]
	}
}

func runHistorySuggestions(arguments []string) error {
	aliases, err := loadAliases()
	if err != nil {
		return err
	}
	suggestions := historySuggestions(aliases, loadHistoryCounts())
	if len(arguments) > 0 && arguments[0] == "add" {
		if len(arguments) < 2 || len(arguments) > 3 {
			return fmt.Errorf("usage: al suggest add NUMBER [NAME]")
		}
		var index int
		if _, err := fmt.Sscanf(arguments[1], "%d", &index); err != nil || index < 1 || index > len(suggestions) {
			return fmt.Errorf("suggestion number must be between 1 and %d", len(suggestions))
		}
		suggestion := suggestions[index-1]
		if len(arguments) == 3 {
			suggestion.Name = arguments[2]
		}
		return addAlias(suggestion.Name, suggestion.Command, fmt.Sprintf("Used %d times in local Bash history", suggestion.Count))
	}
	if len(arguments) != 0 {
		return fmt.Errorf("usage: al suggest [add NUMBER [NAME]]")
	}
	if len(suggestions) == 0 {
		fmt.Println("No repeated long commands need aliases yet.")
		return nil
	}
	fmt.Println("Repeated commands that do not have aliases:")
	for index, suggestion := range suggestions {
		fmt.Printf("%2d  %-6s  %3d uses  %s\n", index+1, suggestion.Name, suggestion.Count, suggestion.Command)
	}
	fmt.Println("Add one with: al suggest add NUMBER [NAME]")
	return nil
}
