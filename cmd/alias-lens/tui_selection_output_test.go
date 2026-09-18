package main

import (
	"bytes"
	"testing"
)

func TestCapturedAliasSelectionIsShownBeforeExecution(t *testing.T) {
	var stdout bytes.Buffer
	var terminal bytes.Buffer

	writeAliasSelection(&stdout, &terminal, "gs", false)

	if stdout.String() != "gs\n" {
		t.Fatalf("machine-readable selection = %q, want alias name only", stdout.String())
	}
	if terminal.String() != "$ gs\n" {
		t.Fatalf("terminal display = %q, want prompt-style alias invocation", terminal.String())
	}
}

func TestCapturedAliasSelectionUsesVisibleFallbackStream(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	writeAliasSelection(&stdout, &stderr, "ll", false)

	if stdout.String() != "ll\n" || stderr.String() != "$ ll\n" {
		t.Fatalf("fallback output was not split safely: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestDirectAliasSelectionIsNotPrintedTwice(t *testing.T) {
	var stdout bytes.Buffer
	var terminal bytes.Buffer

	writeAliasSelection(&stdout, &terminal, "gs", true)

	if stdout.String() != "gs\n" || terminal.Len() != 0 {
		t.Fatalf("direct output was duplicated: stdout=%q terminal=%q", stdout.String(), terminal.String())
	}
}

func TestPromptAcceptedSelectionIsNotPrintedTwice(t *testing.T) {
	t.Setenv("ALIAS_LENS_PROMPT_ACCEPT", "1")
	var stdout bytes.Buffer
	var terminal bytes.Buffer

	writeAliasSelection(&stdout, &terminal, "bad", false)

	if stdout.String() != "bad\n" || terminal.Len() != 0 {
		t.Fatalf("prompt-accepted output was duplicated: stdout=%q terminal=%q", stdout.String(), terminal.String())
	}
}

func TestTabSelectionUsesPromptEditProtocol(t *testing.T) {
	t.Setenv("ALIAS_LENS_PROMPT_ACCEPT", "1")
	var stdout bytes.Buffer
	var terminal bytes.Buffer

	writeAliasEditSelection(&stdout, &terminal, "gs", false)

	if stdout.String() != editSelectionPrefix+"gs\n" || terminal.Len() != 0 {
		t.Fatalf("edit selection output: stdout=%q terminal=%q", stdout.String(), terminal.String())
	}
}
