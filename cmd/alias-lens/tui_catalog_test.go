//go:build !windows

package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	neutralcatalog "alias-lens/internal/catalog"
	"alias-lens/internal/catalogstore"
	tea "alias-lens/internal/tea"
	"github.com/creack/pty/v2"
)

func TestCatalogDrawerStagesSpecificApprovalAndCancels(t *testing.T) {
	key := catalogstore.ApprovalKey{EntryID: strings.Repeat("1", 32), Shell: "bash", Name: "sample", Kind: "command", ImplementationSHA256: strings.Repeat("2", 64), Renderer: "bash/v2"}
	record := catalogstore.ApprovalRecord{Key: key, ApprovedAt: "2026-10-03T00:00:00Z"}
	view := &catalogTUIView{Shell: "bash", Stage: "review", Review: []catalogTUIReviewItem{{Text: "Exact native declaration", Approval: &record}, {Text: "Exact fallback ownership"}}, Text: "Exact native declaration"}
	m := model{catalogView: view, width: 52, height: 24, cursor: 3}
	result, command := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	updated := result.(model)
	if command != nil || len(updated.catalogView.Decisions.Approvals) != 1 || updated.catalogView.Index != 1 {
		t.Fatal("specific review did not stage exactly one approval")
	}
	resized, _ := updated.Update(tea.WindowSizeMsg{Width: 140, Height: 24})
	if resized.(model).cursor != 3 || resized.(model).catalogView.Index != 1 {
		t.Fatal("resize moved staged selection")
	}
	result, command = updated.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if result.(model).catalogView != nil || command != nil {
		t.Fatal("cancel applied staged decisions")
	}
}

func TestCatalogPendingPickerCannotRun(t *testing.T) {
	m := model{aliases: []Alias{{Name: "pending", Command: "printf pending", CatalogID: strings.Repeat("1", 32), CatalogState: "pending"}}, width: 80, height: 24, executeMode: true, aliasMode: aliasModeSearch, shortcutProfile: shortcutLinux}
	result, command := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(model)
	if command != nil || updated.selected != nil || !strings.Contains(updated.status, "pending") {
		t.Fatal("pending catalog entry ran through picker")
	}
}

func TestCatalogDrawerPTYNarrowWideCancellation(t *testing.T) {
	for _, width := range []uint16{52, 140} {
		t.Run(strconv.Itoa(int(width)), func(t *testing.T) {
			command := exec.Command(os.Args[0], "-test.run=^TestCatalogDrawerPTYHelper$")
			command.Env = append(os.Environ(), "AL_CATALOG_DRAWER_PTY=1", "TERM=xterm-256color")
			terminal, err := pty.StartWithSize(command, &pty.Winsize{Rows: 24, Cols: width})
			if err != nil {
				t.Fatal(err)
			}
			defer terminal.Close()
			defer func() {
				if command.Process != nil {
					_ = command.Process.Kill()
				}
				_ = command.Wait()
			}()
			output := &synchronizedBuffer{}
			go func() { _, _ = io.Copy(output, terminal) }()
			wait := func(text string) {
				deadline := time.Now().Add(5 * time.Second)
				for time.Now().Before(deadline) {
					if strings.Contains(output.stringFrom(0), text) {
						return
					}
					time.Sleep(10 * time.Millisecond)
				}
				t.Fatalf("PTY missing %q: %s", text, output.stringFrom(0))
			}
			wait("Exact native review")
			if _, err := terminal.Write([]byte("y")); err != nil {
				t.Fatal(err)
			}
			wait("ownership review")
			if _, err := terminal.Write([]byte("\x1b")); err != nil {
				t.Fatal(err)
			}
			time.Sleep(100 * time.Millisecond)
			if _, err := terminal.Write([]byte("\x03")); err != nil {
				t.Fatal(err)
			}
			if err := command.Wait(); err != nil {
				t.Fatal(err)
			}
			wait("cancelled staged decisions")
		})
	}
}

func TestCatalogDrawerPTYHelper(t *testing.T) {
	if os.Getenv("AL_CATALOG_DRAWER_PTY") != "1" {
		return
	}
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	key := catalogstore.ApprovalKey{EntryID: strings.Repeat("1", 32), Shell: "bash", Name: "sample", Kind: "command", ImplementationSHA256: strings.Repeat("2", 64), Renderer: "bash/v2"}
	record := catalogstore.ApprovalRecord{Key: key, ApprovedAt: "2026-10-03T00:00:00Z"}
	view := &catalogTUIView{Shell: "bash", Stage: "review", Text: "Exact native review", Review: []catalogTUIReviewItem{{Text: "Exact native review", Approval: &record}, {Text: "Exact ownership review"}}}
	_, err := tea.NewProgram(model{catalogView: view, width: 80, height: 24, theme: builtInTheme("phosphor"), shortcutProfile: shortcutLinux}, tea.WithAltScreen()).Run()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(catalogApprovalsPath()); !os.IsNotExist(err) {
		t.Fatal("cancel saved private decisions")
	}
	println("cancelled staged decisions")
}

func TestCatalogConflictDrawerStagesFieldsAndAppliesCatalogOnly(t *testing.T) {
	repo, value := setupCatalogSyncFixture(t)
	value.Entries[0].Description = "remote-description"
	writeCatalogFixture(t, filepath.Join(repo, "catalog.json"), value)
	value.Entries[0].Description = "local-description"
	writeCatalogFixture(t, localCatalogPath(), value)
	preview, err := buildCatalogSyncPullPlan()
	if err != nil {
		t.Fatal(err)
	}
	if err := applyMutationPlan(preview, buildCatalogSyncPullPlan); err != nil {
		t.Fatal(err)
	}
	id := filepath.Base(filepath.Dir(preview.Actions[0].Target.Path))
	resolution, err := loadCatalogConflictResolution(id)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(localCatalogPath())
	if err != nil {
		t.Fatal(err)
	}
	view := &catalogTUIView{Shell: "bash", Stage: "conflict", Conflict: &resolution, Choices: map[string]string{}, Text: catalogConflictReviewText(resolution, 0)}
	m := model{catalogView: view, width: 80, height: 24}
	result, command := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = result.(model)
	if command == nil {
		t.Fatal("field choice did not build preview")
	}
	loaded := command().(catalogTUILoadedMsg)
	m.catalogView = loaded.View
	after, _ := os.ReadFile(localCatalogPath())
	if !bytes.Equal(before, after) {
		t.Fatal("field choice saved before confirmation")
	}
	if m.catalogView.Stage != "conflict-plan" || m.catalogView.Err != "" {
		t.Fatal(m.catalogView.Err)
	}
	result, command = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if command == nil {
		t.Fatal("final confirmation did not apply")
	}
	if message := command().(catalogTUIAppliedMsg); message.Err != nil {
		t.Fatal(message.Err)
	}
	updated, err := readCatalogFile(localCatalogPath())
	if err != nil {
		t.Fatal(err)
	}
	if updated.Entries[0].Description != "remote-description" {
		t.Fatal("chosen semantic field was not saved")
	}
	if _, err := os.Stat(catalogInstalledPath()); !os.IsNotExist(err) {
		t.Fatal("conflict resolution activated a shell")
	}
}

func TestCatalogDefaultBinaryPTYNarrowWideReviewCancellation(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "alias-lens")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build %v %s", err, output)
	}
	for _, width := range []uint16{52, 140} {
		t.Run(strconv.Itoa(int(width)), func(t *testing.T) {
			home := privateTestHome(t)
			t.Setenv("HOME", home)
			t.Setenv("ALIAS_LENS_SHELL", "bash")
			if err := saveConfig(defaultConfig()); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), []byte("alias ordinary='printf ordinary'\n"), 0600); err != nil {
				t.Fatal(err)
			}
			body := "printf reviewed"
			value := neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{{ID: strings.Repeat("3", 32), Name: "reviewed", Kind: "command", Native: map[string]neutralcatalog.NativeImplementation{"bash": {AliasValue: &body}}}}}
			writeCatalogFixture(t, localCatalogPath(), value)
			before, _ := os.ReadFile(localCatalogPath())
			command := exec.Command(binary)
			command.Env = append(os.Environ(), "TERM=xterm-256color", "NO_COLOR=1")
			terminal, err := pty.StartWithSize(command, &pty.Winsize{Rows: 24, Cols: width})
			if err != nil {
				t.Fatal(err)
			}
			defer terminal.Close()
			defer func() { _ = command.Process.Kill(); _ = command.Wait() }()
			output := &synchronizedBuffer{}
			go func() { _, _ = io.Copy(output, terminal) }()
			wait := func(text string) {
				t.Helper()
				deadline := time.Now().Add(5 * time.Second)
				for time.Now().Before(deadline) {
					if strings.Contains(output.stringFrom(0), text) {
						return
					}
					time.Sleep(10 * time.Millisecond)
				}
				t.Fatalf("missing %q: %s", text, output.stringFrom(0))
			}
			wait("ordinary")
			_, _ = terminal.Write([]byte("l"))
			wait("atalog for Bash")
			_, _ = terminal.Write([]byte("e"))
			wait("Exact declaration")
			_, _ = terminal.Write([]byte("\x1b"))
			time.Sleep(100 * time.Millisecond)
			_, _ = terminal.Write([]byte("\x03"))
			if err := command.Wait(); err != nil {
				t.Fatal(err)
			}
			after, _ := os.ReadFile(localCatalogPath())
			if !bytes.Equal(before, after) {
				t.Fatal("cancellation changed catalog")
			}
			if _, err := os.Stat(catalogApprovalsPath()); !os.IsNotExist(err) {
				t.Fatal("cancellation saved approvals")
			}
		})
	}
}
