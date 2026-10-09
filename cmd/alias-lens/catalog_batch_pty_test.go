//go:build !windows

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alderon07/al/internal/catalog"
	"github.com/creack/pty/v2"
)

func TestCatalogBatchPTYManyEntriesCancelAndIndividual(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("Bash runtime unavailable")
	}
	for _, width := range []uint16{52, 140} {
		for _, individual := range []bool{false, true} {
			t.Run(fmt.Sprintf("width-%d-individual-%t", width, individual), func(t *testing.T) {
				_, value := setupCatalogSyncFixture(t)
				value.Entries = nil
				var native strings.Builder
				for index := 0; index < 8; index++ {
					name := fmt.Sprintf("fixture%d", index)
					command := "printf synthetic"
					value.Entries = append(value.Entries, catalog.Entry{ID: fmt.Sprintf("%032x", index+1), Name: name, Kind: "command", Native: map[string]catalog.NativeImplementation{"bash": {AliasValue: &command}}})
					fmt.Fprintf(&native, "alias %s='printf synthetic'\n", name)
				}
				writeCatalogFixture(t, catalogPathFixture(), value)
				if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), ".bash_aliases"), []byte(native.String()), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), ".bashrc"), []byte("# synthetic startup\n"), 0600); err != nil {
					t.Fatal(err)
				}
				before := shadowTreeManifest(t, os.Getenv("HOME"))
				command := exec.Command(os.Args[0], "catalog", "enable", "--shell", "bash")
				command.Env = append(os.Environ(), "AL_INIT_PTY_HELPER=1", "TERM=xterm-256color")
				terminal, err := pty.StartWithSize(command, &pty.Winsize{Rows: 24, Cols: width})
				if err != nil {
					t.Fatal(err)
				}
				defer terminal.Close()
				defer func() { _ = command.Process.Kill(); _ = command.Wait() }()
				output := &synchronizedBuffer{}
				go func() { _, _ = io.Copy(output, terminal) }()
				session := &shellPTY{t: t, file: terminal, cmd: command, output: output}
				session.waitFor(0, "Approve this exact batch?")
				text := output.stringFrom(0)
				if !strings.Contains(text, "8 native approvals and 8 fallback enrollments") || strings.Contains(text, "Approve this exact native implementation?") {
					t.Fatal("batch did not display all captured records first")
				}
				for _, entry := range value.Entries {
					if !strings.Contains(text, entry.Name) || !strings.Contains(text, entry.ID) {
						t.Fatal("batch omitted captured declaration")
					}
				}
				if individual {
					start := len(output.stringFrom(0))
					session.write("i\n")
					for index := 0; index < 16; index++ {
						prompt := "Approve this exact native implementation?"
						if index%2 == 1 {
							prompt = "Enroll this exact native fallback?"
						}
						session.waitFor(start, prompt)
						start = len(output.stringFrom(0))
						session.write("y\n")
					}
					session.waitFor(start, "Apply this catalog installation?")
					if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".local", "state", "alias-lens", "native-approvals.json")); !os.IsNotExist(err) {
						t.Fatal("individual choices saved before final confirmation")
					}
				}
				session.write("n\n")
				done := make(chan error, 1)
				go func() { done <- command.Wait() }()
				select {
				case err := <-done:
					if err != nil {
						t.Fatalf("cancel returned error: %v %s", err, output.stringFrom(0))
					}
				case <-time.After(10 * time.Second):
					t.Fatal("batch cancellation timed out")
				}
				after := shadowTreeManifest(t, os.Getenv("HOME"))
				if !reflect.DeepEqual(before, after) {
					t.Fatal("cancelled review mutated private files")
				}
			})
		}
	}
}
