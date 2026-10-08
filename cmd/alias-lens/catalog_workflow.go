package main

import "github.com/alderon07/al/internal/app"

import "github.com/alderon07/al/internal/presentation"

import (
	"errors"
	"fmt"
	"io"
	"os"

	"strings"

	neutralcatalog "github.com/alderon07/al/internal/catalog"
)

const catalogReportLimit = 8 << 20

var catalogWorkflowStdout io.Writer = os.Stdout
var catalogWorkflowTerminal = func() bool { return fileIsTerminal(os.Stdout) }

func catalogCommandUsageError() error {
	return fmt.Errorf("usage: al catalog preview|import|shadow|diff|plan|review|approve|adopt|enable|rollback [OPTIONS]")
}

func runCatalogWorkflowCommand(arguments []string) (bool, int, error) {
	if len(arguments) == 0 {
		return false, 0, nil
	}
	if handled, code, err := runCatalogLifecycleCommand(arguments); handled {
		return handled, code, err
	}
	switch arguments[0] {
	case "diff":
		code, err := runCatalogDiff(arguments[1:])
		return true, code, err
	default:
		return false, 0, nil
	}
}

func runCatalogDiff(arguments []string) (int, error) {
	jsonOutput := false
	showCode := false
	webOutput := false
	source := ""
	shell := ""
	for index := 0; index < len(arguments); index++ {
		switch arguments[index] {
		case "--json":
			jsonOutput = true
		case "--show-code":
			showCode = true
		case "--web":
			webOutput = true
		case "--from":
			if index+1 >= len(arguments) {
				return 2, fmt.Errorf("--from needs repository or installed")
			}
			index++
			source = arguments[index]
		case "--shell":
			if index+1 >= len(arguments) {
				return 2, fmt.Errorf("--shell needs bash or zsh")
			}
			index++
			shell = arguments[index]
		default:
			return 2, fmt.Errorf("unknown catalog diff option %q", arguments[index])
		}
	}
	if (jsonOutput && showCode) || (webOutput && (jsonOutput || showCode)) {
		return 2, fmt.Errorf("choose one of --json, --show-code, or --web")
	}
	if showCode && !catalogWorkflowTerminal() {
		return 2, fmt.Errorf("--show-code needs an interactive terminal because it can display private command text")
	}
	comparison, err := applicationServices().CompareCatalog(source, shell)
	if err != nil {
		var input *app.CatalogComparisonInputError
		if errors.As(err, &input) {
			return 2, err
		}
		return 1, err
	}
	other, local, report := comparison.Before, comparison.After, comparison.Report
	if webOutput {
		if err := runCatalogDiffWeb(other, local, report); err != nil {
			return 1, err
		}
		return 0, nil
	}
	var output []byte
	if jsonOutput {
		output, err = applicationServices().EncodeCatalogJSON(report)
	} else if showCode {
		output = renderCatalogDiffCode(report, other, local)
	} else {
		output = renderCatalogDiffPlain(report, other, local)
	}
	if err != nil {
		return 1, err
	}
	if len(output) > catalogReportLimit {
		return 1, fmt.Errorf("The comparison is too large to show safely")
	}
	if _, err := catalogWorkflowStdout.Write(output); err != nil {
		return 1, fmt.Errorf("show catalog comparison: %w", err)
	}
	if len(report.Diagnostics) > 0 || len(report.Changes) > 0 {
		return 1, nil
	}
	return 0, nil
}

func renderCatalogDiffCode(report neutralcatalog.SemanticDiffReport, before, after neutralcatalog.Catalog) []byte {
	var output strings.Builder
	output.Write(renderCatalogDiffPlain(report, before, after))
	if len(report.Diagnostics) > 0 || len(report.Changes) == 0 {
		return []byte(output.String())
	}
	beforeEntries := map[string]neutralcatalog.Entry{}
	afterEntries := map[string]neutralcatalog.Entry{}
	for _, entry := range before.Entries {
		beforeEntries[entry.ID] = entry
	}
	for _, entry := range after.Entries {
		afterEntries[entry.ID] = entry
	}
	output.WriteString("\nExact changes:\n")
	for _, change := range report.Changes {
		if change.Scope != "entry" {
			continue
		}
		oldEntry, oldOK := beforeEntries[change.EntryID]
		newEntry, newOK := afterEntries[change.EntryID]
		fmt.Fprintf(&output, "\n%s · %s\n", presentation.EscapePlainText(change.Name), presentation.FriendlyCatalogField(change.Path))
		if oldOK {
			fmt.Fprintf(&output, "before: %s\n", presentation.QuotedCatalogValue(presentation.CatalogFieldValue(oldEntry, change.Path)))
		}
		if newOK {
			fmt.Fprintf(&output, "after:  %s\n", presentation.QuotedCatalogValue(presentation.CatalogFieldValue(newEntry, change.Path)))
		}
	}
	return []byte(output.String())
}

func renderCatalogDiffPlain(report neutralcatalog.SemanticDiffReport, before, after neutralcatalog.Catalog) []byte {
	var output strings.Builder
	if len(report.Diagnostics) > 0 {
		output.WriteString("Alias Lens could not compare the catalogs. Your files were left unchanged.\n")
		for _, item := range report.Diagnostics {
			fmt.Fprintf(&output, "- %s\n", presentation.EscapePlainText(item.Message))
		}
		return []byte(output.String())
	}
	if len(report.Changes) == 0 {
		output.WriteString("Your catalog matches the saved copy. Nothing needs to change.\n")
		return []byte(output.String())
	}
	fmt.Fprintf(&output, "Found %d added, %d removed, and %d changed entries. Nothing has been changed yet.\n\n", report.Summary.Added, report.Summary.Deleted, report.Summary.Changed)
	for _, change := range report.Changes {
		name := presentation.EscapePlainText(change.Name)
		if name == "" {
			name = "catalog"
		}
		switch change.Kind {
		case "added":
			fmt.Fprintf(&output, "+ Add %s\n", name)
		case "deleted":
			fmt.Fprintf(&output, "- Remove %s\n", name)
		default:
			fmt.Fprintf(&output, "~ Change %s: %s\n", name, presentation.FriendlyCatalogField(change.Path))
		}
		writeCatalogChangeDetails(&output, change, before, after)
	}
	output.WriteString("\nReview these changes before combining them.\n")
	return []byte(output.String())
}

func writeCatalogChangeDetails(output *strings.Builder, change neutralcatalog.SemanticChange, before, after neutralcatalog.Catalog) {
	implementation := change.Path == "portable" || strings.HasPrefix(change.Path, "native.") || change.Path == "entry"
	if implementation {
		if change.BeforeSHA256 != "" {
			fmt.Fprintf(output, "  Before: %d bytes, fingerprint %s\n", change.BeforeBytes, presentation.ShortFingerprint(change.BeforeSHA256))
		}
		if change.AfterSHA256 != "" {
			fmt.Fprintf(output, "  After: %d bytes, fingerprint %s\n", change.AfterBytes, presentation.ShortFingerprint(change.AfterSHA256))
		}
		return
	}
	beforeEntries, afterEntries := map[string]neutralcatalog.Entry{}, map[string]neutralcatalog.Entry{}
	for _, entry := range before.Entries {
		beforeEntries[entry.ID] = entry
	}
	for _, entry := range after.Entries {
		afterEntries[entry.ID] = entry
	}
	if entry, ok := beforeEntries[change.EntryID]; ok {
		fmt.Fprintf(output, "  Before: %s\n", presentation.QuotedCatalogValue(presentation.CatalogFieldValue(entry, change.Path)))
	}
	if entry, ok := afterEntries[change.EntryID]; ok {
		fmt.Fprintf(output, "  After: %s\n", presentation.QuotedCatalogValue(presentation.CatalogFieldValue(entry, change.Path)))
	}
}
