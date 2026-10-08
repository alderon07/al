package main

import "github.com/alderon07/al/internal/presentation"

import (
	"fmt"

	"time"
)

func runAutoSyncCommand(arguments []string) error {
	_, err := loadConfig()
	if err != nil {
		return err
	}
	if len(arguments) != 1 || (arguments[0] != "enable" && arguments[0] != "disable" && arguments[0] != "status") {
		return fmt.Errorf("usage: al autosync enable|disable|status")
	}
	if arguments[0] == "status" {
		snapshot, err := applicationServices().SyncStatus()
		if err != nil {
			return err
		}
		state := snapshot.Primary.State
		if cliStyled() {
			cliHeading("Automatic sync")
		}
		cliKeyValue("enabled", fmt.Sprint(snapshot.Enabled))
		cliKeyValue("status", presentation.DefaultString(state.Status, "not started"))
		if state.Message != "" {
			cliKeyValue("message", state.Message)
		}
		if !state.UpdatedAt.IsZero() {
			cliKeyValue("updated", state.UpdatedAt.Local().Format(time.RFC3339))
		}
		for _, tracked := range snapshot.Tracked {
			fmt.Printf("%s: %s -> %s [%s]\n", cliAccent("tracked"), tracked.File.Source, tracked.File.RepositoryPath, presentation.DefaultString(tracked.State.Status, "waiting"))
		}
		return nil
	}

	enabled := arguments[0] == "enable"
	if err := applicationServices().SetAutomaticSync(enabled); err != nil {
		return err
	}
	if enabled {
		cliResult("Automatic sync enabled.")
	} else {
		cliResult("Automatic sync disabled. The current worker will stop on its next check.")
	}
	return nil
}
