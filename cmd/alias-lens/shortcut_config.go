package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	tea "alias-lens/cmd/alias-lens/internal/tea"
)

var shortcutActionNames = map[shortcutAction]string{
	shortcutHelp: "help", shortcutStats: "stats", shortcutSettings: "settings",
	shortcutThemes: "themes", shortcutRevisions: "revisions", shortcutSync: "sync",
	shortcutHealth: "health", shortcutAdd: "add", shortcutEdit: "edit",
	shortcutContext: "context", shortcutDelete: "delete", shortcutRefresh: "refresh",
	shortcutSave:   "save",
	shortcutMoveUp: "up", shortcutMoveDown: "down", shortcutPageUp: "page-up",
	shortcutPageDown: "page-down", shortcutFirst: "first", shortcutLast: "last",
	shortcutUse: "use", shortcutPrompt: "prompt", shortcutQuit: "quit",
	shortcutCommit:  "commit",
	shortcutConfirm: "confirm", shortcutDecline: "decline",
	shortcutOpenDiff:  "sync.diff",
	shortcutStatsQuit: "stats.quit", shortcutStatsPreviousPeriod: "stats.previous-period", shortcutStatsNextPeriod: "stats.next-period",
	shortcutStatsNextView: "stats.next-view", shortcutStatsPreviousView: "stats.previous-view",
	shortcutStatsPreviousRow: "stats.previous-row", shortcutStatsNextRow: "stats.next-row",
	shortcutStatsOverview: "stats.overview", shortcutStatsAliases: "stats.aliases", shortcutStatsCommands: "stats.commands", shortcutStatsGrowth: "stats.growth",
	shortcutStatsPeriod1: "stats.period-1", shortcutStatsPeriod2: "stats.period-2", shortcutStatsPeriod3: "stats.period-3", shortcutStatsPeriod4: "stats.period-4",
	shortcutStatsReload: "stats.reload",
	shortcutDiffClose:   "diff.close", shortcutDiffScrollDown: "diff.scroll-down", shortcutDiffScrollUp: "diff.scroll-up",
	shortcutDiffNext: "diff.next-change", shortcutDiffPrevious: "diff.previous-change", shortcutDiffLayout: "diff.layout",
	shortcutDiffAliases: "diff.aliases", shortcutDiffRestore: "diff.restore", shortcutDiffPanLeft: "diff.pan-left", shortcutDiffPanRight: "diff.pan-right",
	shortcutDiffPageUp: "diff.page-up", shortcutDiffPageDown: "diff.page-down", shortcutDiffFirst: "diff.first", shortcutDiffLast: "diff.last",
}

var statsTranslatedShortcutActions = []shortcutAction{
	shortcutStatsQuit, shortcutStatsPreviousPeriod, shortcutStatsNextPeriod, shortcutStatsNextView, shortcutStatsPreviousView,
	shortcutStatsPreviousRow, shortcutStatsNextRow, shortcutStatsOverview, shortcutStatsAliases, shortcutStatsCommands, shortcutStatsGrowth,
	shortcutStatsPeriod1, shortcutStatsPeriod2, shortcutStatsPeriod3, shortcutStatsPeriod4, shortcutStatsReload,
}

var diffTranslatedShortcutActions = []shortcutAction{
	shortcutDiffClose, shortcutDiffScrollDown, shortcutDiffScrollUp, shortcutDiffNext, shortcutDiffPrevious,
	shortcutDiffLayout, shortcutDiffAliases, shortcutDiffRestore, shortcutDiffPanLeft, shortcutDiffPanRight,
	shortcutDiffPageUp, shortcutDiffPageDown, shortcutDiffFirst, shortcutDiffLast,
}

var translatedShortcutActions = []shortcutAction{
	shortcutMoveUp, shortcutMoveDown, shortcutPageUp, shortcutPageDown,
	shortcutFirst, shortcutLast, shortcutUse, shortcutPrompt, shortcutQuit, shortcutCommit,
}

func (m model) activeTranslatedShortcuts() []shortcutAction {
	navigation := []shortcutAction{shortcutMoveUp, shortcutMoveDown, shortcutPageUp, shortcutPageDown, shortcutFirst, shortcutLast, shortcutQuit}
	switch {
	case m.adding, m.deleteName != "", m.runConfirm != nil, m.settingsOpen, m.helpVisible, m.tourVisible:
		return []shortcutAction{shortcutQuit}
	case m.diff != nil && m.diff.confirmRestore:
		return []shortcutAction{shortcutQuit}
	case m.diff != nil:
		return append([]shortcutAction{shortcutQuit}, diffTranslatedShortcutActions...)
	case m.statsOpen:
		return append([]shortcutAction{shortcutQuit}, statsTranslatedShortcutActions...)
	case m.themePicker, m.revisionOpen:
		return append(navigation, shortcutUse)
	case m.trackedOnly:
		return append(navigation, shortcutCommit, shortcutOpenDiff)
	default:
		return translatedShortcutActions
	}
}

func translateShortcut(message tea.KeyMsg, profile ShortcutProfile, allowed ...shortcutAction) tea.KeyMsg {
	if message.Paste || profile.overrides == "" {
		return message
	}
	if len(allowed) == 0 {
		return message
	}
	var overrides map[string]string
	_ = json.Unmarshal([]byte(profile.overrides), &overrides)
	for _, action := range allowed {
		if _, changed := overrides[shortcutActionName(action)]; !changed {
			continue
		}
		if matchesShortcut(message, profile, action) {
			if action == shortcutQuit {
				return tea.KeyMsg{Type: tea.KeyEsc, Repeat: message.Repeat}
			}
			for _, definition := range shortcutDefinitions {
				if definition.action == action {
					key := shortcutChoiceForProfile(definition, ShortcutProfile{name: profile.name}).bindings[0].key
					return tea.KeyMsg{Type: key.typeCode, Runes: []rune{key.runeCode}, Repeat: message.Repeat}
				}
			}
		}
	}
	if profile.overrides != "" {
		for _, action := range allowed {
			if _, changed := overrides[shortcutActionName(action)]; !changed {
				continue
			}
			for _, definition := range shortcutDefinitions {
				if definition.action != action {
					continue
				}
				for _, binding := range shortcutChoiceForProfile(definition, ShortcutProfile{name: profile.name}).bindings {
					if shortcutKeyMatches(message, binding.key) {
						return tea.KeyMsg{Type: tea.KeyNull}
					}
				}
			}
		}
	}
	return message
}

func shortcutActionName(action shortcutAction) string { return shortcutActionNames[action] }

func shortcutCompletionActions() []string {
	actions := make([]string, 0, len(shortcutDefinitions)+1)
	for _, definition := range shortcutDefinitions {
		actions = append(actions, shortcutActionName(definition.action))
	}
	return append(actions, "launcher")
}

func parseShortcutBinding(value string) (shortcutBinding, error) {
	return parseShortcutBindingWithPlain(value, false)
}

func parseShortcutBindingForAction(action shortcutAction, value string) (shortcutBinding, error) {
	for _, definition := range shortcutDefinitions {
		if definition.action == action {
			return parseShortcutBindingWithPlain(value, definition.scope != "")
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

func validateShortcutOverrides(config AppConfig) error {
	for name, label := range config.Shortcuts {
		if name == "launcher" {
			if _, err := parseLauncherKey(label); err != nil {
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
		var action shortcutAction
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
	profile := resolvedShortcutProfile(config)
	type assignedKey struct {
		key           shortcutKey
		action, scope string
		custom        bool
	}
	seen := []assignedKey{}
	for _, definition := range shortcutDefinitions {
		for _, binding := range shortcutChoiceForProfile(definition, profile).bindings {
			for _, previous := range seen {
				_, custom := config.Shortcuts[shortcutActionName(definition.action)]
				if previous.action != shortcutActionName(definition.action) && shortcutKeysOverlap(previous.key, binding.key) && (previous.scope == definition.scope || (previous.scope == "" || definition.scope == "") && (custom || previous.custom)) {
					return fmt.Errorf("%s conflicts with %s on %s", shortcutActionName(definition.action), previous.action, binding.label)
				}
			}
			_, custom := config.Shortcuts[shortcutActionName(definition.action)]
			seen = append(seen, assignedKey{binding.key, shortcutActionName(definition.action), definition.scope, custom})
		}
	}
	return nil
}

func shortcutKeysOverlap(left, right shortcutKey) bool {
	return left.typeCode == right.typeCode && left.runeCode == right.runeCode &&
		left.alt == right.alt && left.ctrl == right.ctrl && left.meta == right.meta && left.super == right.super &&
		(left.shift == right.shift || left.allowShift || right.allowShift)
}

func parseLauncherKey(label string) (string, error) {
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

func launcherLabel(config AppConfig) string {
	if value := config.Shortcuts["launcher"]; value != "" {
		return value
	}
	return "Ctrl+G"
}
