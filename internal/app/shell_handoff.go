package app

import "github.com/alderon07/al/internal/shell"

func guardedShellIntegration(text string) string { return shell.GuardedIntegration(text) }
func legacyShellEntryHandoff(adapter ShellAdapter, entry Alias) (string, error) {
	return shell.ShellEntryHandoff(adapter, entry)
}
