//go:build !windows

package app

import "testing"

func TestShadowFinalizationOrderAndSummary(t *testing.T) {
	report := shadowFinalizationFixture()
	expected := []string{"invalid", "blocked", "duplicate", "different", "unsupported", "equivalent"}
	for index, status := range expected {
		if report.Results[index].Status != status {
			t.Fatalf("result %d = %s, want %s", index, report.Results[index].Status, status)
		}
	}
	summary := report.Summary
	if summary.Equivalent != 1 || summary.Unsupported != 1 || summary.Different != 1 || summary.Duplicate != 1 || summary.Invalid != 1 || summary.Blocked != 1 {
		t.Fatalf("summary = %+v", summary)
	}
}

func shadowFinalizationFixture() ShadowReport {
	report := ShadowReport{SchemaVersion: 1, Shell: "bash", Diagnostics: []shadowDiagnostic{}, Results: []shadowResult{
		{Unit: 5, Name: "ok", Kind: "command", Status: "equivalent", StartByte: 50, EndByte: 51, StartLine: 6, EndLine: 6, Diagnostics: []shadowDiagnostic{}},
		{Unit: 4, Name: "unsup", Status: "unsupported", StartByte: 40, EndByte: 41, StartLine: 5, EndLine: 5, Diagnostics: []shadowDiagnostic{}},
		{Unit: 3, Name: "diff", Kind: "command", Status: "different", StartByte: 30, EndByte: 31, StartLine: 4, EndLine: 4, DifferentFields: []string{"description"}, Diagnostics: []shadowDiagnostic{}},
		{Unit: 2, Name: "dup", Kind: "command", Status: "duplicate", StartByte: 20, EndByte: 21, StartLine: 3, EndLine: 3, Diagnostics: []shadowDiagnostic{}},
		{Unit: 1, Name: "block", Kind: "command", Status: "blocked", StartByte: 10, EndByte: 11, StartLine: 2, EndLine: 2, Diagnostics: []shadowDiagnostic{}},
		{Unit: 0, Name: "inv", Kind: "command", Status: "invalid", StartByte: 0, EndByte: 1, StartLine: 1, EndLine: 1, Diagnostics: []shadowDiagnostic{}},
	}}
	finalizeShadowReport(&report)
	return report
}
