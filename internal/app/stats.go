package app

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

type UsageEvent struct {
	Name string
	Time time.Time
}

type StatsRow struct {
	Alias   Alias
	Count   int
	LastRun time.Time
}

type StatsData struct {
	Aliases []Alias
	Events  []UsageEvent
}

type AliasUsageSummary struct {
	All     int
	Today   int
	Week    int
	LastRun time.Time
}

func summarizeAliasUses(events []UsageEvent, now time.Time) map[string]AliasUsageSummary {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	week := now.AddDate(0, 0, -7)
	summaries := make(map[string]AliasUsageSummary)
	for _, event := range events {
		summary := summaries[event.Name]
		summary.All++
		if !event.Time.IsZero() {
			if !event.Time.Before(today) {
				summary.Today++
			}
			if !event.Time.Before(week) {
				summary.Week++
			}
			if event.Time.After(summary.LastRun) {
				summary.LastRun = event.Time
			}
		}
		summaries[event.Name] = summary
	}
	return summaries
}

func hasUntimestampedUsage(events []UsageEvent) bool {
	for _, event := range events {
		if event.Time.IsZero() {
			return true
		}
	}
	return false
}

func usageCountsSince(events []UsageEvent, since time.Time) map[string]int {
	counts := make(map[string]int)
	for _, event := range events {
		if since.IsZero() || !event.Time.Before(since) {
			counts[event.Name]++
		}
	}
	return counts
}

func (svc *Services) loadHistoryUsageEvents(aliases []Alias) ([]UsageEvent, error) {
	home, err := svc.dependencies.HomeDir()
	if err != nil {
		return nil, err
	}
	adapter := svc.activeShellAdapter()
	return svc.historyUsageEventsFromShell(svc.historyPathFor(adapter, home), adapter.Name(), aliases)
}

func (svc *Services) historyUsageEventsFromShell(path, shell string, aliases []Alias) ([]UsageEvent, error) {
	adapter, err := svc.shellAdapter(shell)
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
	var events []UsageEvent
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
			events = append(events, UsageEvent{Name: fields[0], Time: eventTime})
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

func rankedStatsRows(data StatsData, period string, now time.Time) ([]StatsRow, error) {
	since, err := statsPeriodSince(period, now)
	if err != nil {
		return nil, err
	}
	counts := usageCountsSince(data.Events, since)
	lastRuns := make(map[string]time.Time, len(counts))
	for _, event := range data.Events {
		if event.Time.IsZero() || (!since.IsZero() && event.Time.Before(since)) {
			continue
		}
		if event.Time.After(lastRuns[event.Name]) {
			lastRuns[event.Name] = event.Time
		}
	}
	rows := make([]StatsRow, 0, len(data.Aliases))
	for _, alias := range data.Aliases {
		if count := counts[alias.Name]; count > 0 {
			rows = append(rows, StatsRow{Alias: alias, Count: count, LastRun: lastRuns[alias.Name]})
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

func (svc *Services) loadStatsData() (StatsData, error) {
	aliases, err := svc.loadAliases()
	if err != nil {
		return StatsData{}, err
	}
	historyEvents, err := svc.loadHistoryUsageEvents(aliases)
	if err != nil {
		return StatsData{}, err
	}
	return StatsData{Aliases: aliases, Events: historyEvents}, nil
}
