package main

import (
	"github.com/alderon07/al/internal/app"

	"fmt"
)

func parseCatalogInitOptions(arguments []string) (app.CatalogInitOptions, error) {
	o := app.CatalogInitOptions{CatalogPath: "alias-lens/catalog.json"}
	for i := 0; i < len(arguments); i++ {
		switch arguments[i] {
		case "--shell", "--catalog-path", "--startup-path":
			if i+1 >= len(arguments) {
				return o, fmt.Errorf("init option needs a value")
			}
			option := arguments[i]
			i++
			if option == "--startup-path" {
				o.StartupPaths = append(o.StartupPaths, arguments[i])
			} else if option == "--shell" {
				o.Shell = arguments[i]
			} else {
				o.CatalogPath = arguments[i]
			}
		case "--apply":
			o.Apply = true
		default:
			if o.Source != "" || len(arguments[i]) == 0 || arguments[i][0] == '-' {
				return o, fmt.Errorf("usage: al init SOURCE [--shell bash|zsh] [--catalog-path PATH] [--apply]")
			}
			o.Source = arguments[i]
		}
	}
	if o.Source == "" {
		return o, fmt.Errorf("usage: al init SOURCE [--shell bash|zsh] [--catalog-path PATH] [--apply]")
	}
	if o.Shell != "" && o.Shell != "bash" && o.Shell != "zsh" {
		return o, fmt.Errorf("--shell needs bash or zsh")
	}
	return o, nil
}

var catalogInitImplementation = func(app.CatalogInitOptions) error {
	return fmt.Errorf("catalog init supports Bash and Zsh on Linux, WSL, and macOS")
}

func runInitCommand(arguments []string) error {
	o, err := parseCatalogInitOptions(arguments)
	if err != nil {
		return err
	}
	return catalogInitImplementation(o)
}

type commandSpec struct {
	Name    string
	Summary string
	Usage   string
}

var publicCommandSpecs = newPublicCommandSpecs()

func newPublicCommandSpecs() []commandSpec {
	rows := [][2]string{
		{"pick", "Select an alias without using it"},
		{"use", "Select and use an alias through the shell integration"},
		{"search", "Find aliases by name or metadata"},
		{"context", "Mark aliases for the current project or folder"},
		{"stats", "Show alias usage"},
		{"export", "Export aliases or usage data"},
		{"import", "Preview or import aliases from a file"},
		{"suggest", "Find repeated commands in shell history"},
		{"meta", "Edit alias metadata"},
		{"describe", "Add missing alias descriptions"},
		{"check", "Check alias syntax without using it"},
		{"scan", "Find likely secrets without showing their values"},
		{"history", "List private alias revisions"},
		{"undo", "Restore a private alias revision"},
		{"doctor", "Diagnose the Alias Lens installation"},
		{"setup", "Install, repair, or remove shell integration"},
		{"data", "Show or clear private local data"},
		{"status", "Show what is ready without changing files"},
		{"plan", "Preview a change without applying it"},
		{"catalog", "Review and install the shell-neutral catalog"},
		{"init", "Enroll a catalog repository and install reviewed entries"},
		{"repo", "Configure a Git repository"},
		{"config", "Show or change Alias Lens settings"},
		{"track", "Add a file to automatic sync"},
		{"untrack", "Remove a file from automatic sync"},
		{"sync", "Synchronize aliases with the configured repository"},
		{"diff", "Compare local and repository aliases"},
		{"autosync", "Configure background synchronization"},
		{"watch", "Check once for changes that need to sync"},
		{"theme", "Show or select a terminal theme"},
		{"shortcuts", "Show or configure keyboard shortcuts"},
		{"completion", "Print Bash or Zsh completion code"},
		{"shell-init", "Print shell integration code"},
		{"--web", "Start the optional local browser"},
		{"--version", "Print the installed version"},
	}
	result := make([]commandSpec, 0, len(rows))
	for _, row := range rows {
		result = append(result, commandSpec{Name: row[0], Summary: row[1], Usage: commandUsage[row[0]]})
	}
	return result
}

func lookupCommandSpec(name string) (commandSpec, bool) {
	if name == "version" || name == "-v" {
		name = "--version"
	}
	for _, spec := range publicCommandSpecs {
		if spec.Name == name {
			return spec, true
		}
	}
	return commandSpec{}, false
}
