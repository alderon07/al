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

var version = "dev"

//go:embed web/*
var web embed.FS

type Alias struct {
	Name        string   `json:"name"`
	Command     string   `json:"command"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Issues      []string `json:"issues,omitempty"`
	Usage       int      `json:"usage,omitempty"`
	Type        string   `json:"type,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Platforms   []string `json:"platforms,omitempty"`
	Favorite    bool     `json:"favorite,omitempty"`
}

func main() {
	if len(os.Args) == 1 {
		runTUI()
		return
	}
	if os.Args[1] == "help" || isHelpFlag(os.Args[1]) {
		if len(os.Args) == 2 {
			printUsage()
		} else if len(os.Args) == 3 {
			printCommandUsage(os.Args[2])
		} else {
			fmt.Fprintln(os.Stderr, "Usage: al help [COMMAND]")
		}
		return
	}
	if len(os.Args) == 3 && isHelpFlag(os.Args[2]) {
		printCommandUsage(os.Args[1])
		return
	}
	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Printf("alias-lens %s\n", version)
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
		config, _ := loadConfig()
		fmt.Printf("Alias Lens will sync only %s in %s\n", config.AliasFile, os.Args[2])
	case "config":
		if err := runConfigCommand(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	case "pick":
		commandOnly := false
		arguments := os.Args[2:]
		if len(arguments) > 0 && arguments[0] == "--command" {
			commandOnly = true
			arguments = arguments[1:]
		}
		if len(arguments) > 1 {
			fmt.Fprintln(os.Stderr, "Usage: al pick [--command] [QUERY]")
			return
		}
		query := ""
		if len(arguments) == 1 {
			query = arguments[0]
		}
		if err := runAliasPicker(query, commandOnly); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	case "shell-init":
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "Usage: al shell-init bash|zsh")
			return
		}
		if err := printShellIntegration(os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	case "suggest":
		if err := runHistorySuggestions(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	case "scan":
		if len(os.Args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: al scan")
			return
		}
		if err := runSecretScan(); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	case "meta":
		if err := runMetadataCommand(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	case "describe":
		if len(os.Args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: al describe")
			return
		}
		if err := addAliasDescriptions(); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	case "history":
		if len(os.Args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: al history")
			return
		}
		if err := runRevisionHistory(); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	case "undo":
		if len(os.Args) > 3 {
			fmt.Fprintln(os.Stderr, "Usage: al undo [REVISION]")
			return
		}
		revision := "latest"
		if len(os.Args) == 3 {
			revision = os.Args[2]
		}
		if err := restoreRevision(revision); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	case "doctor":
		if len(os.Args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: al doctor")
			return
		}
		if err := runDoctor(); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	case "setup":
		if len(os.Args) > 3 {
			fmt.Fprintln(os.Stderr, "Usage: al setup [bash|zsh]")
			return
		}
		shellName := ""
		if len(os.Args) == 3 {
			shellName = os.Args[2]
		}
		if err := runSetup(shellName); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	case "autosync":
		if err := runAutoSyncCommand(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	case "watch":
		if len(os.Args) == 3 && os.Args[2] == "--ensure" {
			config, err := loadConfig()
			if err == nil && config.AutoSync.Enabled {
				err = ensureWatchProcess()
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, "Alias Lens:", err)
			}
			return
		}
		daemon := len(os.Args) == 3 && os.Args[2] == "--daemon"
		if len(os.Args) > 3 || (len(os.Args) == 3 && !daemon) {
			fmt.Fprintln(os.Stderr, "Usage: al watch")
			return
		}
		if err := runWatch(daemon); err != nil && !daemon {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	case "track", "untrack":
		if err := runTrackCommand(os.Args[2:], os.Args[1] == "untrack"); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	case "sync":
		if len(os.Args) > 3 || (len(os.Args) == 3 && os.Args[2] != "--push" && os.Args[2] != "--pull") {
			printUsage()
			return
		}
		if len(os.Args) == 3 && os.Args[2] == "--pull" {
			message, err := pullRepository()
			if err != nil {
				fmt.Fprintln(os.Stderr, "Alias Lens:", err)
				return
			}
			fmt.Println(message)
			return
		}
		push := len(os.Args) == 3 && os.Args[2] == "--push"
		message, err := syncRepository(push)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
			return
		}
		fmt.Println(message)
	case "diff":
		if len(os.Args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: al diff")
			return
		}
		if err := showRepositoryDiff(); err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens:", err)
		}
	default:
		printUsage()
	}
}

func printShellIntegration(name string) error {
	adapter, err := shellAdapter(name)
	if err != nil {
		return err
	}
	_, err = os.Stdout.WriteString(adapter.Integration())
	return err
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
	fmt.Println("Reading aliases from", aliasDisplayPath())
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
	path, err := aliasesPath()
	if err != nil {
		return nil, fmt.Errorf("find alias file: %w", err)
	}
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if createErr := ensureAliasFileExists(path); createErr != nil {
			return nil, createErr
		}
		contents, err = os.ReadFile(path)
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

		name, command, ok := parseAliasDefinition(line)
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
			Category:    category(command),
			Type:        "alias",
		}
		applyMetadata(&alias, metadata)
		aliases = append(aliases, alias)
		notes = nil
		metadata = EntryMetadata{}
	}
	aliases = append(aliases, parseFunctions(string(contents))...)

	sort.Slice(aliases, func(i, j int) bool { return strings.ToLower(aliases[i].Name) < strings.ToLower(aliases[j].Name) })
	annotateUsage(aliases, loadHistoryCounts())
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
		if !platformSupported(alias.Platforms) {
			alias.Issues = append(alias.Issues, "not for "+currentPlatform())
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
		"for": true, "if": true, "local": true, "printf": true, "pwd": true, "read": true,
		"return": true, "source": true, "test": true, "type": true,
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
	if !found || name == "" || value == "" || !aliasName.MatchString(name) {
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
	if len(fields) == 0 {
		return "Custom command for " + name
	}
	tool := filepath.Base(fields[0])
	subcommand := ""
	if len(fields) > 1 {
		subcommand = fields[1]
	}
	if tool == "git" {
		if description, ok := map[string]string{
			"status":   "Show changed files and the current branch",
			"add":      "Stage files for the next commit",
			"commit":   "Create a commit from staged changes",
			"diff":     "Show changes that are not committed",
			"log":      "Show commit history",
			"push":     "Push commits to the remote repository",
			"pull":     "Fetch and merge remote changes",
			"fetch":    "Fetch remote branches and commits",
			"branch":   "List or manage Git branches",
			"checkout": "Switch branches or restore files",
			"switch":   "Switch Git branches",
			"restore":  "Restore working tree files",
			"stash":    "Temporarily store uncommitted changes",
			"merge":    "Merge another Git branch",
			"rebase":   "Reapply commits on another base",
			"clone":    "Clone a Git repository",
			"remote":   "Manage Git remotes",
			"worktree": "Manage linked Git worktrees",
		}[subcommand]; ok {
			return description
		}
		return "Run a Git command"
	}
	if tool == "docker" {
		if description, ok := map[string]string{
			"ps":      "List running Docker containers",
			"images":  "List Docker images",
			"build":   "Build a Docker image",
			"run":     "Start a new Docker container",
			"exec":    "Run a command in a Docker container",
			"logs":    "Show logs from a Docker container",
			"compose": "Manage a Docker Compose application",
		}[subcommand]; ok {
			return description
		}
		return "Run a Docker command"
	}
	if tool == "go" {
		if description, ok := map[string]string{
			"test":  "Run Go tests",
			"build": "Build Go packages",
			"run":   "Compile and run a Go program",
			"fmt":   "Format Go source files",
			"vet":   "Check Go code for suspicious constructs",
			"mod":   "Manage Go module dependencies",
		}[subcommand]; ok {
			return description
		}
		return "Run a Go command"
	}
	if tool == "npm" || tool == "pnpm" || tool == "yarn" || tool == "bun" {
		if description, ok := map[string]string{
			"install": "Install project dependencies",
			"add":     "Add a project dependency",
			"remove":  "Remove a project dependency",
			"test":    "Run the project test script",
			"build":   "Build the project",
			"dev":     "Start the development server",
			"run":     "Run a project script",
		}[subcommand]; ok {
			return description
		}
		return "Run a " + tool + " command"
	}
	if description, ok := map[string]string{
		"ls":         "List files and directories",
		"eza":        "List files and directories",
		"cd":         "Change the current directory",
		"pwd":        "Print the current directory",
		"mkdir":      "Create a directory",
		"cp":         "Copy files or directories",
		"mv":         "Move or rename files",
		"rm":         "Remove files or directories",
		"cat":        "Print file contents",
		"less":       "Read a file one screen at a time",
		"head":       "Show the beginning of a file",
		"tail":       "Show the end of a file",
		"rg":         "Search file contents",
		"grep":       "Search file contents",
		"find":       "Find files and directories",
		"clear":      "Clear the terminal",
		"ssh":        "Connect to a remote machine over SSH",
		"scp":        "Copy files over SSH",
		"curl":       "Transfer data using a URL",
		"wget":       "Download files from a URL",
		"code":       "Open a path in Visual Studio Code",
		"source":     "Load commands into the current shell",
		"kubectl":    "Manage Kubernetes resources",
		"terraform":  "Manage infrastructure with Terraform",
		"systemctl":  "Manage system services",
		"journalctl": "Read system service logs",
		"python":     "Run Python",
		"python3":    "Run Python",
	}[tool]; ok {
		return description
	}
	if tool == "sudo" {
		return "Run a command with administrator privileges"
	}
	return "Run " + tool
}

func legacyDescription(name, command string) string {
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
