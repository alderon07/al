package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	tea "alias-lens/cmd/alias-lens/internal/tea"
)

type ShortcutProfile string

const (
	shortcutWindows ShortcutProfile = "windows"
	shortcutLinux   ShortcutProfile = "linux"
	shortcutMacOS   ShortcutProfile = "macos"
)

type shortcutAction int

const (
	shortcutHelp shortcutAction = iota
	shortcutStats
	shortcutSettings
	shortcutThemes
	shortcutRevisions
	shortcutSync
	shortcutHealth
	shortcutAdd
	shortcutEdit
	shortcutRefresh
	shortcutSave
)

func parseShortcutProfile(value string) (ShortcutProfile, error) {
	switch profile := ShortcutProfile(strings.ToLower(strings.TrimSpace(value))); profile {
	case shortcutWindows, shortcutLinux, shortcutMacOS:
		return profile, nil
	default:
		return "", fmt.Errorf("shortcut style must be windows, linux, or macos")
	}
}

func resolvedShortcutProfile(config AppConfig) ShortcutProfile {
	if config.ShortcutProfile != "" {
		if profile, err := parseShortcutProfile(config.ShortcutProfile); err == nil {
			return profile
		}
	}
	return defaultShortcutProfile()
}

func defaultShortcutProfile() ShortcutProfile {
	return detectShortcutProfile(runtime.GOOS, os.Getenv, os.ReadFile)
}

func detectShortcutProfile(goos string, getenv func(string) string, readFile func(string) ([]byte, error)) ShortcutProfile {
	switch goos {
	case "windows":
		return shortcutWindows
	case "darwin":
		return shortcutMacOS
	case "linux":
		if getenv("WSL_INTEROP") != "" || getenv("WSL_DISTRO_NAME") != "" {
			return shortcutWindows
		}
		for _, path := range []string{"/proc/sys/kernel/osrelease", "/proc/version"} {
			contents, err := readFile(path)
			if err == nil && strings.Contains(strings.ToLower(string(contents)), "microsoft") {
				return shortcutWindows
			}
		}
		return shortcutLinux
	default:
		return shortcutLinux
	}
}

func shortcutProfileLabel(profile ShortcutProfile) string {
	switch profile {
	case shortcutWindows:
		return "Windows"
	case shortcutMacOS:
		return "macOS"
	default:
		return "Linux"
	}
}

func runShortcutsCommand(arguments []string) error {
	config, err := loadConfig()
	if err != nil {
		return err
	}
	if len(arguments) > 1 {
		return errors.New("usage: al shortcuts [windows|linux|macos|test]")
	}
	if len(arguments) == 1 && arguments[0] != "test" {
		profile, err := parseShortcutProfile(arguments[0])
		if err != nil {
			return err
		}
		config.ShortcutProfile = string(profile)
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
	fmt.Printf("Shortcut style: %s (%s)\n", shortcutProfileLabel(profile), source)
	fmt.Println()
	for _, row := range shortcutGuide(profile, false) {
		fmt.Printf("  %-22s %s\n", row[0], row[1])
	}
	fmt.Println()
	if profile == shortcutMacOS {
		fmt.Println("If your terminal keeps a Command shortcut, use the Control shortcut shown beside it.")
	}
	if profile == shortcutMacOS {
		fmt.Println("Your terminal normally uses Cmd+C to copy and Cmd+V to paste.")
	} else {
		fmt.Println("Your terminal may use Ctrl+Shift+C to copy and Ctrl+Shift+V to paste.")
	}
	if len(arguments) == 1 {
		fmt.Println("Your shortcut choice was not changed.")
	} else {
		fmt.Println("Change the style with: al shortcuts windows|linux|macos")
	}
	return nil
}

func shortcutGuide(profile ShortcutProfile, selectMode bool) [][2]string {
	enterAction := "Run the selected alias"
	if selectMode {
		enterAction = "Select without running"
	}
	return [][2]string{
		{"Type", "Search names, commands, and descriptions"},
		{"↑↓ / PgUp PgDn", "Move through results"},
		{"Enter", enterAction},
		{"Tab", "Return the alias to the prompt for editing"},
		{shortcutLabel(profile, shortcutAdd), "Add an alias"},
		{shortcutLabel(profile, shortcutEdit), "Edit description, command, or name"},
		{"Ctrl+D", "Delete an alias after confirmation"},
		{shortcutLabel(profile, shortcutRevisions), "Browse and restore saved versions"},
		{shortcutLabel(profile, shortcutStats), "Open alias usage stats"},
		{shortcutLabel(profile, shortcutSettings), "Customize the TUI footer"},
		{shortcutLabel(profile, shortcutHealth), "Show aliases that need attention"},
		{shortcutLabel(profile, shortcutSync), "Open sync status and tracked files"},
		{"Ctrl+G", "Save alias changes to the local repository"},
		{shortcutLabel(profile, shortcutThemes), "Choose a theme with live preview"},
		{shortcutLabel(profile, shortcutRefresh), "Reload aliases and theme settings"},
		{"? / Esc", "Close this guide"},
	}
}

func shortcutLabel(profile ShortcutProfile, action shortcutAction) string {
	if profile == shortcutMacOS {
		switch action {
		case shortcutHelp:
			return "Cmd+? / ?"
		case shortcutStats:
			return "Cmd+2 / F2"
		case shortcutSettings:
			return "Cmd+, / F3"
		case shortcutThemes:
			return "Cmd+T / Ctrl+T"
		case shortcutRevisions:
			return "Cmd+Z / Ctrl+Z"
		case shortcutSync:
			return "Cmd+Shift+S / Ctrl+F"
		case shortcutHealth:
			return "Cmd+H / Ctrl+H"
		case shortcutAdd:
			return "Cmd+N / Ctrl+A"
		case shortcutEdit:
			return "Cmd+E / Ctrl+E"
		case shortcutRefresh:
			return "Cmd+R / Ctrl+R"
		case shortcutSave:
			return "Cmd+S / Ctrl+S"
		}
	}
	switch action {
	case shortcutHelp:
		return "?"
	case shortcutStats:
		return "F2 / Ctrl+S"
	case shortcutSettings:
		return "F3"
	case shortcutThemes:
		return "Ctrl+T"
	case shortcutRevisions:
		return "Ctrl+Z"
	case shortcutSync:
		return "Ctrl+F"
	case shortcutHealth:
		return "Ctrl+H"
	case shortcutAdd:
		return "Ctrl+A"
	case shortcutEdit:
		return "Ctrl+E"
	case shortcutRefresh:
		return "Ctrl+R"
	case shortcutSave:
		return "Ctrl+S"
	default:
		return ""
	}
}

func matchesShortcut(message tea.KeyMsg, profile ShortcutProfile, action shortcutAction) bool {
	if action == shortcutHelp {
		return message.Type == tea.KeyF1 || keyRune(message, '?', false)
	}
	if action == shortcutStats && message.Type == tea.KeyF2 {
		return true
	}
	if action == shortcutSettings && message.Type == tea.KeyF3 {
		return true
	}
	if profile == shortcutMacOS && matchesMacShortcut(message, action) {
		return true
	}
	switch action {
	case shortcutStats, shortcutSave:
		return message.Type == tea.KeyCtrlS
	case shortcutThemes:
		return message.Type == tea.KeyCtrlT
	case shortcutRevisions:
		return message.Type == tea.KeyCtrlZ
	case shortcutSync:
		return message.Type == tea.KeyCtrlF
	case shortcutHealth:
		return message.Type == tea.KeyCtrlH
	case shortcutAdd:
		return message.Type == tea.KeyCtrlA
	case shortcutEdit:
		return message.Type == tea.KeyCtrlE
	case shortcutRefresh:
		return message.Type == tea.KeyCtrlR
	default:
		return false
	}
}

func matchesMacShortcut(message tea.KeyMsg, action shortcutAction) bool {
	if !message.Super {
		return false
	}
	switch action {
	case shortcutHelp:
		return keyRune(message, '?', true)
	case shortcutStats:
		return keyRune(message, '2', true)
	case shortcutThemes:
		return keyRune(message, 't', true)
	case shortcutRevisions:
		return keyRune(message, 'z', true)
	case shortcutSync:
		return message.Shift && keyRune(message, 's', true)
	case shortcutHealth:
		return keyRune(message, 'h', true)
	case shortcutAdd:
		return keyRune(message, 'n', true)
	case shortcutEdit:
		return keyRune(message, 'e', true)
	case shortcutRefresh:
		return keyRune(message, 'r', true)
	case shortcutSettings:
		return keyRune(message, ',', true)
	case shortcutSave:
		return keyRune(message, 's', true)
	default:
		return false
	}
}

func keyRune(message tea.KeyMsg, expected rune, requireSuper bool) bool {
	return message.Type == tea.KeyRunes && len(message.Runes) == 1 &&
		unicodeLower(message.Runes[0]) == unicodeLower(expected) && (!requireSuper || message.Super)
}

func unicodeLower(value rune) rune {
	if value >= 'A' && value <= 'Z' {
		return value + ('a' - 'A')
	}
	return value
}
