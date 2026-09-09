package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type GitHubRepo struct {
	FullName    string `json:"full_name"`
	Name        string `json:"name"`
	Private     bool   `json:"private"`
	Archived    bool   `json:"archived"`
	Description string `json:"description"`
	Permissions struct {
		Push bool `json:"push"`
	} `json:"permissions"`
}

type repoPickerModel struct {
	repos  []GitHubRepo
	query  string
	cursor int
	width  int
	height int
	result string
	busy   bool
}

type repoConfiguredMsg struct {
	message string
	err     error
}

func runRepoPicker() error {
	repos, err := listWritableGitHubRepositories()
	if err != nil {
		return err
	}
	theme, _ := loadTheme()
	applyTheme(theme)
	program := tea.NewProgram(repoPickerModel{repos: repos, width: 80, height: 24}, tea.WithAltScreen())
	finished, err := program.Run()
	if err != nil {
		return err
	}
	if selected, ok := finished.(repoPickerModel); ok && selected.result != "" {
		fmt.Println(selected.result)
	}
	return nil
}

func listWritableGitHubRepositories() ([]GitHubRepo, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, fmt.Errorf("GitHub CLI is required; install gh and run gh auth login")
	}
	if output, err := exec.Command("gh", "auth", "status").CombinedOutput(); err != nil {
		return nil, fmt.Errorf("GitHub login required: run gh auth login\n%s", strings.TrimSpace(string(output)))
	}
	endpoint := "user/repos?affiliation=owner,collaborator,organization_member&per_page=100&sort=pushed"
	output, err := exec.Command("gh", "api", endpoint, "--paginate", "--slurp").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("list GitHub repositories: %s", strings.TrimSpace(string(output)))
	}
	var pages [][]GitHubRepo
	if err := json.Unmarshal(output, &pages); err != nil {
		return nil, fmt.Errorf("read GitHub response: %w", err)
	}
	var repos []GitHubRepo
	for _, page := range pages {
		for _, repo := range page {
			if repo.Permissions.Push && !repo.Archived {
				repos = append(repos, repo)
			}
		}
	}
	sort.SliceStable(repos, func(i, j int) bool { return repos[i].FullName < repos[j].FullName })
	return repos, nil
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
		filtered := filterGitHubRepos(m.repos, m.query)
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
				return m, cloneAndConfigureCmd(filtered[m.cursor])
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
	filtered := filterGitHubRepos(m.repos, m.query)
	if m.cursor >= len(filtered) {
		m.cursor = max(0, len(filtered)-1)
	}
	header := brandStyle.Render("ALIAS LENS") + "  " + titleStyle.Render("Choose a GitHub repository")
	subtitle := dimStyle.Render(fmt.Sprintf("%d repositories where you have push access", len(m.repos)))
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
		line := style.Render(marker+repo.FullName) + "  " + dimStyle.Render(visibility)
		list.WriteString(line)
		if index < end-1 {
			list.WriteByte('\n')
		}
	}
	if len(filtered) == 0 {
		list.WriteString(dimStyle.Render("No writable repositories match this search."))
	}
	footer := dimStyle.Render("type to filter  ·  ↑↓ move  ·  enter clone/select  ·  esc cancel")
	if m.busy {
		footer = statusStyle.Render("Cloning and configuring repository…")
	}
	page := lipgloss.JoinVertical(lipgloss.Left, header, subtitle, "", search, "", list.String(), "", footer)
	return lipgloss.NewStyle().Width(width).Height(max(18, m.height)).Padding(1, 3).Render(page)
}

func filterGitHubRepos(repos []GitHubRepo, query string) []GitHubRepo {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return repos
	}
	var filtered []GitHubRepo
	for _, repo := range repos {
		if strings.Contains(strings.ToLower(repo.FullName), needle) || strings.Contains(strings.ToLower(repo.Description), needle) {
			filtered = append(filtered, repo)
		}
	}
	return filtered
}

func cloneAndConfigureCmd(repo GitHubRepo) tea.Cmd {
	return func() tea.Msg {
		home, err := os.UserHomeDir()
		if err != nil {
			return repoConfiguredMsg{err: err}
		}
		root := filepath.Join(home, ".local", "share", "alias-lens", "repos")
		if err := os.MkdirAll(root, 0o755); err != nil {
			return repoConfiguredMsg{err: err}
		}
		destination := filepath.Join(root, strings.ReplaceAll(repo.FullName, "/", "--"))
		if _, err := os.Stat(filepath.Join(destination, ".git")); os.IsNotExist(err) {
			output, cloneErr := exec.Command("gh", "repo", "clone", repo.FullName, destination, "--", "--depth=1").CombinedOutput()
			if cloneErr != nil {
				return repoConfiguredMsg{err: fmt.Errorf("clone %s: %s", repo.FullName, strings.TrimSpace(string(output)))}
			}
		}
		if err := configureRepository(destination); err != nil {
			return repoConfiguredMsg{err: err}
		}
		return repoConfiguredMsg{message: "Configured " + repo.FullName + " at " + destination}
	}
}
