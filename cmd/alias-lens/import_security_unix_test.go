//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestImportReaderRejectsFIFOWithoutBlocking(t *testing.T) {
	path := filepath.Join(t.TempDir(), "aliases.fifo")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() {
		_, err := readRegularFile(path, importFileLimit)
		finished <- err
	}()
	select {
	case err := <-finished:
		if err == nil || !strings.Contains(err.Error(), "regular file") {
			t.Fatalf("FIFO read error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("FIFO read blocked")
	}
}

func TestImportReaderRejectsOversizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "aliases")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(importFileLimit + 1); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := readRegularFile(path, importFileLimit); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized import error = %v", err)
	}
}

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
