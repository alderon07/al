package main

import (
	"alias-lens/internal/app"
	terminalui "alias-lens/internal/tui"
	"time"
)

func runTUIWithDiff(repositoryDiff bool) error {
	return terminalui.Browser(applicationServices(), repositoryDiff)
}
func runAliasPicker(query string, commandOnly, executeSelection bool) error {
	return terminalui.Picker(applicationServices(), terminalui.PickerOptions{Query: query, CommandOnly: commandOnly, ExecuteSelection: executeSelection})
}
func runStatsTUI(data app.StatsData, period string, now time.Time) error {
	return terminalui.Stats(applicationServices(), data, period, now)
}
