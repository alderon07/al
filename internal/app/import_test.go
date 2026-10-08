package app

import (
	"strings"
	"testing"
)

func TestBuildImportPlanReportsDuplicatesConflictsAndSyntax(t *testing.T) {
	current := []byte("alias gs='git status'\nalias ll='ls -la'\n")
	source := []byte("alias gs='git switch'\nalias status='git status'\nalias one='echo ok'\nalias two='echo ok'\nalias broken='unterminated\n")
	plan := DefaultServices().buildImportPlan(source, current, DefaultServices().mustShellAdapter("bash"))

	joined := ""
	for _, issue := range plan.Issues {
		joined += issue.Kind + ":" + issue.Message + "\n"
	}
	for _, expected := range []string{"conflict:alias \"gs\"", "duplicate command:alias \"status\"", "duplicate command:aliases \"one\" and \"two\"", "error:quoted alias command"} {
		if !strings.Contains(joined, expected) {
			t.Errorf("issues missing %q:\n%s", expected, joined)
		}
	}
}

func TestImportPreviewEscapesTerminalControlBytes(t *testing.T) {
	input := "git status\x1b]52;c;clipboard\x07"
	got := terminalSafeText(input)
	if strings.ContainsAny(got, "\x1b\x07") {
		t.Fatalf("preview retained terminal control bytes: %q", got)
	}
	if !strings.Contains(got, `\x1b`) || !strings.Contains(got, `\x07`) {
		t.Fatalf("preview did not make control bytes visible: %q", got)
	}
}
