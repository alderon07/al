package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecutableWatchDetectsAtomicReplacement(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "alias-lens")
	if err := os.WriteFile(path, []byte("first build"), 0o700); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	watch := executableWatch{path: path, info: info}
	if watch.changed() {
		t.Fatal("unchanged executable was reported as replaced")
	}
	replacement := filepath.Join(directory, "replacement")
	if err := os.WriteFile(replacement, []byte("second build"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	if !watch.changed() {
		t.Fatal("replaced executable was reported as unchanged")
	}
}

func TestTUIKeepsExecutableUpdateNoticeVisible(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "alias-lens")
	if err := os.WriteFile(path, []byte("first build"), 0o700); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	m := model{
		aliases:         []Alias{{Name: "gs", Command: "git status"}},
		width:           90,
		height:          24,
		executeMode:     true,
		executableWatch: executableWatch{path: path, info: info},
	}
	updated, _ := m.Update(cursorBlinkMsg{})
	if result := updated.(model); result.executableUpdated {
		t.Fatal("unchanged executable showed an update notice")
	}
	replacement := filepath.Join(directory, "replacement")
	if err := os.WriteFile(replacement, []byte("second build"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	updated, _ = m.Update(cursorBlinkMsg{})
	result := updated.(model)
	if !result.executableUpdated {
		t.Fatal("replaced executable did not set the TUI update state")
	}
	for _, size := range []struct{ width, height int }{{48, 18}, {160, 40}} {
		result.width, result.height = size.width, size.height
		view := result.View()
		for _, expected := range []string{"Alias Lens was updated.", "enter al again."} {
			if !strings.Contains(view, expected) {
				t.Fatalf("%dx%d TUI update notice is missing %q:\n%s", size.width, size.height, expected, view)
			}
		}
	}
	result.status = "another status"
	if view := result.View(); !strings.Contains(view, "Alias Lens was updated. Close this screen, then enter al again.") {
		t.Fatal("another status hid the executable update notice")
	}
}
