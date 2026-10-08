package tui

import (
	"os"

	"testing"
)

func privateTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if e := os.Chmod(home, 0o700); e != nil {
		t.Fatal(e)
	}
	return home
}
