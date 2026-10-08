package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	tea "alias-lens/internal/tea"
)

type ShortcutProfile struct {
	name      string
	overrides string
}

func (profile ShortcutProfile) String() string { return profile.name }

var (
	shortcutWindows = ShortcutProfile{name: "windows"}
	shortcutLinux   = ShortcutProfile{name: "linux"}
	shortcutMacOS   = ShortcutProfile{name: "macos"}
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
	shortcutFavorite
	shortcutCatalog
	shortcutDelete
	shortcutRefresh
	shortcutSave
	shortcutMoveUp
	shortcutMoveDown
	shortcutPageUp
	shortcutPageDown
	shortcutFirst
	shortcutLast
	shortcutUse
	shortcutPrompt
	shortcutQuit
	shortcutCommit
	shortcutConfirm
	shortcutDecline
	shortcutOpenDiff
	shortcutStatsQuit
	shortcutStatsPreviousPeriod
	shortcutStatsNextPeriod
	shortcutStatsNextView
	shortcutStatsPreviousView
	shortcutStatsPreviousRow
	shortcutStatsNextRow
	shortcutStatsOverview
	shortcutStatsAliases
	shortcutStatsCommands
	shortcutStatsGrowth
	shortcutStatsPeriod1
	shortcutStatsPeriod2
	shortcutStatsPeriod3
	shortcutStatsPeriod4
	shortcutStatsReload
	shortcutDiffClose
	shortcutDiffScrollDown
	shortcutDiffScrollUp
	shortcutDiffNext
	shortcutDiffPrevious
	shortcutDiffLayout
	shortcutDiffAliases
	shortcutDiffRestore
	shortcutDiffPanLeft
	shortcutDiffPanRight
	shortcutDiffPageUp
	shortcutDiffPageDown
	shortcutDiffFirst
	shortcutDiffLast
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
	scope   string
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
	scopedRuneShortcut("aliases", shortcutFavorite, "f", 'f'),
	scopedRuneShortcut("aliases", shortcutCatalog, "l", 'l'),
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
	commonShortcut(shortcutMoveUp, "Up", tea.KeyUp),
	commonShortcut(shortcutMoveDown, "Down", tea.KeyDown),
	commonShortcut(shortcutPageUp, "PgUp", tea.KeyPgUp),
	commonShortcut(shortcutPageDown, "PgDown", tea.KeyPgDown),
	commonShortcut(shortcutFirst, "Home", tea.KeyHome),
	commonShortcut(shortcutLast, "End", tea.KeyEnd),
	commonShortcut(shortcutUse, "Enter", tea.KeyEnter),
	commonShortcut(shortcutPrompt, "Tab", tea.KeyTab),
	{action: shortcutQuit,
		windows: shortcutBindings(terminalShortcut("Esc", shortcutKey{typeCode: tea.KeyEsc}), terminalShortcut("Ctrl+C", shortcutKey{typeCode: tea.KeyCtrlC})),
		linux:   shortcutBindings(terminalShortcut("Esc", shortcutKey{typeCode: tea.KeyEsc}), terminalShortcut("Ctrl+C", shortcutKey{typeCode: tea.KeyCtrlC})),
		macos:   shortcutBindings(terminalShortcut("Esc", shortcutKey{typeCode: tea.KeyEsc}), terminalShortcut("Ctrl+C", shortcutKey{typeCode: tea.KeyCtrlC}))},
	commonShortcut(shortcutCommit, "Ctrl+G", tea.KeyCtrlG),
	scopedRuneShortcut("confirmation", shortcutConfirm, "y", 'y'),
	scopedRuneShortcut("confirmation", shortcutDecline, "n", 'n'),
	scopedRuneShortcut("sync", shortcutOpenDiff, "d", 'd'),
	scopedRuneShortcut("stats", shortcutStatsQuit, "q", 'q'),
	scopedKeysShortcut("stats", shortcutStatsPreviousPeriod, terminalShortcut("Left", shortcutKey{typeCode: tea.KeyLeft}), terminalShortcut("h", shortcutKey{typeCode: tea.KeyRunes, runeCode: 'h'})),
	scopedKeysShortcut("stats", shortcutStatsNextPeriod, terminalShortcut("Right", shortcutKey{typeCode: tea.KeyRight}), terminalShortcut("l", shortcutKey{typeCode: tea.KeyRunes, runeCode: 'l'})),
	scopedKeysShortcut("stats", shortcutStatsNextView, terminalShortcut("Tab", shortcutKey{typeCode: tea.KeyTab})),
	scopedKeysShortcut("stats", shortcutStatsPreviousView, terminalShortcut("Shift+Tab", shortcutKey{typeCode: tea.KeyShiftTab})),
	scopedKeysShortcut("stats", shortcutStatsPreviousRow, terminalShortcut("Up", shortcutKey{typeCode: tea.KeyUp}), terminalShortcut("k", shortcutKey{typeCode: tea.KeyRunes, runeCode: 'k'})),
	scopedKeysShortcut("stats", shortcutStatsNextRow, terminalShortcut("Down", shortcutKey{typeCode: tea.KeyDown}), terminalShortcut("j", shortcutKey{typeCode: tea.KeyRunes, runeCode: 'j'})),
	scopedRuneShortcut("stats", shortcutStatsOverview, "o", 'o'),
	scopedRuneShortcut("stats", shortcutStatsAliases, "a", 'a'),
	scopedRuneShortcut("stats", shortcutStatsCommands, "c", 'c'),
	scopedRuneShortcut("stats", shortcutStatsGrowth, "g", 'g'),
	scopedRuneShortcut("stats", shortcutStatsPeriod1, "1", '1'),
	scopedRuneShortcut("stats", shortcutStatsPeriod2, "2", '2'),
	scopedRuneShortcut("stats", shortcutStatsPeriod3, "3", '3'),
	scopedRuneShortcut("stats", shortcutStatsPeriod4, "4", '4'),
	scopedRuneShortcut("stats", shortcutStatsReload, "r", 'r'),
	scopedRuneShortcut("diff", shortcutDiffClose, "q", 'q'),
	scopedKeysShortcut("diff", shortcutDiffScrollDown, terminalShortcut("Down", shortcutKey{typeCode: tea.KeyDown}), terminalShortcut("j", shortcutKey{typeCode: tea.KeyRunes, runeCode: 'j'})),
	scopedKeysShortcut("diff", shortcutDiffScrollUp, terminalShortcut("Up", shortcutKey{typeCode: tea.KeyUp}), terminalShortcut("k", shortcutKey{typeCode: tea.KeyRunes, runeCode: 'k'})),
	scopedKeysShortcut("diff", shortcutDiffPageUp, terminalShortcut("PgUp", shortcutKey{typeCode: tea.KeyPgUp})),
	scopedKeysShortcut("diff", shortcutDiffPageDown, terminalShortcut("PgDown", shortcutKey{typeCode: tea.KeyPgDown})),
	scopedKeysShortcut("diff", shortcutDiffFirst, terminalShortcut("Home", shortcutKey{typeCode: tea.KeyHome})),
	scopedKeysShortcut("diff", shortcutDiffLast, terminalShortcut("End", shortcutKey{typeCode: tea.KeyEnd})),
	scopedRuneShortcut("diff", shortcutDiffNext, "n", 'n'),
	scopedRuneShortcut("diff", shortcutDiffPrevious, "p", 'p'),
	scopedRuneShortcut("diff", shortcutDiffLayout, "s", 's'),
	scopedRuneShortcut("diff", shortcutDiffAliases, "a", 'a'),
	scopedRuneShortcut("diff", shortcutDiffRestore, "r", 'r'),
	scopedKeysShortcut("diff", shortcutDiffPanLeft, terminalShortcut("Left", shortcutKey{typeCode: tea.KeyLeft})),
	scopedKeysShortcut("diff", shortcutDiffPanRight, terminalShortcut("Right", shortcutKey{typeCode: tea.KeyRight})),
}

var defaultLetterShortcuts = map[shortcutAction]rune{
	shortcutHelp: 'h', shortcutStats: 's', shortcutSettings: 'o', shortcutThemes: 't',
	shortcutRevisions: 'v', shortcutSync: 'y', shortcutHealth: 'i', shortcutAdd: 'a',
	shortcutEdit: 'e', shortcutContext: 'c', shortcutDelete: 'd', shortcutRefresh: 'r',
	shortcutCommit:              'p',
	shortcutCatalog:             'l',
	shortcutStatsPreviousPeriod: 'h', shortcutStatsNextPeriod: 'l',
	shortcutStatsNextView: 'n', shortcutStatsPreviousView: 'p',
	shortcutStatsPreviousRow: 'k', shortcutStatsNextRow: 'j',
	shortcutDiffScrollDown: 'j', shortcutDiffScrollUp: 'k',
	shortcutDiffPanLeft: 'h', shortcutDiffPanRight: 'l',
}

func commonShortcut(action shortcutAction, label string, key tea.KeyType) shortcutDefinition {
	choice := shortcutBindings(terminalShortcut(label, shortcutKey{typeCode: key}))
	return shortcutDefinition{action: action, windows: choice, linux: choice, macos: choice}
}

func scopedKeysShortcut(scope string, action shortcutAction, bindings ...shortcutBinding) shortcutDefinition {
	choice := shortcutBindings(bindings...)
	return shortcutDefinition{action: action, scope: scope, windows: choice, linux: choice, macos: choice}
}

func scopedRuneShortcut(scope string, action shortcutAction, label string, key rune) shortcutDefinition {
	choice := shortcutBindings(terminalShortcut(label, shortcutKey{typeCode: tea.KeyRunes, runeCode: key}))
	return shortcutDefinition{action: action, scope: scope, windows: choice, linux: choice, macos: choice}
}

func parseShortcutProfile(value string) (ShortcutProfile, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "windows":
		return shortcutWindows, nil
	case "linux":
		return shortcutLinux, nil
	case "macos":
		return shortcutMacOS, nil
	default:
		return ShortcutProfile{}, fmt.Errorf("shortcut style must be windows, linux, or macos")
	}
}

func resolvedShortcutProfile(config AppConfig) ShortcutProfile {
	profile := defaultShortcutProfile()
	if config.ShortcutProfile != "" {
		if selected, err := parseShortcutProfile(config.ShortcutProfile); err == nil {
			profile = selected
		}
	}
	if len(config.Shortcuts) != 0 {
		encoded, _ := json.Marshal(config.Shortcuts)
		profile.overrides = string(encoded)
	}
	return profile
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
	switch profile.name {
	case "windows":
		return "Windows"
	case "macos":
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
				for _, name := range shortcutActionNames {
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
		config.ShortcutProfile = profile.name
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
	for _, definition := range shortcutDefinitions {
		fmt.Printf("  %-12s %s\n", shortcutActionName(definition.action), shortcutLabel(profile, definition.action))
	}
	fmt.Println()
	if profile.name == shortcutMacOS.name {
		fmt.Println("If your terminal keeps a Command shortcut, use the terminal-safe fallback shown beside it.")
	}
	if profile.name == shortcutMacOS.name {
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
		{"/", "Search names, commands, and descriptions; Esc returns to commands"},
		{shortcutLabel(profile, shortcutMoveUp) + " / " + shortcutLabel(profile, shortcutMoveDown), "Move through results"},
		{shortcutLabel(profile, shortcutPageUp) + " / " + shortcutLabel(profile, shortcutPageDown), "Move by page"},
		{shortcutLabel(profile, shortcutFirst) + " / " + shortcutLabel(profile, shortcutLast), "Jump to first or last result"},
		{shortcutLabel(profile, shortcutUse), enterAction},
		{shortcutLabel(profile, shortcutPrompt), "Return the alias to the prompt for editing"},
	}
	if selectMode {
		return append(guide, [2]string{shortcutLabel(profile, shortcutHelp) + " / " + shortcutLabel(profile, shortcutQuit), "Close this guide"})
	}
	return append(guide,
		[2]string{shortcutLabel(profile, shortcutAdd), "Add an alias"},
		[2]string{shortcutLabel(profile, shortcutEdit), "Edit description, command, or name"},
		[2]string{shortcutLabel(profile, shortcutContext), "Mark or unmark an alias for this project or folder"},
		[2]string{shortcutLabel(profile, shortcutFavorite), "Mark or unmark the selected alias as a favorite"},
		[2]string{shortcutLabel(profile, shortcutDelete), "Delete an alias after confirmation"},
		[2]string{shortcutLabel(profile, shortcutCatalog), "Review catalog status, native ownership, and installation"},
		[2]string{shortcutLabel(profile, shortcutRevisions), "Browse and restore saved versions"},
		[2]string{shortcutLabel(profile, shortcutStats), "Open alias usage stats"},
		[2]string{shortcutLabel(profile, shortcutSettings), "Customize the TUI footer"},
		[2]string{shortcutLabel(profile, shortcutHealth), "Show aliases that need attention"},
		[2]string{shortcutLabel(profile, shortcutSync), "Open sync status and tracked files"},
		[2]string{shortcutLabel(profile, shortcutCommit), "Save alias changes to the local repository"},
		[2]string{shortcutLabel(profile, shortcutThemes), "Choose a theme with live preview"},
		[2]string{shortcutLabel(profile, shortcutRefresh), "Reload aliases and theme settings"},
		[2]string{shortcutLabel(profile, shortcutQuit), "Quit or close a dialog"},
		[2]string{shortcutLabel(profile, shortcutHelp) + " / " + shortcutLabel(profile, shortcutQuit), "Close this guide"},
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
	if profile.overrides != "" {
		var overrides map[string]string
		if json.Unmarshal([]byte(profile.overrides), &overrides) == nil {
			if label, ok := overrides[shortcutActionName(definition.action)]; ok {
				if binding, err := parseShortcutBindingForAction(definition.action, label); err == nil {
					return shortcutBindings(binding)
				}
			}
		}
	}
	choice := baseShortcutChoiceForProfile(definition, profile)
	if letter, ok := defaultLetterShortcuts[definition.action]; ok {
		key := shortcutKey{typeCode: tea.KeyRunes, runeCode: letter}
		bindings := make([]shortcutBinding, 0, len(choice.bindings)+1)
		bindings = append(bindings, terminalShortcut(string(letter), key))
		for _, binding := range choice.bindings {
			if binding.key != key {
				bindings = append(bindings, binding)
			}
		}
		return shortcutBindings(bindings...)
	}
	return choice
}

func baseShortcutChoiceForProfile(definition shortcutDefinition, profile ShortcutProfile) shortcutChoice {
	switch profile.name {
	case "windows":
		return definition.windows
	case "macos":
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
