package catalogstore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateDarwinSystemParents(t *testing.T) {
	for _, parent := range []string{"/tmp", "/var/tmp"} {
		t.Run(parent, func(t *testing.T) {
			root, err := os.MkdirTemp(parent, "alias-lens-test-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.RemoveAll(root) })
			path := filepath.Join(root, "state")
			if err := os.WriteFile(path, []byte("synthetic"), 0o600); err != nil {
				t.Fatal(err)
			}
			if data, err := ReadPrivateBytes(path, 1024); err != nil || string(data) != "synthetic" {
				t.Fatalf("read: %q %v", data, err)
			}
		})
	}
}
