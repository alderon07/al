package main

import (
	"fmt"
	"os"

	"strings"
	"time"
)

func runStatsCommand(arguments []string) error {
	period := "all"
	plain := false
	periodSet := false
	for _, argument := range arguments {
		if argument == "--plain" {
			plain = true
			continue
		}
		if periodSet {
			return fmt.Errorf("usage: al stats [--plain] [all|today|week|month|year]")
		}
		period = strings.ToLower(argument)
		periodSet = true
	}
	now := time.Now()
	data, err := applicationServices().LoadStatsData()
	if err != nil {
		return err
	}
	rows, err := applicationServices().RankedStatsRows(data, period, now)
	if err != nil {
		return err
	}
	if !plain && !terminalIsDumb() && fileIsTerminal(os.Stdout) {
		return runStatsTUI(data, period, now)
	}
	if len(rows) == 0 {
		if period != "all" && applicationServices().HasUntimestampedUsage(data.Events) {
			fmt.Printf("Alias uses exist, but your shell history has no dates for them. Start a new shell so Alias Lens can date future commands, then try again.\n")
			return nil
		}
		fmt.Printf("No alias uses found for %s.\n", period)
		return nil
	}
	fmt.Printf("Most used aliases (%s):\n", period)
	for index, row := range rows {
		fmt.Printf("%2d  %-16s %d\n", index+1, row.Alias.Name, row.Count)
	}
	return nil
}

func fileIsTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
