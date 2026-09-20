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

type shortcutKey struct {
	typeCode   tea.KeyType
	runeCode   rune
	super      bool
	shift      bool
	allowShift bool
}

type shortcutChoice struct {
	label string
	keys  []shortcutKey
}

type shortcutDefinition struct {
	action  shortcutAction
	windows shortcutChoice
	linux   shortcutChoice
	macos   shortcutChoice
}

var shortcutDefinitions = []shortcutDefinition{
	{action: shortcutHelp,
		windows: shortcutChoice{label: "F1 / ?", keys: []shortcutKey{{typeCode: tea.KeyF1}, {typeCode: tea.KeyRunes, runeCode: '?', allowShift: true}}},
		linux:   shortcutChoice{label: "F1 / ?", keys: []shortcutKey{{typeCode: tea.KeyF1}, {typeCode: tea.KeyRunes, runeCode: '?', allowShift: true}}},
		macos:   shortcutChoice{label: "Cmd+? / F1 / ?", keys: []shortcutKey{{typeCode: tea.KeyRunes, runeCode: '?', super: true, allowShift: true}, {typeCode: tea.KeyF1}, {typeCode: tea.KeyRunes, runeCode: '?', allowShift: true}}}},
	{action: shortcutStats,
		windows: shortcutChoice{label: "F2 / Ctrl+S", keys: []shortcutKey{{typeCode: tea.KeyF2}, {typeCode: tea.KeyCtrlS}}},
		linux:   shortcutChoice{label: "F2 / Ctrl+S", keys: []shortcutKey{{typeCode: tea.KeyF2}, {typeCode: tea.KeyCtrlS}}},
		macos:   shortcutChoice{label: "Cmd+2 / F2", keys: []shortcutKey{{typeCode: tea.KeyRunes, runeCode: '2', super: true}, {typeCode: tea.KeyF2}}}},
	{action: shortcutSettings,
		windows: shortcutChoice{label: "F3", keys: []shortcutKey{{typeCode: tea.KeyF3}}},
		linux:   shortcutChoice{label: "F3", keys: []shortcutKey{{typeCode: tea.KeyF3}}},
		macos:   shortcutChoice{label: "Cmd+, / F3", keys: []shortcutKey{{typeCode: tea.KeyRunes, runeCode: ',', super: true}, {typeCode: tea.KeyF3}}}},
	{action: shortcutThemes,
		windows: shortcutChoice{label: "Ctrl+T", keys: []shortcutKey{{typeCode: tea.KeyCtrlT}}},
		linux:   shortcutChoice{label: "Ctrl+T", keys: []shortcutKey{{typeCode: tea.KeyCtrlT}}},
		macos:   shortcutChoice{label: "Cmd+T / Ctrl+T", keys: []shortcutKey{{typeCode: tea.KeyRunes, runeCode: 't', super: true}, {typeCode: tea.KeyCtrlT}}}},
	{action: shortcutRevisions,
		windows: shortcutChoice{label: "Ctrl+Z", keys: []shortcutKey{{typeCode: tea.KeyCtrlZ}}},
		linux:   shortcutChoice{label: "Ctrl+Z", keys: []shortcutKey{{typeCode: tea.KeyCtrlZ}}},
		macos:   shortcutChoice{label: "Cmd+Z / Ctrl+Z", keys: []shortcutKey{{typeCode: tea.KeyRunes, runeCode: 'z', super: true}, {typeCode: tea.KeyCtrlZ}}}},
	{action: shortcutSync,
		windows: shortcutChoice{label: "Ctrl+F", keys: []shortcutKey{{typeCode: tea.KeyCtrlF}}},
		linux:   shortcutChoice{label: "Ctrl+F", keys: []shortcutKey{{typeCode: tea.KeyCtrlF}}},
		macos:   shortcutChoice{label: "Cmd+Shift+S / Ctrl+F", keys: []shortcutKey{{typeCode: tea.KeyRunes, runeCode: 's', super: true, shift: true}, {typeCode: tea.KeyCtrlF}}}},
	{action: shortcutHealth,
		windows: shortcutChoice{label: "Ctrl+H", keys: []shortcutKey{{typeCode: tea.KeyCtrlH}}},
		linux:   shortcutChoice{label: "Ctrl+H", keys: []shortcutKey{{typeCode: tea.KeyCtrlH}}},
		macos:   shortcutChoice{label: "Cmd+H / Ctrl+H", keys: []shortcutKey{{typeCode: tea.KeyRunes, runeCode: 'h', super: true}, {typeCode: tea.KeyCtrlH}}}},
	{action: shortcutAdd,
		windows: shortcutChoice{label: "Ctrl+A", keys: []shortcutKey{{typeCode: tea.KeyCtrlA}}},
		linux:   shortcutChoice{label: "Ctrl+A", keys: []shortcutKey{{typeCode: tea.KeyCtrlA}}},
		macos:   shortcutChoice{label: "Cmd+N / Ctrl+A", keys: []shortcutKey{{typeCode: tea.KeyRunes, runeCode: 'n', super: true}, {typeCode: tea.KeyCtrlA}}}},
	{action: shortcutEdit,
		windows: shortcutChoice{label: "Ctrl+E", keys: []shortcutKey{{typeCode: tea.KeyCtrlE}}},
		linux:   shortcutChoice{label: "Ctrl+E", keys: []shortcutKey{{typeCode: tea.KeyCtrlE}}},
		macos:   shortcutChoice{label: "Cmd+E / Ctrl+E", keys: []shortcutKey{{typeCode: tea.KeyRunes, runeCode: 'e', super: true}, {typeCode: tea.KeyCtrlE}}}},
	{action: shortcutRefresh,
		windows: shortcutChoice{label: "Ctrl+R", keys: []shortcutKey{{typeCode: tea.KeyCtrlR}}},
		linux:   shortcutChoice{label: "Ctrl+R", keys: []shortcutKey{{typeCode: tea.KeyCtrlR}}},
		macos:   shortcutChoice{label: "Cmd+R / Ctrl+R", keys: []shortcutKey{{typeCode: tea.KeyRunes, runeCode: 'r', super: true}, {typeCode: tea.KeyCtrlR}}}},
	{action: shortcutSave,
		windows: shortcutChoice{label: "Ctrl+S", keys: []shortcutKey{{typeCode: tea.KeyCtrlS}}},
		linux:   shortcutChoice{label: "Ctrl+S", keys: []shortcutKey{{typeCode: tea.KeyCtrlS}}},
		macos:   shortcutChoice{label: "Cmd+S / Ctrl+S", keys: []shortcutKey{{typeCode: tea.KeyRunes, runeCode: 's', super: true}, {typeCode: tea.KeyCtrlS}}}},
}

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
		return errors.New("usage: al shortcuts [auto|windows|linux|macos|test]")
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
		fmt.Println("If your terminal keeps a Command shortcut, use the terminal-safe fallback shown beside it.")
	}
	if profile == shortcutMacOS {
		fmt.Println("Your terminal normally uses Cmd+C to copy and Cmd+V to paste.")
	} else {
		fmt.Println("Your terminal may use Ctrl+Shift+C to copy and Ctrl+Shift+V to paste.")
	}
	if len(arguments) == 1 {
		fmt.Println("Your shortcut choice was not changed.")
	} else if config.ShortcutProfile != "" {
		fmt.Println("Restore automatic selection with: al shortcuts auto")
	} else {
		fmt.Println("Change the style with: al shortcuts windows|linux|macos")
	}
	return nil
}

func shortcutGuide(profile ShortcutProfile, selectMode bool) [][2]string {
	enterAction := "Use the selected alias"
	if selectMode {
		enterAction = "Select without using it"
	}
	guide := [][2]string{
		{"Type", "Search names, commands, and descriptions"},
		{"↑↓ / PgUp PgDn", "Move through results"},
		{"Enter", enterAction},
		{"Tab", "Return the alias to the prompt for editing"},
	}
	if selectMode {
		return append(guide, [2]string{shortcutLabel(profile, shortcutHelp) + " / Esc", "Close this guide"})
	}
	return append(guide,
		[2]string{shortcutLabel(profile, shortcutAdd), "Add an alias"},
		[2]string{shortcutLabel(profile, shortcutEdit), "Edit description, command, or name"},
		[2]string{"Ctrl+D", "Delete an alias after confirmation"},
		[2]string{shortcutLabel(profile, shortcutRevisions), "Browse and restore saved versions"},
		[2]string{shortcutLabel(profile, shortcutStats), "Open alias usage stats"},
		[2]string{shortcutLabel(profile, shortcutSettings), "Customize the TUI footer"},
		[2]string{shortcutLabel(profile, shortcutHealth), "Show aliases that need attention"},
		[2]string{shortcutLabel(profile, shortcutSync), "Open sync status and tracked files"},
		[2]string{"Ctrl+G", "Save alias changes to the local repository"},
		[2]string{shortcutLabel(profile, shortcutThemes), "Choose a theme with live preview"},
		[2]string{shortcutLabel(profile, shortcutRefresh), "Reload aliases and theme settings"},
		[2]string{shortcutLabel(profile, shortcutHelp) + " / Esc", "Close this guide"},
	)
}

func shortcutLabel(profile ShortcutProfile, action shortcutAction) string {
	for _, definition := range shortcutDefinitions {
		if definition.action == action {
			return shortcutChoiceForProfile(definition, profile).label
		}
	}
	return ""
}

func primaryShortcutLabel(profile ShortcutProfile, action shortcutAction) string {
	label := shortcutLabel(profile, action)
	if primary, _, found := strings.Cut(label, " / "); found {
		return primary
	}
	return label
}

func shortcutChoiceForProfile(definition shortcutDefinition, profile ShortcutProfile) shortcutChoice {
	switch profile {
	case shortcutWindows:
		return definition.windows
	case shortcutMacOS:
		return definition.macos
	default:
		return definition.linux
	}
}

func matchesShortcut(message tea.KeyMsg, profile ShortcutProfile, action shortcutAction) bool {
	resolved, ok := resolveShortcut(message, profile, action)
	return ok && resolved == action
}

func resolveShortcut(message tea.KeyMsg, profile ShortcutProfile, allowed ...shortcutAction) (shortcutAction, bool) {
	if message.Paste || message.Alt || message.Ctrl || message.Meta {
		return 0, false
	}
	for _, action := range allowed {
		for _, definition := range shortcutDefinitions {
			if definition.action != action {
				continue
			}
			for _, key := range shortcutChoiceForProfile(definition, profile).keys {
				if shortcutKeyMatches(message, key) {
					return action, true
				}
			}
		}
	}
	return 0, false
}

func shortcutKeyMatches(message tea.KeyMsg, key shortcutKey) bool {
	if message.Type != key.typeCode || message.Super != key.super {
		return false
	}
	if !key.allowShift && message.Shift != key.shift {
		return false
	}
	if key.typeCode != tea.KeyRunes {
		return true
	}
	return len(message.Runes) == 1 && unicodeLower(message.Runes[0]) == unicodeLower(key.runeCode)
}

func acceptsTextInput(message tea.KeyMsg) bool {
	return !message.Alt && !message.Ctrl && !message.Meta && !message.Super
}

func unicodeLower(value rune) rune {
	if value >= 'A' && value <= 'Z' {
		return value + ('a' - 'A')
	}
	return value
}
