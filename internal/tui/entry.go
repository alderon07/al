package tui

import (
	"context"
	"fmt"
	"github.com/alderon07/al/internal/app"
	"github.com/alderon07/al/internal/presentation"
	"github.com/alderon07/al/internal/providers"
	tea "github.com/alderon07/al/internal/tea"
	"time"
)

type PickerOptions struct {
	Query            string
	CommandOnly      bool
	ExecuteSelection bool
}
type RepositoryOptions struct {
	Config       app.AppConfig
	Provider     providers.RepoProvider
	Repositories []providers.RemoteRepo
	Warnings     []string
}

func Browser(services *app.Services, repositoryDiff bool) error {
	return runTUIWithDiff(services, repositoryDiff)
}
func Picker(services *app.Services, options PickerOptions) error {
	return runAliasPicker(services, options.Query, options.CommandOnly, options.ExecuteSelection)
}
func Stats(services *app.Services, data app.StatsData, period string, now time.Time) error {
	return runStatsTUI(services, data, period, now)
}
func RepositoryPicker(ctx context.Context, services *app.Services, options RepositoryOptions) error {
	theme, _ := services.LoadTheme()
	applyTheme(theme)
	applyFooterConfig(options.Config.Footer)
	applyAppearanceConfig(options.Config.Appearance)
	program := tea.NewProgram(repoPickerModel{services: services, repos: options.Repositories, config: options.Config, provider: options.Provider, warnings: options.Warnings, width: 80, height: 24, ctx: ctx}, tea.WithAltScreen())
	finished, err := program.Run()
	if err != nil {
		return err
	}
	if selected, ok := finished.(repoPickerModel); ok && selected.result != "" {
		fmt.Println(presentation.TerminalSafeText(selected.result))
	}
	return nil
}
