package main

import (
	"bytes"
	"testing"
)

func TestCapturedAliasSelectionIsAlsoShownOnTerminal(t *testing.T) {
	var stdout bytes.Buffer
	var terminal bytes.Buffer

	writeAliasSelection(&stdout, &terminal, "gs", false)

	if stdout.String() != "gs\n" {
		t.Fatalf("machine-readable selection = %q, want alias name only", stdout.String())
	}
	if terminal.String() != "$ gs\n" {
		t.Fatalf("terminal display = %q, want selected alias", terminal.String())
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
