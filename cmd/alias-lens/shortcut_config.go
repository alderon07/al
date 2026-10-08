package main

import "github.com/alderon07/al/internal/shortcuts"

func shortcutCompletionActions() []string { return shortcuts.CompletionActions() }
func validateShortcutOverrides(config AppConfig) error {
	return shortcuts.ValidateOverrides(config.ShortcutProfile, config.Shortcuts)
}
func parseLauncherKey(label string) (string, error) { return shortcuts.ParseLauncherKey(label) }
func launcherLabel(config AppConfig) string         { return shortcuts.LauncherLabel(config.Shortcuts) }
