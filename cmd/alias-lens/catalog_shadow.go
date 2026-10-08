//go:build !windows

package main

import (
	"github.com/alderon07/al/internal/app"

	shellapi "github.com/alderon07/al/internal/shell"

	"encoding/json"

	"fmt"
	"io"
	"os"

	"strings"

	neutralcatalog "github.com/alderon07/al/internal/catalog"
)

var shadowStdout io.Writer = os.Stdout

func runCatalogCommand(arguments []string) (exitCode int, commandErr error) {
	defer func() {
		if recover() != nil {
			exitCode = 2
			commandErr = fmt.Errorf("shadow inspection stopped unexpectedly")
		}
	}()
	if handled, code, err := runCatalogWorkflowCommand(arguments); handled {
		return code, err
	}
	if len(arguments) > 0 && arguments[0] == "preview" {
		return runCatalogPreviewCommand(arguments[1:])
	}
	if len(arguments) > 0 && arguments[0] == "import" {
		return runCatalogImportCommand(arguments[1:])
	}
	if len(arguments) == 0 || arguments[0] != "shadow" {
		return 2, catalogCommandUsageError()
	}
	jsonOutput := false
	shellName := ""
	for index := 1; index < len(arguments); index++ {
		switch arguments[index] {
		case "--json":
			jsonOutput = true
		case "--shell":
			if index+1 >= len(arguments) {
				return 2, fmt.Errorf("--shell requires bash or zsh")
			}
			index++
			shellName = arguments[index]
		default:
			return 2, fmt.Errorf("unknown catalog shadow option %q", arguments[index])
		}
	}
	report, err := applicationServices().InspectCatalogShadow(shellName)
	if err != nil {
		return 2, err
	}
	output, err := renderShadowReport(report, jsonOutput)
	if err != nil {
		return 2, err
	}
	if len(output) > app.ShadowReportLimit {
		return 2, fmt.Errorf("shadow report exceeds 16 MiB")
	}
	written, err := shadowStdout.Write(output)
	if err != nil {
		return 2, fmt.Errorf("write shadow report: %w", err)
	}
	if written != len(output) {
		return 2, fmt.Errorf("write shadow report: %w", io.ErrShortWrite)
	}
	for _, result := range report.Results {
		if result.Status != "equivalent" {
			return 1, nil
		}
	}
	return 0, nil
}

func renderShadowEntry(entry neutralcatalog.Entry, origin string) []byte {
	return shellapi.RenderShadowEntry(entry, origin)
}

func renderShadowReport(report app.ShadowReport, jsonOutput bool) ([]byte, error) {
	if jsonOutput {
		contents, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return nil, err
		}
		return append(contents, '\n'), nil
	}
	var output strings.Builder
	fmt.Fprintf(&output, "Alias Lens shadow inspection · %s\n\n", report.Shell)
	for _, result := range report.Results {
		label := result.Name
		if label == "" {
			label = fmt.Sprintf("lines %d-%d", result.StartLine, result.EndLine)
		}
		fmt.Fprintf(&output, "%-11s %s\n", result.Status, label)
	}
	fmt.Fprintf(&output, "\n%d equivalent · %d unsupported · %d different · %d duplicate · %d invalid · %d blocked\n", report.Summary.Equivalent, report.Summary.Unsupported, report.Summary.Different, report.Summary.Duplicate, report.Summary.Invalid, report.Summary.Blocked)
	return []byte(output.String()), nil
}
