package shortcuts

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"

	tea "alias-lens/internal/tea"
)

type Profile struct {
	name      string
	overrides string
}

func (profile Profile) String() string { return profile.name }

var (
	shortcutWindows = Profile{name: "windows"}
	shortcutLinux   = Profile{name: "linux"}
	shortcutMacOS   = Profile{name: "macos"}
)

type Action int

const (
	Help Action = iota
	Stats
	Settings
	Themes
	Revisions
	Sync
	Health
	Add
	Edit
	Context
	Favorite
	Catalog
	Delete
	Refresh
	Save
	MoveUp
	MoveDown
	PageUp
	PageDown
	First
	Last
	Use
	Prompt
	Quit
	Commit
	Confirm
	Decline
	OpenDiff
	StatsQuit
	StatsPreviousPeriod
	StatsNextPeriod
	StatsNextView
	StatsPreviousView
	StatsPreviousRow
	StatsNextRow
	StatsOverview
	StatsAliases
	StatsCommands
	StatsGrowth
	StatsPeriod1
	StatsPeriod2
	StatsPeriod3
	StatsPeriod4
	StatsReload
	DiffClose
	DiffScrollDown
	DiffScrollUp
	DiffNext
	DiffPrevious
	DiffLayout
	DiffAliases
	DiffRestore
	DiffPanLeft
	DiffPanRight
	DiffPageUp
	DiffPageDown
	DiffFirst
	DiffLast
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
	action  Action
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
	{action: Help,
		windows: shortcutBindings(terminalShortcut("F1", shortcutKey{typeCode: tea.KeyF1}), terminalShortcut("?", shortcutKey{typeCode: tea.KeyRunes, runeCode: '?', allowShift: true})),
		linux: shortcutBindings(terminalShortcut("F1", shortcutKey{typeCode: tea.KeyF1}),
			nativeShortcut("Ctrl+?", shortcutKey{typeCode: tea.KeyRunes, runeCode: '?', ctrl: true, allowShift: true}), terminalShortcut("?", shortcutKey{typeCode: tea.KeyRunes, runeCode: '?', allowShift: true})),
		macos: shortcutBindings(nativeShortcut("Cmd+?", shortcutKey{typeCode: tea.KeyRunes, runeCode: '?', super: true, allowShift: true}),
			terminalShortcut("F1", shortcutKey{typeCode: tea.KeyF1}), terminalShortcut("?", shortcutKey{typeCode: tea.KeyRunes, runeCode: '?', allowShift: true}))},
	{action: Stats,
		windows: shortcutBindings(terminalShortcut("F2", shortcutKey{typeCode: tea.KeyF2})),
		linux:   shortcutBindings(terminalShortcut("F2", shortcutKey{typeCode: tea.KeyF2})),
		macos:   shortcutBindings(nativeShortcut("Cmd+2", shortcutKey{typeCode: tea.KeyRunes, runeCode: '2', super: true}), terminalShortcut("F2", shortcutKey{typeCode: tea.KeyF2}))},
	{action: Settings,
		windows: shortcutBindings(terminalShortcut("F3", shortcutKey{typeCode: tea.KeyF3}), nativeShortcut("Ctrl+,", shortcutKey{typeCode: tea.KeyRunes, runeCode: ',', ctrl: true})),
		linux:   shortcutBindings(terminalShortcut("F3", shortcutKey{typeCode: tea.KeyF3}), nativeShortcut("Ctrl+,", shortcutKey{typeCode: tea.KeyRunes, runeCode: ',', ctrl: true})),
		macos:   shortcutBindings(nativeShortcut("Cmd+,", shortcutKey{typeCode: tea.KeyRunes, runeCode: ',', super: true}), terminalShortcut("F3", shortcutKey{typeCode: tea.KeyF3}))},
	{action: Themes,
		windows: shortcutBindings(terminalShortcut("F4", shortcutKey{typeCode: tea.KeyF4})),
		linux:   shortcutBindings(terminalShortcut("F4", shortcutKey{typeCode: tea.KeyF4})),
		macos:   shortcutBindings(nativeShortcut("Cmd+4", shortcutKey{typeCode: tea.KeyRunes, runeCode: '4', super: true}), terminalShortcut("F4", shortcutKey{typeCode: tea.KeyF4}))},
	{action: Revisions,
		windows: shortcutBindings(nativeShortcut("Ctrl+Z", shortcutKey{typeCode: tea.KeyCtrlZ}), terminalShortcut("F8", shortcutKey{typeCode: tea.KeyF8})),
		linux:   shortcutBindings(nativeShortcut("Ctrl+Z", shortcutKey{typeCode: tea.KeyCtrlZ}), terminalShortcut("F8", shortcutKey{typeCode: tea.KeyF8})),
		macos:   shortcutBindings(nativeShortcut("Cmd+Z", shortcutKey{typeCode: tea.KeyRunes, runeCode: 'z', super: true}), terminalShortcut("F8", shortcutKey{typeCode: tea.KeyF8}))},
	{action: Sync,
		windows: shortcutBindings(terminalShortcut("F6", shortcutKey{typeCode: tea.KeyF6})),
		linux:   shortcutBindings(terminalShortcut("F6", shortcutKey{typeCode: tea.KeyF6})),
		macos:   shortcutBindings(nativeShortcut("Cmd+6", shortcutKey{typeCode: tea.KeyRunes, runeCode: '6', super: true}), terminalShortcut("F6", shortcutKey{typeCode: tea.KeyF6}))},
	{action: Health,
		windows: shortcutBindings(terminalShortcut("F7", shortcutKey{typeCode: tea.KeyF7})),
		linux:   shortcutBindings(terminalShortcut("F7", shortcutKey{typeCode: tea.KeyF7})),
		macos:   shortcutBindings(nativeShortcut("Cmd+7", shortcutKey{typeCode: tea.KeyRunes, runeCode: '7', super: true}), terminalShortcut("F7", shortcutKey{typeCode: tea.KeyF7}))},
	{action: Add,
		windows: shortcutBindings(terminalShortcut("Ctrl+N", shortcutKey{typeCode: tea.KeyCtrlN})),
		linux:   shortcutBindings(terminalShortcut("Ctrl+N", shortcutKey{typeCode: tea.KeyCtrlN})),
		macos:   shortcutBindings(nativeShortcut("Cmd+N", shortcutKey{typeCode: tea.KeyRunes, runeCode: 'n', super: true}), terminalShortcut("Ctrl+N", shortcutKey{typeCode: tea.KeyCtrlN}))},
	{action: Edit,
		windows: shortcutBindings(terminalShortcut("Ctrl+E", shortcutKey{typeCode: tea.KeyCtrlE})),
		linux:   shortcutBindings(terminalShortcut("Ctrl+E", shortcutKey{typeCode: tea.KeyCtrlE})),
		macos:   shortcutBindings(nativeShortcut("Cmd+Shift+E", shortcutKey{typeCode: tea.KeyRunes, runeCode: 'e', super: true, shift: true}), terminalShortcut("Ctrl+E", shortcutKey{typeCode: tea.KeyCtrlE}))},
	{action: Context,
		windows: shortcutBindings(terminalShortcut("Ctrl+B", shortcutKey{typeCode: tea.KeyCtrlB})),
		linux:   shortcutBindings(terminalShortcut("Ctrl+B", shortcutKey{typeCode: tea.KeyCtrlB})),
		macos:   shortcutBindings(nativeShortcut("Cmd+B", shortcutKey{typeCode: tea.KeyRunes, runeCode: 'b', super: true}), terminalShortcut("Ctrl+B", shortcutKey{typeCode: tea.KeyCtrlB}))},
	scopedRuneShortcut("aliases", Favorite, "f", 'f'),
	scopedRuneShortcut("aliases", Catalog, "l", 'l'),
	{action: Delete,
		windows: shortcutBindings(terminalShortcut("Delete", shortcutKey{typeCode: tea.KeyDelete}), nativeShortcut("Ctrl+D", shortcutKey{typeCode: tea.KeyCtrlD})),
		linux:   shortcutBindings(terminalShortcut("Delete", shortcutKey{typeCode: tea.KeyDelete}), nativeShortcut("Ctrl+D", shortcutKey{typeCode: tea.KeyCtrlD})),
		macos:   shortcutBindings(nativeShortcut("Cmd+Backspace", shortcutKey{typeCode: tea.KeyBackspace, super: true}), terminalShortcut("Delete", shortcutKey{typeCode: tea.KeyDelete}))},
	{action: Refresh,
		windows: shortcutBindings(terminalShortcut("F5", shortcutKey{typeCode: tea.KeyF5}), terminalShortcut("Ctrl+R", shortcutKey{typeCode: tea.KeyCtrlR})),
		linux:   shortcutBindings(terminalShortcut("Ctrl+R", shortcutKey{typeCode: tea.KeyCtrlR}), terminalShortcut("F5", shortcutKey{typeCode: tea.KeyF5})),
		macos:   shortcutBindings(nativeShortcut("Cmd+R", shortcutKey{typeCode: tea.KeyRunes, runeCode: 'r', super: true}), terminalShortcut("F5", shortcutKey{typeCode: tea.KeyF5}), terminalShortcut("Ctrl+R", shortcutKey{typeCode: tea.KeyCtrlR}))},
	{action: Save,
		windows: shortcutBindings(terminalShortcut("Ctrl+S", shortcutKey{typeCode: tea.KeyCtrlS})),
		linux:   shortcutBindings(terminalShortcut("Ctrl+S", shortcutKey{typeCode: tea.KeyCtrlS})),
		macos:   shortcutBindings(nativeShortcut("Cmd+S", shortcutKey{typeCode: tea.KeyRunes, runeCode: 's', super: true}), terminalShortcut("Ctrl+S", shortcutKey{typeCode: tea.KeyCtrlS}))},
	commonShortcut(MoveUp, "Up", tea.KeyUp),
	commonShortcut(MoveDown, "Down", tea.KeyDown),
	commonShortcut(PageUp, "PgUp", tea.KeyPgUp),
	commonShortcut(PageDown, "PgDown", tea.KeyPgDown),
	commonShortcut(First, "Home", tea.KeyHome),
	commonShortcut(Last, "End", tea.KeyEnd),
	commonShortcut(Use, "Enter", tea.KeyEnter),
	commonShortcut(Prompt, "Tab", tea.KeyTab),
	{action: Quit,
		windows: shortcutBindings(terminalShortcut("Esc", shortcutKey{typeCode: tea.KeyEsc}), terminalShortcut("Ctrl+C", shortcutKey{typeCode: tea.KeyCtrlC})),
		linux:   shortcutBindings(terminalShortcut("Esc", shortcutKey{typeCode: tea.KeyEsc}), terminalShortcut("Ctrl+C", shortcutKey{typeCode: tea.KeyCtrlC})),
		macos:   shortcutBindings(terminalShortcut("Esc", shortcutKey{typeCode: tea.KeyEsc}), terminalShortcut("Ctrl+C", shortcutKey{typeCode: tea.KeyCtrlC}))},
	commonShortcut(Commit, "Ctrl+G", tea.KeyCtrlG),
	scopedRuneShortcut("confirmation", Confirm, "y", 'y'),
	scopedRuneShortcut("confirmation", Decline, "n", 'n'),
	scopedRuneShortcut("sync", OpenDiff, "d", 'd'),
	scopedRuneShortcut("stats", StatsQuit, "q", 'q'),
	scopedKeysShortcut("stats", StatsPreviousPeriod, terminalShortcut("Left", shortcutKey{typeCode: tea.KeyLeft}), terminalShortcut("h", shortcutKey{typeCode: tea.KeyRunes, runeCode: 'h'})),
	scopedKeysShortcut("stats", StatsNextPeriod, terminalShortcut("Right", shortcutKey{typeCode: tea.KeyRight}), terminalShortcut("l", shortcutKey{typeCode: tea.KeyRunes, runeCode: 'l'})),
	scopedKeysShortcut("stats", StatsNextView, terminalShortcut("Tab", shortcutKey{typeCode: tea.KeyTab})),
	scopedKeysShortcut("stats", StatsPreviousView, terminalShortcut("Shift+Tab", shortcutKey{typeCode: tea.KeyShiftTab})),
	scopedKeysShortcut("stats", StatsPreviousRow, terminalShortcut("Up", shortcutKey{typeCode: tea.KeyUp}), terminalShortcut("k", shortcutKey{typeCode: tea.KeyRunes, runeCode: 'k'})),
	scopedKeysShortcut("stats", StatsNextRow, terminalShortcut("Down", shortcutKey{typeCode: tea.KeyDown}), terminalShortcut("j", shortcutKey{typeCode: tea.KeyRunes, runeCode: 'j'})),
	scopedRuneShortcut("stats", StatsOverview, "o", 'o'),
	scopedRuneShortcut("stats", StatsAliases, "a", 'a'),
	scopedRuneShortcut("stats", StatsCommands, "c", 'c'),
	scopedRuneShortcut("stats", StatsGrowth, "g", 'g'),
	scopedRuneShortcut("stats", StatsPeriod1, "1", '1'),
	scopedRuneShortcut("stats", StatsPeriod2, "2", '2'),
	scopedRuneShortcut("stats", StatsPeriod3, "3", '3'),
	scopedRuneShortcut("stats", StatsPeriod4, "4", '4'),
	scopedRuneShortcut("stats", StatsReload, "r", 'r'),
	scopedRuneShortcut("diff", DiffClose, "q", 'q'),
	scopedKeysShortcut("diff", DiffScrollDown, terminalShortcut("Down", shortcutKey{typeCode: tea.KeyDown}), terminalShortcut("j", shortcutKey{typeCode: tea.KeyRunes, runeCode: 'j'})),
	scopedKeysShortcut("diff", DiffScrollUp, terminalShortcut("Up", shortcutKey{typeCode: tea.KeyUp}), terminalShortcut("k", shortcutKey{typeCode: tea.KeyRunes, runeCode: 'k'})),
	scopedKeysShortcut("diff", DiffPageUp, terminalShortcut("PgUp", shortcutKey{typeCode: tea.KeyPgUp})),
	scopedKeysShortcut("diff", DiffPageDown, terminalShortcut("PgDown", shortcutKey{typeCode: tea.KeyPgDown})),
	scopedKeysShortcut("diff", DiffFirst, terminalShortcut("Home", shortcutKey{typeCode: tea.KeyHome})),
	scopedKeysShortcut("diff", DiffLast, terminalShortcut("End", shortcutKey{typeCode: tea.KeyEnd})),
	scopedRuneShortcut("diff", DiffNext, "n", 'n'),
	scopedRuneShortcut("diff", DiffPrevious, "p", 'p'),
	scopedRuneShortcut("diff", DiffLayout, "s", 's'),
	scopedRuneShortcut("diff", DiffAliases, "a", 'a'),
	scopedRuneShortcut("diff", DiffRestore, "r", 'r'),
	scopedKeysShortcut("diff", DiffPanLeft, terminalShortcut("Left", shortcutKey{typeCode: tea.KeyLeft})),
	scopedKeysShortcut("diff", DiffPanRight, terminalShortcut("Right", shortcutKey{typeCode: tea.KeyRight})),
}

var defaultLetterShortcuts = map[Action]rune{
	Help: 'h', Stats: 's', Settings: 'o', Themes: 't',
	Revisions: 'v', Sync: 'y', Health: 'i', Add: 'a',
	Edit: 'e', Context: 'c', Delete: 'd', Refresh: 'r',
	Commit:              'p',
	Catalog:             'l',
	StatsPreviousPeriod: 'h', StatsNextPeriod: 'l',
	StatsNextView: 'n', StatsPreviousView: 'p',
	StatsPreviousRow: 'k', StatsNextRow: 'j',
	DiffScrollDown: 'j', DiffScrollUp: 'k',
	DiffPanLeft: 'h', DiffPanRight: 'l',
}

func commonShortcut(action Action, label string, key tea.KeyType) shortcutDefinition {
	choice := shortcutBindings(terminalShortcut(label, shortcutKey{typeCode: key}))
	return shortcutDefinition{action: action, windows: choice, linux: choice, macos: choice}
}

func scopedKeysShortcut(scope string, action Action, bindings ...shortcutBinding) shortcutDefinition {
	choice := shortcutBindings(bindings...)
	return shortcutDefinition{action: action, scope: scope, windows: choice, linux: choice, macos: choice}
}

func scopedRuneShortcut(scope string, action Action, label string, key rune) shortcutDefinition {
	choice := shortcutBindings(terminalShortcut(label, shortcutKey{typeCode: tea.KeyRunes, runeCode: key}))
	return shortcutDefinition{action: action, scope: scope, windows: choice, linux: choice, macos: choice}
}

func ParseProfile(value string) (Profile, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "windows":
		return shortcutWindows, nil
	case "linux":
		return shortcutLinux, nil
	case "macos":
		return shortcutMacOS, nil
	default:
		return Profile{}, fmt.Errorf("shortcut style must be windows, linux, or macos")
	}
}

func ResolveProfile(value string, overrides map[string]string) Profile {
	profile := DefaultProfile()
	if value != "" {
		if selected, err := ParseProfile(value); err == nil {
			profile = selected
		}
	}
	if len(overrides) != 0 {
		encoded, _ := json.Marshal(overrides)
		profile.overrides = string(encoded)
	}
	return profile
}

func DefaultProfile() Profile {
	return detectProfile(runtime.GOOS, os.Getenv, os.ReadFile)
}

func detectProfile(goos string, getenv func(string) string, readFile func(string) ([]byte, error)) Profile {
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

func ProfileLabel(profile Profile) string {
	switch profile.name {
	case "windows":
		return "Windows"
	case "macos":
		return "macOS"
	default:
		return "Linux"
	}
}

func Guide(profile Profile, selectMode bool) [][2]string {
	enterAction := "Use the selected alias"
	if selectMode {
		enterAction = "Select without using it"
	}
	guide := [][2]string{
		{"/", "Search names, commands, and descriptions; Esc returns to commands"},
		{Label(profile, MoveUp) + " / " + Label(profile, MoveDown), "Move through results"},
		{Label(profile, PageUp) + " / " + Label(profile, PageDown), "Move by page"},
		{Label(profile, First) + " / " + Label(profile, Last), "Jump to first or last result"},
		{Label(profile, Use), enterAction},
		{Label(profile, Prompt), "Return the alias to the prompt for editing"},
	}
	if selectMode {
		return append(guide, [2]string{Label(profile, Help) + " / " + Label(profile, Quit), "Close this guide"})
	}
	return append(guide,
		[2]string{Label(profile, Add), "Add an alias"},
		[2]string{Label(profile, Edit), "Edit description, command, or name"},
		[2]string{Label(profile, Context), "Mark or unmark an alias for this project or folder"},
		[2]string{Label(profile, Favorite), "Mark or unmark the selected alias as a favorite"},
		[2]string{Label(profile, Delete), "Delete an alias after confirmation"},
		[2]string{Label(profile, Catalog), "Review catalog status, native ownership, and installation"},
		[2]string{Label(profile, Revisions), "Browse and restore saved versions"},
		[2]string{Label(profile, Stats), "Open alias usage stats"},
		[2]string{Label(profile, Settings), "Customize the TUI footer"},
		[2]string{Label(profile, Health), "Show aliases that need attention"},
		[2]string{Label(profile, Sync), "Open sync status and tracked files"},
		[2]string{Label(profile, Commit), "Save alias changes to the local repository"},
		[2]string{Label(profile, Themes), "Choose a theme with live preview"},
		[2]string{Label(profile, Refresh), "Reload aliases and theme settings"},
		[2]string{Label(profile, Quit), "Quit or close a dialog"},
		[2]string{Label(profile, Help) + " / " + Label(profile, Quit), "Close this guide"},
	)
}

func Label(profile Profile, action Action) string {
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

func PrimaryLabel(profile Profile, action Action) string {
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

func shortcutChoiceForProfile(definition shortcutDefinition, profile Profile) shortcutChoice {
	if profile.overrides != "" {
		var overrides map[string]string
		if json.Unmarshal([]byte(profile.overrides), &overrides) == nil {
			if label, ok := overrides[ActionName(definition.action)]; ok {
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

func baseShortcutChoiceForProfile(definition shortcutDefinition, profile Profile) shortcutChoice {
	switch profile.name {
	case "windows":
		return definition.windows
	case "macos":
		return definition.macos
	default:
		return definition.linux
	}
}

func Matches(message tea.KeyMsg, profile Profile, action Action) bool {
	resolved, ok := Resolve(message, profile, action)
	return ok && resolved == action
}

func Resolve(message tea.KeyMsg, profile Profile, allowed ...Action) (Action, bool) {
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

func AcceptsTextInput(message tea.KeyMsg) bool {
	return !message.Alt && !message.Ctrl && !message.Meta && !message.Super
}

func unicodeLower(value rune) rune {
	if value >= 'A' && value <= 'Z' {
		return value + ('a' - 'A')
	}
	return value
}
