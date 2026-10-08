//go:build !windows

package app

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
		_, err := readRegularFile(path, ImportFileLimit)
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
	if err := file.Truncate(ImportFileLimit + 1); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := readRegularFile(path, ImportFileLimit); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized import error = %v", err)
	}
}
