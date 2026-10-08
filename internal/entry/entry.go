package entry

import (
	"path/filepath"
	"sort"
	"strings"
)

type Alias struct {
	CatalogID    string   `json:"catalog_id,omitempty"`
	CatalogState string   `json:"catalog_state,omitempty"`
	Name         string   `json:"name"`
	Command      string   `json:"command"`
	Description  string   `json:"description"`
	Category     string   `json:"category"`
	Issues       []string `json:"issues,omitempty"`
	Usage        int      `json:"usage,omitempty"`
	Type         string   `json:"type,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	Platforms    []string `json:"platforms,omitempty"`
	Favorite     bool     `json:"favorite,omitempty"`
}

type EntryMetadata struct {
	Tags      []string
	Platforms []string
	Favorite  bool
	Category  string
}

func IsSectionHeading(note string) bool {
	return len(note) <= 30 && note == strings.ToUpper(note)
}

func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func Category(command string) string {
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

func Describe(name, command string) string {
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

func LegacyDescription(name, command string) string {
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

func ParseMetadataComment(line string) (EntryMetadata, bool) {
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
		values := SplitMetadataValues(value)
		switch strings.ToLower(key) {
		case "tags", "collections":
			metadata.Tags = values
		case "platforms":
			metadata.Platforms = values
		case "favorite":
			metadata.Favorite = strings.EqualFold(value, "true") || value == "1" || strings.EqualFold(value, "yes")
		case "category":
			metadata.Category = NormalizeCategory(value)
		}
	}
	return metadata, true
}

func SplitMetadataValues(value string) []string {
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

func ApplyMetadata(alias *Alias, metadata EntryMetadata) {
	alias.Tags = append([]string(nil), metadata.Tags...)
	alias.Platforms = append([]string(nil), metadata.Platforms...)
	alias.Favorite = metadata.Favorite
	if metadata.Category != "" {
		alias.Category = metadata.Category
	}
}

func NormalizeCategory(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func PlatformName(goos string, getenv func(string) string) string {
	if goos == "linux" && (getenv("WSL_DISTRO_NAME") != "" || getenv("WSL_INTEROP") != "") {
		return "wsl"
	}
	if goos == "darwin" {
		return "macos"
	}
	return goos
}

func PlatformSupported(platforms []string, current string) bool {
	if len(platforms) == 0 {
		return true
	}
	for _, platform := range platforms {
		if platform == "all" || platform == current || (current == "wsl" && platform == "linux") {
			return true
		}
	}
	return false
}

func MetadataLine(metadata EntryMetadata) string {
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
	if metadata.Category != "" {
		fields = append(fields, "category="+metadata.Category)
	}
	if len(fields) == 0 {
		return ""
	}
	return "# al: " + strings.Join(fields, " ")
}

func MetadataForAlias(alias Alias) EntryMetadata {
	metadata := EntryMetadata{Tags: alias.Tags, Platforms: alias.Platforms, Favorite: alias.Favorite}
	if alias.Category != Category(alias.Command) {
		metadata.Category = alias.Category
	}
	return metadata
}
