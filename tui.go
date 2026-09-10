package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	aliases     []Alias
	query       string
	cursor      int
	width       int
	height      int
	status      string
	theme       Theme
	adding      bool
	field       int
	form        [3]string
	editingName string
	deleteName  string
	healthOnly  bool
	trackedOnly bool
	tracked     []trackedFileItem
	trackedRepo string
	trackedErr  string
	selectMode  bool
	executeMode bool
	selected    *Alias
}

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
	theme, themeErr := loadTheme()
	status := ""
	if themeErr != nil {
		status = themeErr.Error()
	}

	options := []tea.ProgramOption{tea.WithAltScreen()}
	if terminal, openErr := os.OpenFile("/dev/tty", os.O_RDWR, 0); openErr == nil {
		defer terminal.Close()
		lipgloss.SetDefaultRenderer(lipgloss.NewRenderer(terminal))
		options = append(options, tea.WithInput(terminal), tea.WithOutput(terminal))
	}
	applyTheme(theme)
	finished, err := tea.NewProgram(model{aliases: aliases, width: 80, height: 24, theme: theme, status: status, executeMode: true}, options...).Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Alias Lens could not start:", err)
		return
	}
	selected, ok := finished.(model)
	if ok && selected.selected != nil {
		fmt.Println(selected.selected.Name)
	}
}

func runAliasPicker(query string, commandOnly bool) error {
	aliases, err := loadAliases()
	if err != nil {
		return err
	}
	theme, _ := loadTheme()
	options := []tea.ProgramOption{tea.WithAltScreen()}
	if terminal, openErr := os.OpenFile("/dev/tty", os.O_RDWR, 0); openErr == nil {
		defer terminal.Close()
		lipgloss.SetDefaultRenderer(lipgloss.NewRenderer(terminal))
		options = append(options, tea.WithInput(terminal), tea.WithOutput(terminal))
	}
	applyTheme(theme)
	initial := model{aliases: aliases, query: query, width: 80, height: 24, theme: theme, selectMode: true}
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
	} else {
		fmt.Println(selected.selected.Name)
	}
	return nil
}

func (model) Init() tea.Cmd { return nil }

func (m model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width = message.Width
		m.height = message.Height
		return m, nil
	case tea.KeyMsg:
		if m.adding {
			return m.updateAddForm(message)
		}
		if m.deleteName != "" {
			return m.updateDeleteConfirmation(message)
		}
		m.status = ""
		if message.Type == tea.KeyCtrlF && !m.selectMode {
			m.trackedOnly = !m.trackedOnly
			m.cursor = 0
			if m.trackedOnly {
				m = m.refreshTrackedFiles()
			}
			return m, nil
		}
		if m.trackedOnly {
			return m.updateTrackedFiles(message)
		}
		matches := m.currentAliases()
		switch message.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyCtrlR:
			aliases, err := loadAliases()
			if err != nil {
				m.status = "Reload failed: " + err.Error()
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
		case tea.KeyCtrlT:
			m.theme = nextTheme(m.theme.Preset)
			applyTheme(m.theme)
			if err := saveTheme(m.theme); err != nil {
				m.status = "Theme changed but could not be saved: " + err.Error()
			} else {
				m.status = "Theme: " + m.theme.Name
			}
		case tea.KeyCtrlA:
			m.adding = true
			m.field = 0
			m.form = [3]string{}
			m.editingName = ""
		case tea.KeyCtrlE:
			if len(matches) > 0 {
				selected := matches[m.cursor]
				m.adding = true
				m.editingName = selected.Name
				m.field = 0
				m.form = [3]string{selected.Name, selected.Command, selected.Description}
			}
		case tea.KeyCtrlD:
			if len(matches) > 0 {
				m.deleteName = matches[m.cursor].Name
			}
		case tea.KeyCtrlH:
			m.healthOnly = !m.healthOnly
			m.query = ""
			m.cursor = 0
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
		case tea.KeyEnter:
			if len(matches) > 0 {
				selected := matches[m.cursor]
				m.selected = &selected
				return m, tea.Quit
			}
		case tea.KeyRunes:
			m.healthOnly = false
			m.query += string(message.Runes)
			m.cursor = 0
		}
	}
	return m, nil
}

func (m model) View() string {
	width := max(48, m.width)
	height := max(18, m.height)
	contentWidth := max(40, min(width-8, 108))
	matches := m.currentAliases()
	cursor := min(m.cursor, max(0, len(matches)-1))

	headerDetails := fmt.Sprintf("  •  %d aliases  •  %s", len(m.aliases), syncStatusLabel())
	if contentWidth >= 84 {
		headerDetails = fmt.Sprintf("  •  %d loaded  •  %d issues  •  %s  •  %s", len(m.aliases), healthIssueCount(m.aliases), m.theme.Name, syncStatusLabel())
	}
	header := brandStyle.Render("ALIAS LENS") + "  " + lipgloss.NewStyle().Foreground(cyanColor).Render(aliasDisplayPath()) + dimStyle.Render(headerDetails)
	title := titleStyle.Render("Find the shortcut before you forget it.") + "\n" + dimStyle.Render("Search, inspect, and rediscover the commands you already own.")
	if m.selectMode {
		title = titleStyle.Render("Choose an alias to use in your shell.") + "\n" + dimStyle.Render("Enter selects it. Esc returns without changing the prompt.")
	} else if m.executeMode {
		title = titleStyle.Render("Choose an alias to run.") + "\n" + dimStyle.Render("Enter executes it. Esc exits without running anything.")
	}
	if m.adding {
		return m.addFormView(width, height, contentWidth, header)
	}
	if m.trackedOnly {
		return m.trackedFilesView(width, height, contentWidth, header)
	}
	search := lipgloss.NewStyle().
		Width(contentWidth-3).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(acidColor).
		Render(acidStyle("$") + " " + searchText(m.query))

	var body strings.Builder
	if m.healthOnly {
		body.WriteString(lipgloss.NewStyle().Bold(true).Foreground(coralColor).Render("⚠ ALIAS HEALTH"))
		body.WriteByte('\n')
	} else if strings.TrimSpace(m.query) == "" {
		body.WriteString(lipgloss.NewStyle().Bold(true).Foreground(amberColor).Render("✦ SUGGESTED FOR YOU"))
		body.WriteByte('\n')
	}
	if m.healthOnly && len(matches) == 0 {
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

	footer := dimStyle.Render("↑↓ move  ·  enter ") + cyanStyle("select") + dimStyle.Render("  ·  ^f ") + cyanStyle("files") + dimStyle.Render("  ·  ^h ") + cyanStyle("health") + dimStyle.Render("  ·  esc quit")
	if contentWidth < 60 {
		footer = dimStyle.Render("↑↓  ·  enter ") + cyanStyle("select") + dimStyle.Render("  ·  ^f ") + cyanStyle("files") + dimStyle.Render("  ·  esc quit")
	}
	if m.selectMode {
		footer = dimStyle.Render("type to search  ·  ↑↓ move  ·  enter select  ·  esc cancel")
	} else if m.executeMode {
		footer = dimStyle.Render("↑↓ move  ·  enter ") + cyanStyle("execute") + dimStyle.Render("  ·  ^f files  ·  ^h health  ·  esc quit")
	}
	if m.status != "" {
		footer = statusStyle.Render(truncate(m.status, contentWidth))
	}
	if m.deleteName != "" {
		footer = lipgloss.NewStyle().Bold(true).Foreground(coralColor).Render("Delete " + m.deleteName + "?  y confirm  ·  n cancel")
	}

	page := lipgloss.JoinVertical(lipgloss.Left, header, "", title, "", search, "", body.String(), "", footer)
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Padding(1, 3).
		Render(page)
}

func (m model) refreshTrackedFiles() model {
	config, err := loadConfig()
	if err != nil {
		m.tracked = nil
		m.trackedRepo = ""
		m.trackedErr = err.Error()
		return m
	}
	m.trackedRepo = config.Repository
	m.trackedErr = ""
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
	switch message.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		m.trackedOnly = false
		m.cursor = 0
	case tea.KeyCtrlR:
		m = m.refreshTrackedFiles()
		m.status = fmt.Sprintf("Reloaded %d tracked files", len(m.tracked))
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
	body.WriteString(lipgloss.NewStyle().Bold(true).Foreground(amberColor).Render("TRACKED CONFIG FILES"))
	body.WriteString(dimStyle.Render(fmt.Sprintf("  %d enrolled", len(m.tracked))))
	body.WriteByte('\n')
	if m.trackedRepo == "" {
		body.WriteString(dimStyle.Render("Repository: not configured"))
	} else {
		body.WriteString(dimStyle.Render("Repository: ") + cyanStyle(compactHomePath(m.trackedRepo)))
	}
	body.WriteString("\n\n")

	if m.trackedErr != "" {
		body.WriteString(lipgloss.NewStyle().Foreground(coralColor).Render(wrapText("Could not load tracked files: "+m.trackedErr, contentWidth)))
	} else if len(m.tracked) == 0 {
		body.WriteString(titleStyle.Render("No extra config files are tracked."))
		body.WriteString("\n" + dimStyle.Render("Add one with ") + cyanStyle("al track PATH") + dimStyle.Render(". "+aliasDisplayPath()+" remains the primary file."))
	} else {
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

	footer := dimStyle.Render("↑↓ move  ·  ctrl+r refresh  ·  ctrl+f or esc aliases  ·  ctrl+c quit")
	if m.status != "" {
		footer = statusStyle.Render(truncate(m.status, contentWidth))
	}
	page := lipgloss.JoinVertical(lipgloss.Left, header, "", titleStyle.Render("See what Alias Lens keeps in sync."), "", body.String(), "", footer)
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 3).Render(page)
}

func (m model) trackedVisibleCount() int { return max(1, (m.height-12)/5) }

func renderTrackedFile(item trackedFileItem, active bool, width int) string {
	cardWidth := max(34, width-3)
	marker := "  "
	if active {
		marker = "▶ "
	}
	status := defaultString(item.State.Status, "waiting")
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
	lineTwo := cyanStyle("↳ repo/") + lipgloss.NewStyle().Foreground(inkColor).Render(truncate(filepath.ToSlash(item.Config.RepositoryPath), cardWidth-10))
	detail := item.State.Message
	if item.Error != "" {
		detail = item.Error
	}
	if detail == "" {
		detail = "Waiting for the first sync cycle"
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
	case tea.KeyCtrlS:
		return m.saveAliasForm()
	case tea.KeyBackspace, tea.KeyDelete:
		if m.form[m.field] != "" {
			_, size := utf8.DecodeLastRuneInString(m.form[m.field])
			m.form[m.field] = m.form[m.field][:len(m.form[m.field])-size]
		}
	case tea.KeyRunes:
		m.form[m.field] += string(message.Runes)
	}
	return m, nil
}

func (m model) saveAliasForm() (tea.Model, tea.Cmd) {
	var saveErr error
	if m.editingName == "" {
		saveErr = addAlias(m.form[0], m.form[1], m.form[2])
	} else {
		saveErr = editAlias(m.editingName, m.form[0], m.form[1], m.form[2])
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
	labels := []string{"ALIAS NAME", "COMMAND", "WHAT IT DOES"}
	hints := []string{"ex: gpf", "ex: git push --force-with-lease", "ex: Safely force-push the current branch"}
	var form strings.Builder
	heading := "＋ ADD AN ALIAS"
	message := "It will be placed beside related commands in " + aliasDisplayPath() + "."
	if m.editingName != "" {
		heading = "✎ EDIT " + m.editingName
		message = "Saving will move it beside related commands if needed."
	}
	form.WriteString(lipgloss.NewStyle().Bold(true).Foreground(amberColor).Render(heading))
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
		field := lipgloss.NewStyle().Width(contentWidth-5).Padding(0, 1).Border(lipgloss.RoundedBorder()).BorderForeground(border).Render(value + cursor)
		form.WriteString("\n\n" + lipgloss.NewStyle().Bold(true).Foreground(colorForField(index)).Render(labels[index]) + "\n" + field)
	}
	footer := dimStyle.Render("tab/enter next  ·  ctrl+s save  ·  esc cancel")
	if m.status != "" {
		footer = lipgloss.NewStyle().Foreground(coralColor).Render(m.status)
	}
	page := lipgloss.JoinVertical(lipgloss.Left, header, "", form.String(), "", footer)
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
		lineOne += "  " + lipgloss.NewStyle().Foreground(violetColor).Render("FUNCTION")
	}
	if alias.Favorite {
		lineOne += "  " + lipgloss.NewStyle().Foreground(amberColor).Render("★")
	}
	if len(alias.Tags) > 0 {
		lineOne += "  " + dimStyle.Render("#"+strings.Join(alias.Tags, " #"))
	}
	description := lipgloss.NewStyle().Foreground(inkColor).Render(wrapText(alias.Description, cardWidth-6))
	lineTwo := lipgloss.NewStyle().Foreground(cyanColor).Render("↳ " + truncate(alias.Command, cardWidth-6))
	if len(alias.Issues) > 0 {
		lineTwo += "\n" + lipgloss.NewStyle().Foreground(coralColor).Render("⚠ "+strings.Join(alias.Issues, " · "))
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
	if query == "" {
		return dimStyle.Render("search aliases…") + acidStyle("█")
	}
	return lipgloss.NewStyle().Foreground(inkColor).Render(query) + acidStyle("█")
}

func truncate(value string, width int) string {
	if width <= 1 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	return string(runes[:width-1]) + "…"
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
	line := words[0]
	for _, word := range words[1:] {
		if utf8.RuneCountInString(line)+1+utf8.RuneCountInString(word) <= width {
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
	runeCount := utf8.RuneCountInString(value)
	if runeCount >= width {
		return truncate(value, width)
	}
	return value + strings.Repeat(" ", width-runeCount)
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
