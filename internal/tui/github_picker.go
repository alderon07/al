package tui

import "github.com/alderon07/al/internal/app"

import "github.com/alderon07/al/internal/presentation"

import (
	"context"
	"fmt"
	"github.com/alderon07/al/internal/providers"
	"strings"
	"unicode/utf8"

	tea "github.com/alderon07/al/internal/tea"
	"github.com/charmbracelet/lipgloss"
)

type repoPickerModel struct {
	services *app.Services

	repos    []providers.RemoteRepo
	config   appConfig
	provider providers.RepoProvider
	warnings []string
	query    string
	cursor   int
	width    int
	height   int
	result   string
	busy     bool
	ctx      context.Context
}

type repoConfiguredMsg struct {
	message string
	err     error
}

func (m repoPickerModel) Init() tea.Cmd { return nil }

func (m repoPickerModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = message.Width, message.Height
	case repoConfiguredMsg:
		m.busy = false
		if message.err != nil {
			m.result = "Alias Lens: " + message.err.Error()
		} else {
			m.result = message.message
		}
		return m, tea.Quit
	case tea.KeyMsg:
		if m.busy {
			return m, nil
		}
		filtered := filterRemoteRepos(m.repos, m.query)
		switch message.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyUp:
			m.cursor = max(0, m.cursor-1)
		case tea.KeyDown:
			m.cursor = min(max(0, len(filtered)-1), m.cursor+1)
		case tea.KeyBackspace, tea.KeyDelete:
			if m.query != "" {
				_, size := utf8.DecodeLastRuneInString(m.query)
				m.query = m.query[:len(m.query)-size]
				m.cursor = 0
			}
		case tea.KeyEnter:
			if len(filtered) > 0 {
				m.busy = true
				return m, cloneAndConfigureCmd(m.service(),

					m.ctx, m.config, m.provider, filtered[m.cursor])
			}
		case tea.KeyRunes:
			if !acceptsTextInput(message) {
				return m, nil
			}
			m.query = appendSearchQuery(m.query, string(message.Runes))
			m.cursor = 0
		}
	}
	return m, nil
}

func (m repoPickerModel) View() string {
	if message := smallTerminalMessage(m.width, m.height); message != "" {
		return message
	}
	width := max(48, m.width)
	frame := newMainTUIFrame(width, m.height)
	contentWidth := frame.contentWidth
	filtered := filterRemoteRepos(m.repos, m.query)
	if m.cursor >= len(filtered) {
		m.cursor = max(0, len(filtered)-1)
	}
	brand := ""
	if label := compactBrandLabel(); label != "" {
		brand = brandStyle.Render(label)
	}
	pickerTitle := titleStyle.Render("Choose a remote repository")
	if contentWidth >= 52 {
		pickerTitle = pixelIconLabel(iconRepository, "Choose a remote repository", titleStyle)
	}
	header := pickerTitle
	if brand != "" {
		header = brand + "  " + pickerTitle
	}
	subtitle := dimStyle.Render(fmt.Sprintf("%d writable repositories across configured providers", len(m.repos)))
	search := lipgloss.NewStyle().Width(searchFieldWidth(contentWidth)-2).Padding(0, 1).Border(lipgloss.RoundedBorder()).BorderForeground(acidColor).Render(acidStyle(markerPrefix(iconSearch)) + searchTextCursorAtWidth(m.query, true, searchFieldWidth(contentWidth)))

	var list strings.Builder
	visible := max(1, m.height-12)
	start := 0
	if m.cursor >= visible {
		start = m.cursor - visible + 1
	}
	end := min(len(filtered), start+visible)
	for index := start; index < end; index++ {
		repo := filtered[index]
		marker := "  "
		style := lipgloss.NewStyle().Foreground(inkColor)
		if index == m.cursor {
			marker = "▶ "
			style = style.Bold(true).Foreground(acidColor)
		}
		visibility := "public"
		if repo.Private {
			visibility = "private"
		}
		providerBadge := lipgloss.NewStyle().Bold(true).Foreground(pageColor).Background(colorForCategory(repo.Provider)).Padding(0, 1).Render(strings.ToUpper(presentation.TerminalSafeText(repo.ProviderTag)))
		line := style.Render(marker+presentation.TerminalSafeText(repo.FullName)) + "  " + providerBadge + "  " + dimStyle.Render(visibility)
		list.WriteString(line)
		if index < end-1 {
			list.WriteByte('\n')
		}
	}
	if len(filtered) == 0 {
		list.WriteString(dimStyle.Render("No writable repositories match this search."))
	}
	footer := dimStyle.Render("type to filter  ·  ↑↓ move  ·  enter clone/select  ·  esc cancel")
	if len(m.warnings) > 0 {
		footer += "\n" + dimStyle.Render("Unavailable: "+presentation.TerminalSafeText(strings.Join(m.warnings, " · ")))
	}
	if m.busy {
		footer = statusStyle.Render("Cloning and configuring repository…")
	}
	page := lipgloss.JoinVertical(lipgloss.Left, header, subtitle, "", search, "", list.String())
	return frame.renderWithFooter(page, footer)
}

func filterRemoteRepos(repos []providers.RemoteRepo, query string) []providers.RemoteRepo {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return repos
	}
	var filtered []providers.RemoteRepo
	for _, repo := range repos {
		if strings.Contains(strings.ToLower(repo.FullName), needle) || strings.Contains(strings.ToLower(repo.Description), needle) || strings.Contains(repo.Provider, needle) {
			filtered = append(filtered, repo)
		}
	}
	return filtered
}

func cloneAndConfigureCmd(services *app.Services, ctx context.Context, config appConfig, connected providers.RepoProvider, repo providers.RemoteRepo) tea.Cmd {
	return func() tea.Msg {
		result, err := services.CloneAndConfigureRepository(ctx, config, connected, repo)
		if err != nil {
			return repoConfiguredMsg{err: err}
		}
		return repoConfiguredMsg{message: "Configured " + result.Name + " at " + result.Directory}
	}
}
