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
	shortcutContext
	shortcutDelete
	shortcutRefresh
	shortcutSave
)

type shortcutKey struct {
	typeCode   tea.KeyType
	runeCode   rune
	alt        bool
	ctrl       bool
	meta       bool
	super      bool
	shift      bool
	allowShift bool
}

type shortcutBinding struct {
	label        string
	key          shortcutKey
	terminalSafe bool
}

type shortcutChoice struct {
	bindings []shortcutBinding
}

type shortcutDefinition struct {
	action  shortcutAction
	windows shortcutChoice
	linux   shortcutChoice
	macos   shortcutChoice
}

func shortcutBindings(bindings ...shortcutBinding) shortcutChoice {
	return shortcutChoice{bindings: bindings}
}

func nativeShortcut(label string, key shortcutKey) shortcutBinding {
	return shortcutBinding{label: label, key: key}
}

func terminalShortcut(label string, key shortcutKey) shortcutBinding {
	return shortcutBinding{label: label, key: key, terminalSafe: true}
}

var shortcutDefinitions = []shortcutDefinition{
	{action: shortcutHelp,
		windows: shortcutBindings(terminalShortcut("F1", shortcutKey{typeCode: tea.KeyF1}), terminalShortcut("?", shortcutKey{typeCode: tea.KeyRunes, runeCode: '?', allowShift: true})),
		linux: shortcutBindings(terminalShortcut("F1", shortcutKey{typeCode: tea.KeyF1}),
			nativeShortcut("Ctrl+?", shortcutKey{typeCode: tea.KeyRunes, runeCode: '?', ctrl: true, allowShift: true}), terminalShortcut("?", shortcutKey{typeCode: tea.KeyRunes, runeCode: '?', allowShift: true})),
		macos: shortcutBindings(nativeShortcut("Cmd+?", shortcutKey{typeCode: tea.KeyRunes, runeCode: '?', super: true, allowShift: true}),
			terminalShortcut("F1", shortcutKey{typeCode: tea.KeyF1}), terminalShortcut("?", shortcutKey{typeCode: tea.KeyRunes, runeCode: '?', allowShift: true}))},
	{action: shortcutStats,
		windows: shortcutBindings(terminalShortcut("F2", shortcutKey{typeCode: tea.KeyF2})),
		linux:   shortcutBindings(terminalShortcut("F2", shortcutKey{typeCode: tea.KeyF2})),
		macos:   shortcutBindings(nativeShortcut("Cmd+2", shortcutKey{typeCode: tea.KeyRunes, runeCode: '2', super: true}), terminalShortcut("F2", shortcutKey{typeCode: tea.KeyF2}))},
	{action: shortcutSettings,
		windows: shortcutBindings(terminalShortcut("F3", shortcutKey{typeCode: tea.KeyF3}), nativeShortcut("Ctrl+,", shortcutKey{typeCode: tea.KeyRunes, runeCode: ',', ctrl: true})),
		linux:   shortcutBindings(terminalShortcut("F3", shortcutKey{typeCode: tea.KeyF3}), nativeShortcut("Ctrl+,", shortcutKey{typeCode: tea.KeyRunes, runeCode: ',', ctrl: true})),
		macos:   shortcutBindings(nativeShortcut("Cmd+,", shortcutKey{typeCode: tea.KeyRunes, runeCode: ',', super: true}), terminalShortcut("F3", shortcutKey{typeCode: tea.KeyF3}))},
	{action: shortcutThemes,
		windows: shortcutBindings(terminalShortcut("F4", shortcutKey{typeCode: tea.KeyF4})),
		linux:   shortcutBindings(terminalShortcut("F4", shortcutKey{typeCode: tea.KeyF4})),
		macos:   shortcutBindings(nativeShortcut("Cmd+4", shortcutKey{typeCode: tea.KeyRunes, runeCode: '4', super: true}), terminalShortcut("F4", shortcutKey{typeCode: tea.KeyF4}))},
	{action: shortcutRevisions,
		windows: shortcutBindings(nativeShortcut("Ctrl+Z", shortcutKey{typeCode: tea.KeyCtrlZ}), terminalShortcut("F8", shortcutKey{typeCode: tea.KeyF8})),
		linux:   shortcutBindings(nativeShortcut("Ctrl+Z", shortcutKey{typeCode: tea.KeyCtrlZ}), terminalShortcut("F8", shortcutKey{typeCode: tea.KeyF8})),
		macos:   shortcutBindings(nativeShortcut("Cmd+Z", shortcutKey{typeCode: tea.KeyRunes, runeCode: 'z', super: true}), terminalShortcut("F8", shortcutKey{typeCode: tea.KeyF8}))},
	{action: shortcutSync,
		windows: shortcutBindings(terminalShortcut("F6", shortcutKey{typeCode: tea.KeyF6})),
		linux:   shortcutBindings(terminalShortcut("F6", shortcutKey{typeCode: tea.KeyF6})),
		macos:   shortcutBindings(nativeShortcut("Cmd+6", shortcutKey{typeCode: tea.KeyRunes, runeCode: '6', super: true}), terminalShortcut("F6", shortcutKey{typeCode: tea.KeyF6}))},
	{action: shortcutHealth,
		windows: shortcutBindings(terminalShortcut("F7", shortcutKey{typeCode: tea.KeyF7})),
		linux:   shortcutBindings(terminalShortcut("F7", shortcutKey{typeCode: tea.KeyF7})),
		macos:   shortcutBindings(nativeShortcut("Cmd+7", shortcutKey{typeCode: tea.KeyRunes, runeCode: '7', super: true}), terminalShortcut("F7", shortcutKey{typeCode: tea.KeyF7}))},
	{action: shortcutAdd,
		windows: shortcutBindings(terminalShortcut("Ctrl+N", shortcutKey{typeCode: tea.KeyCtrlN})),
		linux:   shortcutBindings(terminalShortcut("Ctrl+N", shortcutKey{typeCode: tea.KeyCtrlN})),
		macos:   shortcutBindings(nativeShortcut("Cmd+N", shortcutKey{typeCode: tea.KeyRunes, runeCode: 'n', super: true}), terminalShortcut("Ctrl+N", shortcutKey{typeCode: tea.KeyCtrlN}))},
	{action: shortcutEdit,
		windows: shortcutBindings(terminalShortcut("Ctrl+E", shortcutKey{typeCode: tea.KeyCtrlE})),
		linux:   shortcutBindings(terminalShortcut("Ctrl+E", shortcutKey{typeCode: tea.KeyCtrlE})),
		macos:   shortcutBindings(nativeShortcut("Cmd+Shift+E", shortcutKey{typeCode: tea.KeyRunes, runeCode: 'e', super: true, shift: true}), terminalShortcut("Ctrl+E", shortcutKey{typeCode: tea.KeyCtrlE}))},
	{action: shortcutContext,
		windows: shortcutBindings(terminalShortcut("Ctrl+B", shortcutKey{typeCode: tea.KeyCtrlB})),
		linux:   shortcutBindings(terminalShortcut("Ctrl+B", shortcutKey{typeCode: tea.KeyCtrlB})),
		macos:   shortcutBindings(nativeShortcut("Cmd+B", shortcutKey{typeCode: tea.KeyRunes, runeCode: 'b', super: true}), terminalShortcut("Ctrl+B", shortcutKey{typeCode: tea.KeyCtrlB}))},
	{action: shortcutDelete,
		windows: shortcutBindings(terminalShortcut("Delete", shortcutKey{typeCode: tea.KeyDelete}), nativeShortcut("Ctrl+D", shortcutKey{typeCode: tea.KeyCtrlD})),
		linux:   shortcutBindings(terminalShortcut("Delete", shortcutKey{typeCode: tea.KeyDelete}), nativeShortcut("Ctrl+D", shortcutKey{typeCode: tea.KeyCtrlD})),
		macos:   shortcutBindings(nativeShortcut("Cmd+Backspace", shortcutKey{typeCode: tea.KeyBackspace, super: true}), terminalShortcut("Delete", shortcutKey{typeCode: tea.KeyDelete}))},
	{action: shortcutRefresh,
		windows: shortcutBindings(terminalShortcut("F5", shortcutKey{typeCode: tea.KeyF5}), terminalShortcut("Ctrl+R", shortcutKey{typeCode: tea.KeyCtrlR})),
		linux:   shortcutBindings(terminalShortcut("Ctrl+R", shortcutKey{typeCode: tea.KeyCtrlR}), terminalShortcut("F5", shortcutKey{typeCode: tea.KeyF5})),
		macos:   shortcutBindings(nativeShortcut("Cmd+R", shortcutKey{typeCode: tea.KeyRunes, runeCode: 'r', super: true}), terminalShortcut("F5", shortcutKey{typeCode: tea.KeyF5}), terminalShortcut("Ctrl+R", shortcutKey{typeCode: tea.KeyCtrlR}))},
	{action: shortcutSave,
		windows: shortcutBindings(terminalShortcut("Ctrl+S", shortcutKey{typeCode: tea.KeyCtrlS})),
		linux:   shortcutBindings(terminalShortcut("Ctrl+S", shortcutKey{typeCode: tea.KeyCtrlS})),
		macos:   shortcutBindings(nativeShortcut("Cmd+S", shortcutKey{typeCode: tea.KeyRunes, runeCode: 's', super: true}), terminalShortcut("Ctrl+S", shortcutKey{typeCode: tea.KeyCtrlS}))},
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
		[2]string{shortcutLabel(profile, shortcutContext), "Mark or unmark an alias for this project or folder"},
		[2]string{shortcutLabel(profile, shortcutDelete), "Delete an alias after confirmation"},
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
			choice := shortcutChoiceForProfile(definition, profile)
			labels := make([]string, 0, len(choice.bindings))
			for _, binding := range choice.bindings {
				labels = append(labels, binding.label)
			}
			return strings.Join(labels, " / ")
		}
	}
	return ""
}

func primaryShortcutLabel(profile ShortcutProfile, action shortcutAction) string {
	for _, definition := range shortcutDefinitions {
		if definition.action != action {
			continue
		}
		choice := shortcutChoiceForProfile(definition, profile)
		if len(choice.bindings) > 0 {
			return choice.bindings[0].label
		}
	}
	return ""
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
	if message.Paste {
		return 0, false
	}
	for _, action := range allowed {
		for _, definition := range shortcutDefinitions {
			if definition.action != action {
				continue
			}
			for _, binding := range shortcutChoiceForProfile(definition, profile).bindings {
				if shortcutKeyMatches(message, binding.key) {
					return action, true
				}
			}
		}
	}
	return 0, false
}

func shortcutKeyMatches(message tea.KeyMsg, key shortcutKey) bool {
	if message.Type != key.typeCode || message.Alt != key.alt || message.Ctrl != key.ctrl || message.Meta != key.meta || message.Super != key.super {
		return false
	}
	if !key.allowShift && message.Shift != key.shift {
		return false
	}
	if key.typeCode != tea.KeyRunes {
		return true
	}
	if key.ctrl && key.runeCode == '?' && len(message.Runes) == 1 && message.Runes[0] == '_' {
		// Legacy terminals encode Ctrl+/ and Ctrl+? as the ASCII unit separator,
		// which Bubble Tea reports as Ctrl+_.
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
