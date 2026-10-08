package main

import (
	"github.com/alderon07/al/internal/app"

	"fmt"
	"os"
)

func runDoctor() error {
	checks := applicationServices().DoctorChecks()
	failures := 0
	if cliStyled() {
		fmt.Println(cliReportHeading("doctor"))
	}
	for _, check := range checks {
		if !check.OK {
			failures++
		}
		cliDoctorRow(check.OK, check.Name, check.Message)
	}
	if failures > 0 {
		return fmt.Errorf("%d checks need attention", failures)
	}
	if cliStyled() {
		fmt.Println()
	}
	cliResult("Alias Lens is ready.")
	return nil
}

func runSetupCommand(arguments []string) error {
	action := app.SetupInstall
	shellName := ""
	for _, argument := range arguments {
		switch argument {
		case "--repair":
			if action != app.SetupInstall {
				return fmt.Errorf("usage: al setup [--repair|--remove] [bash|zsh]")
			}
			action = app.SetupRepair
		case "--remove":
			if action != app.SetupInstall {
				return fmt.Errorf("usage: al setup [--repair|--remove] [bash|zsh]")
			}
			action = app.SetupRemove
		case "bash", "zsh":
			if shellName != "" {
				return fmt.Errorf("usage: al setup [--repair|--remove] [bash|zsh]")
			}
			shellName = argument
		default:
			return fmt.Errorf("usage: al setup [--repair|--remove] [bash|zsh]")
		}
	}
	if action == app.SetupRemove {
		return removeSetup(shellName)
	}
	return runSetup(shellName, action == app.SetupRepair)
}

func runSetup(shellName string, repair bool) error {
	result, err := applicationServices().Setup(app.SetupRequest{Shell: shellName, Repair: repair, Interactive: applicationServices().InteractiveInput(os.Stdin)})
	for _, notice := range result.Notices {
		if notice.Styled {
			cliResult(notice.Message)
		} else {
			fmt.Println(notice.Message)
		}
	}
	if err != nil {
		return err
	}
	if repair || !applicationServices().InteractiveInput(os.Stdin) {
		return nil
	}
	return offerDefaultAliases(result.AliasPath, os.Stdin, os.Stdout)
}

func removeSetup(shellName string) error {
	result, err := applicationServices().RemoveSetup(shellName)
	if err != nil {
		return err
	}
	fmt.Printf("Removed Alias Lens %s shell integration. Start a new %s shell to finish. Your aliases, configuration, revisions, and repositories were kept.\n", result.DisplayName, result.Shell)
	return nil
}

func ensureAliasFileExists(aliasPath string) error {
	notices, err := applicationServices().EnsureAliasFile(aliasPath)
	for _, notice := range notices {
		fmt.Println(notice.Message)
	}
	return err
}
