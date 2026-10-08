package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestTrackedDarwinNativeACLBoundary(t *testing.T) {
	for _, target := range []string{"file", "parent"} {
		t.Run(target, func(t *testing.T) {
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
			if _, err := settings.observeTrackedSource(source); err != nil {
				t.Fatalf("ordinary native source refused: %v", err)
			}
			path, rights := source, "write,append"
			if target == "parent" {
				path, rights = parent, "add_file,delete_child"
			}
			if output, err := exec.Command("/bin/chmod", "+a", "everyone allow "+rights, path).CombinedOutput(); err != nil {
				t.Fatalf("add synthetic ACL: %v %s", err, output)
			}
			if observed, err := settings.observeTrackedSource(source); err == nil || len(observed.contents) != 0 {
				t.Fatal("write granting native ACL returned bytes")
			}
			if output, err := exec.Command("/bin/chmod", "-N", path).CombinedOutput(); err != nil {
				t.Fatalf("remove synthetic ACL: %v %s", err, output)
			}
			if output, err := exec.Command("/bin/chmod", "+a", "everyone deny delete", path).CombinedOutput(); err != nil {
				t.Fatalf("add deny ACL: %v %s", err, output)
			}
			if _, err := settings.observeTrackedSource(source); err != nil {
				t.Fatalf("ordinary deny ACL refused: %v", err)
			}
		})
	}
}
