package main

import (
	"bytes"
	"testing"
)

func TestAliasSelectionWritesOnlyTheMachineReadableName(t *testing.T) {
	var stdout bytes.Buffer

	writeAliasSelection(&stdout, "gs")

	if stdout.String() != "gs\n" {
		t.Fatalf("machine-readable selection = %q, want alias name only", stdout.String())
	}
}
