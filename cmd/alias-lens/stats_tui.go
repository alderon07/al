package main

import (
	"fmt"
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
	if width > 96 {
		width = 96
	}
	// Leave a small right gutter. Some terminals render box-drawing runes wider
	// than their reported cell width, so an exact-width meter can wrap its count.
	inner := width - 12
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Accent)).Bold(true)
	text := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Text))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Muted))
	panel := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Text)).Background(lipgloss.Color(m.theme.Panel)).Padding(1, 2).Width(inner)
	page := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Text)).Background(lipgloss.Color(m.theme.Background)).Padding(1, 2).Width(width - 8)

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
	if m.errorText != "" {
		body.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Git)).Bold(true).Render("Stats could not load."))
		body.WriteString("\n")
		body.WriteString(muted.Render(truncate(m.errorText, inner-4)))
	} else if len(rows) == 0 {
		body.WriteString(accent.Render("No uses in this window yet."))
		body.WriteString("\n")
		body.WriteString(muted.Render("Run an alias, then refresh after your shell writes its history."))
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
		barWidth := inner - nameWidth - 17
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
		body.WriteString(text.Render(truncate(selected.Command, inner-12)))
		if len(selected.Tags) > 0 {
			body.WriteString("\n")
			body.WriteString(muted.Render("tags        "))
			body.WriteString(accent.Render(strings.Join(selected.Tags, "  ")))
		}
	}

	closeHint := m.closeHint
	if closeHint == "" {
		closeHint = "q close"
	}
	foot := muted.Render("←/→ period   ↑/↓ inspect   r refresh   " + closeHint)
	note := muted.Render("Counts come only from the active terminal history")
	content := header + "\n\n" + strings.Join(tabs, " ") + "\n\n" + panel.Render(strings.TrimRight(body.String(), "\n")) + "\n\n" + note + "\n" + foot
	if m.appHeader != "" {
		content = m.appHeader + "\n\n" + content
	}
	return page.Render(content)
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
	case tea.KeyCtrlS:
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
		closeHint:   "esc return",
		errorText:   m.statsErr,
	}).View()
}
