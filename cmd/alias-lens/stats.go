package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type usageEvent struct {
	Name string
	Time time.Time
}

func usageLogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "alias-lens", "usage.tsv"), nil
}

func recordAliasUse(name string) error {
	if !aliasName.MatchString(name) {
		return fmt.Errorf("invalid alias name %q", name)
	}
	path, err := usageLogPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := file.Chmod(0o600); err != nil {
		return err
	}
	_, err = fmt.Fprintf(file, "%d\t%s\n", time.Now().Unix(), name)
	return err
}

func loadUsageEvents() ([]usageEvent, error) {
	path, err := usageLogPath()
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
	var events []usageEvent
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		stamp, name, found := strings.Cut(scanner.Text(), "\t")
		unix, parseErr := strconv.ParseInt(stamp, 10, 64)
		if !found || parseErr != nil || !aliasName.MatchString(name) {
			continue
		}
		events = append(events, usageEvent{Name: name, Time: time.Unix(unix, 0)})
	}
	return events, scanner.Err()
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

func runStatsCommand(arguments []string) error {
	if len(arguments) > 1 {
		return fmt.Errorf("usage: al stats [all|today|week|year]")
	}
	period := "all"
	if len(arguments) == 1 {
		period = strings.ToLower(arguments[0])
	}
	now := time.Now()
	var since time.Time
	switch period {
	case "all":
	case "today":
		since = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "week":
		since = now.AddDate(0, 0, -7)
	case "year":
		since = now.AddDate(-1, 0, 0)
	default:
		return fmt.Errorf("period must be all, today, week, or year")
	}
	aliases, err := loadAliases()
	if err != nil {
		return err
	}
	events, err := loadUsageEvents()
	if err != nil {
		return err
	}
	counts := usageCountsSince(events, since)
	type row struct {
		Name  string
		Count int
	}
	rows := make([]row, 0, len(aliases))
	for _, alias := range aliases {
		if count := counts[alias.Name]; count > 0 {
			rows = append(rows, row{Name: alias.Name, Count: count})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Count == rows[j].Count {
			return rows[i].Name < rows[j].Name
		}
		return rows[i].Count > rows[j].Count
	})
	if len(rows) == 0 {
		fmt.Printf("No Alias Lens launches recorded for %s.\n", period)
		return nil
	}
	fmt.Printf("Most used aliases (%s):\n", period)
	for index, row := range rows {
		fmt.Printf("%2d  %-16s %d\n", index+1, row.Name, row.Count)
	}
	return nil
}
