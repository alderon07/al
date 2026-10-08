package main

import "github.com/alderon07/al/internal/presentation"

import (
	"github.com/alderon07/al/internal/app"

	"fmt"

	"sort"
)

func runAliasCheck(arguments []string) (int, error) {
	strict := false
	if len(arguments) == 1 && arguments[0] == "--strict" {
		strict = true
	} else if len(arguments) != 0 {
		return 2, fmt.Errorf("usage: al check [--strict]")
	}
	if active, err := applicationServices().CatalogManagedEditing(); err != nil {
		return 2, err
	} else if active {
		return runCatalogCheck(strict)
	}
	findings, err := applicationServices().CheckEntries()
	if err != nil {
		return 2, err
	}
	adapter := applicationServices().ActiveShellAdapter()
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Line == findings[j].Line {
			return findings[i].Severity < findings[j].Severity
		}
		return findings[i].Line < findings[j].Line
	})
	errors, warnings := 0, 0
	for _, finding := range findings {
		if finding.Severity == app.CheckError {
			errors++
		} else {
			warnings++
		}
		location := ""
		if finding.Line > 0 {
			location = fmt.Sprintf(" line %d", finding.Line)
		}
		severity := fmt.Sprintf("%-5s", finding.Severity)
		if finding.Severity == app.CheckError {
			severity = cliAttention(severity)
		} else {
			severity = cliAccent(severity)
		}
		fmt.Printf("%s%s  %s\n", severity, location, finding.Message)
	}
	if len(findings) == 0 {
		fmt.Printf("%s  %s is valid for %s.\n", cliPositive("OK"), applicationServices().AliasDisplayPath(), adapter.DisplayName())
		return 0, nil
	}
	fmt.Printf("Found %s and %s in %s.\n", presentation.FindingCount(errors, "error"), presentation.FindingCount(warnings, "warning"), applicationServices().AliasDisplayPath())
	if errors > 0 || strict && warnings > 0 {
		return 1, nil
	}
	return 0, nil
}
