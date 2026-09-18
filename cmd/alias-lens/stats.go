package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

type usageEvent struct {
	Name string
	Time time.Time
}

type statsRow struct {
	Alias Alias
	Count int
}

type statsData struct {
	Aliases []Alias
	Events  []usageEvent
}

func hasUntimestampedUsage(events []usageEvent) bool {
	for _, event := range events {
		if event.Time.IsZero() {
			return true
		}
	}
	return false
}

func usageCountsSince(events []usageEvent, since time.Time) map[string]int {
	counts := make(map[string]int)
	for _, event := range events {
		if since.IsZero() || !event.Time.Before(since) {
			counts[event.Name]++
		}
	}
	return counts
}

func loadHistoryUsageEvents(aliases []Alias) ([]usageEvent, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	adapter := activeShellAdapter()
	return historyUsageEventsFromShell(historyPathFor(adapter, home), adapter.Name(), aliases)
}

func historyUsageEventsFromShell(path, shell string, aliases []Alias) ([]usageEvent, error) {
	adapter, err := shellAdapter(shell)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	known := make(map[string]bool, len(aliases))
	for _, alias := range aliases {
		known[alias.Name] = true
	}
	var events []usageEvent
	state := shellHistoryState{}
	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 1024*1024)
	for scanner.Scan() {
		line, eventTime, skip := adapter.HistoryUsageLine(scanner.Text(), &state)
		if skip {
			continue
		}

		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) > 0 && known[fields[0]] {
			events = append(events, usageEvent{Name: fields[0], Time: eventTime})
		}
	}
	return events, scanner.Err()
}

func statsPeriodSince(period string, now time.Time) (time.Time, error) {
	switch period {
	case "all":
		return time.Time{}, nil
	case "today":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()), nil
	case "week":
		return now.AddDate(0, 0, -7), nil
	case "month":
		return now.AddDate(0, -1, 0), nil
	case "year":
		return now.AddDate(-1, 0, 0), nil
	default:
		return time.Time{}, fmt.Errorf("period must be all, today, week, month, or year")
	}
}

func rankedStatsRows(data statsData, period string, now time.Time) ([]statsRow, error) {
	since, err := statsPeriodSince(period, now)
	if err != nil {
		return nil, err
	}
	counts := usageCountsSince(data.Events, since)
	rows := make([]statsRow, 0, len(data.Aliases))
	for _, alias := range data.Aliases {
		if count := counts[alias.Name]; count > 0 {
			rows = append(rows, statsRow{Alias: alias, Count: count})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Count == rows[j].Count {
			return rows[i].Alias.Name < rows[j].Alias.Name
		}
		return rows[i].Count > rows[j].Count
	})
	return rows, nil
}

func loadStatsData() (statsData, error) {
	aliases, err := loadAliases()
	if err != nil {
		return statsData{}, err
	}
	historyEvents, err := loadHistoryUsageEvents(aliases)
	if err != nil {
		return statsData{}, err
	}
	return statsData{Aliases: aliases, Events: historyEvents}, nil
}

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
	data, err := loadStatsData()
	if err != nil {
		return err
	}
	rows, err := rankedStatsRows(data, period, now)
	if err != nil {
		return err
	}
	if !plain && !terminalIsDumb() && fileIsTerminal(os.Stdout) {
		return runStatsTUI(data, period, now)
	}
	if len(rows) == 0 {
		if period != "all" && hasUntimestampedUsage(data.Events) {
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
