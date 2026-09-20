package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "alias-lens/cmd/alias-lens/internal/tea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

var statsPeriods = []string{"all", "today", "month", "year"}

var activityPeriods = []string{"all", "today", "week", "year"}

var cleanupPeriods = []string{"month", "quarter", "year", "never"}

var statsViews = []struct {
	key   string
	label string
}{
	{"o", "overview"},
	{"a", "activity"},
	{"c", "cleanup"},
	{"g", "groups"},
}

type statsModel struct {
	data            statsData
	periodIndex     int
	viewIndex       int
	selected        int
	width           int
	height          int
	theme           Theme
	now             time.Time
	appHeader       string
	closeHint       string
	errorText       string
	shortcutProfile ShortcutProfile
}

func runStatsTUI(data statsData, period string, now time.Time) error {
	theme, _ := loadTheme()
	applyTheme(theme)
	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("could not read Alias Lens settings: %w", err)
	}
	applyFooterConfig(config.Footer)
	applyAppearanceConfig(config.Appearance)
	index := 0
	viewIndex := 0
	periods := statsPeriods
	if period == "week" {
		viewIndex = 1
		periods = activityPeriods
	}
	for candidate, name := range periods {
		if name == period {
			index = candidate
		}
	}
	options := []tea.ProgramOption{tea.WithAltScreen(), tea.WithReportFocus()}
	if terminal, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		defer terminal.Close()
		renderer := lipgloss.NewRenderer(terminal)
		if noColorRequested() {
			renderer.SetColorProfile(termenv.Ascii)
		}
		lipgloss.SetDefaultRenderer(renderer)
		options = append(options, tea.WithInput(terminal), tea.WithOutput(terminal))
	}
	_, err = tea.NewProgram(statsModel{data: data, periodIndex: index, viewIndex: viewIndex, width: 80, height: 24, theme: theme, now: now, shortcutProfile: resolvedShortcutProfile(config)}, options...).Run()
	return err
}

func (m statsModel) Init() tea.Cmd { return nil }

func (m statsModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = message.Width, message.Height
	case tea.KeyMsg:
		if matchesShortcut(message, m.shortcutProfile, shortcutStats) {
			return m, tea.Quit
		}
		if matchesShortcut(message, m.shortcutProfile, shortcutRefresh) {
			data, err := loadStatsData()
			if err != nil {
				m.errorText = err.Error()
			} else {
				m.data = data
				m.errorText = ""
			}
			return m, nil
		}
		switch message.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "left", "h":
			m.periodIndex = (m.periodIndex + len(statsPeriods) - 1) % len(statsPeriods)
			m.selected = 0
		case "right", "l":
			m.periodIndex = (m.periodIndex + 1) % len(statsPeriods)
			m.selected = 0
		case "tab":
			m.viewIndex = (m.viewIndex + 1) % len(statsViews)
			m.periodIndex = defaultPeriodForStatsView(m.viewIndex)
		case "shift+tab":
			m.viewIndex = (m.viewIndex + len(statsViews) - 1) % len(statsViews)
			m.periodIndex = defaultPeriodForStatsView(m.viewIndex)
		case "o":
			m.viewIndex = 0
			m.periodIndex = 0
		case "a":
			m.viewIndex = 1
			m.periodIndex = 2
		case "c":
			m.viewIndex = 2
			m.periodIndex = 0
		case "g":
			m.viewIndex = 3
			m.periodIndex = 0
		case "1", "2", "3", "4":
			m.periodIndex = int(message.Runes[0] - '1')
			m.selected = 0
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			rows, _ := rankedStatsRows(m.data, statsPeriods[m.periodIndex], m.now)
			if m.selected+1 < len(rows) {
				m.selected++
			}
		case "r":
			data, err := loadStatsData()
			if err != nil {
				m.errorText = err.Error()
				break
			}
			m.data = data
			m.now = time.Now()
			m.errorText = ""
		}
	}
	return m, nil
}

func (m statsModel) View() string {
	if message := smallTerminalMessage(m.width, m.height); message != "" {
		return message
	}
	periods := statsPeriodsForView(m.viewIndex)
	periodIndex := min(m.periodIndex, len(periods)-1)
	period := periods[periodIndex]
	rankingPeriod := period
	if m.viewIndex == 2 {
		rankingPeriod = "all"
	}
	rows, _ := rankedStatsRows(m.data, rankingPeriod, m.now)
	width := m.width
	if width < 48 {
		width = 48
	}
	height := m.height
	if height < 18 {
		height = 18
	}
	inner := width - 4
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Accent)).Bold(true)
	text := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Text))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Muted))
	panel := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Text)).Background(lipgloss.Color(m.theme.Background)).Padding(1, 2).Width(inner)
	page := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Text)).Background(lipgloss.Color(m.theme.Background)).Padding(1, 2).Width(width).Height(height)

	total := 0
	for _, row := range rows {
		total += row.Count
	}
	header := pixelIconLabel(iconStats, "Alias rhythm", accent) + muted.Render(fmt.Sprintf("  %d uses · %d aliases", total, len(rows)))
	var tabs []string
	for index, name := range periods {
		label := fmt.Sprintf("%d %s", index+1, name)
		if index == periodIndex {
			tabs = append(tabs, lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Background)).Background(lipgloss.Color(m.theme.Accent)).Bold(true).Padding(0, 1).Render(label))
		} else {
			tabs = append(tabs, muted.Padding(0, 1).Render(label))
		}
	}
	var viewTabs []string
	for index, view := range statsViews {
		label := view.key + " " + view.label
		if index == m.viewIndex {
			viewTabs = append(viewTabs, accent.Render("["+label+"]"))
		} else {
			viewTabs = append(viewTabs, muted.Render(label))
		}
	}

	bodyContent := ""
	if m.errorText != "" {
		var body strings.Builder
		body.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Git)).Bold(true).Render("Stats could not load."))
		body.WriteString("\n")
		body.WriteString(muted.Render(truncate(m.errorText, inner-4)))
		bodyContent = body.String()
	} else {
		styles := statsChartStyles{accent: accent, text: text, muted: muted, theme: m.theme}
		switch m.viewIndex {
		case 1:
			bodyContent = renderActivityHistogram(m.data.Events, period, m.now, inner, styles)
		case 2:
			bodyContent = renderStaleAliases(m.data, period, m.now, inner, m.height, styles)
		case 3:
			bodyContent = renderGroupShare(m.data, period, m.now, inner, styles)
		default:
			bodyContent = renderStatsOverview(m, rows, inner, styles)
		}
	}

	closeHint := m.closeHint
	if closeHint == "" {
		closeHint = "q close"
	}
	filterLabel := "period"
	if m.viewIndex == 1 {
		filterLabel = "timeframe"
	} else if m.viewIndex == 2 {
		filterLabel = "age filter"
	}
	footerText := "tab view   ←/→ " + filterLabel + "   r refresh   " + closeHint
	if m.viewIndex == 0 && width >= 76 {
		footerText = "tab view   ←/→ period   ↑/↓ inspect   r refresh   " + closeHint
	}
	foot := muted.Render(footerText)
	note := muted.Render("Counts come only from the active terminal history")
	top := header + "\n\n" + strings.Join(viewTabs, "   ") + "\n" + strings.Join(tabs, " ") + "\n\n" + panel.Render(bodyContent)
	if m.appHeader != "" {
		top = m.appHeader + "\n\n" + top
	}
	bottom := note + "\n" + foot
	if m.appHeader != "" {
		bottom = footerWithNavigation(bottom, inner, m.shortcutProfile)
	}
	bottom = footerWithMaker(bottom, inner)
	reservedBottomRow := 1
	if m.appHeader != "" {
		// Embedded pages use the main browser's outer frame, which supplies the
		// final bottom row after Bubble Tea clips the rendered view.
		reservedBottomRow = 0
	}
	spacerHeight := max(1, height-lipgloss.Height(top)-lipgloss.Height(bottom)-reservedBottomRow)
	content := top + strings.Repeat("\n", spacerHeight) + bottom
	return preserveStatsBackground(page.Render(content), m.theme.Background)
}

func statsPeriodsForView(viewIndex int) []string {
	if viewIndex == 2 {
		return cleanupPeriods
	}
	if viewIndex == 1 {
		return activityPeriods
	}
	return statsPeriods
}

func defaultPeriodForStatsView(viewIndex int) int {
	if viewIndex == 1 {
		return 2
	}
	return 0
}

func preserveStatsBackground(rendered, color string) string {
	const reset = "\x1b[0m"
	styledMarker := lipgloss.NewStyle().Background(lipgloss.Color(color)).Render("x")
	markerIndex := strings.IndexByte(styledMarker, 'x')
	if markerIndex <= 0 || !strings.Contains(rendered, reset) {
		return rendered
	}
	backgroundSequence := styledMarker[:markerIndex]
	return strings.ReplaceAll(rendered, reset, reset+backgroundSequence) + reset
}

func (m *model) openStatsView() {
	m.statsOpen = true
	m.statsPeriod = 0
	m.statsViewIndex = 0
	m.statsSelected = 0
	m.refreshStatsData()
}

func (m *model) refreshStatsData() {
	m.statsNow = time.Now()
	m.statsErr = ""
	data, err := loadStatsData()
	if err != nil {
		m.statsData = statsData{}
		m.statsErr = err.Error()
		return
	}
	m.statsData = data
}

func (m model) updateStatsView(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if matchesShortcut(message, m.shortcutProfile, shortcutRefresh) {
		m.refreshStatsData()
		return m, nil
	}
	switch message.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		m.statsOpen = false
		return m, nil
	case tea.KeyLeft:
		m.statsPeriod = (m.statsPeriod + len(statsPeriods) - 1) % len(statsPeriods)
		m.statsSelected = 0
	case tea.KeyRight, tea.KeyTab:
		if message.Type == tea.KeyTab {
			m.statsViewIndex = (m.statsViewIndex + 1) % len(statsViews)
			m.statsPeriod = defaultPeriodForStatsView(m.statsViewIndex)
		} else {
			m.statsPeriod = (m.statsPeriod + 1) % len(statsPeriods)
			m.statsSelected = 0
		}
	case tea.KeyUp:
		m.statsSelected = max(0, m.statsSelected-1)
	case tea.KeyDown:
		rows, _ := rankedStatsRows(m.statsData, statsPeriods[m.statsPeriod], m.statsNow)
		m.statsSelected = min(max(0, len(rows)-1), m.statsSelected+1)
	case tea.KeyRunes:
		if message.Paste || !acceptsTextInput(message) {
			return m, nil
		}
		if len(message.Runes) != 1 {
			return m, nil
		}
		switch message.Runes[0] {
		case 'q':
			m.statsOpen = false
		case 'h':
			m.statsPeriod = (m.statsPeriod + len(statsPeriods) - 1) % len(statsPeriods)
			m.statsSelected = 0
		case 'l':
			m.statsPeriod = (m.statsPeriod + 1) % len(statsPeriods)
			m.statsSelected = 0
		case 'k':
			m.statsSelected = max(0, m.statsSelected-1)
		case 'j':
			rows, _ := rankedStatsRows(m.statsData, statsPeriods[m.statsPeriod], m.statsNow)
			m.statsSelected = min(max(0, len(rows)-1), m.statsSelected+1)
		case '1', '2', '3', '4':
			m.statsPeriod = int(message.Runes[0] - '1')
			m.statsSelected = 0
		case 'r':
			m.refreshStatsData()
		case 'o':
			m.statsViewIndex = 0
			m.statsPeriod = 0
		case 'a':
			m.statsViewIndex = 1
			m.statsPeriod = 2
		case 'c':
			m.statsViewIndex = 2
			m.statsPeriod = 0
		case 'g':
			m.statsViewIndex = 3
			m.statsPeriod = 0
		}
	}
	return m, nil
}

func (m model) statsView(header string) string {
	return (statsModel{
		data:            m.statsData,
		periodIndex:     m.statsPeriod,
		viewIndex:       m.statsViewIndex,
		selected:        m.statsSelected,
		width:           m.width,
		height:          m.height,
		theme:           m.theme,
		now:             m.statsNow,
		appHeader:       header,
		closeHint:       primaryShortcutLabel(m.shortcutProfile, shortcutStats) + "/esc return",
		errorText:       m.statsErr,
		shortcutProfile: m.shortcutProfile,
	}).View()
}
