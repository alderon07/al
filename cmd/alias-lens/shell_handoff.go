package main

import "alias-lens/internal/shell"

func guardedShellIntegration(text string) string { return shell.GuardedIntegration(text) }
func legacyShellEntryHandoff(adapter ShellAdapter, entry Alias) (string, error) {
	return shell.ShellEntryHandoff(adapter, entry)
}
