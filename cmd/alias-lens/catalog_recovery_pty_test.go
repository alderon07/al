//go:build !windows

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alderon07/al/internal/transaction"
	"github.com/creack/pty/v2"
)

func TestCompiledCatalogRecoveryPTY(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal("Bash is required for recovery PTY evidence")
	}
	binary := filepath.Join(t.TempDir(), "alias-lens")
	if output, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, output)
	}
	for _, width := range []uint16{52, 140} {
		for _, blocked := range []bool{false, true} {
			t.Run(fmt.Sprintf("width-%d-blocked-%t", width, blocked), func(t *testing.T) {
				home := t.TempDir()
				if err := os.Chmod(home, 0700); err != nil {
					t.Fatal(err)
				}
				stateRoot := filepath.Join(home, ".local", "state", "alias-lens")
				if err := os.MkdirAll(stateRoot, 0700); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(stateRoot, "sync-state.json")
				baseline := []byte(`{"local_hash":"` + strings.Repeat("a", 64) + `","remote_hash":"` + strings.Repeat("b", 64) + `","status":"waiting","message":"synthetic private baseline","updated_at":"2026-10-09T10:00:00Z"}` + "\n")
				if err := os.WriteFile(path, baseline, 0600); err != nil {
					t.Fatal(err)
				}
				expected, err := transaction.InspectWorkflowTarget(path, 1<<20, false)
				if err != nil {
					t.Fatal(err)
				}
				spec := transaction.WorkflowSpec{OperationID: "recovery-pty", StateRoot: stateRoot, PrivateRoots: []string{stateRoot}, Targets: []transaction.WorkflowTarget{{Path: path, Role: transaction.WorkflowPrivate, Expected: expected, Planned: bytes.Replace(baseline, []byte("waiting"), []byte("synced"), 1), Mode: 0600, RecoveryOrder: 3}}}
				stop := errors.New("synthetic interruption")
				if err := transaction.ApplyWorkflow(spec, func(boundary transaction.WorkflowBoundary) error {
					if boundary.Stage == "backup_synced" {
						return stop
					}
					return nil
				}); !errors.Is(err, stop) {
					t.Fatal(err)
				}
				journal := filepath.Join(stateRoot, "workflows", "recovery-pty.workflow")
				manifest, _, err := transaction.ReadWorkflowJournal(stateRoot, journal)
				if err != nil {
					t.Fatal(err)
				}
				current := bytes.Replace(baseline, []byte("waiting"), []byte("offline"), 1)
				current = bytes.Replace(current, []byte("synthetic private baseline"), []byte("synthetic private replacement"), 1)
				if blocked {
					current = bytes.Replace(current, []byte(strings.Repeat("a", 64)), []byte(strings.Repeat("c", 64)), 1)
				}
				if err := os.WriteFile(path, current, 0600); err != nil {
					t.Fatal(err)
				}
				identity, err := transaction.InspectWorkflowTarget(path, 1<<20, false)
				if err != nil {
					t.Fatal(err)
				}
				var blockedJournal []byte
				for attempt := 0; attempt < 2; attempt++ {
					command := exec.Command(bash, "--noprofile", "--norc", "-c", `exec "$1" catalog recover`, "recovery-pty", binary)
					command.Env = []string{"HOME=" + home, "XDG_CONFIG_HOME=" + filepath.Join(home, ".config"), "PATH=" + os.Getenv("PATH"), "TERM=xterm-256color", "ALIAS_LENS_SHELL=bash"}
					terminal, err := pty.StartWithSize(command, &pty.Winsize{Rows: 24, Cols: width})
					if err != nil {
						t.Fatal(err)
					}
					output := &synchronizedBuffer{}
					copied := make(chan struct{})
					go func() { _, _ = io.Copy(output, terminal); close(copied) }()
					done := make(chan error, 1)
					go func() { done <- command.Wait() }()
					select {
					case err = <-done:
					case <-time.After(15 * time.Second):
						_ = command.Process.Kill()
						_ = terminal.Close()
						t.Fatal("recovery timed out")
					}
					<-copied
					_ = terminal.Close()
					text := output.stringFrom(0)
					if blocked {
						if err == nil || !strings.Contains(text, "target 0") {
							t.Fatal("authority drift was not blocked", text)
						}
						journalBytes, readErr := os.ReadFile(journal)
						if readErr != nil {
							t.Fatal(readErr)
						}
						if attempt == 0 {
							blockedJournal = journalBytes
						} else if !bytes.Equal(blockedJournal, journalBytes) {
							t.Fatal("blocked retry grew the journal")
						}
					} else if err != nil || !strings.Contains(text, "Incomplete workflows recovered.") {
						t.Fatal("diagnostic drift did not recover", text)
					}
					if strings.Contains(text, home) || strings.Contains(text, "synthetic private") || strings.Contains(text, strings.Repeat("a", 64)) {
						t.Fatal("recovery exposed private state")
					}
					actual, inspectErr := transaction.InspectWorkflowTarget(path, 1<<20, false)
					if inspectErr != nil || actual != identity {
						t.Fatal("recovery changed current target identity")
					}
					backup, readErr := os.ReadFile(manifest.Actions[0].BackupPath)
					if readErr != nil || !bytes.Equal(backup, baseline) {
						t.Fatal("recovery changed the original backup")
					}
				}
				if !blocked {
					if _, err := os.Stat(journal); !os.IsNotExist(err) {
						t.Fatal("recovered journal remained")
					}
				}
			})
		}
	}
}
