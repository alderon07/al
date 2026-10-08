package tui

import "alias-lens/internal/entry"

import "alias-lens/internal/presentation"

import "alias-lens/internal/app"

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"strings"
	"time"
	"unicode/utf8"

	tea "alias-lens/internal/tea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/rivo/uniseg"
)

var (
	acidColor   = lipgloss.Color("#B8FF6A")
	cyanColor   = lipgloss.Color("#72DDF7")
	coralColor  = lipgloss.Color("#FF8A65")
	amberColor  = lipgloss.Color("#FFD166")
	violetColor = lipgloss.Color("#C6A0F6")
	inkColor    = lipgloss.Color("#F3F6EE")
	mutedColor  = lipgloss.Color("#8EA6A2")
	pageColor   = lipgloss.Color("#0D1211")
	panelColor  = lipgloss.Color("#151C1A")
	activeColor = lipgloss.Color("#21302A")
	lineColor   = lipgloss.Color("#33443F")

	brandStyle   = lipgloss.NewStyle().Bold(true).Foreground(pageColor).Background(acidColor).Padding(0, 1)
	dimStyle     = lipgloss.NewStyle().Foreground(mutedColor)
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(inkColor)
	aliasStyle   = lipgloss.NewStyle().Bold(true).Foreground(acidColor)
	commandStyle = lipgloss.NewStyle().Foreground(mutedColor)
	statusStyle  = lipgloss.NewStyle().Foreground(acidColor)
)

type model struct {
	services *app.Services

	catalogView       *catalogTUIView
	catalogStandalone bool
	aliases           []aliasEntry
	context           app.ContextRanking
	query             string
	cursor            int
	width             int
	height            int
	status            string
	theme             themePalette
	themePicker       bool
	themeCursor       int
	themeBefore       themePalette
	settingsOpen      bool
	settingsField     int
	settingsForm      [settingsFieldCount]string
	settingsCursor    [settingsFieldCount]int
	settingsBefore    appearanceSettings
	helpVisible       bool
	helpQuery         string
	statsOpen         bool
	statsData         app.StatsData
	statsPeriod       int
	statsViewIndex    int
	statsSelected     int
	statsNow          time.Time
	statsErr          string
	browserUsage      map[string]app.AliasUsageSummary
	browserUsageState string
	browserUsageReady bool
	tourVisible       bool
	adding            bool
	field             int
	form              [5]string
	editingName       string
	editingMetadata   app.EntryMetadata
	deleteName        string
	runConfirm        *aliasEntry
	revisionOpen      bool
	revisionCursor    int
	revisions         []app.Revision
	revisionErr       string
	diff              *terminalDiff
	healthOnly        bool
	trackedOnly       bool
	tracked           []trackedFileItem
	trackedRepo       string
	trackedErr        string
	autoSyncEnabled   bool
	syncInterval      int
	primarySync       trackedFileItem
	selectMode        bool
	executeMode       bool
	selected          *aliasEntry
	editSelection     bool
	cursorHidden      bool
	terminalBlurred   bool
	shortcutProfile   shortcutProfile
	shortcutsOpen     bool
	shortcutCursor    int
	shortcutCapture   bool
	shortcutPending   string
	shortcutFilter    string
	shortcutFiltering bool
	shortcutLauncher  string
	aliasMode         aliasInputMode
	lastPageShortcut  tuiPage
	lastShortcutAt    time.Time
	executableWatch   app.ExecutableWatch
	executableUpdated bool
}

func (m model) TerminalBackground() string {
	return ""
}

func themeCanvasAvailable() bool {
	return !noColorRequested() && lipgloss.ColorProfile() <= termenv.ANSI256
}

type cursorBlinkMsg struct{}

const cursorBlinkInterval = 500 * time.Millisecond

const pageShortcutRepeatWindow = 1200 * time.Millisecond

type aliasInputMode int

const (
	aliasModeLegacy aliasInputMode = iota
	aliasModeCommand
	aliasModeSearch
)

type tuiPage int

const (
	pageAliases tuiPage = iota
	pageHelp
	pageStats
	pageSettings
	pageThemes
	pageRevisions
	pageSync
	pageHealth
	pageShortcuts
)

type trackedFileItem struct {
	Config trackedFileConfig
	State  app.SyncState
	Error  string
}

func runTUIWithDiff(services *app.Services, repositoryDiff bool) error {
	config, configErr := services.Settings.Load()
	if configErr != nil {
		return fmt.Errorf("could not read settings: %w", configErr)
	}
	var aliases []aliasEntry
	if !repositoryDiff {
		var err error
		aliases, err = services.Entries()
		if err != nil {
			return fmt.Errorf("could not read %s: %w", services.AliasDisplayPath(), err)
		}
	}
	ranking, contextErr := services.CurrentContextRanking()
	if terminalIsDumb() {
		if repositoryDiff {
			return fmt.Errorf("al diff --tui needs a terminal; use al diff for plain output")
		}
		printPlainAliasList(os.Stderr, aliases, "")
		return nil
	}
	theme, themeErr := services.LoadTheme()
	applyFooterConfig(config.Footer)
	applyAppearanceConfig(config.Appearance)
	status := ""
	if themeErr != nil {
		status = themeErr.Error()
	}
	if contextErr != nil {
		status = "Context ranking unavailable: " + contextErr.Error()
	}

	options := []tea.ProgramOption{tea.WithAltScreen(), tea.WithReportFocus()}
	var terminal *os.File
	var terminalOutput io.Writer = os.Stderr
	stdoutIsTerminal := fileIsTerminal(os.Stdout)
	if !stdoutIsTerminal {
		options = append(options, tea.WithOutput(os.Stderr))
	}
	if openedTerminal, openErr := os.OpenFile("/dev/tty", os.O_RDWR, 0); openErr == nil {
		terminal = openedTerminal
		terminalOutput = terminal
		defer terminal.Close()
		renderer := lipgloss.NewRenderer(terminal)
		if noColorRequested() {
			renderer.SetColorProfile(termenv.Ascii)
		}
		lipgloss.SetDefaultRenderer(renderer)
		options = append(options, tea.WithInput(terminal), tea.WithOutput(terminal))
	}
	applyTheme(theme)
	initial := model{services: services, aliases: aliases, context: ranking, width: 80, height: 24, theme: theme, status: status, executeMode: !repositoryDiff, tourVisible: !repositoryDiff &&
		services.TourShouldShow(), shortcutProfile: resolvedShortcutProfile(config), shortcutLauncher: launcherLabel(config), aliasMode: aliasModeCommand, executableWatch: services.WatchRunningExecutable()}
	if !repositoryDiff {
		initial.refreshBrowserUsage()
	}
	if repositoryDiff {
		if err := initial.openRepositoryDiff(); err != nil {
			return err
		}
		initial.diff.fromCLI = true
	}
	finished, err := tea.NewProgram(initial, options...).Run()
	if err != nil {
		return fmt.Errorf("could not start terminal view: %w", err)
	}
	selected, ok := finished.(model)
	if ok && selected.selected != nil {
		if selected.editSelection {
			writeAliasEditSelection(os.Stdout, terminalOutput, selected.selected.Name, stdoutIsTerminal)
		} else {
			writeAliasSelection(os.Stdout, terminalOutput, selected.selected.Name, stdoutIsTerminal)
		}
	}
	return nil
}

const editSelectionPrefix = "__alias_lens_edit__:"

func writeAliasSelection(stdout, terminal io.Writer, name string, stdoutIsTerminal bool) {
	if terminal != nil && !stdoutIsTerminal && os.Getenv("ALIAS_LENS_PROMPT_ACCEPT") == "" {
		fmt.Fprintf(terminal, "$ %s\n", name)
	}
	fmt.Fprintln(stdout, name)
}

func writeAliasEditSelection(stdout, terminal io.Writer, name string, stdoutIsTerminal bool) {
	if os.Getenv("ALIAS_LENS_PROMPT_ACCEPT") != "" {
		fmt.Fprintln(stdout, editSelectionPrefix+name)
		return
	}
	if terminal != nil && !stdoutIsTerminal {
		fmt.Fprintf(terminal, "$ %s\n", name)
	}
	if stdoutIsTerminal {
		fmt.Fprintln(stdout, name)
	}
}

func runAliasPicker(services *app.Services, query string, commandOnly, executeSelection bool) error {
	config, err := services.Settings.Load()
	if err != nil {
		return fmt.Errorf("could not read Alias Lens settings: %w", err)
	}
	aliases, err := services.Entries()
	if err != nil {
		return err
	}
	ranking, contextErr := services.CurrentContextRanking()
	if terminalIsDumb() {
		printPlainAliasList(os.Stderr, aliases, query)
		return nil
	}
	theme, _ := services.LoadTheme()
	applyFooterConfig(config.Footer)
	applyAppearanceConfig(config.Appearance)
	options := []tea.ProgramOption{tea.WithAltScreen(), tea.WithReportFocus()}
	var terminal *os.File
	var terminalOutput io.Writer = os.Stderr
	stdoutIsTerminal := fileIsTerminal(os.Stdout)
	if !stdoutIsTerminal {
		options = append(options, tea.WithOutput(os.Stderr))
	}
	if openedTerminal, openErr := os.OpenFile("/dev/tty", os.O_RDWR, 0); openErr == nil {
		terminal = openedTerminal
		terminalOutput = terminal
		defer terminal.Close()
		renderer := lipgloss.NewRenderer(terminal)
		if noColorRequested() {
			renderer.SetColorProfile(termenv.Ascii)
		}
		lipgloss.SetDefaultRenderer(renderer)
		options = append(options, tea.WithInput(terminal), tea.WithOutput(terminal))
	}
	applyTheme(theme)
	initial := model{services: services, aliases: aliases, context: ranking, query: query, width: 80, height: 24, theme: theme, selectMode: !executeSelection, executeMode: executeSelection, shortcutProfile: resolvedShortcutProfile(config), shortcutLauncher: launcherLabel(config), aliasMode: aliasModeSearch, executableWatch: services.WatchRunningExecutable()}
	initial.refreshBrowserUsage()
	if contextErr != nil {
		initial.status = "Context ranking unavailable: " + contextErr.Error()
	}
	finished, err := tea.NewProgram(initial, options...).Run()
	if err != nil {
		return err
	}
	selected, ok := finished.(model)
	if !ok || selected.selected == nil {
		return nil
	}
	if commandOnly {
		fmt.Println(selected.selected.Command)
	} else if selected.editSelection && executeSelection {
		fmt.Println(editSelectionPrefix + selected.selected.Name)
	} else if executeSelection {
		writeAliasSelection(os.Stdout, terminalOutput, selected.selected.Name, stdoutIsTerminal)
	} else {
		fmt.Println(selected.selected.Name)
	}
	return nil
}

func (model) Init() tea.Cmd { return blinkCursor() }

func blinkCursor() tea.Cmd {
	return tea.Tick(cursorBlinkInterval, func(time.Time) tea.Msg { return cursorBlinkMsg{} })
}

func (m model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case catalogTUILoadedMsg:
		m.catalogView = message.View
		return m, nil
	case catalogTUIAppliedMsg:
		if message.Err != nil {
			m.catalogView.Loading = false
			m.catalogView.Err = message.Err.Error()
			return m, nil
		}
		m.catalogView = nil
		if m.catalogStandalone {
			return m, tea.Quit
		}
		m.reloadAliasesAndTheme()
		m.status = "Catalog plan applied"
		return m, nil
	case cursorBlinkMsg:
		if !m.executableUpdated && m.executableWatch.Changed() {
			m.executableUpdated = true
		}
		if m.searchFocused() {
			m.cursorHidden = !m.cursorHidden
		} else {
			m.cursorHidden = true
		}
		return m, blinkCursor()
	case tea.FocusMsg:
		m.terminalBlurred = false
		m.cursorHidden = false
		return m, nil
	case tea.BlurMsg:
		m.terminalBlurred = true
		m.cursorHidden = true
		return m, nil
	case tea.WindowSizeMsg:
		m.width = message.Width
		m.height = message.Height
		return m, nil
	case tea.KeyReleaseMsg:
		if target, ok := pageForShortcut(message.Key, m.shortcutProfile); ok && target == m.lastPageShortcut {
			m.lastShortcutAt = time.Time{}
		}
		return m, nil
	case tea.KeyMsg:
		if m.catalogView != nil {
			return m.updateCatalogTUI(message)
		}
		if !m.selectMode && m.aliasMode == aliasModeCommand && !m.adding && !m.settingsOpen && !m.helpVisible && !m.shortcutsOpen && !m.statsOpen && !m.themePicker && !m.revisionOpen && !m.trackedOnly && m.diff == nil && matchesShortcut(message, m.shortcutProfile, shortcutCatalog) {
			m.catalogView = &catalogTUIView{Shell: m.service().ActiveShellAdapter().Name(), Loading: true}
			return m, loadCatalogTUI(m.service(), m.service().ActiveShellAdapter().Name())
		}
		if m.shortcutsOpen {
			return m.updateShortcutEditor(message)
		}
		if m.helpVisible && message.Type == tea.KeyTab {
			m.lastShortcutAt = time.Time{}
			return m.updateHelp(message)
		}
		if m.helpVisible && plainTextKey(message) {
			m.lastShortcutAt = time.Time{}
			return m.updateHelp(message)
		}
		if !m.tourVisible && !m.adding && m.deleteName == "" && m.runConfirm == nil && m.diff == nil && !m.helpVisible && !m.statsOpen && !m.settingsOpen && !m.themePicker && !m.revisionOpen && !m.trackedOnly {
			if m.aliasMode == aliasModeSearch {
				if message.Type == tea.KeyEsc {
					m.lastShortcutAt = time.Time{}
					m.aliasMode = aliasModeCommand
					return m, m.wideAliasRedraw()
				}
				if message.Type == tea.KeyBackspace || message.Type == tea.KeyDelete {
					m.lastShortcutAt = time.Time{}
					if m.query != "" {
						_, size := utf8.DecodeLastRuneInString(m.query)
						m.query = m.query[:len(m.query)-size]
						m.cursor = 0
					}
					return m, m.wideAliasRedraw()
				}
				if plainTextKey(message) {
					m.lastShortcutAt = time.Time{}
					m.healthOnly = false
					input := string(message.Runes)
					if message.Type == tea.KeySpace {
						input = " "
					}
					m.query = appendSearchQuery(m.query, input)
					m.cursor = 0
					return m, m.wideAliasRedraw()
				}
			} else if m.aliasMode == aliasModeCommand && message.Type == tea.KeyRunes && acceptsTextInput(message) && string(message.Runes) == "/" {
				m.lastShortcutAt = time.Time{}
				m.healthOnly = false
				m.aliasMode = aliasModeSearch
				m.cursorHidden = false
				return m, m.wideAliasRedraw()
			}
		}
		if m.diff != nil && !m.diff.confirmRestore {
			if _, handled := resolveShortcut(message, m.shortcutProfile, diffTranslatedShortcutActions...); handled {
				m.lastShortcutAt = time.Time{}
				return m.updateDiff(translateShortcut(message, m.shortcutProfile, diffTranslatedShortcutActions...))
			}
		}
		if m.statsOpen {
			if _, handled := resolveShortcut(message, m.shortcutProfile, statsTranslatedShortcutActions...); handled {
				m.lastShortcutAt = time.Time{}
				return m.updateStatsView(translateShortcut(message, m.shortcutProfile, statsTranslatedShortcutActions...))
			}
		}
		if m.trackedOnly && matchesShortcut(message, m.shortcutProfile, shortcutOpenDiff) {
			m.lastShortcutAt = time.Time{}
			if err := m.openRepositoryDiff(); err != nil {
				m.status = err.Error()
			}
			return m, nil
		}
		allowed := m.activeTranslatedShortcuts()
		if (m.adding || m.settingsOpen || m.helpVisible) && plainTextKey(message) {
			allowed = nil
		}
		message = translateShortcut(message, m.shortcutProfile, allowed...)
		m.cursorHidden = false
		if _, shortcut := pageForShortcut(message, m.shortcutProfile); !shortcut {
			m.lastShortcutAt = time.Time{}
		}
		if m.tourVisible {
			return m.updateTour(message)
		}
		if m.adding {
			return m.updateAddForm(message)
		}
		if m.deleteName != "" {
			return m.updateDeleteConfirmation(message)
		}
		if m.runConfirm != nil {
			return m.updateRunConfirmation(message)
		}
		if m.diff != nil {
			return m.updateDiff(message)
		}
		if m.settingsOpen && (plainTextKey(message) || !matchesShortcut(message, m.shortcutProfile, shortcutSettings) && footerSettingsHandles(message, m.shortcutProfile)) {
			return m.updateFooterSettings(message)
		}
		if updated, handled := m.updatePageNavigation(message); handled {
			return updated, nil
		}
		if m.helpVisible {
			return m.updateHelp(message)
		}
		if m.statsOpen {
			return m.updateStatsView(message)
		}
		if m.settingsOpen {
			return m.updateFooterSettings(message)
		}
		if m.themePicker {
			return m.updateThemePicker(message)
		}
		if m.revisionOpen {
			return m.updateRevisionDrawer(message)
		}
		m.status = ""
		if m.trackedOnly {
			return m.updateTrackedFiles(message)
		}
		matches := m.currentAliases()
		if matchesShortcut(message, m.shortcutProfile, shortcutRefresh) {
			m.reloadAliasesAndTheme()
			return m, m.wideAliasRedraw()
		}
		if matchesShortcut(message, m.shortcutProfile, shortcutAdd) {
			m.startAddForm()
			return m, nil
		}
		if matchesShortcut(message, m.shortcutProfile, shortcutEdit) {
			m.startEditForm(matches)
			return m, nil
		}
		if !m.selectMode && matchesShortcut(message, m.shortcutProfile, shortcutContext) {
			m.toggleSelectedContext(matches)
			return m, m.wideAliasRedraw()
		}
		if !m.selectMode && matchesShortcut(message, m.shortcutProfile, shortcutFavorite) {
			m.toggleSelectedFavorite(matches)
			return m, m.wideAliasRedraw()
		}
		if matchesShortcut(message, m.shortcutProfile, shortcutDelete) && (message.Type != tea.KeyDelete || m.query == "") {
			if len(matches) > 0 {
				m.deleteName = matches[m.cursor].Name
			}
			return m, nil
		}
		if matchesShortcut(message, m.shortcutProfile, shortcutCommit) {
			result, err := m.service().SyncRepository(false)
			if err != nil {
				m.status = err.Error()
			} else {
				m.status = result
			}
			return m, nil
		}
		previousCursor := m.cursor
		switch message.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyCtrlG:
			message, err := m.service().SyncRepository(false)
			if err != nil {
				m.status = err.Error()
			} else {
				m.status = message
			}
		case tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
			}
		case tea.KeyDown:
			if m.cursor < len(matches)-1 {
				m.cursor++
			}
		case tea.KeyPgUp:
			m.cursor = max(0, m.cursor-m.visibleCount())
		case tea.KeyPgDown:
			m.cursor = min(max(0, len(matches)-1), m.cursor+m.visibleCount())
		case tea.KeyHome:
			m.cursor = 0
		case tea.KeyEnd:
			m.cursor = max(0, len(matches)-1)
		case tea.KeyBackspace, tea.KeyDelete:
			if m.query != "" {
				_, size := utf8.DecodeLastRuneInString(m.query)
				m.query = m.query[:len(m.query)-size]
				m.cursor = 0
			}
		case tea.KeyTab, tea.KeyEnter:
			if len(matches) > 0 {
				selected := matches[m.cursor]
				if !m.service().CatalogEntryRunnable(selected) {
					m.status = "Catalog entry is " + selected.CatalogState + "; enter al catalog enable --shell " +
						m.service().ActiveShellAdapter().Name()
					return m, nil
				}
				m.editSelection = message.Type == tea.KeyTab
				if !m.editSelection && m.executeMode &&
					m.service().IsDangerousCommand(selected.Command) {
					m.runConfirm = &selected
					return m, nil
				}
				m.selected = &selected
				return m, tea.Quit
			} else if len(m.aliases) == 0 && strings.TrimSpace(m.query) == "" && !m.selectMode {
				m.startAddForm()
			}
		case tea.KeyRunes:
			if !acceptsTextInput(message) {
				return m, nil
			}
			if m.aliasMode == aliasModeCommand {
				return m, nil
			}
			m.healthOnly = false
			m.query = appendSearchQuery(m.query, string(message.Runes))
			m.cursor = 0
		}
		if m.cursor != previousCursor {
			return m, m.wideAliasRedraw()
		}
	}
	return m, nil
}

func (m *model) reloadAliasesAndTheme() {
	aliases, err := m.service().Entries()
	if err != nil {
		m.status = "Could not reload aliases: " + err.Error()
	} else {
		m.aliases = aliases
		if m.browserUsageReady {
			m.refreshBrowserUsage()
		}
		m.cursor = 0
		m.status = fmt.Sprintf("Reloaded %d aliases", len(aliases))
	}
	if ranking, err := m.service().CurrentContextRanking(); err == nil {
		m.context = ranking
	} else {
		m.status = "Context ranking unavailable: " + err.Error()
	}
	theme, err := m.service().LoadTheme()
	if err == nil {
		m.theme = theme
		applyTheme(theme)
	}
}

func (m *model) toggleSelectedContext(matches []aliasEntry) {
	if len(matches) == 0 {
		return
	}
	selected := matches[min(m.cursor, len(matches)-1)]
	count := 0
	for _, alias := range m.aliases {
		if alias.Name == selected.Name {
			count++
		}
	}
	if count > 1 {
		m.status = "Duplicate alias name; run al check before marking it"
		return
	}
	result, err := m.service().ToggleEntryContext(selected, m.context)
	if err != nil {
		m.status = err.Error()
		return
	}
	m.context = result.Ranking
	if result.Marked {
		m.status = "Marked " + selected.Name + " for this " + map[string]string{app.ContextRepository: "project", app.ContextDirectory: "folder"}[result.Kind]
	} else {
		m.status = "Removed the local context mark from " + selected.Name
	}
	for index, alias := range m.currentAliases() {
		if alias.Name == selected.Name && alias.Command == selected.Command && alias.Type == selected.Type {
			m.cursor = index
			break
		}
	}
}

func (m *model) toggleSelectedFavorite(matches []aliasEntry) {
	if len(matches) == 0 {
		return
	}
	selected := matches[min(m.cursor, len(matches)-1)]
	count := 0
	for _, alias := range m.aliases {
		if alias.Name == selected.Name {
			count++
		}
	}
	if count > 1 {
		m.status = "Duplicate alias name; run al check before changing its favorite status"
		return
	}
	metadata := entry.MetadataForAlias(selected)
	metadata.Favorite = !metadata.Favorite
	err := m.service().EditMetadata(selected.Name, metadata)
	if err != nil {
		m.status = "Could not change favorite: " + err.Error()
		return
	}
	for index := range m.aliases {
		if m.aliases[index].Name == selected.Name && m.aliases[index].Command == selected.Command && m.aliases[index].Type == selected.Type {
			m.aliases[index].Favorite = metadata.Favorite
			break
		}
	}
	if metadata.Favorite {
		m.status = "Marked " + selected.Name + " as a favorite"
	} else {
		m.status = "Removed " + selected.Name + " from favorites"
	}
	for index, alias := range m.currentAliases() {
		if alias.Name == selected.Name && alias.Command == selected.Command && alias.Type == selected.Type {
			m.cursor = index
			break
		}
	}
}

func (m *model) startEditForm(matches []aliasEntry) {
	if len(matches) == 0 {
		return
	}
	selected := matches[m.cursor]
	m.adding = true
	m.editingName = selected.Name
	m.field = 2
	m.form = [5]string{selected.Name, selected.Command, selected.Description, strings.Join(selected.Tags, ","), selected.Category}
	m.editingMetadata = app.EntryMetadata{Platforms: selected.Platforms, Favorite: selected.Favorite}
}

func (m model) updatePageNavigation(message tea.KeyMsg) (model, bool) {
	target, ok := pageForShortcut(message, m.shortcutProfile)
	if !ok || (m.selectMode && target != pageHelp) {
		return m, false
	}
	now := time.Now()
	if message.Repeat {
		if target == m.lastPageShortcut {
			m.lastShortcutAt = now
		}
		return m, true
	}
	// Legacy terminals cannot mark key repeats or releases, so hold off toggling
	// the same non-text shortcut until its press stream has gone quiet.
	legacyRepeatGuard := message.Type != tea.KeyRunes || message.Ctrl || message.Meta || message.Super || message.Alt
	if legacyRepeatGuard && target == m.lastPageShortcut && !m.lastShortcutAt.IsZero() && now.Sub(m.lastShortcutAt) < pageShortcutRepeatWindow {
		m.lastShortcutAt = now
		return m, true
	}
	m.lastPageShortcut = target
	m.lastShortcutAt = now
	if m.currentPage() == target {
		m.closePages()
		return m, true
	}

	m.closePages()
	switch target {
	case pageHelp:
		m.helpVisible = true
	case pageStats:
		m.openStatsView()
	case pageSettings:
		m.openFooterSettings()
	case pageThemes:
		m.openThemePicker()
	case pageRevisions:
		m.openRevisionDrawer()
	case pageSync:
		m.trackedOnly = true
		m.cursor = 0
		m = m.refreshTrackedFiles()
	case pageHealth:
		m.healthOnly = true
		m.query = ""
		m.cursor = 0
	}
	return m, true
}

func pageForShortcut(message tea.KeyMsg, profile shortcutProfile) (tuiPage, bool) {
	action, ok := resolveShortcut(message, profile, shortcutHelp, shortcutStats, shortcutSettings, shortcutThemes, shortcutRevisions, shortcutSync, shortcutHealth)
	if !ok {
		return pageAliases, false
	}
	switch action {
	case shortcutHelp:
		return pageHelp, true
	case shortcutStats:
		return pageStats, true
	case shortcutSettings:
		return pageSettings, true
	case shortcutThemes:
		return pageThemes, true
	case shortcutRevisions:
		return pageRevisions, true
	case shortcutSync:
		return pageSync, true
	case shortcutHealth:
		return pageHealth, true
	default:
		return pageAliases, false
	}
}

func (m model) currentPage() tuiPage {
	switch {
	case m.helpVisible:
		return pageHelp
	case m.shortcutsOpen:
		return pageShortcuts
	case m.statsOpen:
		return pageStats
	case m.settingsOpen:
		return pageSettings
	case m.themePicker:
		return pageThemes
	case m.revisionOpen:
		return pageRevisions
	case m.trackedOnly:
		return pageSync
	case m.healthOnly:
		return pageHealth
	default:
		return pageAliases
	}
}

func (m *model) closePages() {
	if m.themePicker {
		m.theme = m.themeBefore
		m.themeBefore = themePalette{}
		applyTheme(m.theme)
	}
	if m.settingsOpen {
		applyAppearanceConfig(m.settingsBefore.Appearance)
		applyFooterConfig(m.settingsBefore.Footer)
	}
	m.helpVisible = false
	m.shortcutsOpen = false
	m.shortcutCapture = false
	m.shortcutPending = ""
	m.shortcutFiltering = false
	m.shortcutFilter = ""
	m.helpQuery = ""
	m.statsOpen = false
	m.settingsOpen = false
	m.settingsField = 0
	m.settingsForm = [settingsFieldCount]string{}
	m.settingsCursor = [settingsFieldCount]int{}
	m.settingsBefore = appearanceSettings{}
	m.themePicker = false
	m.revisionOpen = false
	m.revisions = nil
	m.revisionErr = ""
	m.diff = nil
	m.trackedOnly = false
	m.healthOnly = false
	m.status = ""
}

func (m model) View() string {
	if message := smallTerminalMessage(m.width, m.height); message != "" {
		return message
	}
	frame := newMainTUIFrame(m.width, m.height)
	height, contentWidth := frame.height, frame.contentWidth
	matches := m.currentAliases()
	cursor := min(m.cursor, max(0, len(matches)-1))

	headerDetails := fmt.Sprintf("  •  %d aliases", len(m.aliases))
	if m.diff != nil && m.diff.fromCLI {
		headerDetails = "  •  repository diff"
	}
	if issues := healthIssueCount(m.service(),

		m.aliases); issues > 0 {
		label := "issues"
		if issues == 1 {
			label = "issue"
		}
		headerDetails += fmt.Sprintf("  •  %d %s", issues, label)
	}
	brand := ""
	if label := compactBrandLabel(); label != "" {
		brand = brandStyle.Render(label) + "  "
	}
	header := brand + lipgloss.NewStyle().Foreground(cyanColor).Render(m.service().AliasDisplayPath()) + dimStyle.Render(headerDetails)
	if m.executableUpdated {
		notice := wrapText("Alias Lens was updated. Close this screen, then enter al again.", contentWidth)
		header += "\n" + lipgloss.NewStyle().Bold(true).Foreground(amberColor).Render(notice)
	}
	title := titleStyle.Render("Aliases")
	if m.selectMode {
		title = titleStyle.Render("Choose an alias")
	} else if m.executeMode {
		title = titleStyle.Render("Choose an alias to use.")
		if contentWidth >= 78 {
			title += "  " + dimStyle.Render("Press Enter to use it. Esc leaves without choosing.")
		}
	}
	if len(m.aliases) == 0 {
		title = pixelIconLabel(iconAlias, "Set up your first shortcut.", titleStyle) + "\n" + dimStyle.Render("Create an alias here or add one to the active alias file.")
		if m.selectMode {
			title = pixelIconLabel(iconAlias, "No aliases are available to select.", titleStyle) + "\n" + dimStyle.Render("Open Alias Lens normally to create one.")
		}
	}
	if height < 20 {
		title = strings.SplitN(title, "\n", 2)[0]
	}
	if m.catalogView != nil {
		return m.catalogTUIView(frame, header)
	}
	if m.tourVisible {
		return m.tourView(frame, header)
	}
	if m.adding {
		return m.addFormView(frame, header)
	}
	if m.themePicker {
		return m.themePickerView(frame, header)
	}
	if m.settingsOpen {
		return m.footerSettingsView(frame, header)
	}
	if m.helpVisible {
		return m.helpView(frame, header)
	}
	if m.shortcutsOpen {
		return m.shortcutEditorView(frame, header)
	}
	if m.statsOpen {
		return m.statsView(frame, header)
	}
	if m.runConfirm != nil {
		return m.runConfirmationView(frame, header)
	}
	if m.diff != nil {
		return m.diffView(frame, header)
	}
	if m.revisionOpen {
		return m.revisionDrawerView(frame, header)
	}
	if m.trackedOnly {
		return m.trackedFilesView(frame, header)
	}
	wideAliasBrowser := m.width >= 120 && len(matches) > 0 && !m.healthOnly
	if wideAliasBrowser {
		frame = newFullWidthTUIFrame(m.width, m.height, mainTUIHorizontalPadding)
		contentWidth = frame.contentWidth
	}
	searchPlaceholder := "search aliases…"
	if m.aliasMode == aliasModeCommand {
		searchPlaceholder = "press / to search aliases…"
	}
	searchFocused := m.searchFocused()
	searchPrefix := markerPrefix(iconSearch)
	searchBorder := lineColor
	if searchFocused {
		searchPrefix += "SEARCH "
		searchBorder = acidColor
	}
	search := lipgloss.NewStyle().Width(searchFieldWidth(contentWidth)-2).Padding(0, 1).Border(lipgloss.RoundedBorder()).BorderForeground(searchBorder).Render(acidStyle(searchPrefix) + searchTextWithCursorAtWidth(m.query, searchPlaceholder, searchFocused && !m.cursorHidden, searchFieldWidth(contentWidth)-lipgloss.Width(searchPrefix)+lipgloss.Width(markerPrefix(iconSearch))))

	moveKeys := primaryShortcutLabel(m.shortcutProfile, shortcutMoveUp) + "/" + primaryShortcutLabel(m.shortcutProfile, shortcutMoveDown)
	if moveKeys == "Up/Down" {
		moveKeys = "↑↓"
	}
	useKey := strings.ToLower(primaryShortcutLabel(m.shortcutProfile, shortcutUse))
	promptKey := strings.ToLower(primaryShortcutLabel(m.shortcutProfile, shortcutPrompt))
	helpKey := primaryShortcutLabel(m.shortcutProfile, shortcutHelp)
	quitKey := strings.ToLower(primaryShortcutLabel(m.shortcutProfile, shortcutQuit))
	footer := dimStyle.Render(moveKeys+" move  ·  "+useKey+" ") + cyanStyle("select") + dimStyle.Render("  ·  "+promptKey+" edit at prompt  ·  "+helpKey+" ") + cyanStyle("help") + dimStyle.Render("  ·  "+quitKey+" quit")
	if contentWidth < 96 {
		footer = dimStyle.Render(useKey+" ") + cyanStyle("select") + dimStyle.Render("  ·  "+primaryShortcutLabel(m.shortcutProfile, shortcutStats)+" stats  ·  "+helpKey+" help  ·  "+quitKey+" quit")
	}
	if m.selectMode {
		footer = dimStyle.Render("type · " + moveKeys + " move · " + useKey + " select · " + helpKey + " help · " + quitKey + " cancel")
		if contentWidth >= 96 {
			footer = dimStyle.Render("type to search  ·  " + moveKeys + " move  ·  " + useKey + " select  ·  " + helpKey + " help  ·  " + quitKey + " cancel")
		}
	} else if m.executeMode {
		footer = dimStyle.Render(useKey+" ") + cyanStyle("use") + dimStyle.Render("  ·  "+promptKey+" edit  ·  "+primaryShortcutLabel(m.shortcutProfile, shortcutStats)+" stats  ·  "+helpKey+" help  ·  "+quitKey+" quit")
		if contentWidth >= 96 {
			footer = dimStyle.Render(moveKeys+" move  ·  "+useKey+" ") + cyanStyle("use") + dimStyle.Render("  ·  "+promptKey+" edit  ·  "+helpKey+" help  ·  "+quitKey+" quit")
		}
	}
	if m.aliasMode == aliasModeCommand && !m.selectMode {
		footer = dimStyle.Render("command mode  ·  l catalog  ·  / search  ·  " + primaryShortcutLabel(m.shortcutProfile, shortcutFavorite) + " favorite  ·  " + useKey + " use  ·  " + helpKey + " help  ·  " + quitKey + " quit")
	} else if m.aliasMode == aliasModeSearch && !m.selectMode {
		footer = dimStyle.Render("search mode  ·  type to filter  ·  esc commands  ·  " + useKey + " use")
	}
	if contentWidth < 50 {
		action := "select"
		if m.executeMode {
			action = "use"
		}
		footer = dimStyle.Render(useKey+" ") + cyanStyle(action) + dimStyle.Render("  ·  "+helpKey+" help  ·  "+quitKey+" quit")
		if m.selectMode {
			footer = dimStyle.Render(useKey + " select  ·  " + helpKey + " help  ·  " + quitKey + " cancel")
		} else if m.aliasMode == aliasModeCommand {
			footer = dimStyle.Render("/ search  ·  " + primaryShortcutLabel(m.shortcutProfile, shortcutFavorite) + " favorite  ·  " + helpKey + " help")
		} else if m.aliasMode == aliasModeSearch {
			footer = dimStyle.Render("type to search  ·  esc commands")
		}
	}
	if contentWidth >= 79 && !m.selectMode {
		footer = footerWithNavigation(footer, contentWidth, m.shortcutProfile)
	}
	if m.status != "" {
		footer = statusStyle.Render(truncate(m.status, contentWidth))
	}
	if m.deleteName != "" {
		footer = lipgloss.NewStyle().Bold(true).Foreground(coralColor).Render("Delete " + m.deleteName + "?  " + shortcutLabel(m.shortcutProfile, shortcutConfirm) + " confirm  ·  " + shortcutLabel(m.shortcutProfile, shortcutDecline) + " cancel")
	}

	overview := ""
	if len(m.aliases) > 0 && contentWidth >= 68 && height >= 24 {
		overview = aliasOverview(m.aliases)
	}
	contentHeight := frame.contentHeight()
	bodyBudget := max(1, contentHeight-frame.measureHeight(header)-frame.measureHeight(title)-frame.measureHeight(search)-frame.measureHeight(overview)-frame.measureHeight(footer)-frame.makerHeight()-4)
	var body strings.Builder
	bodyLeadHeight := 0
	if wideAliasBrowser {
		body.WriteString(m.wideAliasBrowser(matches, cursor, contentWidth, bodyBudget))
	} else if len(m.aliases) == 0 && strings.TrimSpace(m.query) == "" && !m.healthOnly {
		body.WriteString(m.emptyStateView(contentWidth, height))
	} else if m.healthOnly {
		lead := pixelIconLabel(iconHealth, "ALIAS HEALTH", lipgloss.NewStyle().Bold(true).Foreground(coralColor))
		body.WriteString(lead)
		body.WriteByte('\n')
		bodyLeadHeight = lipgloss.Height(lead)
	} else if strings.TrimSpace(m.query) == "" {
		label := "ALIASES"
		if contentWidth >= 68 {
			if m.context.Working.Repository != "" {
				label += "  ·  PROJECT " + truncate(presentation.TerminalSafeText(filepath.Base(m.context.Working.Repository)), max(8, contentWidth-42))
			} else if m.context.Working.Directory != "" {
				label += "  ·  THIS FOLDER"
			}
		}
		lead := pixelIconLabel(iconSpark, label, lipgloss.NewStyle().Bold(true).Foreground(amberColor))
		body.WriteString(lead)
		body.WriteByte('\n')
		bodyLeadHeight = lipgloss.Height(lead)
	}
	if !wideAliasBrowser {
		if len(m.aliases) == 0 && strings.TrimSpace(m.query) == "" && !m.healthOnly {
			// The empty state above replaces the normal search result list.
		} else if m.healthOnly && len(matches) == 0 {
			body.WriteString(statusStyle.Render("No health issues found."))
		} else if len(matches) == 0 {
			body.WriteString(titleStyle.Render(fmt.Sprintf("No alias matched %q", m.query)))
			if close := closestAliases(m.service(),

				m.aliases, m.query); len(close) > 0 {
				body.WriteString("\n" + dimStyle.Render("Did you mean ") + aliasStyle.Render(strings.Join(close, "  ")) + dimStyle.Render(" ?"))
			}
		} else {
			aliasBudget := max(3, bodyBudget-bodyLeadHeight-1)
			start, end := aliasWindow(matches, cursor, contentWidth, aliasBudget)
			for index := start; index < end; index++ {
				body.WriteString(renderAlias(matches[index], index == cursor, contentWidth, m.context.Match(matches[index]) > 0))
				if index < end-1 {
					body.WriteByte('\n')
				}
			}
			summary := fmt.Sprintf("Showing %d-%d of %d · ↑↓ browse · / search", start+1, end, len(matches))
			if strings.TrimSpace(m.query) == "" && len(matches) < len(m.aliases) {
				summary = fmt.Sprintf("Showing %d-%d of %d suggestions · / search all %d aliases", start+1, end, len(matches), len(m.aliases))
			}
			body.WriteString("\n" + dimStyle.Render(summary))
		}
	}
	sections := []string{header, "", title, search}
	if overview != "" {
		sections = append(sections, overview)
	}
	sections = append(sections, "", body.String())
	page := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return frame.renderWithFooter(page, footer)
}

func aliasOverview(aliases []aliasEntry) string {
	favorites := 0
	affected := 0
	categories := make(map[string]struct{})
	for _, alias := range aliases {
		if alias.Favorite {
			favorites++
		}
		if len(alias.Issues) > 0 {
			affected++
		}
		if alias.Category != "" {
			categories[alias.Category] = struct{}{}
		}
	}
	attention := fmt.Sprintf("%d need attention", affected)
	if affected == 1 {
		attention = "1 needs attention"
	}
	attentionStyle := dimStyle
	if affected > 0 {
		attentionStyle = lipgloss.NewStyle().Foreground(coralColor)
	}
	return dimStyle.Render(fmt.Sprintf("%d favorites  │  %d categories  │  ", favorites, len(categories))) + attentionStyle.Render(attention)
}

func (m *model) startAddForm() {
	m.adding = true
	m.field = 0
	m.form = [5]string{}
	m.editingName = ""
	m.editingMetadata = app.EntryMetadata{}
}

func (m model) emptyStateView(contentWidth, height int) string {
	var body strings.Builder
	if contentWidth < 60 {
		if m.selectMode {
			return dimStyle.Render("Open ") + cyanStyle("al") + dimStyle.Render(" and press ") + aliasStyle.Render(shortcutLabel(m.shortcutProfile, shortcutAdd)) + dimStyle.Render(" to create an alias.")
		}
		body.WriteString(aliasStyle.Render(shortcutLabel(m.shortcutProfile, shortcutAdd)) + dimStyle.Render("  Create your first alias"))
		body.WriteString("\n" + dimStyle.Render(shortcutLabel(m.shortcutProfile, shortcutRefresh)) + dimStyle.Render("  Reload aliases"))
		return body.String()
	}
	if mark := fullBrandMark(); mark != "" && height >= 24 {
		body.WriteString(mark + "\n")
	}
	body.WriteString(pixelIconLabel(iconAlias, "No aliases yet.", titleStyle))
	body.WriteString("\n" + dimStyle.Render(wrapText("Alias Lens is reading "+
		m.service().AliasDisplayPath()+" for "+
		m.service().ActiveShellAdapter().DisplayName()+".", contentWidth)))
	if m.selectMode {
		body.WriteString("\n\n" + dimStyle.Render("Open ") + cyanStyle("al") + dimStyle.Render(" and press ") + aliasStyle.Render(shortcutLabel(m.shortcutProfile, shortcutAdd)) + dimStyle.Render(" to create one."))
		return body.String()
	}
	body.WriteString("\n\n" + aliasStyle.Render("Enter") + dimStyle.Render(" or ") + aliasStyle.Render(shortcutLabel(m.shortcutProfile, shortcutAdd)) + dimStyle.Render("  Create your first alias"))
	body.WriteString("\n" + dimStyle.Render(shortcutLabel(m.shortcutProfile, shortcutRefresh)) + dimStyle.Render("  Reload aliases added outside Alias Lens"))
	return body.String()
}

func (m model) updateHelp(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if message.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	if message.Type == tea.KeyEsc || matchesShortcut(message, m.shortcutProfile, shortcutHelp) && (!plainTextKey(message) || message.Type == tea.KeyRunes && string(message.Runes) == "?") {
		m.helpVisible = false
		m.helpQuery = ""
		return m, nil
	}
	if message.Type == tea.KeyTab {
		m.helpVisible = false
		m.shortcutsOpen = true
		m.shortcutCursor = 0
		m.shortcutFilter = ""
		m.status = ""
		return m, nil
	}
	switch message.Type {
	case tea.KeyBackspace, tea.KeyDelete:
		if m.helpQuery != "" {
			_, size := utf8.DecodeLastRuneInString(m.helpQuery)
			m.helpQuery = m.helpQuery[:len(m.helpQuery)-size]
		}
	case tea.KeySpace:
		if !acceptsTextInput(message) {
			return m, nil
		}
		m.helpQuery = appendSearchQuery(m.helpQuery, " ")
	case tea.KeyRunes:
		if !acceptsTextInput(message) {
			return m, nil
		}
		m.helpQuery = appendSearchQuery(m.helpQuery, string(message.Runes))
	}
	return m, nil
}

func (m model) updateRunConfirmation(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if message.Type == tea.KeyCtrlC {
		m.runConfirm = nil
		return m, tea.Quit
	}
	if message.Type == tea.KeyEsc || matchesShortcut(message, m.shortcutProfile, shortcutDecline) {
		m.runConfirm = nil
		m.status = "Canceled"
		return m, nil
	}
	if !matchesShortcut(message, m.shortcutProfile, shortcutConfirm) {
		return m, nil
	}
	selected := *m.runConfirm
	m.runConfirm = nil
	m.selected = &selected
	return m, tea.Quit
}

func (m model) runConfirmationView(frame tuiFrame, header string) string {
	contentWidth := frame.contentWidth
	if m.runConfirm == nil {
		return ""
	}
	alias := *m.runConfirm
	reasons := m.service().DangerousCommandReasons(alias.Command)
	reason := "Alias Lens marked this command for review."
	if len(reasons) > 0 {
		reason = "Why: " + strings.Join(reasons, "; ") + "."
	}

	name := lipgloss.NewStyle().Bold(true).Foreground(amberColor).Render(presentation.TerminalSafeText(alias.Name))
	command := lipgloss.NewStyle().Width(max(32, contentWidth-4)).Padding(1, 2).Foreground(inkColor).Background(panelColor).Border(lipgloss.ThickBorder(), false, false, false, true).BorderForeground(coralColor).Render(markerPrefix(iconCommand) + wrapText(presentation.TerminalSafeText(alias.Command), max(24, contentWidth-13)))
	body := pixelIconLabel(iconHealth, "Review before using this alias", titleStyle) +
		"\n" + dimStyle.Render("Alias ") + name + dimStyle.Render(" may make changes that are hard to undo.") +
		"\n\n" + command +
		"\n\n" + lipgloss.NewStyle().Foreground(coralColor).Render(wrapText(reason, contentWidth))
	footer := aliasStyle.Render(shortcutLabel(m.shortcutProfile, shortcutConfirm)) + dimStyle.Render(" use alias  ·  ") + aliasStyle.Render(shortcutLabel(m.shortcutProfile, shortcutDecline)) + dimStyle.Render(" or "+strings.ToLower(primaryShortcutLabel(m.shortcutProfile, shortcutQuit))+" cancel")
	page := lipgloss.JoinVertical(lipgloss.Left, header, "", body)
	return frame.renderWithFooter(page, footer)
}

func (m model) helpView(frame tuiFrame, header string) string {
	contentWidth := frame.contentWidth
	shortcuts := shortcutGuide(m.shortcutProfile, m.selectMode)
	if query := strings.TrimSpace(strings.ToLower(m.helpQuery)); query != "" {
		filtered := shortcuts[:0]
		for _, shortcut := range shortcuts {
			if strings.Contains(strings.ToLower(shortcut[0]+" "+shortcut[1]), query) {
				filtered = append(filtered, shortcut)
			}
		}
		shortcuts = filtered
	}

	keyWidth := 14
	for _, shortcut := range shortcuts {
		keyWidth = max(keyWidth, lipgloss.Width(shortcut[0])+1)
	}
	keyWidth = min(keyWidth, 28)
	if contentWidth < 60 {
		keyWidth = min(keyWidth, max(14, contentWidth/2))
	}
	searchPrefix := markerPrefix(iconHelp) + "FILTER "
	search := lipgloss.NewStyle().Width(searchFieldWidth(contentWidth)-2).Padding(0, 1).Border(lipgloss.RoundedBorder()).BorderForeground(acidColor).Render(acidStyle(searchPrefix) + searchTextWithCursorAtWidth(m.helpQuery, "filter shortcuts…", true, searchFieldWidth(contentWidth)-lipgloss.Width(searchPrefix)+lipgloss.Width(markerPrefix(iconSearch))))
	title := pixelIconLabel(iconHelp, "Keyboard guide", titleStyle) + "\n" + dimStyle.Render("Type to filter commands and shortcuts.")
	footerText := "tab configure shortcuts  ·  esc close"
	footerView := footerWithNavigation(dimStyle.Render(wrapText(footerText, contentWidth)), contentWidth, m.shortcutProfile)
	rowBudget := max(1, frame.contentHeight()-frame.measureHeight(header)-frame.measureHeight(title)-frame.measureHeight(search)-frame.measureHeight(footerView)-frame.makerHeight()-4)
	var rows strings.Builder
	visible := min(len(shortcuts), rowBudget)
	for index, shortcut := range shortcuts[:visible] {
		keyLabel := truncate(shortcut[0], max(1, keyWidth-1))
		key := aliasStyle.Render(fmt.Sprintf("%-*s", keyWidth, keyLabel))
		descriptionWidth := max(12, contentWidth-keyWidth-2)
		rows.WriteString(key + dimStyle.Render(truncate(shortcut[1], descriptionWidth)))
		if index < len(shortcuts)-1 {
			rows.WriteByte('\n')
		}
	}
	if len(shortcuts) == 0 {
		rows.WriteString(dimStyle.Render("No shortcut matched " + fmt.Sprintf("%q", m.helpQuery)))
	}

	if len(shortcuts) > visible {
		footerText = fmt.Sprintf("showing %d of %d  ·  tab configure  ·  esc close", visible, len(shortcuts))
		footerView = footerWithNavigation(dimStyle.Render(wrapText(footerText, contentWidth)), contentWidth, m.shortcutProfile)
	}
	page := lipgloss.JoinVertical(lipgloss.Left, header, "", title, "", search, "", rows.String())
	return frame.renderWithFooter(page, footerView)
}

func pageNavigationHint(width int, profiles ...shortcutProfile) string {
	if width < 79 {
		return ""
	}
	profile := shortcutLinux
	if len(profiles) > 0 {
		profile = profiles[0]
	}
	labels := []string{
		primaryShortcutLabel(profile, shortcutHelp),
		primaryShortcutLabel(profile, shortcutStats),
		primaryShortcutLabel(profile, shortcutSettings),
		primaryShortcutLabel(profile, shortcutThemes),
		primaryShortcutLabel(profile, shortcutRevisions),
		primaryShortcutLabel(profile, shortcutSync),
		primaryShortcutLabel(profile, shortcutHealth),
	}
	formatHint := func() string {
		return fmt.Sprintf("%s help  ·  %s stats  ·  %s footer  ·  %s themes  ·  %s versions  ·  %s sync  ·  %s health",
			labels[0], labels[1], labels[2], labels[3], labels[4], labels[5], labels[6])
	}
	navigation := formatHint()
	if lipgloss.Width(navigation) > width {
		for index := range labels {
			labels[index] = compactKeyLabel(labels[index])
		}
		navigation = formatHint()
	}
	return navigation
}

func compactKeyLabel(label string) string {
	label = strings.ReplaceAll(label, "Cmd+Shift+", "⌘⇧")
	label = strings.ReplaceAll(label, "Cmd+", "⌘")
	return strings.ReplaceAll(label, "Ctrl+", "^")
}

func (m *model) openThemePicker() {
	m.themePicker = true
	m.themeBefore = m.theme
	m.themeCursor = 0
	for index, theme := range availableThemes() {
		if theme.Preset == m.theme.Preset {
			m.themeCursor = index
			break
		}
	}
	m.status = ""
}

func (m model) updateThemePicker(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	themes := availableThemes()
	if len(themes) == 0 {
		m.themePicker = false
		return m, nil
	}
	switch message.Type {
	case tea.KeyCtrlC:
		m.theme = m.themeBefore
		applyTheme(m.theme)
		return m, tea.Quit
	case tea.KeyEsc:
		m.theme = m.themeBefore
		m.themePicker = false
		m.themeBefore = themePalette{}
		m.status = "Theme unchanged"
		applyTheme(m.theme)
		return m, nil
	case tea.KeyEnter:
		if err := m.service().SaveTheme(m.theme); err != nil {
			m.status = "Could not save theme: " + err.Error()
			return m, nil
		}
		m.themePicker = false
		m.themeBefore = themePalette{}
		m.status = "Theme: " + m.theme.Name
		return m, nil
	case tea.KeyUp:
		m.themeCursor = max(0, m.themeCursor-1)
	case tea.KeyDown:
		m.themeCursor = min(len(themes)-1, m.themeCursor+1)
	case tea.KeyPgUp:
		m.themeCursor = max(0, m.themeCursor-m.themePickerVisibleCount())
	case tea.KeyPgDown:
		m.themeCursor = min(len(themes)-1, m.themeCursor+m.themePickerVisibleCount())
	case tea.KeyHome:
		m.themeCursor = 0
	case tea.KeyEnd:
		m.themeCursor = len(themes) - 1
	default:
		return m, nil
	}
	m.theme = themes[m.themeCursor]
	m.status = ""
	applyTheme(m.theme)
	return m, nil
}

func (m model) themePickerVisibleCount() int {
	return max(5, m.height-12)
}

func (m model) themePickerView(frame tuiFrame, header string) string {
	contentWidth := frame.contentWidth
	themes := availableThemes()
	title := pixelIconLabel(iconTheme, "Choose a theme", titleStyle) + "\n" + dimStyle.Render("The preview changes as you move. Save only when it looks right.")
	footer := dimStyle.Render("↑↓ preview  ·  pgup/pgdn jump  ·  enter ") + cyanStyle("save") + dimStyle.Render("  ·  esc restore")
	footer = footerWithNavigation(footer, contentWidth, m.shortcutProfile)
	if m.status != "" {
		footer = lipgloss.NewStyle().Foreground(coralColor).Render(truncate(m.status, contentWidth))
	}
	rowBudget := max(1, frame.contentHeight()-frame.measureHeight(header)-frame.measureHeight(title)-frame.measureHeight(footer)-frame.makerHeight()-3)
	cursor := min(max(0, m.themeCursor), max(0, len(themes)-1))
	visible := min(len(themes), rowBudget)
	if len(themes) > visible && visible > 1 {
		visible--
	}
	start := max(0, cursor-visible/2)
	start = min(start, max(0, len(themes)-visible))
	end := min(len(themes), start+visible)

	var rows strings.Builder
	for index := start; index < end; index++ {
		theme := themes[index]
		marker := "  "
		rowStyle := lipgloss.NewStyle().Width(contentWidth-2).Padding(0, 1).Foreground(mutedColor)
		if index == cursor {
			marker = "▶ "
			rowStyle = rowStyle.Bold(true).Foreground(inkColor).Background(activeColor)
		}
		swatches := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Accent)).Render("●") +
			lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Secondary)).Render("●") +
			lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Git)).Render("●") +
			lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Files)).Render("●") +
			lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Dev)).Render("●")
		labelWidth := max(12, contentWidth-25)
		label := fmt.Sprintf("%s%-*s", marker, labelWidth, truncate(theme.Name, labelWidth))
		rows.WriteString(rowStyle.Render(label + "  " + swatches))
		if index < end-1 {
			rows.WriteByte('\n')
		}
	}
	if len(themes) > visible {
		rows.WriteString("\n" + dimStyle.Render(matchSummary(start, end, len(themes))))
	}

	page := lipgloss.JoinVertical(lipgloss.Left, header, "", title, "", rows.String())
	return frame.renderWithFooter(page, footer)
}

func (m model) refreshTrackedFiles() model {
	snapshot, err := m.service().BrowserSyncStatus()
	if err != nil {
		m.tracked = nil
		m.trackedRepo = ""
		m.trackedErr = err.Error()
		m.autoSyncEnabled = false
		m.syncInterval = 0
		m.primarySync = trackedFileItem{}
		return m
	}
	m.trackedRepo = snapshot.Repository
	m.trackedErr = ""
	m.autoSyncEnabled = snapshot.Enabled
	m.syncInterval = snapshot.IntervalSeconds
	m.primarySync = trackedFileItem{Config: snapshot.Primary.File, State: snapshot.Primary.State}
	if snapshot.Primary.Error != nil {
		m.primarySync.Error = snapshot.Primary.Error.Error()
	}
	m.tracked = make([]trackedFileItem, 0, len(snapshot.Tracked))
	for _, unit := range snapshot.Tracked {
		item := trackedFileItem{Config: unit.File, State: unit.State}
		if unit.Error != nil {
			item.Error = unit.Error.Error()
		}
		m.tracked = append(m.tracked, item)
	}
	return m
}

func (m model) updateTrackedFiles(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if matchesShortcut(message, m.shortcutProfile, shortcutRefresh) {
		m = m.refreshTrackedFiles()
		m.status = "Sync status refreshed"
		return m, nil
	}
	if matchesShortcut(message, m.shortcutProfile, shortcutCommit) {
		result, err := m.service().SyncRepository(false)
		m = m.refreshTrackedFiles()
		if err != nil {
			m.status = err.Error()
		} else {
			m.status = result
		}
		return m, nil
	}
	switch message.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		m.trackedOnly = false
		m.cursor = 0
	case tea.KeyCtrlG:
		message, err := m.service().SyncRepository(false)
		m = m.refreshTrackedFiles()
		if err != nil {
			m.status = err.Error()
		} else {
			m.status = message
		}
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyDown:
		if m.cursor < len(m.tracked)-1 {
			m.cursor++
		}
	case tea.KeyPgUp:
		m.cursor = max(0, m.cursor-m.trackedVisibleCount())
	case tea.KeyPgDown:
		m.cursor = min(max(0, len(m.tracked)-1), m.cursor+m.trackedVisibleCount())
	case tea.KeyHome:
		m.cursor = 0
	case tea.KeyEnd:
		m.cursor = max(0, len(m.tracked)-1)
	case tea.KeyRunes:
		if message.String() == "d" {
			if err := m.openRepositoryDiff(); err != nil {
				m.status = err.Error()
			}
		}
	}
	return m, nil
}

func (m model) trackedFilesView(frame tuiFrame, header string) string {
	contentWidth := frame.contentWidth
	var body strings.Builder
	body.WriteString(pixelIconLabel(iconSync, "SYNC STATUS", lipgloss.NewStyle().Bold(true).Foreground(amberColor)))
	autoLabel := "AUTO OFF"
	autoColor := mutedColor
	if m.autoSyncEnabled {
		autoLabel = "AUTO ON"
		autoColor = acidColor
	}
	body.WriteString("  " + lipgloss.NewStyle().Bold(true).Foreground(pageColor).Background(autoColor).Padding(0, 1).Render(autoLabel))
	if m.autoSyncEnabled && m.syncInterval > 0 {
		body.WriteString(dimStyle.Render(fmt.Sprintf("  every %ds", m.syncInterval)))
	}
	body.WriteByte('\n')
	if m.trackedRepo == "" {
		body.WriteString(dimStyle.Render("Repository: not configured"))
	} else {
		body.WriteString(dimStyle.Render("Repository: ") + cyanStyle(compactHomePath(m.trackedRepo)))
	}
	if frame.height < 24 {
		body.WriteByte('\n')
	} else {
		body.WriteString("\n\n")
	}

	if m.trackedErr != "" {
		body.WriteString(lipgloss.NewStyle().Foreground(coralColor).Render(wrapText("Could not load sync status: "+m.trackedErr, contentWidth)))
	} else {
		body.WriteString(pixelIconLabel(iconAlias, "PRIMARY ALIAS FILE", titleStyle))
		body.WriteString("\n" + renderTrackedFile(m.primarySync, false, contentWidth))
		sectionSeparator := "\n\n"
		if frame.height < 24 {
			sectionSeparator = "\n"
		}
		body.WriteString(sectionSeparator + pixelIconLabel(iconRepository, "EXTRA TRACKED FILES", lipgloss.NewStyle().Bold(true).Foreground(amberColor)))
		body.WriteString(dimStyle.Render(fmt.Sprintf("  %d enrolled", len(m.tracked))))
		body.WriteString("\n")
		if len(m.tracked) == 0 {
			body.WriteString(dimStyle.Render("None. Add one with ") + cyanStyle("al track PATH") + dimStyle.Render("."))
		}
	}
	if m.trackedErr == "" && len(m.tracked) > 0 {
		cursor := min(m.cursor, len(m.tracked)-1)
		visible := m.trackedVisibleCount()
		start := 0
		if cursor >= visible {
			start = cursor - visible + 1
		}
		end := min(len(m.tracked), start+visible)
		for index := start; index < end; index++ {
			body.WriteString(renderTrackedFile(m.tracked[index], index == cursor, contentWidth))
			if index < end-1 {
				body.WriteByte('\n')
			}
		}
		if len(m.tracked) > visible {
			body.WriteString("\n" + dimStyle.Render(matchSummary(start, end, len(m.tracked))))
		}
	}

	footer := dimStyle.Render("↑↓ move  ·  " + shortcutLabel(m.shortcutProfile, shortcutOpenDiff) + " compare aliases  ·  " + strings.ToLower(shortcutLabel(m.shortcutProfile, shortcutCommit)) + " save to repo  ·  " + strings.ToLower(shortcutLabel(m.shortcutProfile, shortcutRefresh)) + " refresh  ·  " + strings.ToLower(shortcutLabel(m.shortcutProfile, shortcutSync)) + " or " + strings.ToLower(primaryShortcutLabel(m.shortcutProfile, shortcutQuit)) + " aliases")
	if m.status != "" {
		footer = statusStyle.Render(truncate(m.status, contentWidth))
	}
	footer = footerWithNavigation(footer, contentWidth, m.shortcutProfile)
	sections := []string{header, "", pixelIconLabel(iconSync, "See what Alias Lens keeps in sync.", titleStyle), "", body.String()}
	if frame.height < 24 {
		sections = []string{header, body.String()}
	}
	page := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return frame.renderWithFooter(page, footer)
}

func (m model) trackedVisibleCount() int { return max(1, (m.height-21)/5) }

func renderTrackedFile(item trackedFileItem, active bool, width int) string {
	cardWidth := max(34, width-3)
	marker := "  "
	if active {
		marker = "▶ "
	}
	status := presentation.DefaultString(item.State.Status, "waiting")
	if item.Config.RepositoryPath == "" {
		status = "not set"
	}
	if item.Error != "" {
		status = "error"
	}
	statusColor := mutedColor
	switch status {
	case "synced", "pushed", "pulled":
		statusColor = acidColor
	case "conflict", "error":
		statusColor = coralColor
	case "offline":
		statusColor = amberColor
	}
	statusBadge := lipgloss.NewStyle().Bold(true).Foreground(pageColor).Background(statusColor).Padding(0, 1).Render(strings.ToUpper(status))
	lineOne := aliasStyle.Render(marker+compactHomePath(item.Config.Source)) + "  " + statusBadge
	lineTwo := dimStyle.Render("No repository configured")
	if item.Config.RepositoryPath != "" {
		lineTwo = cyanStyle(markerPrefix(iconRepository)+"repo/") + lipgloss.NewStyle().Foreground(inkColor).Render(truncate(filepath.ToSlash(item.Config.RepositoryPath), cardWidth-10))
	}
	detail := item.State.Message
	if item.Error != "" {
		detail = item.Error
	}
	if detail == "" {
		if item.Config.RepositoryPath == "" {
			detail = "Configure a repository with al repo"
		} else {
			detail = "Waiting for the first sync cycle"
		}
	}
	lineThree := dimStyle.Render(wrapText(detail, cardWidth-6))
	if !item.State.UpdatedAt.IsZero() {
		lineThree += "\n" + dimStyle.Render("Updated "+item.State.UpdatedAt.Local().Format("Jan 2, 15:04"))
	}
	borderColor := lineColor
	if active {
		borderColor = acidColor
	}
	return lipgloss.NewStyle().Width(cardWidth).Padding(0, 1).Border(lipgloss.ThickBorder(), false, false, false, true).BorderForeground(borderColor).Render(lineOne + "\n" + lineTwo + "\n" + lineThree)
}

func compactHomePath(path string) string {
	home, err := os.UserHomeDir()
	if err == nil && path == home {
		return "~"
	}
	if err == nil && strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~/" + filepath.ToSlash(strings.TrimPrefix(path, home+string(filepath.Separator)))
	}
	return filepath.ToSlash(path)
}

func (m model) updateAddForm(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.status = ""
	if matchesShortcut(message, m.shortcutProfile, shortcutSave) {
		return m.saveAliasForm()
	}
	switch message.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		m.adding = false
		m.status = "Add canceled"
	case tea.KeyTab, tea.KeyEnter:
		if m.field < len(m.form)-1 {
			m.field++
		} else if message.Type == tea.KeyEnter {
			return m.saveAliasForm()
		} else {
			m.field = 0
		}
	case tea.KeyShiftTab:
		m.field = (m.field + len(m.form) - 1) % len(m.form)
	case tea.KeyBackspace, tea.KeyDelete:
		if m.form[m.field] != "" {
			_, size := utf8.DecodeLastRuneInString(m.form[m.field])
			m.form[m.field] = m.form[m.field][:len(m.form[m.field])-size]
		}
	case tea.KeySpace:
		if !acceptsTextInput(message) {
			return m, nil
		}
		if m.field == 0 {
			m.status = "Alias names cannot contain spaces"
		} else {
			m.form[m.field] += " "
		}
	case tea.KeyRunes:
		if !acceptsTextInput(message) {
			return m, nil
		}
		m.form[m.field] += string(message.Runes)
	}
	return m, nil
}

func (m model) saveAliasForm() (tea.Model, tea.Cmd) {
	metadata := m.editingMetadata
	metadata.Tags = entry.SplitMetadataValues(m.form[3])
	metadata.Category = entry.NormalizeCategory(m.form[4])
	if metadata.Category == entry.Category(m.form[1]) {
		metadata.Category = ""
	}
	if metadata.Category != "" && m.metadataInvalid(metadata) {
		m.status = "Category may only use letters, numbers, dot, dash, and underscore"
		return m, nil
	}
	var saveErr error
	if m.editingName == "" {
		saveErr = m.service().AddAliasWithMetadata(m.form[0], m.form[1], m.form[2], metadata)
	} else {
		saveErr = m.service().EditAliasWithMetadata(m.editingName, m.form[0], m.form[1], m.form[2], metadata)
	}
	if saveErr != nil {
		m.status = saveErr.Error()
		return m, nil
	}
	aliases, err := m.service().Entries()
	if err != nil {
		m.status = "Alias saved, but reload failed: " + err.Error()
		return m, nil
	}
	m.aliases = aliases
	if m.browserUsageReady {
		m.refreshBrowserUsage()
	}
	m.query = strings.TrimSpace(m.form[0])
	m.cursor = 0
	m.adding = false
	if m.editingName == "" {
		m.status = "Added " + m.query + " beside related aliases"
	} else {
		m.status = "Updated " + m.query + " and regrouped it"
	}
	m.editingName = ""
	m.editingMetadata = app.EntryMetadata{}
	return m, nil
}

func (m model) updateDeleteConfirmation(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if message.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	if message.Type == tea.KeyEsc || matchesShortcut(message, m.shortcutProfile, shortcutDecline) {
		m.status = "Delete canceled"
		m.deleteName = ""
		return m, nil
	}
	if !matchesShortcut(message, m.shortcutProfile, shortcutConfirm) {
		return m, nil
	}
	name := m.deleteName
	if err := m.service().DeleteAlias(name); err != nil {
		m.status = err.Error()
		m.deleteName = ""
		return m, nil
	}
	aliases, err := m.service().Entries()
	if err != nil {
		m.status = "Deleted, but reload failed: " + err.Error()
	} else {
		m.aliases = aliases
		if m.browserUsageReady {
			m.refreshBrowserUsage()
		}
		m.cursor = 0
		m.status = "Deleted " + name + " · backup saved"
	}
	m.deleteName = ""
	return m, nil
}

func (m model) addFormView(frame tuiFrame, header string) string {
	contentWidth := frame.contentWidth
	labels := []string{"ALIAS NAME", "COMMAND", "WHAT IT DOES", "TAGS", "CATEGORY"}
	hints := []string{"ex: gpf", "ex: git push --force-with-lease", "ex: Safely force-push the current branch", "ex: git,daily", "ex: git"}
	if frame.height < 24 {
		hints = []string{"ex: gpf", "ex: git status", "ex: Check status", "ex: git,daily", "ex: git"}
	}
	var form strings.Builder
	headingIcon := iconAlias
	heading := "ADD AN ALIAS"
	message := "It will be placed beside related commands in " +
		m.service().AliasDisplayPath() + "."
	if m.editingName != "" {
		headingIcon = iconEdit
		heading = "EDIT " + m.editingName
		message = "Update its description, command, or name. Tags and category are editable here too."
	}
	form.WriteString(pixelIconLabel(headingIcon, heading, lipgloss.NewStyle().Bold(true).Foreground(amberColor)))
	form.WriteString("\n" + dimStyle.Render(message))
	fieldSeparator := "\n\n"
	if frame.height < 24 {
		fieldSeparator = "\n"
	}
	for index := range m.form {
		border := lineColor
		cursor := ""
		if index == m.field {
			border = acidColor
			cursor = acidStyle("█")
		}
		value := m.form[index]
		if value == "" && index != m.field {
			value = dimStyle.Render(hints[index])
		}
		label := lipgloss.NewStyle().Bold(true).Width(14).Foreground(colorForField(index)).Render(labels[index])
		field := lipgloss.NewStyle().Width(max(20, contentWidth-20)).Padding(0, 1).Background(panelColor).Foreground(inkColor).Border(lipgloss.ThickBorder(), false, false, false, true).BorderForeground(border).Render(value + cursor)
		form.WriteString(fieldSeparator + label + field)
	}
	footer := dimStyle.Render("tab/enter next  ·  " + strings.ToLower(shortcutLabel(m.shortcutProfile, shortcutSave)) + " save  ·  esc cancel")
	if m.status != "" {
		footer = lipgloss.NewStyle().Foreground(coralColor).Render(m.status)
	}
	sections := []string{header, "", form.String()}
	if frame.height < 24 {
		sections = []string{header, form.String()}
	}
	page := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return frame.renderWithFooter(page, footer)
}

func (m model) currentAliases() []aliasEntry {
	if m.healthOnly {
		var unhealthy []aliasEntry
		for _, alias := range m.aliases {
			if len(alias.Issues) > 0 {
				unhealthy = append(unhealthy, alias)
			}
		}
		return unhealthy
	}
	if strings.TrimSpace(m.query) == "" {
		return suggestedAliasesForContext(m.service(),

			m.aliases, m.context)
	}
	return filterAliasesForContext(m.service(),

		m.aliases, m.query, m.context)
}

func (m model) searchFocused() bool {
	return m.aliasMode != aliasModeCommand && !m.terminalBlurred && !m.tourVisible && !m.adding && !m.themePicker && !m.settingsOpen && !m.helpVisible && !m.shortcutsOpen && !m.statsOpen && m.deleteName == "" && m.runConfirm == nil && !m.revisionOpen && !m.trackedOnly
}

func (m model) wideAliasRedraw() tea.Cmd {
	if m.width >= 120 && !m.healthOnly {
		return tea.ClearScreen
	}
	return nil
}

func renderAlias(alias aliasEntry, active bool, width int, contextual ...bool) string {
	alias = terminalSafeAlias(alias)
	cardWidth := max(34, width-3)
	marker := "  "
	if active {
		marker = "▶ "
	}

	categoryColor := colorForCategory(alias.Category)
	category := lipgloss.NewStyle().Bold(true).Foreground(pageColor).Background(categoryColor).Padding(0, 1).Render(strings.ToUpper(alias.Category))
	lineOne := aliasStyle.Render(marker+alias.Name) + "  " + category
	if alias.Type == "function" {
		lineOne += "  " + pixelIconLabel(iconFunction, "FUNCTION", lipgloss.NewStyle().Foreground(violetColor))
	}
	if alias.Favorite {
		if marker := interfaceMarker(iconFavorite); marker != "" {
			lineOne += "  " + lipgloss.NewStyle().Foreground(amberColor).Render(marker)
		}
	}
	if len(contextual) > 0 && contextual[0] {
		lineOne += "  " + lipgloss.NewStyle().Foreground(cyanColor).Render(markerPrefix(iconContext)+"LOCAL")
	}
	if len(alias.Tags) > 0 {
		lineOne += "  " + dimStyle.Render("#"+strings.Join(alias.Tags, " #"))
	}
	lineTwo := lipgloss.NewStyle().Foreground(cyanColor).Render(markerPrefix(iconCommand) + truncate(alias.Command, cardWidth-8))
	content := lineOne + "\n" + lineTwo
	if alias.Description != "" {
		space := cardWidth - lipgloss.Width(lineOne) - 5
		if space >= 12 {
			content = lineOne + dimStyle.Render("  "+truncate(alias.Description, space)) + "\n" + lineTwo
		} else {
			content += "\n" + lipgloss.NewStyle().Foreground(inkColor).Render(wrapText(alias.Description, cardWidth-6))
		}
	}
	if len(alias.Issues) > 0 {
		content += "\n" + lipgloss.NewStyle().Foreground(coralColor).Render(markerPrefix(iconHealth)+strings.Join(alias.Issues, " · "))
	}

	borderColor := lineColor
	if active {
		borderColor = acidColor
	}
	return lipgloss.NewStyle().Width(cardWidth).Padding(0, 1).Border(lipgloss.ThickBorder(), false, false, false, true).BorderForeground(borderColor).Render(content)
}

func terminalSafeAlias(alias aliasEntry) aliasEntry {
	alias.Name = presentation.TerminalSafeText(alias.Name)
	alias.Command = presentation.TerminalSafeText(alias.Command)
	alias.Description = presentation.TerminalSafeText(alias.Description)
	alias.Category = presentation.TerminalSafeText(alias.Category)
	alias.Tags = append([]string(nil), alias.Tags...)
	for index := range alias.Tags {
		alias.Tags[index] = presentation.TerminalSafeText(alias.Tags[index])
	}
	alias.Issues = append([]string(nil), alias.Issues...)
	for index := range alias.Issues {
		alias.Issues[index] = presentation.TerminalSafeText(alias.Issues[index])
	}
	return alias
}

func (m model) visibleCount() int { return max(1, (m.height-14)/4) }

func aliasWindow(aliases []aliasEntry, cursor, width, budget int) (int, int) {
	if len(aliases) == 0 {
		return 0, 0
	}
	cursor = min(max(0, cursor), len(aliases)-1)
	budget = max(3, budget)
	for start := 0; start <= cursor; start++ {
		used := 0
		end := start
		for end < len(aliases) {
			cardHeight := lipgloss.Height(renderAlias(aliases[end], end == cursor, width))
			if used+cardHeight > budget && end > start {
				break
			}
			used += cardHeight
			end++
		}
		if cursor < end {
			return start, end
		}
	}
	return cursor, cursor + 1
}

func acidStyle(value string) string { return lipgloss.NewStyle().Foreground(acidColor).Render(value) }

func cyanStyle(value string) string { return lipgloss.NewStyle().Foreground(cyanColor).Render(value) }

func applyTheme(theme themePalette) {
	if noColorRequested() {
		lipgloss.SetColorProfile(termenv.Ascii)
	}
	acidColor = lipgloss.Color(theme.Accent)
	cyanColor = lipgloss.Color(theme.Secondary)
	coralColor = lipgloss.Color(theme.Git)
	amberColor = lipgloss.Color(theme.Dev)
	violetColor = lipgloss.Color(theme.Files)
	inkColor = lipgloss.Color(theme.Text)
	mutedColor = lipgloss.Color(theme.Muted)
	pageColor = terminalBackgroundColor(theme.Background)
	panelColor = terminalBackgroundColor(theme.Panel)
	activeColor = lipgloss.Color(theme.Selected)
	lineColor = lipgloss.Color(theme.Border)

	brandStyle = lipgloss.NewStyle().Bold(true).Foreground(pageColor).Background(acidColor).Padding(0, 1)
	dimStyle = lipgloss.NewStyle().Foreground(mutedColor)
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(inkColor)
	aliasStyle = lipgloss.NewStyle().Bold(true).Foreground(acidColor)
	commandStyle = lipgloss.NewStyle().Foreground(mutedColor)
	statusStyle = lipgloss.NewStyle().Foreground(acidColor)
}

func colorForCategory(category string) lipgloss.Color {
	switch category {
	case "git":
		return coralColor
	case "docker":
		return cyanColor
	case "files":
		return violetColor
	case "dev":
		return amberColor
	default:
		return mutedColor
	}
}

func colorForField(index int) lipgloss.Color {
	switch index {
	case 0:
		return acidColor
	case 1:
		return cyanColor
	default:
		return amberColor
	}
}

func searchText(query string) string {
	return searchTextWithPlaceholder(query, "search aliases…")
}

func searchTextCursor(query string, visible bool) string {
	return searchTextWithCursor(query, "search aliases…", visible)
}

func searchTextCursorAtWidth(query string, visible bool, fieldWidth int) string {
	return searchTextWithCursorAtWidth(query, "search aliases…", visible, fieldWidth)
}

func searchTextWithPlaceholder(query, placeholder string) string {
	return searchTextWithCursor(query, placeholder, true)
}

func searchTextWithCursor(query, placeholder string, visible bool) string {
	return searchTextWithCursorAtWidth(query, placeholder, visible, mainTUISearchMaxWidth)
}

func searchTextWithCursorAtWidth(query, placeholder string, visible bool, fieldWidth int) string {
	cursor := " "
	if visible {
		cursor = acidStyle("█")
	}
	available := max(1, fieldWidth-5-lipgloss.Width(markerPrefix(iconSearch)))
	if query == "" {
		return dimStyle.Render(ansi.Truncate(placeholder, available, "…")) + cursor
	}
	visibleQuery := presentation.TerminalSafeText(query)
	if overflow := lipgloss.Width(visibleQuery) - available; overflow > 0 {
		visibleQuery = ansi.TruncateLeft(visibleQuery, overflow+1, "…")
	}
	return lipgloss.NewStyle().Foreground(inkColor).Render(visibleQuery) + cursor
}

const maxSearchQueryRunes = 256

func appendSearchQuery(current, input string) string {
	remaining := maxSearchQueryRunes - utf8.RuneCountInString(current)
	if remaining <= 0 {
		return current
	}
	var addition strings.Builder
	for _, character := range input {
		if remaining == 0 {
			break
		}
		addition.WriteRune(character)
		remaining--
	}
	return current + addition.String()
}

func truncate(value string, width int) string {
	if width <= 1 {
		return ""
	}
	const scanByteLimit = 4096
	bounded := value
	clipped := false
	if len(bounded) > scanByteLimit {
		boundary := scanByteLimit
		for boundary > 0 && !utf8.RuneStart(bounded[boundary]) {
			boundary--
		}
		bounded = bounded[:boundary]
		clipped = true
	}
	limit := width - 1
	var result strings.Builder
	cellWidth := 0
	lastWithinLimit := 0
	complete := !clipped
	graphemes := uniseg.NewGraphemes(bounded)
	for graphemes.Next() {
		cluster := graphemes.Str()
		clusterWidth := graphemes.Width()
		if cellWidth+clusterWidth > width {
			complete = false
			break
		}
		result.WriteString(cluster)
		cellWidth += clusterWidth
		if cellWidth <= limit {
			lastWithinLimit = result.Len()
		}
	}
	if complete {
		return result.String()
	}
	return result.String()[:lastWithinLimit] + "…"
}

func wrapText(value string, width int) string {
	if width < 1 {
		return ""
	}
	words := strings.Fields(value)
	if len(words) == 0 {
		return ""
	}
	var lines []string
	line := truncate(words[0], width)
	for _, word := range words[1:] {
		word = truncate(word, width)
		if lipgloss.Width(line)+1+lipgloss.Width(word) <= width {
			line += " " + word
			continue
		}
		lines = append(lines, line)
		line = word
	}
	lines = append(lines, line)
	return strings.Join(lines, "\n")
}

func padRight(value string, width int) string {
	cellWidth := lipgloss.Width(value)
	if cellWidth > width {
		return ansi.Truncate(value, width, "…")
	}
	return value + strings.Repeat(" ", width-cellWidth)
}

func noColorRequested() bool { return os.Getenv("NO_COLOR") != "" }

func terminalIsDumb() bool { return strings.EqualFold(strings.TrimSpace(os.Getenv("TERM")), "dumb") }

func smallTerminalMessage(width, height int) string {
	if width > 0 && height > 0 && (width < 48 || height < 18) {
		return fmt.Sprintf("Alias Lens needs at least 48 columns and 18 rows. Current terminal: %dx%d.\n", width, height)
	}
	return ""
}

func printPlainAliasList(output io.Writer, aliases []aliasEntry, query string) {
	needle := strings.ToLower(strings.TrimSpace(query))
	for _, alias := range aliases {
		if needle != "" && !strings.Contains(strings.ToLower(alias.Name+" "+alias.Command+" "+alias.Description), needle) {
			continue
		}
		fmt.Fprintf(output, "%s\t%s\n", presentation.TerminalSafeText(alias.Name), presentation.TerminalSafeText(alias.Command))
	}
	fmt.Fprintln(output, "Use 'al search QUERY' for non-interactive search.")
}

func matchSummary(start, end, total int) string {
	if total > end-start {
		return fmt.Sprintf("showing %d-%d of %d", start+1, end, total)
	}
	suffix := "es"
	if total == 1 {
		suffix = ""
	}
	return fmt.Sprintf("%d match%s", total, suffix)
}
