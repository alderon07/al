//go:build !windows

package main

import (
	"path/filepath"
	"strings"

	"testing"
)

func TestImportReadErrorEscapesTerminalControlBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing\x1b]52;c;payload\a")
	err := runImportCommand([]string{path})
	if err == nil {
		t.Fatal("missing import unexpectedly succeeded")
	}
	if strings.ContainsAny(err.Error(), "\x1b\a") {
		t.Fatalf("import error contains terminal control bytes: %q", err)
	}
}
