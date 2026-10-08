package main

import (
	"alias-lens/internal/shortcuts"
	tea "alias-lens/internal/tea"
	"errors"
	"fmt"
)

type ShortcutProfile = shortcuts.Profile

const shortcutAdd = shortcuts.Add

var shortcutMacOS, _ = shortcuts.ParseProfile("macos")

func runShortcutsCommand(arguments []string) error {
	config, err := loadConfig()
	if err != nil {
		return err
	}
	if len(arguments) > 0 && (arguments[0] == "set" || arguments[0] == "reset" || arguments[0] == "reset-all") {
		switch arguments[0] {
		case "set":
			if len(arguments) != 3 {
				return errors.New("usage: al shortcuts set ACTION KEY")
			}
			if config.Shortcuts == nil {
				config.Shortcuts = make(map[string]string)
			}
			config.Shortcuts[arguments[1]] = arguments[2]
		case "reset":
			if len(arguments) != 2 {
				return errors.New("usage: al shortcuts reset ACTION")
			}
			if arguments[1] != "launcher" {
				known := false
				for _, descriptor := range shortcuts.Descriptors() {
					name := descriptor.Name
					known = known || name == arguments[1]
				}
				if !known {
					return fmt.Errorf("unknown action %q; run al shortcuts", arguments[1])
				}
			}
			delete(config.Shortcuts, arguments[1])
		case "reset-all":
			if len(arguments) != 1 {
				return errors.New("usage: al shortcuts reset-all")
			}
			config.Shortcuts = nil
		}
		if err := saveConfig(config); err != nil {
			return err
		}
		fmt.Println("Shortcuts saved. Open Alias Lens again; reload your shell integration for a launcher change.")
		return nil
	}
	if len(arguments) > 1 {
		return errors.New("usage: al shortcuts [auto|windows|linux|macos|test|set ACTION KEY|reset ACTION|reset-all]")
	}
	if len(arguments) == 1 && arguments[0] != "test" {
		if arguments[0] == "auto" {
			config.ShortcutProfile = ""
			if err := saveConfig(config); err != nil {
				return err
			}
			profile := defaultShortcutProfile()
			fmt.Printf("Shortcut style changed to %s, chosen automatically for this computer. Open Alias Lens again to use it.\n", shortcutProfileLabel(profile))
			return nil
		}
		profile, err := parseShortcutProfile(arguments[0])
		if err != nil {
			return err
		}
		config.ShortcutProfile = profile.String()
		if err := saveConfig(config); err != nil {
			return err
		}
		fmt.Printf("Shortcut style changed to %s. Open Alias Lens again to use it.\n", shortcutProfileLabel(profile))
		return nil
	}

	profile := resolvedShortcutProfile(config)
	source := "chosen automatically for this computer"
	if config.ShortcutProfile != "" {
		source = "your saved choice"
	}
	fmt.Printf("%s: %s (%s)\n", cliAccent("Shortcut style"), shortcutProfileLabel(profile), source)
	fmt.Println()
	for _, row := range shortcutGuide(profile, false) {
		fmt.Printf("  %s %s\n", cliAccent(fmt.Sprintf("%-22s", row[0])), row[1])
	}
	fmt.Printf("  %-22s %s\n", launcherLabel(config), "Launch from the shell prompt (launcher)")
	fmt.Println("\nConfigurable actions:")
	for _, definition := range shortcuts.Descriptors() {
		fmt.Printf("  %-12s %s\n", shortcutActionName(definition.Action), shortcutLabel(profile, definition.Action))
	}
	fmt.Println()
	if profile.String() == shortcutMacOS.String() {
		fmt.Println("If your terminal keeps a Command shortcut, use the terminal-safe fallback shown beside it.")
	}
	if profile.String() == shortcutMacOS.String() {
		fmt.Println("Your terminal normally uses Cmd+C to copy and Cmd+V to paste.")
	} else {
		fmt.Println("Your terminal may use Ctrl+Shift+C to copy and Ctrl+Shift+V to paste.")
	}
	fmt.Println("F1 through F8 remain available in every shortcut style.")
	if len(arguments) == 1 {
		fmt.Println("Your shortcut choice was not changed.")
	} else if config.ShortcutProfile != "" {
		fmt.Println("Restore automatic selection with: al shortcuts auto")
	} else {
		fmt.Println("Change the style with: al shortcuts windows|linux|macos")
	}
	return nil
}

func parseShortcutProfile(value string) (ShortcutProfile, error) {
	return shortcuts.ParseProfile(value)
}
func defaultShortcutProfile() ShortcutProfile { return shortcuts.DefaultProfile() }
func resolvedShortcutProfile(config AppConfig) ShortcutProfile {
	return shortcuts.ResolveProfile(config.ShortcutProfile, config.Shortcuts)
}
func shortcutProfileLabel(profile ShortcutProfile) string { return shortcuts.ProfileLabel(profile) }
func shortcutGuide(profile ShortcutProfile, selectMode bool) [][2]string {
	return shortcuts.Guide(profile, selectMode)
}
func shortcutActionName(action shortcuts.Action) string { return shortcuts.ActionName(action) }
func shortcutLabel(profile ShortcutProfile, action shortcuts.Action) string {
	return shortcuts.Label(profile, action)
}
func matchesShortcut(message tea.KeyMsg, profile ShortcutProfile, action shortcuts.Action) bool {
	return shortcuts.Matches(message, profile, action)
}
