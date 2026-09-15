package main

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var statsPeriods = []string{"all", "today", "week", "year"}

type statsModel struct {
	data        statsData
	periodIndex int
	selected    int
	width       int
	height      int
	theme       Theme
	now         time.Time
	appHeader   string
	closeHint   string
	errorText   string
}

func runStatsTUI(data statsData, period string, now time.Time) error {
	theme, _ := loadTheme()
	applyTheme(theme)
	index := 0
	for candidate, name := range statsPeriods {
		if name == period {
			index = candidate
		}
	}
	options := []tea.ProgramOption{tea.WithAltScreen(), tea.WithReportFocus()}
	if terminal, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		defer terminal.Close()
		lipgloss.SetDefaultRenderer(lipgloss.NewRenderer(terminal))
		options = append(options, tea.WithInput(terminal), tea.WithOutput(terminal))
	}
	_, err := tea.NewProgram(statsModel{data: data, periodIndex: index, width: 80, height: 24, theme: theme, now: now}, options...).Run()
	return err
}

func (m statsModel) Init() tea.Cmd { return nil }

func (m statsModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = message.Width, message.Height
	case tea.KeyMsg:
		switch message.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "left", "h":
			m.periodIndex = (m.periodIndex + len(statsPeriods) - 1) % len(statsPeriods)
			m.selected = 0
		case "right", "l", "tab":
			m.periodIndex = (m.periodIndex + 1) % len(statsPeriods)
			m.selected = 0
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
	period := statsPeriods[m.periodIndex]
	rows, _ := rankedStatsRows(m.data, period, m.now)
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
	panel := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Text)).Background(lipgloss.Color(m.theme.Panel)).Padding(1, 2).Width(inner)
	page := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Text)).Background(lipgloss.Color(m.theme.Background)).Padding(1, 2).Width(width).Height(height)

	total := 0
	for _, row := range rows {
		total += row.Count
	}
	header := accent.Render("◒ Alias rhythm") + muted.Render(fmt.Sprintf("  %d uses · %d aliases", total, len(rows)))
	var tabs []string
	for index, name := range statsPeriods {
		label := fmt.Sprintf("%d %s", index+1, name)
		if index == m.periodIndex {
			tabs = append(tabs, lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Background)).Background(lipgloss.Color(m.theme.Accent)).Bold(true).Padding(0, 1).Render(label))
		} else {
			tabs = append(tabs, muted.Padding(0, 1).Render(label))
		}
	}

	var body strings.Builder
	bodyWidth := inner
	showCoverage := m.errorText == "" && len(m.data.Aliases) > 0
	wideCoverage := showCoverage && inner >= 76
	if wideCoverage {
		bodyWidth -= 30
	}
	if m.errorText != "" {
		body.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Git)).Bold(true).Render("Stats could not load."))
		body.WriteString("\n")
		body.WriteString(muted.Render(truncate(m.errorText, bodyWidth-4)))
	} else if len(rows) == 0 {
		if period != "all" && hasUntimestampedUsage(m.data.Events) {
			body.WriteString(accent.Render("Your alias history has no dates yet."))
			body.WriteString("\n")
			body.WriteString(muted.Render("Start a new shell. Alias Lens will date future commands without changing how history looks."))
		} else {
			body.WriteString(accent.Render("No uses in this window yet."))
			body.WriteString("\n")
			body.WriteString(muted.Render("No matching aliases were found in terminal history."))
		}
	} else {
		visible := len(rows)
		if limit := m.height - 14; limit > 0 && visible > limit {
			visible = limit
		}
		if m.selected >= visible {
			m.selected = visible - 1
		}
		maxCount := rows[0].Count
		nameWidth := 16
		barWidth := bodyWidth - nameWidth - 17
		if barWidth < 8 {
			barWidth = 8
		}
		for index, row := range rows[:visible] {
			filled := row.Count * barWidth / maxCount
			if filled < 1 {
				filled = 1
			}
			marker := "  "
			nameStyle := text
			barColor := m.theme.Secondary
			if index == m.selected {
				marker = accent.Render("› ")
				nameStyle = accent
				barColor = m.theme.Accent
			}
			bar := lipgloss.NewStyle().Foreground(lipgloss.Color(barColor)).Render(strings.Repeat("━", filled)) + muted.Render(strings.Repeat("─", barWidth-filled))
			body.WriteString(fmt.Sprintf("%s%2d  %s %s %4d", marker, index+1, nameStyle.Render(fmt.Sprintf("%-*s", nameWidth, truncate(row.Alias.Name, nameWidth))), bar, row.Count))
			body.WriteString("\n")
		}
		selected := rows[m.selected].Alias
		body.WriteString("\n")
		body.WriteString(muted.Render("expands to  "))
		body.WriteString(text.Render(truncate(selected.Command, bodyWidth-12)))
		if len(selected.Tags) > 0 {
			body.WriteString("\n")
			body.WriteString(muted.Render("tags        "))
			body.WriteString(accent.Render(strings.Join(selected.Tags, "  ")))
		}
	}
	bodyContent := strings.TrimRight(body.String(), "\n")
	if showCoverage {
		coverage := renderCoveragePie(len(rows), len(m.data.Aliases), m.theme)
		if wideCoverage {
			bodyContent = lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().Width(bodyWidth).Render(bodyContent), strings.Repeat(" ", 3), coverage)
		} else {
			bodyContent = coverage + "\n\n" + bodyContent
		}
	}

	closeHint := m.closeHint
	if closeHint == "" {
		closeHint = "q close"
	}
	foot := muted.Render("←/→ period   ↑/↓ inspect   r refresh   " + closeHint)
	note := muted.Render("Counts come only from the active terminal history")
	content := header + "\n\n" + strings.Join(tabs, " ") + "\n\n" + panel.Render(bodyContent) + "\n\n" + note + "\n" + foot
	if m.appHeader != "" {
		content = m.appHeader + "\n\n" + content
	}
	return page.Render(content)
}

func renderCoveragePie(used, total int, theme Theme) string {
	const (
		columns = 10
		rows    = 5
		width   = columns * 2
	)
	share := 0.0
	if total > 0 {
		share = float64(used) / float64(total)
	}
	usedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Accent))
	unusedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Muted))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Text)).Bold(true).Width(width).Align(lipgloss.Center)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Muted))

	lines := []string{labelStyle.Render("Alias coverage")}
	for row := 0; row < rows; row++ {
		var line strings.Builder
		for column := 0; column < columns; column++ {
			x := (float64(column) + 0.5 - float64(columns)/2) / (float64(columns) / 2)
			y := (float64(row) + 0.5 - float64(rows)/2) / (float64(rows) / 2)
			if x*x+y*y > 1 {
				line.WriteString("  ")
				continue
			}
			angle := math.Atan2(x, -y)
			if angle < 0 {
				angle += 2 * math.Pi
			}
			if angle/(2*math.Pi) < share {
				line.WriteString(usedStyle.Render("██"))
			} else {
				line.WriteString(unusedStyle.Render("██"))
			}
		}
		lines = append(lines, line.String())
	}
	lines = append(lines,
		labelStyle.Render(fmt.Sprintf("%d of %d used", used, total)),
		usedStyle.Render("■")+muted.Render(" used  ")+unusedStyle.Render("■")+muted.Render(" unused"),
	)
	return strings.Join(lines, "\n")
}

func (m *model) openStatsView() {
	m.statsOpen = true
	m.statsPeriod = 0
	m.statsSelected = 0
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
	switch message.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		m.statsOpen = false
		return m, nil
	case tea.KeyF2, tea.KeyCtrlS:
		m.statsOpen = false
		return m, nil
	case tea.KeyCtrlR:
		m.openStatsView()
		return m, nil
	case tea.KeyLeft:
		m.statsPeriod = (m.statsPeriod + len(statsPeriods) - 1) % len(statsPeriods)
		m.statsSelected = 0
	case tea.KeyRight, tea.KeyTab:
		m.statsPeriod = (m.statsPeriod + 1) % len(statsPeriods)
		m.statsSelected = 0
	case tea.KeyUp:
		m.statsSelected = max(0, m.statsSelected-1)
	case tea.KeyDown:
		rows, _ := rankedStatsRows(m.statsData, statsPeriods[m.statsPeriod], m.statsNow)
		m.statsSelected = min(max(0, len(rows)-1), m.statsSelected+1)
	case tea.KeyRunes:
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
			m.openStatsView()
		}
	}
	return m, nil
}

func (m model) statsView(header string) string {
	return (statsModel{
		data:        m.statsData,
		periodIndex: m.statsPeriod,
		selected:    m.statsSelected,
		width:       m.width,
		height:      m.height,
		theme:       m.theme,
		now:         m.statsNow,
		appHeader:   header,
		closeHint:   "F2/esc return",
		errorText:   m.statsErr,
	}).View()
}
