package main

import "alias-lens/internal/presentation"

import (
	"alias-lens/internal/app"

	"fmt"
)

func runContextCommand(arguments []string) error {
	if len(arguments) == 0 {
		return fmt.Errorf("usage: al context list|add NAME [--repo|--directory]|remove NAME [--all|--repo|--directory]")
	}
	switch arguments[0] {
	case "list":
		if len(arguments) != 1 {
			return fmt.Errorf("usage: al context list")
		}
		bindings, err := applicationServices().ContextAssociations()
		if err != nil {
			return err
		}
		if len(bindings) == 0 {
			fmt.Println("No aliases are marked for a project or folder. In one, run: al context add NAME")
			return nil
		}
		for _, binding := range bindings {
			fmt.Printf("%s\t%s\t%s\t%s\n", presentation.TerminalSafeText(binding.Shell), presentation.TerminalSafeText(binding.Name), presentation.TerminalSafeText(binding.Kind), presentation.TerminalSafeText(binding.Path))
		}
		return nil
	case "add", "remove":
		if len(arguments) < 2 || len(arguments) > 3 || applicationServices().ValidateContextName(arguments[0], arguments[1]) != nil {
			return fmt.Errorf("usage: al context %s NAME [--repo|--directory%s]", arguments[0], map[bool]string{true: "|--all", false: ""}[arguments[0] == "remove"])
		}
		option := "auto"
		if len(arguments) == 3 {
			switch arguments[2] {
			case "--repo":
				option = app.ContextRepository
			case "--directory":
				option = app.ContextDirectory
			case "--all":
				if arguments[0] != "remove" {
					return fmt.Errorf("--all is only valid with al context remove")
				}
				option = "all"
			default:
				return fmt.Errorf("choose --repo, --directory, or --all")
			}
		}
		name := arguments[1]
		if arguments[0] == "remove" {
			result, err := applicationServices().RemoveContext(name, option)
			if err != nil {
				return err
			}
			fmt.Printf("Removed %d local context association(s) for %s.\n", result.Removed, name)
			return nil
		}
		result, err := applicationServices().AddContext(name, option)
		if err != nil {
			return err
		}
		fmt.Printf("Marked %s for this %s. It remains available everywhere.\n", name, map[string]string{app.ContextRepository: "project", app.ContextDirectory: "folder"}[result.Kind])
		return nil
	default:
		return fmt.Errorf("usage: al context list|add NAME [--repo|--directory]|remove NAME [--all|--repo|--directory]")
	}
}
