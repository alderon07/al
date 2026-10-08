package tui

import (
	"alias-lens/internal/shortcuts"
	tea "alias-lens/internal/tea"
)

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
	case m.adding, m.settingsOpen, m.helpVisible, m.deleteName != "", m.runConfirm != nil, m.tourVisible:
		return []shortcutAction{shortcutQuit}
	case m.diff != nil && m.diff.confirmRestore:
		return []shortcutAction{shortcutQuit}
	case m.diff != nil:
		return append(append([]shortcutAction{}, diffTranslatedShortcutActions...), shortcutQuit)
	case m.statsOpen:
		return append(append([]shortcutAction{}, statsTranslatedShortcutActions...), shortcutQuit)
	case m.themePicker, m.revisionOpen:
		return append(navigation, shortcutUse)
	case m.trackedOnly:
		return append(navigation, shortcutCommit, shortcutOpenDiff)
	default:
		return translatedShortcutActions
	}
}

func translateShortcut(message tea.KeyMsg, profile shortcutProfile, allowed ...shortcutAction) tea.KeyMsg {
	return shortcuts.Translate(message, sharedShortcutProfile(profile), allowed...)
}
func shortcutActionName(action shortcutAction) string { return shortcuts.ActionName(action) }
func plainTextKey(message tea.KeyMsg) bool            { return shortcuts.PlainTextKey(message) }

func launcherLabel(config appConfig) string { return shortcuts.LauncherLabel(config.Shortcuts) }
