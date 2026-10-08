//go:build !windows

package main

import (
	"os/exec"
	"testing"
)

func pinInitTestValidator(t *testing.T) {
	t.Helper()
	original := applicationDependencies.FindTrustedShell
	applicationDependencies.FindTrustedShell = func(shell string) (string, error) { return exec.LookPath(shell) }
	t.Cleanup(func() { applicationDependencies.FindTrustedShell = original })
}
