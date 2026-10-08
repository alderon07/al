//go:build !windows

package main

import (
	"github.com/creack/pty/v2"

	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatalogSyncPTYHelper(t *testing.T) {
	if os.Getenv("AL_CATALOG_SYNC_PTY") != "1" {
		return
	}
	repo, value := setupCatalogSyncFixture(t)
	value.Entries[0].Description = "remote synthetic edit"
	writeCatalogFixture(t, filepath.Join(repo, "catalog.json"), value)
	if err := runCatalogSyncPull(false); err != nil {
		t.Fatal(err)
	}
}

func TestCatalogSemanticPullPTY(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestCatalogSyncPTYHelper$")
	cmd.Env = append(os.Environ(), "AL_CATALOG_SYNC_PTY=1", "TERM=xterm-256color")
	terminal, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		t.Fatal(err)
	}
	defer terminal.Close()
	read := make(chan string, 1)
	go func() { data, _ := io.ReadAll(terminal); read <- string(data) }()
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	output := <-read
	if !strings.Contains(output, "catalog") || !strings.Contains(output, "al sync --pull --apply") {
		t.Fatal("PTY preview missing", output)
	}
}
