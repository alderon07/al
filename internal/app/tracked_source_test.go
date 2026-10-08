package app

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestTrackedSourceOwnedSymlinksAndMissing(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("trusted observation is unavailable")
	}
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	source := filepath.Join(home, "notes")
	if err := os.WriteFile(source, []byte("ordinary data"), 0o640); err != nil {
		t.Fatal(err)
	}
	settings := DefaultServices().internalSettings()
	for _, target := range []string{"notes", source} {
		link := filepath.Join(home, "enrolled")
		os.Remove(link)
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		observed, err := settings.observeTrackedSource(link)
		if err != nil || string(observed.contents) != "ordinary data" {
			t.Fatalf("owned link observation failed: %v", err)
		}
		if err := settings.verifyTrackedSource(link, observed); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := settings.observeTrackedSource(filepath.Join(home, "absent")); !os.IsNotExist(err) {
		t.Fatalf("missing source lost its classification: %v", err)
	}
	if _, err := settings.observeTrackedSource(filepath.Join(home, "absent-parent", "absent")); !os.IsNotExist(err) {
		t.Fatalf("missing parent lost its classification: %v", err)
	}
	parentLink := filepath.Join(home, "linked-directory")
	if err := os.Symlink(home, parentLink); err != nil {
		t.Fatal(err)
	}
	observed, err := settings.observeTrackedSource(filepath.Join(parentLink, "notes"))
	if err != nil || string(observed.contents) != "ordinary data" {
		t.Fatalf("owned parent symlink failed: %v", err)
	}
	if err := os.Remove(parentLink); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(home, ".ssh"), parentLink); err != nil {
		t.Fatal(err)
	}
	if _, err := settings.observeTrackedSource(filepath.Join(parentLink, "notes")); err == nil || os.IsNotExist(err) {
		t.Fatalf("credential parent substitution was not refused: %v", err)
	}
}

func TestTrackedSourceRejectsUnsafeReferentsAndParents(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("trusted observation is unavailable")
	}
	for _, kind := range []string{"credential-link", "writable-parent", "hard-link", "private-config"} {
		t.Run(kind, func(t *testing.T) {
			home := privateTestHome(t)
			t.Setenv("HOME", home)
			settings := DefaultServices().internalSettings()
			source := filepath.Join(home, "enrolled")
			target := filepath.Join(home, "notes")
			switch kind {
			case "credential-link":
				target = filepath.Join(home, ".env")
			case "private-config":
				var err error
				target, err = settings.configPath()
				if err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
					t.Fatal(err)
				}
			case "writable-parent":
				parent := filepath.Join(home, "shared")
				if err := os.Mkdir(parent, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(parent, 0o777); err != nil {
					t.Fatal(err)
				}
				source = filepath.Join(parent, "enrolled")
			}
			if err := os.WriteFile(target, []byte("synthetic sentinel"), 0o600); err != nil {
				t.Fatal(err)
			}
			var err error
			if kind == "hard-link" {
				err = os.Link(target, source)
			} else {
				err = os.Symlink(target, source)
			}
			if err != nil {
				t.Fatal(err)
			}
			observed, err := settings.observeTrackedSource(source)
			if err == nil || len(observed.contents) != 0 {
				t.Fatal("unsafe source returned bytes")
			}
		})
	}
}

func TestTrackedSourceSubstitutionBetweenValidationAndOpen(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("trusted observation is unavailable")
	}
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	source := filepath.Join(home, "enrolled")
	unsafe := filepath.Join(home, ".env")
	for _, path := range []string{source, unsafe} {
		if err := os.WriteFile(path, []byte("synthetic sentinel"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	settings := DefaultServices().internalSettings()
	checks := 0
	file, _, err := openTrustedTrackedSource(source, func(path string) error {
		if path == source {
			checks++
			if checks == 2 {
				if err := os.Remove(source); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(unsafe, source); err != nil {
					t.Fatal(err)
				}
			}
		}
		return settings.validateTrackedSourcePath(path)
	})
	if file != nil {
		file.Close()
	}
	if err == nil || file != nil {
		t.Fatal("substituted source was opened")
	}
}

func TestTrackedSourcePublicationRejectsChangedIdentity(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("trusted observation is unavailable")
	}
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	source := filepath.Join(home, "enrolled")
	if err := os.WriteFile(source, []byte("ordinary data"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := DefaultServices()
	observed, err := svc.internalSettings().observeTrackedSource(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(source, source+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, observed.contents, 0o600); err != nil {
		t.Fatal(err)
	}
	tracked := TrackedFileConfig{Source: source, RepositoryPath: "notes"}
	repository := filepath.Join(home, "repository")
	err = svc.withMutation(func(session *mutationSession) error {
		return svc.pushObservedTrackedFileInSession(session, AppConfig{Repository: repository}, tracked, observed, filepath.Join(home, "state"))
	})
	if err == nil {
		t.Fatal("changed source identity was published")
	}
	if _, err := os.Stat(repository); !os.IsNotExist(err) {
		t.Fatal("repository modified before identity rejection")
	}
}

func TestTrackedSourcePublicationRejectsUnobservedBytes(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("trusted observation is unavailable")
	}
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	source := filepath.Join(home, "enrolled")
	if err := os.WriteFile(source, []byte("ordinary data"), 0o600); err != nil {
		t.Fatal(err)
	}
	repository := filepath.Join(home, "repository")
	tracked := TrackedFileConfig{Source: source, RepositoryPath: "notes"}
	err := DefaultServices().pushTrackedFile(AppConfig{Repository: repository}, tracked, []byte("synthetic sentinel"), filepath.Join(home, "state"))
	if err == nil {
		t.Fatal("unobserved bytes were published")
	}
	if _, err := os.Stat(repository); !os.IsNotExist(err) {
		t.Fatal("repository modified before rejection")
	}
}

func TestTrackedSourceMissingFilesWait(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("trusted observation is unavailable")
	}
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	repository := filepath.Join(home, "repository")
	if err := os.Mkdir(repository, 0o700); err != nil {
		t.Fatal(err)
	}
	tracked := TrackedFileConfig{Source: filepath.Join(home, "absent", "notes"), RepositoryPath: "notes"}
	svc := DefaultServices()
	if err := svc.reconcileTrackedFile(AppConfig{Repository: repository}, tracked); err != nil {
		t.Fatal(err)
	}
	statePath, err := svc.trackedStatePath(tracked)
	if err != nil {
		t.Fatal(err)
	}
	state, err := loadSyncStateAt(statePath)
	if err != nil || state.Status != "waiting" {
		t.Fatalf("missing sources should wait: %v, %s", err, state.Status)
	}
}
