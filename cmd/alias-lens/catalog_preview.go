//go:build !windows

package main

import (
	"fmt"
	"strings"

	"github.com/alderon07/al/internal/app"
	"github.com/alderon07/al/internal/presentation"
)

func renderCatalogPreviewReport(report app.ShadowReport, jsonOutput bool) ([]byte, error) {
	if jsonOutput {
		return renderShadowReport(report, true)
	}
	var output strings.Builder
	fmt.Fprintf(&output, "Alias Lens catalog preview · %s\n\n", report.Shell)
	for _, diagnostic := range report.Diagnostics {
		fmt.Fprintf(&output, "report [%s] %s\n", diagnostic.Code, diagnostic.Message)
	}
	for _, result := range report.Results {
		if result.Name != "" && result.Status != "unsupported" {
			fmt.Fprintf(&output, "%-11s %s (lines %d-%d)\n", result.Status, presentation.TerminalSafeText(result.Name), result.StartLine, result.EndLine)
		} else {
			fmt.Fprintf(&output, "%-11s lines %d-%d\n", result.Status, result.StartLine, result.EndLine)
		}
		for _, diagnostic := range result.Diagnostics {
			fmt.Fprintf(&output, "  [%s] %s\n", diagnostic.Code, diagnostic.Message)
		}
		if result.Status != "equivalent" && len(result.Diagnostics) == 0 {
			fmt.Fprintln(&output, "  Details unavailable or omitted by the diagnostic limit. Next: review this range in the native file, then run al catalog preview again.")
		}
		if output.Len() > app.ShadowReportLimit {
			return nil, fmt.Errorf("catalog preview report exceeds 16 MiB")
		}
	}
	fmt.Fprintf(&output, "\n%d equivalent · %d unsupported · %d different · %d duplicate · %d invalid · %d blocked\n", report.Summary.Equivalent, report.Summary.Unsupported, report.Summary.Different, report.Summary.Duplicate, report.Summary.Invalid, report.Summary.Blocked)
	return []byte(output.String()), nil
}
