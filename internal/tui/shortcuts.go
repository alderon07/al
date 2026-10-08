package tui

import (
	"alias-lens/internal/shortcuts"
	tea "alias-lens/internal/tea"
)

type shortcutProfile = shortcuts.Profile

type shortcutAction = shortcuts.Action

var (
	shortcutWindows = baseShortcutProfile("windows")
	shortcutLinux   = baseShortcutProfile("linux")
	shortcutMacOS   = baseShortcutProfile("macos")
)

const (
	shortcutHelp                = shortcuts.Help
	shortcutStats               = shortcuts.Stats
	shortcutSettings            = shortcuts.Settings
	shortcutThemes              = shortcuts.Themes
	shortcutRevisions           = shortcuts.Revisions
	shortcutSync                = shortcuts.Sync
	shortcutHealth              = shortcuts.Health
	shortcutAdd                 = shortcuts.Add
	shortcutEdit                = shortcuts.Edit
	shortcutContext             = shortcuts.Context
	shortcutFavorite            = shortcuts.Favorite
	shortcutCatalog             = shortcuts.Catalog
	shortcutDelete              = shortcuts.Delete
	shortcutRefresh             = shortcuts.Refresh
	shortcutSave                = shortcuts.Save
	shortcutMoveUp              = shortcuts.MoveUp
	shortcutMoveDown            = shortcuts.MoveDown
	shortcutPageUp              = shortcuts.PageUp
	shortcutPageDown            = shortcuts.PageDown
	shortcutFirst               = shortcuts.First
	shortcutLast                = shortcuts.Last
	shortcutUse                 = shortcuts.Use
	shortcutPrompt              = shortcuts.Prompt
	shortcutQuit                = shortcuts.Quit
	shortcutCommit              = shortcuts.Commit
	shortcutConfirm             = shortcuts.Confirm
	shortcutDecline             = shortcuts.Decline
	shortcutOpenDiff            = shortcuts.OpenDiff
	shortcutStatsQuit           = shortcuts.StatsQuit
	shortcutStatsPreviousPeriod = shortcuts.StatsPreviousPeriod
	shortcutStatsNextPeriod     = shortcuts.StatsNextPeriod
	shortcutStatsNextView       = shortcuts.StatsNextView
	shortcutStatsPreviousView   = shortcuts.StatsPreviousView
	shortcutStatsPreviousRow    = shortcuts.StatsPreviousRow
	shortcutStatsNextRow        = shortcuts.StatsNextRow
	shortcutStatsOverview       = shortcuts.StatsOverview
	shortcutStatsAliases        = shortcuts.StatsAliases
	shortcutStatsCommands       = shortcuts.StatsCommands
	shortcutStatsGrowth         = shortcuts.StatsGrowth
	shortcutStatsPeriod1        = shortcuts.StatsPeriod1
	shortcutStatsPeriod2        = shortcuts.StatsPeriod2
	shortcutStatsPeriod3        = shortcuts.StatsPeriod3
	shortcutStatsPeriod4        = shortcuts.StatsPeriod4
	shortcutStatsReload         = shortcuts.StatsReload
	shortcutDiffClose           = shortcuts.DiffClose
	shortcutDiffScrollDown      = shortcuts.DiffScrollDown
	shortcutDiffScrollUp        = shortcuts.DiffScrollUp
	shortcutDiffNext            = shortcuts.DiffNext
	shortcutDiffPrevious        = shortcuts.DiffPrevious
	shortcutDiffLayout          = shortcuts.DiffLayout
	shortcutDiffAliases         = shortcuts.DiffAliases
	shortcutDiffRestore         = shortcuts.DiffRestore
	shortcutDiffPanLeft         = shortcuts.DiffPanLeft
	shortcutDiffPanRight        = shortcuts.DiffPanRight
	shortcutDiffPageUp          = shortcuts.DiffPageUp
	shortcutDiffPageDown        = shortcuts.DiffPageDown
	shortcutDiffFirst           = shortcuts.DiffFirst
	shortcutDiffLast            = shortcuts.DiffLast
)

func baseShortcutProfile(value string) shortcutProfile {
	profile, _ := shortcuts.ParseProfile(value)
	return profile
}
func sharedShortcutProfile(profile shortcutProfile) shortcuts.Profile { return profile }
func parseShortcutProfile(value string) (shortcutProfile, error) {
	return shortcuts.ParseProfile(value)
}
func defaultShortcutProfile() shortcutProfile { return shortcuts.DefaultProfile() }
func resolvedShortcutProfile(config appConfig) shortcutProfile {
	return shortcuts.ResolveProfile(config.ShortcutProfile, config.Shortcuts)
}
func shortcutProfileLabel(profile shortcutProfile) string {
	return shortcuts.ProfileLabel(sharedShortcutProfile(profile))
}
func shortcutGuide(profile shortcutProfile, selectMode bool) [][2]string {
	return shortcuts.Guide(sharedShortcutProfile(profile), selectMode)
}
func shortcutLabel(profile shortcutProfile, action shortcutAction) string {
	return shortcuts.Label(sharedShortcutProfile(profile), action)
}
func primaryShortcutLabel(profile shortcutProfile, action shortcutAction) string {
	return shortcuts.PrimaryLabel(sharedShortcutProfile(profile), action)
}
func matchesShortcut(message tea.KeyMsg, profile shortcutProfile, action shortcutAction) bool {
	return shortcuts.Matches(message, sharedShortcutProfile(profile), action)
}
func resolveShortcut(message tea.KeyMsg, profile shortcutProfile, allowed ...shortcutAction) (shortcutAction, bool) {
	return shortcuts.Resolve(message, sharedShortcutProfile(profile), allowed...)
}
func acceptsTextInput(message tea.KeyMsg) bool { return shortcuts.AcceptsTextInput(message) }
