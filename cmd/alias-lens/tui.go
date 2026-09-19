package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	tea "alias-lens/cmd/alias-lens/internal/tea"
	"github.com/charmbracelet/lipgloss"
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
	aliases         []Alias
	query           string
	cursor          int
	width           int
	height          int
	status          string
	theme           Theme
	themePicker     bool
	themeCursor     int
	themeBefore     Theme
	settingsOpen    bool
	settingsField   int
	settingsForm    [2]string
	settingsBefore  FooterConfig
	helpVisible     bool
	helpQuery       string
	statsOpen       bool
	statsData       statsData
	statsPeriod     int
	statsViewIndex  int
	statsSelected   int
	statsNow        time.Time
	statsErr        string
	tourVisible     bool
	adding          bool
	field           int
	form            [5]string
	editingName     string
	editingMetadata EntryMetadata
	deleteName      string
	runConfirm      *Alias
	revisionOpen    bool
	revisionCursor  int
	revisionConfirm bool
	revisions       []Revision
	revisionErr     string
	healthOnly      bool
	trackedOnly     bool
	tracked         []trackedFileItem
	trackedRepo     string
	trackedErr      string
	autoSyncEnabled bool
	syncInterval    int
	primarySync     trackedFileItem
	selectMode      bool
	executeMode     bool
	selected        *Alias
	editSelection   bool
	cursorHidden    bool
	terminalBlurred bool
	shortcutProfile ShortcutProfile
}

type cursorBlinkMsg struct{}

const cursorBlinkInterval = 500 * time.Millisecond

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
)

type trackedFileItem struct {
	Config TrackedFileConfig
	State  SyncState
	Error  string
}

func runTUI() {
	aliases, err := loadAliases()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not read %s: %v\n", aliasDisplayPath(), err)
		return
	}
	if terminalIsDumb() {
		printPlainAliasList(os.Stderr, aliases, "")
		return
	}
	theme, themeErr := loadTheme()
	config, _ := loadConfig()
	applyFooterConfig(config.Footer)
	status := ""
	if themeErr != nil {
		status = themeErr.Error()
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
	finished, err := tea.NewProgram(model{aliases: aliases, width: 80, height: 24, theme: theme, status: status, executeMode: true, tourVisible: tourShouldShow(), shortcutProfile: resolvedShortcutProfile(config)}, options...).Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Alias Lens could not start:", err)
		return
	}
	selected, ok := finished.(model)
	if ok && selected.selected != nil {
		if selected.editSelection {
			writeAliasEditSelection(os.Stdout, terminalOutput, selected.selected.Name, stdoutIsTerminal)
		} else {
			writeAliasSelection(os.Stdout, terminalOutput, selected.selected.Name, stdoutIsTerminal)
		}
	}
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

func runAliasPicker(query string, commandOnly, executeSelection bool) error {
	aliases, err := loadAliases()
	if err != nil {
		return err
	}
	if terminalIsDumb() {
		printPlainAliasList(os.Stderr, aliases, query)
		return nil
	}
	theme, _ := loadTheme()
	config, _ := loadConfig()
	applyFooterConfig(config.Footer)
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
	initial := model{aliases: aliases, query: query, width: 80, height: 24, theme: theme, selectMode: !executeSelection, executeMode: executeSelection, shortcutProfile: resolvedShortcutProfile(config)}
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
	case cursorBlinkMsg:
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
	case tea.KeyMsg:
		m.cursorHidden = false
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
		if m.revisionOpen && m.revisionConfirm {
			return m.updateRevisionDrawer(message)
		}
		if m.settingsOpen && matchesShortcut(message, m.shortcutProfile, shortcutSave) {
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
			return m, nil
		}
		if matchesShortcut(message, m.shortcutProfile, shortcutAdd) {
			m.startAddForm()
			return m, nil
		}
		if matchesShortcut(message, m.shortcutProfile, shortcutEdit) {
			m.startEditForm(matches)
			return m, nil
		}
		switch message.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyCtrlD:
			if len(matches) > 0 {
				m.deleteName = matches[m.cursor].Name
			}
		case tea.KeyCtrlG:
			message, err := syncRepository(false)
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
				m.editSelection = message.Type == tea.KeyTab
				if !m.editSelection && m.executeMode && isDangerousCommand(selected.Command) {
					m.runConfirm = &selected
					return m, nil
				}
				m.selected = &selected
				return m, tea.Quit
			} else if len(m.aliases) == 0 && strings.TrimSpace(m.query) == "" && !m.selectMode {
				m.startAddForm()
			}
		case tea.KeyRunes:
			if message.Super {
				return m, nil
			}
			m.healthOnly = false
			m.query += string(message.Runes)
			m.cursor = 0
		}
	}
	return m, nil
}

func (m *model) reloadAliasesAndTheme() {
	aliases, err := loadAliases()
	if err != nil {
		m.status = "Could not reload aliases: " + err.Error()
	} else {
		m.aliases = aliases
		m.cursor = 0
		m.status = fmt.Sprintf("Reloaded %d aliases", len(aliases))
	}
	theme, err := loadTheme()
	if err == nil {
		m.theme = theme
		applyTheme(theme)
	}
}

func (m *model) startEditForm(matches []Alias) {
	if len(matches) == 0 {
		return
	}
	selected := matches[m.cursor]
	m.adding = true
	m.editingName = selected.Name
	m.field = 2
	m.form = [5]string{selected.Name, selected.Command, selected.Description, strings.Join(selected.Tags, ","), selected.Category}
	m.editingMetadata = EntryMetadata{Platforms: selected.Platforms, Favorite: selected.Favorite}
}

func (m model) updatePageNavigation(message tea.KeyMsg) (model, bool) {
	target, ok := pageForShortcut(message, m.shortcutProfile)
	if !ok || (m.selectMode && target != pageHelp) {
		return m, false
	}
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

func pageForShortcut(message tea.KeyMsg, profile ShortcutProfile) (tuiPage, bool) {
	if matchesShortcut(message, profile, shortcutHelp) {
		return pageHelp, true
	}
	switch {
	case matchesShortcut(message, profile, shortcutStats):
		return pageStats, true
	case matchesShortcut(message, profile, shortcutSettings):
		return pageSettings, true
	case matchesShortcut(message, profile, shortcutThemes):
		return pageThemes, true
	case matchesShortcut(message, profile, shortcutRevisions):
		return pageRevisions, true
	case matchesShortcut(message, profile, shortcutSync):
		return pageSync, true
	case matchesShortcut(message, profile, shortcutHealth):
		return pageHealth, true
	default:
		return pageAliases, false
	}
}

func (m model) currentPage() tuiPage {
	switch {
	case m.helpVisible:
		return pageHelp
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
		m.themeBefore = Theme{}
		applyTheme(m.theme)
	}
	if m.settingsOpen {
		applyFooterConfig(m.settingsBefore)
	}
	m.helpVisible = false
	m.helpQuery = ""
	m.statsOpen = false
	m.settingsOpen = false
	m.settingsField = 0
	m.settingsForm = [2]string{}
	m.settingsBefore = FooterConfig{}
	m.themePicker = false
	m.revisionOpen = false
	m.revisionConfirm = false
	m.revisions = nil
	m.revisionErr = ""
	m.trackedOnly = false
	m.healthOnly = false
	m.status = ""
}

func (m model) View() string {
	if message := smallTerminalMessage(m.width, m.height); message != "" {
		return message
	}
	width := max(48, m.width)
	height := max(18, m.height)
	contentWidth := max(40, min(width-8, 108))
	matches := m.currentAliases()
	cursor := min(m.cursor, max(0, len(matches)-1))

	headerDetails := fmt.Sprintf("  •  %d aliases", len(m.aliases))
	if issues := healthIssueCount(m.aliases); issues > 0 {
		label := "issues"
		if issues == 1 {
			label = "issue"
		}
		headerDetails += fmt.Sprintf("  •  %d %s", issues, label)
	}
	brand := brandStyle.Render("ALIAS LENS")
	if contentWidth >= 48 {
		brand = pixelIconLabel(iconBrand, "ALIAS LENS", brandStyle)
	}
	header := brand + "  " + lipgloss.NewStyle().Foreground(cyanColor).Render(aliasDisplayPath()) + dimStyle.Render(headerDetails)
	title := pixelIconLabel(iconSearch, "Find the shortcut before you forget it.", titleStyle) + "\n" + dimStyle.Render("Search, inspect, and rediscover the commands you already own.")
	if m.selectMode {
		title = pixelIconLabel(iconAlias, "Choose an alias to use in your shell.", titleStyle) + "\n" + dimStyle.Render("Enter selects it. Esc returns without changing the prompt.")
	} else if m.executeMode {
		title = pixelIconLabel(iconCommand, "Choose an alias to run.", titleStyle) + "\n" + dimStyle.Render("Press Enter to run it. Press Esc to leave without running anything.")
	}
	if len(m.aliases) == 0 {
		title = pixelIconLabel(iconAlias, "Set up your first shortcut.", titleStyle) + "\n" + dimStyle.Render("Create an alias here or add one to the active alias file.")
		if m.selectMode {
			title = pixelIconLabel(iconAlias, "No aliases are available to select.", titleStyle) + "\n" + dimStyle.Render("Open Alias Lens normally to create one.")
		}
	}
	if m.tourVisible {
		return m.tourView(width, height, contentWidth, header)
	}
	if m.adding {
		return m.addFormView(width, height, contentWidth, header)
	}
	if m.themePicker {
		return m.themePickerView(width, height, contentWidth, header)
	}
	if m.settingsOpen {
		return m.footerSettingsView(width, height, contentWidth, header)
	}
	if m.helpVisible {
		return m.helpView(width, height, contentWidth, header)
	}
	if m.statsOpen {
		return m.statsView(header)
	}
	if m.runConfirm != nil {
		return m.runConfirmationView(width, height, contentWidth, header)
	}
	if m.revisionOpen {
		return m.revisionDrawerView(width, height, contentWidth, header)
	}
	if m.trackedOnly {
		return m.trackedFilesView(width, height, contentWidth, header)
	}
	search := lipgloss.NewStyle().
		Width(contentWidth-3).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(acidColor).
		Render(acidStyle(renderPixelIcon(iconSearch)) + " " + searchTextCursor(m.query, m.searchFocused() && !m.cursorHidden))

	var body strings.Builder
	if len(m.aliases) == 0 && strings.TrimSpace(m.query) == "" && !m.healthOnly {
		body.WriteString(m.emptyStateView(contentWidth))
	} else if m.healthOnly {
		body.WriteString(pixelIconLabel(iconHealth, "ALIAS HEALTH", lipgloss.NewStyle().Bold(true).Foreground(coralColor)))
		body.WriteByte('\n')
	} else if strings.TrimSpace(m.query) == "" {
		body.WriteString(pixelIconLabel(iconSpark, "SUGGESTED FOR YOU", lipgloss.NewStyle().Bold(true).Foreground(amberColor)))
		body.WriteByte('\n')
	}
	if len(m.aliases) == 0 && strings.TrimSpace(m.query) == "" && !m.healthOnly {
		// The empty state above replaces the normal search result list.
	} else if m.healthOnly && len(matches) == 0 {
		body.WriteString(statusStyle.Render("No health issues found."))
	} else if len(matches) == 0 {
		body.WriteString(titleStyle.Render(fmt.Sprintf("No alias matched %q", m.query)))
		if close := closestAliases(m.aliases, m.query); len(close) > 0 {
			body.WriteString("\n" + dimStyle.Render("Did you mean ") + aliasStyle.Render(strings.Join(close, "  ")) + dimStyle.Render(" ?"))
		}
	} else {
		start, end := aliasWindow(matches, cursor, contentWidth, height)
		for index := start; index < end; index++ {
			body.WriteString(renderAlias(matches[index], index == cursor, contentWidth))
			if index < end-1 {
				body.WriteString("\n\n")
			}
		}
		if strings.TrimSpace(m.query) != "" {
			body.WriteString("\n" + dimStyle.Render(matchSummary(start, end, len(matches))))
		}
	}

	footer := dimStyle.Render("↑↓ move  ·  enter ") + cyanStyle("select") + dimStyle.Render("  ·  tab edit at prompt  ·  ? ") + cyanStyle("help") + dimStyle.Render("  ·  esc quit")
	if contentWidth < 96 {
		footer = dimStyle.Render("enter ") + cyanStyle("select") + dimStyle.Render("  ·  F2 stats  ·  ? help  ·  esc quit")
	}
	if m.selectMode {
		footer = dimStyle.Render("type · ↑↓ move · enter select · ? help · esc cancel")
		if contentWidth >= 96 {
			footer = dimStyle.Render("type to search  ·  ↑↓ move  ·  enter select  ·  ? help  ·  esc cancel")
		}
	} else if m.executeMode {
		footer = dimStyle.Render("enter ") + cyanStyle("run") + dimStyle.Render("  ·  tab edit  ·  F2 stats  ·  ? help  ·  esc quit")
		if contentWidth >= 96 {
			footer = dimStyle.Render("↑↓ move  ·  enter ") + cyanStyle("run") + dimStyle.Render("  ·  tab edit  ·  ? help  ·  esc quit")
		}
	}
	if contentWidth < 50 {
		action := "select"
		if m.executeMode {
			action = "run"
		}
		footer = dimStyle.Render("enter ") + cyanStyle(action) + dimStyle.Render("  ·  ? help  ·  esc quit")
		if m.selectMode {
			footer = dimStyle.Render("enter select  ·  ? help  ·  esc cancel")
		}
	}
	if contentWidth >= 79 && !m.selectMode {
		footer += "\n" + dimStyle.Render(pageNavigationHint(contentWidth, m.shortcutProfile))
	}
	if m.status != "" {
		footer = statusStyle.Render(truncate(m.status, contentWidth))
	}
	if m.deleteName != "" {
		footer = lipgloss.NewStyle().Bold(true).Foreground(coralColor).Render("Delete " + m.deleteName + "?  y confirm  ·  n cancel")
	}
	page := lipgloss.JoinVertical(lipgloss.Left, header, "", title, "", search, "", body.String(), "", footer)
	page = pageWithMaker(page, contentWidth, height)
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Padding(1, 3).
		Render(page)
}

func (m *model) startAddForm() {
	m.adding = true
	m.field = 0
	m.form = [5]string{}
	m.editingName = ""
	m.editingMetadata = EntryMetadata{}
}

func (m model) emptyStateView(contentWidth int) string {
	var body strings.Builder
	if contentWidth < 60 {
		if m.selectMode {
			return dimStyle.Render("Open ") + cyanStyle("al") + dimStyle.Render(" and press ") + aliasStyle.Render(shortcutLabel(m.shortcutProfile, shortcutAdd)) + dimStyle.Render(" to create an alias.")
		}
		body.WriteString(aliasStyle.Render(shortcutLabel(m.shortcutProfile, shortcutAdd)) + dimStyle.Render("  Create your first alias"))
		body.WriteString("\n" + dimStyle.Render(shortcutLabel(m.shortcutProfile, shortcutRefresh)) + dimStyle.Render("  Reload aliases"))
		return body.String()
	}
	body.WriteString(pixelIconLabel(iconAlias, "No aliases yet.", titleStyle))
	body.WriteString("\n" + dimStyle.Render(wrapText("Alias Lens is reading "+aliasDisplayPath()+" for "+activeShellAdapter().DisplayName()+".", contentWidth)))
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
	if message.Type == tea.KeyEsc || (message.Type == tea.KeyRunes && len(message.Runes) == 1 && message.Runes[0] == '?') {
		m.helpVisible = false
		m.helpQuery = ""
		return m, nil
	}
	switch message.Type {
	case tea.KeyBackspace, tea.KeyDelete:
		if m.helpQuery != "" {
			_, size := utf8.DecodeLastRuneInString(m.helpQuery)
			m.helpQuery = m.helpQuery[:len(m.helpQuery)-size]
		}
	case tea.KeySpace:
		m.helpQuery += " "
	case tea.KeyRunes:
		if message.Super {
			return m, nil
		}
		m.helpQuery += string(message.Runes)
	}
	return m, nil
}

func (m model) updateRunConfirmation(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if message.Type == tea.KeyCtrlC {
		m.runConfirm = nil
		return m, tea.Quit
	}
	if message.Type == tea.KeyEsc || message.String() == "n" {
		m.runConfirm = nil
		m.status = "Run canceled"
		return m, nil
	}
	if message.String() != "y" {
		return m, nil
	}
	selected := *m.runConfirm
	m.runConfirm = nil
	m.selected = &selected
	return m, tea.Quit
}

func (m model) runConfirmationView(width, height, contentWidth int, header string) string {
	if m.runConfirm == nil {
		return ""
	}
	alias := *m.runConfirm
	reasons := dangerousCommandReasons(alias.Command)
	reason := "Alias Lens marked this command for review."
	if len(reasons) > 0 {
		reason = "Why: " + strings.Join(reasons, "; ") + "."
	}

	name := lipgloss.NewStyle().Bold(true).Foreground(amberColor).Render(alias.Name)
	command := lipgloss.NewStyle().
		Width(max(32, contentWidth-4)).
		Padding(1, 2).
		Foreground(inkColor).
		Background(panelColor).
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(coralColor).
		Render(renderPixelIcon(iconCommand) + " " + wrapText(alias.Command, max(24, contentWidth-13)))
	body := pixelIconLabel(iconHealth, "Review before running", titleStyle) +
		"\n" + dimStyle.Render("Alias ") + name + dimStyle.Render(" may make changes that are hard to undo.") +
		"\n\n" + command +
		"\n\n" + lipgloss.NewStyle().Foreground(coralColor).Render(wrapText(reason, contentWidth))
	footer := aliasStyle.Render("y") + dimStyle.Render(" run alias  ·  ") + aliasStyle.Render("n") + dimStyle.Render(" or esc cancel")
	page := lipgloss.JoinVertical(lipgloss.Left, header, "", body, "", footer)
	page = pageWithMaker(page, contentWidth, height)
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 3).Render(page)
}

func (m model) helpView(width, height, contentWidth int, header string) string {
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

	keyWidth := 16
	if m.shortcutProfile == shortcutMacOS {
		keyWidth = 23
	}
	if contentWidth < 60 {
		keyWidth = min(keyWidth, max(14, contentWidth/2))
	}
	var rows strings.Builder
	visible := min(len(shortcuts), max(3, height-14))
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

	search := lipgloss.NewStyle().Width(contentWidth-3).Padding(0, 1).Border(lipgloss.RoundedBorder()).BorderForeground(acidColor).Render(acidStyle(renderPixelIcon(iconHelp)) + " " + searchTextWithPlaceholder(m.helpQuery, "filter shortcuts…"))
	title := pixelIconLabel(iconHelp, "Keyboard guide", titleStyle) + "\n" + dimStyle.Render("Type to filter commands and shortcuts.")
	footer := "? or esc close  ·  ctrl+c quit"
	if len(shortcuts) > visible {
		footer = fmt.Sprintf("showing %d of %d  ·  type to filter  ·  esc close", visible, len(shortcuts))
	}
	footerView := dimStyle.Render(footer)
	if navigation := pageNavigationHint(contentWidth, m.shortcutProfile); navigation != "" {
		footerView += "\n" + dimStyle.Render(navigation)
	}
	page := lipgloss.JoinVertical(lipgloss.Left, header, "", title, "", search, "", rows.String(), "", footerView)
	page = pageWithMaker(page, contentWidth, height)
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 3).Render(page)
}

func pageNavigationHint(width int, profiles ...ShortcutProfile) string {
	if width < 79 {
		return ""
	}
	profile := shortcutLinux
	if len(profiles) > 0 {
		profile = profiles[0]
	}
	if profile == shortcutMacOS {
		return "? help  ·  ⌘2 stats  ·  F3 settings  ·  ⌘T themes  ·  ⌘Z versions  ·  ⌘⇧S sync  ·  ⌘H health"
	}
	return "? help  ·  F2 stats  ·  F3 settings  ·  ^T themes  ·  ^Z versions  ·  ^F sync  ·  ^H health"
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
		m.themeBefore = Theme{}
		m.status = "Theme unchanged"
		applyTheme(m.theme)
		return m, nil
	case tea.KeyEnter:
		if err := saveTheme(m.theme); err != nil {
			m.status = "Could not save theme: " + err.Error()
			return m, nil
		}
		m.themePicker = false
		m.themeBefore = Theme{}
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

func (m model) themePickerView(width, height, contentWidth int, header string) string {
	themes := availableThemes()
	cursor := min(max(0, m.themeCursor), max(0, len(themes)-1))
	visible := min(len(themes), m.themePickerVisibleCount())
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

	title := pixelIconLabel(iconTheme, "Choose a theme", titleStyle) + "\n" + dimStyle.Render("The preview changes as you move. Save only when it looks right.")
	footer := dimStyle.Render("↑↓ preview  ·  pgup/pgdn jump  ·  enter ") + cyanStyle("save") + dimStyle.Render("  ·  esc restore")
	if navigation := pageNavigationHint(contentWidth, m.shortcutProfile); navigation != "" {
		footer += "\n" + dimStyle.Render(navigation)
	}
	if m.status != "" {
		footer = lipgloss.NewStyle().Foreground(coralColor).Render(truncate(m.status, contentWidth))
	}
	page := lipgloss.JoinVertical(lipgloss.Left, header, "", title, "", rows.String(), "", footer)
	page = pageWithMaker(page, contentWidth, height)
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 3).Render(page)
}

func (m model) refreshTrackedFiles() model {
	config, err := loadConfig()
	if err != nil {
		m.tracked = nil
		m.trackedRepo = ""
		m.trackedErr = err.Error()
		m.autoSyncEnabled = false
		m.syncInterval = 0
		m.primarySync = trackedFileItem{}
		return m
	}
	m.trackedRepo = config.Repository
	m.trackedErr = ""
	m.autoSyncEnabled = config.AutoSync.Enabled
	m.syncInterval = config.AutoSync.IntervalSeconds
	primarySource, pathErr := aliasesPath()
	if pathErr != nil {
		primarySource = aliasDisplayPath()
	}
	primaryRepositoryPath := config.AliasFile
	if config.Repository == "" {
		primaryRepositoryPath = ""
	}
	m.primarySync = trackedFileItem{Config: TrackedFileConfig{Source: primarySource, RepositoryPath: primaryRepositoryPath}}
	if pathErr != nil {
		m.primarySync.Error = pathErr.Error()
	} else if state, stateErr := loadSyncState(); stateErr != nil {
		m.primarySync.Error = stateErr.Error()
	} else {
		m.primarySync.State = state
	}
	m.tracked = make([]trackedFileItem, 0, len(config.TrackedFiles))
	for _, tracked := range config.TrackedFiles {
		item := trackedFileItem{Config: tracked}
		statePath, pathErr := trackedStatePath(tracked)
		if pathErr != nil {
			item.Error = pathErr.Error()
		} else if state, stateErr := loadSyncStateAt(statePath); stateErr != nil {
			item.Error = stateErr.Error()
		} else {
			item.State = state
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
	switch message.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		m.trackedOnly = false
		m.cursor = 0
	case tea.KeyCtrlG:
		message, err := syncRepository(false)
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
	}
	return m, nil
}

func (m model) trackedFilesView(width, height, contentWidth int, header string) string {
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
	body.WriteString("\n\n")

	if m.trackedErr != "" {
		body.WriteString(lipgloss.NewStyle().Foreground(coralColor).Render(wrapText("Could not load sync status: "+m.trackedErr, contentWidth)))
	} else {
		body.WriteString(pixelIconLabel(iconAlias, "PRIMARY ALIAS FILE", titleStyle))
		body.WriteString("\n" + renderTrackedFile(m.primarySync, false, contentWidth))
		body.WriteString("\n\n" + pixelIconLabel(iconRepository, "EXTRA TRACKED FILES", lipgloss.NewStyle().Bold(true).Foreground(amberColor)))
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

	footer := dimStyle.Render("↑↓ move  ·  ctrl+g save to repo  ·  ctrl+r refresh  ·  ctrl+f or esc aliases  ·  ctrl+c quit")
	if m.status != "" {
		footer = statusStyle.Render(truncate(m.status, contentWidth))
	}
	if navigation := pageNavigationHint(contentWidth, m.shortcutProfile); navigation != "" {
		footer += "\n" + dimStyle.Render(navigation)
	}
	page := lipgloss.JoinVertical(lipgloss.Left, header, "", pixelIconLabel(iconSync, "See what Alias Lens keeps in sync.", titleStyle), "", body.String(), "", footer)
	page = pageWithMaker(page, contentWidth, height)
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 3).Render(page)
}

func (m model) trackedVisibleCount() int { return max(1, (m.height-21)/5) }

func renderTrackedFile(item trackedFileItem, active bool, width int) string {
	cardWidth := max(34, width-3)
	marker := "  "
	if active {
		marker = "▶ "
	}
	status := defaultString(item.State.Status, "waiting")
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
		lineTwo = cyanStyle(renderPixelIcon(iconRepository)+" repo/") + lipgloss.NewStyle().Foreground(inkColor).Render(truncate(filepath.ToSlash(item.Config.RepositoryPath), cardWidth-14))
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
		if m.field == 0 {
			m.status = "Alias names cannot contain spaces"
		} else {
			m.form[m.field] += " "
		}
	case tea.KeyRunes:
		if message.Super {
			return m, nil
		}
		m.form[m.field] += string(message.Runes)
	}
	return m, nil
}

func (m model) saveAliasForm() (tea.Model, tea.Cmd) {
	metadata := m.editingMetadata
	metadata.Tags = splitMetadataValues(m.form[3])
	metadata.Category = normalizeCategory(m.form[4])
	if metadata.Category == category(m.form[1]) {
		metadata.Category = ""
	}
	if metadata.Category != "" && !aliasName.MatchString(metadata.Category) {
		m.status = "Category may only use letters, numbers, dot, dash, and underscore"
		return m, nil
	}
	var saveErr error
	if m.editingName == "" {
		saveErr = addAliasWithMetadata(m.form[0], m.form[1], m.form[2], metadata)
	} else {
		saveErr = editAliasWithMetadata(m.editingName, m.form[0], m.form[1], m.form[2], metadata)
	}
	if saveErr != nil {
		m.status = saveErr.Error()
		return m, nil
	}
	aliases, err := loadAliases()
	if err != nil {
		m.status = "Alias saved, but reload failed: " + err.Error()
		return m, nil
	}
	m.aliases = aliases
	m.query = strings.TrimSpace(m.form[0])
	m.cursor = 0
	m.adding = false
	if m.editingName == "" {
		m.status = "Added " + m.query + " beside related aliases"
	} else {
		m.status = "Updated " + m.query + " and regrouped it"
	}
	m.editingName = ""
	m.editingMetadata = EntryMetadata{}
	return m, nil
}

func (m model) updateDeleteConfirmation(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if message.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	if message.Type == tea.KeyEsc || message.String() == "n" {
		m.status = "Delete canceled"
		m.deleteName = ""
		return m, nil
	}
	if message.String() != "y" {
		return m, nil
	}
	name := m.deleteName
	if err := deleteAlias(name); err != nil {
		m.status = err.Error()
		m.deleteName = ""
		return m, nil
	}
	aliases, err := loadAliases()
	if err != nil {
		m.status = "Deleted, but reload failed: " + err.Error()
	} else {
		m.aliases = aliases
		m.cursor = 0
		m.status = "Deleted " + name + " · backup saved"
	}
	m.deleteName = ""
	return m, nil
}

func (m model) addFormView(width, height, contentWidth int, header string) string {
	labels := []string{"ALIAS NAME", "COMMAND", "WHAT IT DOES", "TAGS", "CATEGORY"}
	hints := []string{"ex: gpf", "ex: git push --force-with-lease", "ex: Safely force-push the current branch", "ex: git,daily", "ex: git"}
	var form strings.Builder
	headingIcon := iconAlias
	heading := "ADD AN ALIAS"
	message := "It will be placed beside related commands in " + aliasDisplayPath() + "."
	if m.editingName != "" {
		headingIcon = iconEdit
		heading = "EDIT " + m.editingName
		message = "Update its description, command, or name. Tags and category are editable here too."
	}
	form.WriteString(pixelIconLabel(headingIcon, heading, lipgloss.NewStyle().Bold(true).Foreground(amberColor)))
	form.WriteString("\n" + dimStyle.Render(message))
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
		form.WriteString("\n\n" + label + field)
	}
	footer := dimStyle.Render("tab/enter next  ·  ctrl+s save  ·  esc cancel")
	if m.status != "" {
		footer = lipgloss.NewStyle().Foreground(coralColor).Render(m.status)
	}
	page := lipgloss.JoinVertical(lipgloss.Left, header, "", form.String(), "", footer)
	page = pageWithMaker(page, contentWidth, height)
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 3).Render(page)
}

func (m model) currentAliases() []Alias {
	if m.healthOnly {
		var unhealthy []Alias
		for _, alias := range m.aliases {
			if len(alias.Issues) > 0 {
				unhealthy = append(unhealthy, alias)
			}
		}
		return unhealthy
	}
	if strings.TrimSpace(m.query) == "" {
		return suggestedAliases(m.aliases)
	}
	return filterAliases(m.aliases, m.query)
}

func (m model) searchFocused() bool {
	return !m.terminalBlurred && !m.tourVisible && !m.adding && !m.themePicker && !m.settingsOpen && !m.helpVisible && !m.statsOpen && m.deleteName == "" && m.runConfirm == nil && !m.revisionOpen && !m.trackedOnly
}

func renderAlias(alias Alias, active bool, width int) string {
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
		lineOne += "  " + lipgloss.NewStyle().Foreground(amberColor).Render(renderPixelIcon(iconFavorite))
	}
	if len(alias.Tags) > 0 {
		lineOne += "  " + dimStyle.Render("#"+strings.Join(alias.Tags, " #"))
	}
	description := lipgloss.NewStyle().Foreground(inkColor).Render(wrapText(alias.Description, cardWidth-6))
	lineTwo := lipgloss.NewStyle().Foreground(cyanColor).Render(renderPixelIcon(iconCommand) + " " + truncate(alias.Command, cardWidth-10))
	if len(alias.Issues) > 0 {
		lineTwo += "\n" + lipgloss.NewStyle().Foreground(coralColor).Render(renderPixelIcon(iconHealth)+" "+strings.Join(alias.Issues, " · "))
	}

	borderColor := lineColor
	if active {
		borderColor = acidColor
	}
	return lipgloss.NewStyle().
		Width(cardWidth).
		Padding(0, 1).
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(borderColor).
		Render(lineOne + "\n" + description + "\n" + lineTwo)
}

func (m model) visibleCount() int { return max(1, (m.height-14)/4) }

func aliasWindow(aliases []Alias, cursor, width, height int) (int, int) {
	if len(aliases) == 0 {
		return 0, 0
	}
	cursor = min(max(0, cursor), len(aliases)-1)
	budget := max(3, height-14)
	for start := 0; start <= cursor; start++ {
		used := 0
		end := start
		for end < len(aliases) {
			cardHeight := lipgloss.Height(renderAlias(aliases[end], end == cursor, width))
			separatorHeight := 0
			if end > start {
				separatorHeight = 1
			}
			if used+separatorHeight+cardHeight > budget && end > start {
				break
			}
			used += separatorHeight + cardHeight
			end++
		}
		if cursor < end {
			return start, end
		}
	}
	return cursor, cursor + 1
}

func suggestedAliases(aliases []Alias) []Alias {
	type ranked struct {
		alias Alias
		score int
	}
	preferred := map[string]int{
		"git status -sb": 100,
		"eza -l --icons --git --group-directories-first": 95,
		"git add":                              90,
		"git commit":                           85,
		"git log --oneline --graph --decorate": 80,
		"docker ps":                            75,
	}
	var rankedAliases []ranked
	for _, alias := range aliases {
		lower := strings.ToLower(alias.Command)
		if strings.Contains(lower, "--force") || strings.Contains(lower, "reset --hard") || strings.Contains(lower, "clean -fd") || strings.Contains(lower, "branch -d") {
			continue
		}
		score := preferred[alias.Command]
		score += min(alias.Usage, 25) * 3
		if alias.Favorite {
			score += 150
		}
		if len(alias.Name) == 2 {
			score += 12
		} else if len(alias.Name) == 3 {
			score += 6
		}
		rankedAliases = append(rankedAliases, ranked{alias: alias, score: score})
	}
	sort.SliceStable(rankedAliases, func(i, j int) bool {
		if rankedAliases[i].score == rankedAliases[j].score {
			return rankedAliases[i].alias.Name < rankedAliases[j].alias.Name
		}
		return rankedAliases[i].score > rankedAliases[j].score
	})
	limit := min(12, len(rankedAliases))
	result := make([]Alias, limit)
	for index := range limit {
		result[index] = rankedAliases[index].alias
	}
	return result
}

func filterAliases(aliases []Alias, query string) []Alias {
	needle := normalize(query)
	if needle == "" {
		return nil
	}
	if utf8.RuneCountInString(needle) == 1 {
		var prefixes []Alias
		for _, alias := range aliases {
			if strings.HasPrefix(normalize(alias.Name), needle) {
				prefixes = append(prefixes, alias)
			}
		}
		sort.SliceStable(prefixes, func(i, j int) bool {
			if len(prefixes[i].Name) == len(prefixes[j].Name) {
				return prefixes[i].Name < prefixes[j].Name
			}
			return len(prefixes[i].Name) < len(prefixes[j].Name)
		})
		return prefixes
	}
	type rankedAlias struct {
		alias Alias
		score int
	}
	var ranked []rankedAlias
	for _, alias := range aliases {
		name := normalize(alias.Name)
		score := -1
		switch {
		case name == needle:
			score = 0
		case strings.HasPrefix(name, needle):
			score = 10
		case strings.Contains(name, needle):
			score = 20
		case wordsMatch(alias.Command, needle):
			score = 30
		case wordsMatch(alias.Description, needle):
			score = 40
		case wordsMatch(alias.Category, needle):
			score = 42
		case wordsMatch(strings.Join(alias.Tags, " "), needle):
			score = 45
		case wordsMatch(strings.Join(alias.Platforms, " "), needle):
			score = 50
		case alias.Type == needle:
			score = 55
		default:
			distance := levenshtein(name, needle)
			if distance <= max(1, len(needle)/3) {
				score = 60 + distance
			}
		}
		if score >= 0 {
			ranked = append(ranked, rankedAlias{alias: alias, score: score})
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return len(ranked[i].alias.Name) < len(ranked[j].alias.Name)
		}
		return ranked[i].score < ranked[j].score
	})
	matches := make([]Alias, len(ranked))
	for index := range ranked {
		matches[index] = ranked[index].alias
	}
	return matches
}

func wordsMatch(value, needle string) bool {
	for _, word := range strings.Fields(value) {
		if strings.HasPrefix(normalize(word), needle) {
			return true
		}
	}
	return false
}

func healthIssueCount(aliases []Alias) int {
	count := 0
	for _, alias := range aliases {
		count += len(alias.Issues)
	}
	return count
}

func closestAliases(aliases []Alias, query string) []string {
	needle := normalize(query)
	limit := max(1, len(needle)/3)
	var close []string
	for _, alias := range aliases {
		if levenshtein(normalize(alias.Name), needle) <= limit {
			close = append(close, alias.Name)
			if len(close) == 4 {
				break
			}
		}
	}
	return close
}

func normalize(value string) string {
	var result strings.Builder
	for _, char := range strings.ToLower(value) {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			result.WriteRune(char)
		}
	}
	return result.String()
}

func levenshtein(left, right string) int {
	row := make([]int, len(right)+1)
	for column := range row {
		row[column] = column
	}
	for i := 1; i <= len(left); i++ {
		next := make([]int, len(right)+1)
		next[0] = i
		for j := 1; j <= len(right); j++ {
			cost := 0
			if left[i-1] != right[j-1] {
				cost = 1
			}
			next[j] = min(min(next[j-1]+1, row[j]+1), row[j-1]+cost)
		}
		row = next
	}
	return row[len(right)]
}

func acidStyle(value string) string { return lipgloss.NewStyle().Foreground(acidColor).Render(value) }

func cyanStyle(value string) string { return lipgloss.NewStyle().Foreground(cyanColor).Render(value) }

func applyTheme(theme Theme) {
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
	pageColor = lipgloss.Color(theme.Background)
	panelColor = lipgloss.Color(theme.Panel)
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

func searchTextWithPlaceholder(query, placeholder string) string {
	return searchTextWithCursor(query, placeholder, true)
}

func searchTextWithCursor(query, placeholder string, visible bool) string {
	cursor := " "
	if visible {
		cursor = acidStyle("█")
	}
	if query == "" {
		return dimStyle.Render(placeholder) + cursor
	}
	return lipgloss.NewStyle().Foreground(inkColor).Render(query) + cursor
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
	if cellWidth >= width {
		return truncate(value, width)
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

func printPlainAliasList(output io.Writer, aliases []Alias, query string) {
	needle := strings.ToLower(strings.TrimSpace(query))
	for _, alias := range aliases {
		if needle != "" && !strings.Contains(strings.ToLower(alias.Name+" "+alias.Command+" "+alias.Description), needle) {
			continue
		}
		fmt.Fprintf(output, "%s\t%s\n", alias.Name, alias.Command)
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
