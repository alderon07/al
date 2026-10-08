package main

import (
	"alias-lens/internal/app"
	"fmt"
	"os"
	"strings"
	"time"
)

func runExportCommand(arguments []string) error {
	if len(arguments) == 0 || (arguments[0] != "aliases" && arguments[0] != "stats") {
		return fmt.Errorf("usage: al export aliases|stats [--format json|yaml|csv] [--period PERIOD] [--output PATH]")
	}
	kind := arguments[0]
	format := "json"
	period := "all"
	outputPath := ""
	for index := 1; index < len(arguments); index++ {
		if index+1 >= len(arguments) {
			return fmt.Errorf("%s requires a value", arguments[index])
		}
		value := arguments[index+1]
		switch arguments[index] {
		case "--format":
			format = strings.ToLower(value)
		case "--period":
			period = strings.ToLower(value)
		case "--output":
			outputPath = value
		default:
			return fmt.Errorf("unknown export option %q", arguments[index])
		}
		index++
	}
	if format != "json" && format != "yaml" && format != "csv" {
		return fmt.Errorf("format must be json, yaml, or csv")
	}
	if kind == "aliases" && period != "all" {
		return fmt.Errorf("--period applies only to stats exports")
	}

	now := time.Now()
	contents, err := applicationServices().ExportData(app.ExportRequest{Kind: kind, Format: format, Period: period, At: now, OutputPath: outputPath})
	if err != nil {
		return err
	}
	if outputPath == "" {
		_, err = os.Stdout.Write(contents)
		return err
	}
	fmt.Fprintf(os.Stderr, "Exported %s to %s\n", kind, outputPath)
	return nil
}
