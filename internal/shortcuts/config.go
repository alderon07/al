package shortcuts

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	tea "github.com/alderon07/al/internal/tea"
)

var shortcutActionNames = map[Action]string{
	Catalog: "catalog",
	Help:    "help", Stats: "stats", Settings: "settings",
	Themes: "themes", Revisions: "revisions", Sync: "sync",
	Health: "health", Add: "add", Edit: "edit",
	Context: "context", Favorite: "favorite", Delete: "delete", Refresh: "refresh",
	Save:   "save",
	MoveUp: "up", MoveDown: "down", PageUp: "page-up",
	PageDown: "page-down", First: "first", Last: "last",
	Use: "use", Prompt: "prompt", Quit: "quit",
	Commit:  "commit",
	Confirm: "confirm", Decline: "decline",
	OpenDiff:  "sync.diff",
	StatsQuit: "stats.quit", StatsPreviousPeriod: "stats.previous-period", StatsNextPeriod: "stats.next-period",
	StatsNextView: "stats.next-view", StatsPreviousView: "stats.previous-view",
	StatsPreviousRow: "stats.previous-row", StatsNextRow: "stats.next-row",
	StatsOverview: "stats.overview", StatsAliases: "stats.aliases", StatsCommands: "stats.commands", StatsGrowth: "stats.growth",
	StatsPeriod1: "stats.period-1", StatsPeriod2: "stats.period-2", StatsPeriod3: "stats.period-3", StatsPeriod4: "stats.period-4",
	StatsReload: "stats.reload",
	DiffClose:   "diff.close", DiffScrollDown: "diff.scroll-down", DiffScrollUp: "diff.scroll-up",
	DiffNext: "diff.next-change", DiffPrevious: "diff.previous-change", DiffLayout: "diff.layout",
	DiffAliases: "diff.aliases", DiffRestore: "diff.restore", DiffPanLeft: "diff.pan-left", DiffPanRight: "diff.pan-right",
	DiffPageUp: "diff.page-up", DiffPageDown: "diff.page-down", DiffFirst: "diff.first", DiffLast: "diff.last",
}

func Translate(message tea.KeyMsg, profile Profile, allowed ...Action) tea.KeyMsg {
	if message.Paste || profile.overrides == "" {
		return message
	}
	if len(allowed) == 0 {
		return message
	}
	var overrides map[string]string
	_ = json.Unmarshal([]byte(profile.overrides), &overrides)
	for _, action := range allowed {
		if _, changed := overrides[ActionName(action)]; !changed {
			continue
		}
		if Matches(message, profile, action) {
			if action == Quit {
				return tea.KeyMsg{Type: tea.KeyEsc, Repeat: message.Repeat}
			}
			for _, definition := range shortcutDefinitions {
				if definition.action == action {
					key := baseShortcutChoiceForProfile(definition, profile).bindings[0].key
					return tea.KeyMsg{Type: key.typeCode, Runes: []rune{key.runeCode}, Repeat: message.Repeat}
				}
			}
		}
	}
	if profile.overrides != "" {
		for _, action := range allowed {
			if _, changed := overrides[ActionName(action)]; !changed {
				continue
			}
			for _, definition := range shortcutDefinitions {
				if definition.action != action {
					continue
				}
				for _, binding := range shortcutChoiceForProfile(definition, Profile{name: profile.name}).bindings {
					if shortcutKeyMatches(message, binding.key) {
						return tea.KeyMsg{Type: tea.KeyNull}
					}
				}
			}
		}
	}
	return message
}

func ActionName(action Action) string { return shortcutActionNames[action] }

func PlainTextKey(message tea.KeyMsg) bool {
	return !message.Paste && AcceptsTextInput(message) && (message.Type == tea.KeySpace || message.Type == tea.KeyRunes)
}

func CompletionActions() []string {
	actions := make([]string, 0, len(shortcutDefinitions)+1)
	for _, definition := range shortcutDefinitions {
		actions = append(actions, ActionName(definition.action))
	}
	return append(actions, "launcher")
}

func parseShortcutBinding(value string) (shortcutBinding, error) {
	return parseShortcutBindingWithPlain(value, false)
}

func parseShortcutBindingForAction(action Action, value string) (shortcutBinding, error) {
	for _, definition := range shortcutDefinitions {
		if definition.action == action {
			if definition.scope == "" && strings.TrimSpace(value) == "/" {
				return shortcutBinding{}, fmt.Errorf("/ opens alias search; choose another key")
			}
			return parseShortcutBindingWithPlain(value, definition.scope != "" || action != Save)
		}
	}
	return parseShortcutBindingWithPlain(value, false)
}

func parseShortcutBindingWithPlain(value string, allowPlain bool) (shortcutBinding, error) {
	parts := strings.Split(strings.TrimSpace(value), "+")
	if len(parts) == 0 || len(parts) > 4 {
		return shortcutBinding{}, fmt.Errorf("invalid key %q", value)
	}
	key := shortcutKey{}
	seen := map[string]bool{}
	for _, modifier := range parts[:len(parts)-1] {
		modifier = strings.ToLower(strings.TrimSpace(modifier))
		if seen[modifier] {
			return shortcutBinding{}, fmt.Errorf("duplicate modifier in %q", value)
		}
		seen[modifier] = true
		switch modifier {
		case "ctrl":
			key.ctrl = true
		case "alt":
			key.alt = true
		case "cmd", "super":
			key.super = true
		case "shift":
			key.shift = true
		default:
			return shortcutBinding{}, fmt.Errorf("unknown modifier %q", modifier)
		}
	}
	name := strings.TrimSpace(parts[len(parts)-1])
	canonical := strings.ToLower(name)
	switch canonical {
	case "f1":
		key.typeCode = tea.KeyF1
	case "f2":
		key.typeCode = tea.KeyF2
	case "f3":
		key.typeCode = tea.KeyF3
	case "f4":
		key.typeCode = tea.KeyF4
	case "f5":
		key.typeCode = tea.KeyF5
	case "f6":
		key.typeCode = tea.KeyF6
	case "f7":
		key.typeCode = tea.KeyF7
	case "f8":
		key.typeCode = tea.KeyF8
	case "up":
		key.typeCode = tea.KeyUp
	case "down":
		key.typeCode = tea.KeyDown
	case "left":
		key.typeCode = tea.KeyLeft
	case "right":
		key.typeCode = tea.KeyRight
	case "home":
		key.typeCode = tea.KeyHome
	case "end":
		key.typeCode = tea.KeyEnd
	case "pgup":
		key.typeCode = tea.KeyPgUp
	case "pgdown":
		key.typeCode = tea.KeyPgDown
	case "delete":
		key.typeCode = tea.KeyDelete
	case "backspace":
		key.typeCode = tea.KeyBackspace
	case "enter":
		key.typeCode = tea.KeyEnter
	case "tab":
		if key.shift && !key.ctrl && !key.alt && !key.super {
			key.typeCode = tea.KeyShiftTab
			key.shift = false
		} else {
			key.typeCode = tea.KeyTab
		}
	case "esc":
		key.typeCode = tea.KeyEsc
	case "space":
		if !allowPlain && !key.ctrl && !key.alt && !key.super {
			return shortcutBinding{}, fmt.Errorf("plain text keys are reserved for search and forms; use Ctrl, Alt, or Cmd")
		}
		key.typeCode = tea.KeySpace
	default:
		runes := []rune(canonical)
		if len(runes) != 1 || !unicode.IsPrint(runes[0]) {
			return shortcutBinding{}, fmt.Errorf("unknown key %q", name)
		}
		if !allowPlain && !key.ctrl && !key.alt && !key.super {
			return shortcutBinding{}, fmt.Errorf("plain text keys are reserved for search and forms; use Ctrl, Alt, or Cmd")
		}
		if key.ctrl && !key.alt && !key.super && runes[0] >= 'a' && runes[0] <= 'z' {
			switch runes[0] {
			case 'i', 'j', 'm':
				return shortcutBinding{}, fmt.Errorf("Ctrl+%s is indistinguishable from Tab, Enter, or newline in legacy terminals", name)
			default:
				if strings.ContainsRune("abcdefghnrstuz", runes[0]) {
					key.typeCode = tea.KeyType(runes[0] - 'a' + 1)
					key.ctrl = false
				} else {
					key.typeCode = tea.KeyRunes
					key.runeCode = runes[0]
				}
			}
		} else {
			key.typeCode = tea.KeyRunes
			key.runeCode = runes[0]
		}
	}
	label := strings.Join(parts, "+")
	return shortcutBinding{label: label, key: key, terminalSafe: !key.super}, nil
}

func ValidateOverrides(profileName string, overrides map[string]string) error {
	for name, label := range overrides {
		if name == "launcher" {
			if _, err := ParseLauncherKey(label); err != nil {
				return err
			}
			continue
		}
		known := false
		for _, actionName := range shortcutActionNames {
			known = known || actionName == name
		}
		if !known {
			return fmt.Errorf("unknown action %q; run al shortcuts", name)
		}
		var action Action
		for candidate, actionName := range shortcutActionNames {
			if actionName == name {
				action = candidate
				break
			}
		}
		if _, err := parseShortcutBindingForAction(action, label); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	profile := ResolveProfile(profileName, overrides)
	type assignedKey struct {
		key           shortcutKey
		action, scope string
	}
	seen := []assignedKey{}
	for _, definition := range shortcutDefinitions {
		for _, binding := range shortcutChoiceForProfile(definition, profile).bindings {
			for _, previous := range seen {
				if previous.action != ActionName(definition.action) && shortcutKeysOverlap(previous.key, binding.key) && previous.scope == definition.scope {
					return fmt.Errorf("%s conflicts with %s on %s", ActionName(definition.action), previous.action, binding.label)
				}
			}
			seen = append(seen, assignedKey{binding.key, ActionName(definition.action), definition.scope})
		}
	}
	return nil
}

func shortcutKeysOverlap(left, right shortcutKey) bool {
	return left.typeCode == right.typeCode && left.runeCode == right.runeCode &&
		left.alt == right.alt && left.ctrl == right.ctrl && left.meta == right.meta && left.super == right.super &&
		(left.shift == right.shift || left.allowShift || right.allowShift)
}

func ParseLauncherKey(label string) (string, error) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(label)), "+")
	if len(parts) != 2 || parts[0] != "ctrl" || len(parts[1]) != 1 || parts[1][0] < 'a' || parts[1][0] > 'z' {
		return "", fmt.Errorf("launcher must be Ctrl plus one letter, for example Ctrl+G")
	}
	switch parts[1] {
	case "c", "i", "j", "m", "x", "z":
		return "", fmt.Errorf("Ctrl+%s is reserved by the shell or terminal", strings.ToUpper(parts[1]))
	}
	return strings.ToUpper(parts[1]), nil
}

func LauncherLabel(overrides map[string]string) string {
	if value := overrides["launcher"]; value != "" {
		return value
	}
	return "Ctrl+G"
}
