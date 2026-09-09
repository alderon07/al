package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type repoPickerModel struct {
	repos    []RemoteRepo
	config   AppConfig
	warnings []string
	query    string
	cursor   int
	width    int
	height   int
	result   string
	busy     bool
}

type repoConfiguredMsg struct {
	message string
	err     error
}

func runRepoPicker(only string) error {
	config, err := loadConfig()
	if err != nil {
		return err
	}
	repos, warnings := listRemoteRepositories(context.Background(), config, only)
	if len(repos) == 0 {
		if len(warnings) > 0 {
			return fmt.Errorf("no repositories available\n%s", strings.Join(warnings, "\n"))
		}
		return fmt.Errorf("no writable repositories found")
	}
	theme, _ := loadTheme()
	applyTheme(theme)
	program := tea.NewProgram(repoPickerModel{repos: repos, config: config, warnings: warnings, width: 80, height: 24}, tea.WithAltScreen())
	finished, err := program.Run()
	if err != nil {
		return err
	}
	if selected, ok := finished.(repoPickerModel); ok && selected.result != "" {
		fmt.Println(selected.result)
	}
	return nil
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
				return m, cloneAndConfigureCmd(m.config, filtered[m.cursor])
			}
		case tea.KeyRunes:
			m.query += string(message.Runes)
			m.cursor = 0
		}
	}
	return m, nil
}

func (m repoPickerModel) View() string {
	width := max(48, m.width)
	contentWidth := max(40, min(width-8, 100))
	filtered := filterRemoteRepos(m.repos, m.query)
	if m.cursor >= len(filtered) {
		m.cursor = max(0, len(filtered)-1)
	}
	header := brandStyle.Render("ALIAS LENS") + "  " + titleStyle.Render("Choose a remote repository")
	subtitle := dimStyle.Render(fmt.Sprintf("%d writable repositories across configured providers", len(m.repos)))
	search := lipgloss.NewStyle().Width(contentWidth-3).Padding(0, 1).Border(lipgloss.RoundedBorder()).BorderForeground(acidColor).Render(acidStyle("⌕") + " " + searchText(m.query))

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
		providerBadge := lipgloss.NewStyle().Bold(true).Foreground(pageColor).Background(colorForCategory(repo.Provider)).Padding(0, 1).Render(strings.ToUpper(repo.ProviderTag))
		line := style.Render(marker+repo.FullName) + "  " + providerBadge + "  " + dimStyle.Render(visibility)
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
		footer += "\n" + dimStyle.Render("Unavailable: "+strings.Join(m.warnings, " · "))
	}
	if m.busy {
		footer = statusStyle.Render("Cloning and configuring repository…")
	}
	page := lipgloss.JoinVertical(lipgloss.Left, header, subtitle, "", search, "", list.String(), "", footer)
	return lipgloss.NewStyle().Width(width).Height(max(18, m.height)).Padding(1, 3).Render(page)
}

func filterRemoteRepos(repos []RemoteRepo, query string) []RemoteRepo {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return repos
	}
	var filtered []RemoteRepo
	for _, repo := range repos {
		if strings.Contains(strings.ToLower(repo.FullName), needle) || strings.Contains(strings.ToLower(repo.Description), needle) || strings.Contains(repo.Provider, needle) {
			filtered = append(filtered, repo)
		}
	}
	return filtered
}

func cloneAndConfigureCmd(config AppConfig, repo RemoteRepo) tea.Cmd {
	return func() tea.Msg {
		home, err := os.UserHomeDir()
		if err != nil {
			return repoConfiguredMsg{err: err}
		}
		root := filepath.Join(home, ".local", "share", "alias-lens", "repos")
		if err := os.MkdirAll(root, 0o755); err != nil {
			return repoConfiguredMsg{err: err}
		}
		destination := filepath.Join(root, repo.Provider, strings.ReplaceAll(repo.FullName, "/", "--"))
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return repoConfiguredMsg{err: err}
		}
		if _, err := os.Stat(filepath.Join(destination, ".git")); os.IsNotExist(err) {
			provider := providerByID(config, repo.Provider)
			if provider == nil {
				return repoConfiguredMsg{err: fmt.Errorf("provider %s is no longer configured", repo.Provider)}
			}
			if cloneErr := provider.Clone(context.Background(), repo, destination); cloneErr != nil {
				return repoConfiguredMsg{err: cloneErr}
			}
		}
		if err := configureRepository(destination); err != nil {
			return repoConfiguredMsg{err: err}
		}
		return repoConfiguredMsg{message: "Configured " + repo.FullName + " at " + destination}
	}
}
