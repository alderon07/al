package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

//go:embed web/*
var web embed.FS

type Alias struct {
	Name        string   `json:"name"`
	Command     string   `json:"command"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Issues      []string `json:"issues,omitempty"`
}

func main() {
	if len(os.Args) == 1 {
		runTUI()
		return
	}
	switch os.Args[1] {
	case "--web":
		runWeb()
	case "repo":
		if len(os.Args) == 2 {
			if err := runRepoPicker(""); err != nil {
				fmt.Fprintln(os.Stderr, "Alias Lens:", err)
			}
			return
		}
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "Usage: al repo [/path/to/dotfiles]")
			return
		}
		if os.Args[2] == "github" || os.Args[2] == "bitbucket" || os.Args[2] == "gitlab" {
			if err := runRepoPicker(os.Args[2]); err != nil {
				fmt.Fprintln(os.Stderr, "Alias Lens:", err)
			}
			return
		}
		if err := configureRepository(os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
			return
		}
		fmt.Println("Alias Lens will sync only .bash_aliases in", os.Args[2])
	case "config":
		if err := runConfigCommand(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	case "sync":
		if len(os.Args) > 3 || (len(os.Args) == 3 && os.Args[2] != "--push") {
			printUsage()
			return
		}
		push := len(os.Args) == 3 && os.Args[2] == "--push"
		message, err := syncRepository(push)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
			return
		}
		fmt.Println(message)
	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage: al [--web | repo [PROVIDER|PATH] | config | sync [--push]]")
	fmt.Println("  al               Open Alias Lens in the terminal")
	fmt.Println("  al --web         Start the optional browser interface")
	fmt.Println("  al repo          Pick from writable GitHub, Bitbucket, and GitLab repositories")
	fmt.Println("  al repo PROVIDER Limit the picker to one configured provider")
	fmt.Println("  al repo PATH     Choose an existing local Git repository")
	fmt.Println("  al config        Show safe provider configuration and setup commands")
	fmt.Println("  al sync          Commit only .bash_aliases to that repository")
	fmt.Println("  al sync --push   Commit and explicitly push it")
}

func runWeb() {
	if len(os.Args) > 2 {
		printUsage()
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/aliases", aliasesHandler)

	static, err := fs.Sub(web, "web")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServer(http.FS(static)))

	address := "127.0.0.1:8787"
	fmt.Printf("Alias Lens web mode is running at http://%s\n", address)
	fmt.Println("Reading aliases from ~/.bash_aliases")
	if err := http.ListenAndServe(address, mux); err != nil {
		panic(err)
	}
}

func aliasesHandler(w http.ResponseWriter, r *http.Request) {
	aliases, err := loadAliases()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(aliases)
}

func loadAliases() ([]Alias, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("find home directory: %w", err)
	}
	path := filepath.Join(home, ".bash_aliases")
	contents, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Alias{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var aliases []Alias
	var notes []string
	for _, line := range strings.Split(string(contents), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
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

		name, command, ok := parseAliasDefinition(line)
		if !ok {
			notes = nil
			continue
		}
		description := describe(name, command)
		if len(notes) > 0 {
			description = notes[len(notes)-1]
		}
		aliases = append(aliases, Alias{
			Name:        name,
			Command:     command,
			Description: description,
			Category:    category(command),
		})
		notes = nil
	}

	sort.Slice(aliases, func(i, j int) bool { return strings.ToLower(aliases[i].Name) < strings.ToLower(aliases[j].Name) })
	annotateHealth(aliases)
	return aliases, nil
}

func annotateHealth(aliases []Alias) {
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
		fields := strings.Fields(alias.Command)
		if len(fields) > 0 && shouldCheckExecutable(fields[0]) {
			if _, err := exec.LookPath(fields[0]); err != nil {
				alias.Issues = append(alias.Issues, "missing executable: "+fields[0])
			}
		}
	}
}

func isDangerousCommand(command string) bool {
	lower := strings.ToLower(command)
	patterns := []string{"--force", "reset --hard", "clean -fd", "rm -rf", "chmod -r", "chown -r"}
	for _, pattern := range patterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return strings.Contains(command, "branch -D")
}

func shouldCheckExecutable(command string) bool {
	builtins := map[string]bool{
		".": true, "alias": true, "cd": true, "command": true, "echo": true, "export": true,
		"printf": true, "pwd": true, "read": true, "source": true, "test": true, "type": true,
	}
	return !builtins[command] && !strings.ContainsAny(command, "$()`")
}

func parseAliasDefinition(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "alias ") {
		return "", "", false
	}
	definition := strings.TrimSpace(strings.TrimPrefix(trimmed, "alias "))
	name, value, found := strings.Cut(definition, "=")
	name = strings.TrimSpace(name)
	value = strings.TrimSpace(value)
	if !found || name == "" || value == "" {
		return "", "", false
	}
	if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
		value = strings.TrimSuffix(strings.TrimPrefix(value, "'"), "'")
		value = strings.ReplaceAll(value, "'\\''", "'")
	} else if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
		if decoded, err := strconv.Unquote(value); err == nil {
			value = decoded
		} else {
			value = strings.TrimSuffix(strings.TrimPrefix(value, "\""), "\"")
		}
	}
	return name, value, true
}

func isSectionHeading(note string) bool {
	return len(note) <= 30 && note == strings.ToUpper(note)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func category(command string) string {
	first := strings.Fields(command)
	if len(first) == 0 {
		return "custom"
	}
	switch first[0] {
	case "git":
		return "git"
	case "docker":
		return "docker"
	case "eza", "ls":
		return "files"
	case "npm", "pnpm", "bun", "go":
		return "dev"
	default:
		return "custom"
	}
}

func describe(name, command string) string {
	fields := strings.Fields(command)
	if len(fields) >= 2 && fields[0] == "git" {
		return "Runs git " + strings.Join(fields[1:], " ")
	}
	if len(fields) >= 2 && fields[0] == "docker" {
		return "Runs docker " + strings.Join(fields[1:], " ")
	}
	if len(fields) > 0 {
		return "Runs " + fields[0]
	}
	return "Custom command for " + name
}
