package app

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestTrackedSourceInvalidatedSourceCanBeUntracked(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("trusted observation is unavailable")
	}
	for _, kind := range []string{"file-mode", "parent-mode", "hardlink", "credential-link"} {
		t.Run(kind, func(t *testing.T) {
			home := privateTestHome(t)
			t.Setenv("HOME", home)
			parent := filepath.Join(home, "notes-directory")
			if err := os.Mkdir(parent, 0o700); err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(parent, "notes")
			if err := os.WriteFile(source, []byte("ordinary data"), 0o600); err != nil {
				t.Fatal(err)
			}
			settings := DefaultServices().internalSettings()
			if err := settings.Track(source, nil); err != nil {
				t.Fatal(err)
			}
			var err error
			switch kind {
			case "file-mode":
				err = os.Chmod(source, 0o666)
			case "parent-mode":
				err = os.Chmod(parent, 0o777)
			case "hardlink":
				err = os.Link(source, filepath.Join(home, "copy"))
			case "credential-link":
				if err := os.Remove(source); err != nil {
					t.Fatal(err)
				}
				err = os.Symlink(filepath.Join(home, ".env"), source)
			}
			if err != nil {
				t.Fatal(err)
			}
			if observed, err := settings.observeTrackedSource(source); err == nil || len(observed.contents) != 0 {
				t.Fatal("invalidated source returned bytes")
			}
			if _, err := settings.loadConfig(); err != nil {
				t.Fatalf("invalidated source blocked ordinary settings: %v", err)
			}
			if err := settings.Untrack(source); err != nil {
				t.Fatalf("invalidated source could not be removed: %v", err)
			}
			config, err := settings.loadConfig()
			if err != nil || len(config.TrackedFiles) != 0 {
				t.Fatalf("untrack did not persist removal: %v", err)
			}
		})
	}
}
