//go:build !windows

package app

import (
	"strings"
	"testing"
	"time"
)

func waitForPTYText(t *testing.T, output *synchronizedBuffer, offset int, expected string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		contents := output.stringFrom(offset)
		if strings.Contains(contents, expected) {
			return contents
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("PTY redraw did not contain %q:\n%q", expected, output.stringFrom(offset))
	return ""
}
