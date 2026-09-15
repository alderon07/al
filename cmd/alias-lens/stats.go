package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
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
	var bashTime time.Time
	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		eventTime := time.Time{}
		if shell == "zsh" && strings.HasPrefix(line, ": ") {
			if separator := strings.IndexByte(line, ';'); separator >= 0 {
				metadata := strings.TrimPrefix(line[:separator], ": ")
				stamp, _, _ := strings.Cut(metadata, ":")
				if unix, parseErr := strconv.ParseInt(stamp, 10, 64); parseErr == nil {
					eventTime = time.Unix(unix, 0)
				}
				line = line[separator+1:]
			}
		} else if shell == "bash" && strings.HasPrefix(line, "#") {
			if unix, parseErr := strconv.ParseInt(strings.TrimPrefix(line, "#"), 10, 64); parseErr == nil {
				bashTime = time.Unix(unix, 0)
				continue
			}
		} else if shell == "bash" {
			eventTime = bashTime
			bashTime = time.Time{}
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
	case "year":
		return now.AddDate(-1, 0, 0), nil
	default:
		return time.Time{}, fmt.Errorf("period must be all, today, week, or year")
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
			return fmt.Errorf("usage: al stats [--plain] [all|today|week|year]")
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
	if !plain && fileIsTerminal(os.Stdout) {
		return runStatsTUI(data, period, now)
	}
	if len(rows) == 0 {
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
