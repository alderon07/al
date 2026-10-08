package main

import (
	"encoding/json"

	"fmt"

	"slices"
	"strings"

	workflowplan "alias-lens/internal/plan"
)

func runTrackCommand(arguments []string, remove bool) error {
	if len(arguments) < 1 || len(arguments) > 2 {
		return fmt.Errorf("usage: al %s SOURCE [REPOSITORY_PATH]", map[bool]string{true: "untrack", false: "track"}[remove])
	}
	if remove {
		return applicationServices().Settings.Untrack(arguments[0])
	}
	var repositoryPath *string
	if len(arguments) == 2 {
		repositoryPath = &arguments[1]
	}
	return applicationServices().Settings.Track(arguments[0], repositoryPath)
}

func runConfigCommand(arguments []string) error {
	if len(arguments) > 0 && arguments[0] == "profile" {
		return runConfigProfileCommand(arguments[1:])
	}
	config, err := loadConfig()
	if err != nil {
		return err
	}
	if len(arguments) == 0 {
		contents, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(contents))
		fmt.Println("Credentials: GitHub CLI · BITBUCKET_API_TOKEN · GITLAB_TOKEN (never stored in config)")
		return nil
	}

	switch arguments[0] {
	case "shell":
		if len(arguments) != 2 {
			return fmt.Errorf("usage: al config shell bash|zsh")
		}
		adapter, err := applicationServices().ShellAdapter(arguments[1])
		if err != nil {
			return err
		}
		oldAdapter, _ := applicationServices().ShellAdapter(config.Shell)
		if oldAdapter != nil && config.AliasFile == oldAdapter.AliasFilename() {
			config.AliasFile = adapter.AliasFilename()
		}
		config.Shell = adapter.Name()
	case "provider":
		if len(arguments) < 2 || len(arguments) > 3 {
			return fmt.Errorf("usage: al config provider github [HOST] | bitbucket WORKSPACE | gitlab [HOST]")
		}
		name := strings.ToLower(arguments[1])
		settings := config.Providers[name]
		settings.Enabled = true
		if settings.Protocol == "" {
			settings.Protocol = "auto"
		}
		switch name {
		case "github":
			settings.Host = "github.com"
			if len(arguments) == 3 {
				settings.Host = arguments[2]
			}
		case "bitbucket":
			if len(arguments) != 3 {
				return fmt.Errorf("usage: al config provider bitbucket WORKSPACE")
			}
			settings.Host = "bitbucket.org"
			if !slices.Contains(settings.Workspaces, arguments[2]) {
				settings.Workspaces = append(settings.Workspaces, arguments[2])
			}
		case "gitlab":
			settings.Host = "gitlab.com"
			if len(arguments) == 3 {
				settings.Host = arguments[2]
			}
		default:
			return fmt.Errorf("unsupported provider %q (use github, bitbucket, or gitlab)", name)
		}
		config.Providers[name] = settings
	case "protocol":
		if len(arguments) != 3 {
			return fmt.Errorf("usage: al config protocol PROVIDER auto|ssh|https")
		}
		name, protocol := strings.ToLower(arguments[1]), strings.ToLower(arguments[2])
		settings, exists := config.Providers[name]
		if !exists {
			return fmt.Errorf("configure %s first with: al config provider %s", name, name)
		}
		if protocol != "auto" && protocol != "ssh" && protocol != "https" {
			return fmt.Errorf("protocol must be auto, ssh, or https")
		}
		settings.Protocol = protocol
		config.Providers[name] = settings
	case "disable":
		if len(arguments) != 2 {
			return fmt.Errorf("usage: al config disable PROVIDER")
		}
		name := strings.ToLower(arguments[1])
		settings, exists := config.Providers[name]
		if !exists {
			return fmt.Errorf("provider %s is not configured", name)
		}
		settings.Enabled = false
		config.Providers[name] = settings
	case "footer-message":
		if len(arguments) != 2 {
			return fmt.Errorf("usage: al config footer-message MESSAGE")
		}
		config.Footer.Message = arguments[1]
	case "footer-icon":
		if len(arguments) != 2 {
			return fmt.Errorf("usage: al config footer-icon ICON")
		}
		config.Footer.Icon = arguments[1]
	case "footer-reset":
		if len(arguments) != 1 {
			return fmt.Errorf("usage: al config footer-reset")
		}
		config.Footer = defaultFooterConfig()
	default:
		return fmt.Errorf("usage: al config [shell|provider|protocol|disable|profile|footer-message|footer-icon|footer-reset]")
	}
	if err := saveConfig(config); err != nil {
		return err
	}
	cliResult("Alias Lens configuration updated")
	return nil
}

func runConfigProfileCommand(arguments []string) error {
	if len(arguments) == 1 && arguments[0] == "list" {
		observed, err := observeConfig()
		if err != nil {
			return err
		}
		if len(observed.Config.Profiles) == 0 {
			fmt.Println("No machine profiles are active.")
			fmt.Println("Add one with: al config profile add NAME")
			return nil
		}
		fmt.Println("Active machine profiles:")
		for _, profile := range observed.Config.Profiles {
			fmt.Println("-", profile)
		}
		return nil
	}
	if len(arguments) != 2 || (arguments[0] != "add" && arguments[0] != "remove") {
		return fmt.Errorf("usage: al config profile list|add NAME|remove NAME")
	}
	preview, err := applicationServices().BuildProfilePlan(arguments[0], arguments[1])
	if err != nil {
		return err
	}
	fmt.Print(cliPlanText(workflowplan.RenderPlain(preview)))
	if len(preview.Actions) == 0 {
		fmt.Println("Nothing needed to change.")
		return nil
	}
	action, name := arguments[0], arguments[1]
	if err := applicationServices().ApplyProfile(preview, action, name); err != nil {
		return err
	}
	if action == "add" {
		fmt.Printf("Machine profile %q is now active.\n", name)
	} else {
		fmt.Printf("Machine profile %q is no longer active.\n", name)
	}
	fmt.Printf("Your current aliases were left unchanged. Enter al catalog preview --from %s to review the new selection.\n", activeShellNameForConfig())
	return nil
}

func activeShellNameForConfig() string {
	observed, err := observeConfig()
	if err == nil && (observed.Config.Shell == "bash" || observed.Config.Shell == "zsh") {
		return observed.Config.Shell
	}
	return "bash"
}
